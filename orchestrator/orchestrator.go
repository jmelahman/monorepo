// Package orchestrator is the embeddable API of local-preview: everything
// the standalone server does — mirror clones, content-addressed builds,
// on-demand supervised backends, lineage-forked state, Host-header preview
// routing — behind a small facade another Go application can wire into its
// own HTTP server.
//
// Typical embedding:
//
//	orch, err := orchestrator.New(orchestrator.Options{
//		DataDir: filepath.Join(dataDir, "previews"),
//		Addr:    ":8080", // the host application's listen address
//	})
//	...
//	srv.Handler = orch.WrapHost(appHandler) // previews by subdomain, app otherwise
//	...
//	orch.RegisterRepo(ctx, "myapp", "/path/to/repo")
//	orch.RequestDeploy(ctx, "myapp", "some-branch", false)
package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/jmelahman/local-preview/internal/build"
	"github.com/jmelahman/local-preview/internal/config"
	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/gitrepo"
	"github.com/jmelahman/local-preview/internal/proxy"
	"github.com/jmelahman/local-preview/internal/retain"
	"github.com/jmelahman/local-preview/internal/store"
	"github.com/jmelahman/local-preview/internal/supervise"
	"github.com/jmelahman/local-preview/internal/watch"
)

// ErrNotFound is returned when a repo or deploy does not exist.
var ErrNotFound = errors.New("not found")

// ErrConflict is returned when a name is already registered with a
// different source.
var ErrConflict = errors.New("conflict")

// RunSpec describes one build step handed to a Runner. ScratchDir is the
// extracted commit tree on the host; Dir is the working directory relative
// to it. Image is the manifest-declared build image for the step's side
// ("" = unspecified) — custom runners should honor it when set, since it's
// the repo's explicit contract.
type RunSpec struct {
	RepoName   string
	SHA        string
	ScratchDir string
	Dir        string
	Argv       []string
	Image      string
	// Devcontainer is the target repo's devcontainer resolved at the
	// deployed commit (zero when the commit carries none or the manifest
	// opts out) — the default environment for steps without an explicit
	// Image. Custom runners with their own devcontainer machinery can use
	// it directly.
	Devcontainer Devcontainer
}

// Devcontainer is the honored subset of a repo's devcontainer.json.
type Devcontainer struct {
	// Image is the container image builds and runs default into.
	Image string
	// Mounts are the devcontainer's named cache volumes.
	Mounts []DevcontainerMount
	// Home is the remote user's home directory — steps run with HOME set to
	// it so toolchain caches resolve onto the mounted volumes.
	Home string
}

// DevcontainerMount is one named-volume mount of a devcontainer.
type DevcontainerMount struct {
	Source string
	Target string
}

// Runner executes build steps. Omit it for the default behavior — the
// side's manifest image, else the repo's devcontainer, else the host —
// or inject an implementation to take over step execution entirely.
type Runner interface {
	Run(ctx context.Context, spec RunSpec, output io.Writer) error
}

// Options configures an Orchestrator.
type Options struct {
	// DataDir is the root for the orchestrator's database, mirror clones,
	// artifacts, state dirs, and logs. Required.
	DataDir string
	// Addr is the address the embedding server listens on, used only to
	// construct preview URLs (e.g. ":8080"). Optional.
	Addr string
	// PreviewDomain is the base domain previews are served under. Defaults
	// to "preview.localhost".
	PreviewDomain string
	// PreviewBaseURL is the public base URL of previews, e.g.
	// "https://preview.example.com". Set it when the embedding server sits
	// behind a TLS-terminating proxy: its scheme, host, and port replace the
	// http:// and the listen port that Addr and PreviewDomain imply, which
	// are only right when clients reach the server directly. It supersedes
	// PreviewDomain; setting both to different hosts is an error. Optional.
	PreviewBaseURL string
	// DBPath overrides the SQLite location; ":memory:" is allowed (deploy
	// history becomes ephemeral; artifacts still use DataDir).
	DBPath string
	// BuildConcurrency is the number of deploys built in parallel.
	// Defaults to 2.
	BuildConcurrency int
	// Runner executes build steps. Defaults to manifest image / repo
	// devcontainer / host execution, in that order of precedence per side.
	Runner Runner
	// MaxWarm caps concurrently running preview processes; beyond it the
	// least-recently-used are stopped. 0 defaults to 8; negative disables
	// the cap.
	MaxWarm int
	// ManifestSources are the locations tried, in order, for the preview
	// manifest at each deployed commit. Defaults to preview.toml at the
	// repo root. Embedders can add their own config file with a table —
	// e.g. {Path: ".kanban.toml", Table: "previews"}. Artifact hashes cover
	// the parsed manifest, not its location, so moving unchanged config
	// between sources doesn't invalidate caches.
	ManifestSources []ManifestSource
	// LocalManifestDir, when set, is a directory searched for out-of-repo
	// manifests (`<dir>/<repo>.toml`, plain preview.toml schema) when no
	// in-repo source matches — onboarding repos whose upstream can't carry a
	// manifest. The file is read from the server's disk at build time, not
	// from the deployed commit.
	LocalManifestDir string
	// PollInterval is how often watched repos (SetRepoWatch) are fetched
	// for new commits. 0 defaults to 1 minute; negative disables watching.
	PollInterval time.Duration
	// RetentionInterval is how often the retention sweep runs. 0 defaults
	// to 1 hour; negative disables the background sweep (CollectGarbage
	// still works). The sweep evicts nothing until a policy is set with
	// SetRetentionPolicy, but it always collects stale staging leftovers.
	RetentionInterval time.Duration
}

// ManifestSource locates a preview manifest: a TOML file at the target
// repo's root, optionally rooted at a top-level table of that file.
type ManifestSource struct {
	Path  string
	Table string
}

// Repo is a registered repository.
type Repo struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Source string `json:"source"`
	// Watch marks the repo for polling: new branch tips deploy
	// automatically. WatchBranches narrows which branches (comma-separated
	// globs, empty = all).
	Watch         bool   `json:"watch"`
	WatchBranches string `json:"watch_branches"`
	CreatedAt     string `json:"created_at"`
}

// Deploy is one commit's preview deployment.
type Deploy struct {
	ID           int64  `json:"id"`
	Repo         string `json:"repo"`
	SHA          string `json:"sha"`
	ShortSHA     string `json:"short_sha"`
	Ref          string `json:"ref,omitempty"`
	Branch       string `json:"branch,omitempty"`
	AuthorName   string `json:"author_name,omitempty"`
	AuthorEmail  string `json:"author_email,omitempty"`
	FeHash       string `json:"fe_hash,omitempty"`
	BeHash       string `json:"be_hash,omitempty"`
	Status       string `json:"status"`
	Error        string `json:"error,omitempty"`
	AttemptCount int64  `json:"attempt_count"`
	// PreviewURL is set once the deploy is ready.
	PreviewURL string `json:"preview_url,omitempty"`
	// Process is the live backend state when ready: running, starting, idle
	// (backends start on demand), or crashed — the last run or start attempt
	// ended badly, with ProcessError carrying the detail. FeProcess and
	// FeProcessError are the same for a process-mode frontend, absent for
	// static frontends.
	Process        string `json:"process,omitempty"`
	ProcessError   string `json:"process_error,omitempty"`
	FeProcess      string `json:"fe_process,omitempty"`
	FeProcessError string `json:"fe_process_error,omitempty"`
	// Artifacts are the deploy's named downloadable artifacts, present on
	// ready deploys whose manifest declares [artifacts.<name>] sections.
	Artifacts []DeployArtifact `json:"artifacts,omitempty"`
	CreatedAt string           `json:"created_at"`
	UpdatedAt string           `json:"updated_at"`
}

// DeployArtifact is one named downloadable artifact of a ready deploy.
type DeployArtifact struct {
	Name  string         `json:"name"`
	Hash  string         `json:"hash"`
	Files []ArtifactFile `json:"files"`
}

// ArtifactFile is one downloadable file within an artifact. Serve its
// content via ArtifactFilePath.
type ArtifactFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// Deploy statuses.
const (
	StatusQueued   = db.DeployQueued
	StatusBuilding = db.DeployBuilding
	StatusReady    = db.DeployReady
	StatusFailed   = db.DeployFailed
	StatusEvicted  = db.DeployEvicted
)

// Orchestrator owns the preview pipeline. Create with New; release with
// Close.
type Orchestrator struct {
	opts     Options
	database *db.Store
	files    *store.Store
	git      *gitrepo.Manager
	super    *supervise.Manager
	queue    *build.Queue
	watcher  *watch.Watcher
	sweeper  *retain.Sweeper
	stop     context.CancelFunc
	// previewBase is the resolved public address space of previews, from
	// Options' PreviewDomain/PreviewBaseURL/Addr.
	previewBase config.PreviewBase
	// dbPath is the SQLite location actually opened, so storage reporting
	// sizes the real database rather than a default that may not exist.
	dbPath string
}

// runnerAdapter bridges the public Runner onto the internal interface.
type runnerAdapter struct{ r Runner }

func (a runnerAdapter) Run(ctx context.Context, spec build.RunSpec, output io.Writer) error {
	pub := RunSpec{
		RepoName:   spec.RepoName,
		SHA:        spec.SHA,
		ScratchDir: spec.ScratchDir,
		Dir:        spec.Dir,
		Argv:       spec.Argv,
		Image:      spec.Image,
		Devcontainer: Devcontainer{
			Image: spec.Devcontainer.Image,
			Home:  spec.Devcontainer.Home,
		},
	}
	for _, m := range spec.Devcontainer.Mounts {
		pub.Devcontainer.Mounts = append(pub.Devcontainer.Mounts, DevcontainerMount(m))
	}
	return a.r.Run(ctx, pub, output)
}

// New starts an orchestrator: it verifies git, opens the database, sweeps
// stale scratch space, reclaims orphaned processes from an unclean exit,
// and launches the build workers.
func New(opts Options) (*Orchestrator, error) {
	if opts.DataDir == "" {
		return nil, fmt.Errorf("orchestrator: DataDir is required")
	}
	previewBase, err := config.ResolvePreviewBase(opts.PreviewDomain, opts.PreviewBaseURL, opts.Addr)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: %w", err)
	}
	opts.PreviewDomain = previewBase.Domain
	if opts.BuildConcurrency <= 0 {
		opts.BuildConcurrency = 2
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := os.MkdirAll(opts.DataDir, 0o755); err != nil {
		cancel()
		return nil, fmt.Errorf("orchestrator: create data dir: %w", err)
	}
	dbPath := opts.DBPath
	if dbPath == "" {
		dbPath = opts.DataDir + "/preview.db"
	}
	database, err := db.Open(dbPath)
	if err != nil {
		cancel()
		return nil, err
	}

	files := store.New(opts.DataDir+"/artifacts", opts.DataDir+"/state", opts.DataDir+"/tmp")
	files.SweepTmp(24 * time.Hour)
	gitMgr := gitrepo.NewManager(opts.DataDir + "/repos")
	if opts.MaxWarm == 0 {
		opts.MaxWarm = 8
	}
	super := supervise.New(database, files, opts.DataDir+"/logs")
	super.ReclaimOrphans()
	super.SetMaxWarm(opts.MaxWarm)
	super.StartReaper(ctx)

	var runner build.Runner
	if opts.Runner != nil {
		runner = runnerAdapter{r: opts.Runner}
	}
	queue := build.NewQueue(database, gitMgr, files, super, opts.DataDir+"/logs", runner)
	if len(opts.ManifestSources) > 0 {
		refs := make([]build.ManifestRef, len(opts.ManifestSources))
		for i, s := range opts.ManifestSources {
			refs[i] = build.ManifestRef{Path: s.Path, Table: s.Table}
		}
		queue.SetManifestRefs(refs)
	}
	if opts.LocalManifestDir != "" {
		queue.SetLocalManifestDir(opts.LocalManifestDir)
	}
	queue.Start(ctx, opts.BuildConcurrency)
	if opts.PollInterval == 0 {
		opts.PollInterval = watch.DefaultInterval
	}
	watcher := watch.New(database, gitMgr, queue, opts.PollInterval)
	watcher.Start(ctx)

	sweeper := retain.New(database, super, files)
	if opts.RetentionInterval > 0 {
		sweeper.SetInterval(opts.RetentionInterval)
	}
	if opts.RetentionInterval >= 0 {
		sweeper.Start(ctx)
	}

	return &Orchestrator{
		opts:        opts,
		database:    database,
		files:       files,
		git:         gitMgr,
		super:       super,
		queue:       queue,
		watcher:     watcher,
		sweeper:     sweeper,
		stop:        cancel,
		dbPath:      dbPath,
		previewBase: previewBase,
	}, nil
}

// Close stops the build workers and every supervised backend process, then
// closes the database. Backends never outlive the orchestrator.
func (o *Orchestrator) Close() error {
	o.stop()
	o.super.StopAll()
	return o.database.Close()
}

// WrapHost returns the top-level handler: requests whose Host is a preview
// subdomain (<sha>-<repo>.<domain>) are served by the orchestrator;
// everything else falls through to next.
func (o *Orchestrator) WrapHost(next http.Handler) http.Handler {
	rt := proxy.New(o.database, o.files, supervise.LocalBackends{M: o.super}, o.opts.PreviewDomain, next)
	rt.SetRunLogs(o.super)
	return rt
}

// RegisterRepo registers a repository (mirror clone of source, a local path
// or clone URL). Idempotent: re-registering the same name+source returns
// the existing repo; the same name with a different source is ErrConflict.
// The name must be a lowercase DNS label — it becomes the subdomain segment.
func (o *Orchestrator) RegisterRepo(ctx context.Context, name, source string) (Repo, error) {
	if err := gitrepo.ValidateName(name); err != nil {
		return Repo{}, err
	}
	if existing, err := o.database.GetRepoByName(name); err == nil {
		if existing.Source == source {
			return toRepo(existing), nil
		}
		return Repo{}, fmt.Errorf("repo %q is registered with source %q: %w", name, existing.Source, ErrConflict)
	}
	gr, err := o.git.Add(ctx, name, source, nil)
	if err != nil {
		return Repo{}, err
	}
	created, err := o.database.CreateRepo(name, source, gr.Path, db.RepoReady)
	if errors.Is(err, db.ErrConflict) {
		return Repo{}, ErrConflict
	}
	if err != nil {
		return Repo{}, err
	}
	return toRepo(created), nil
}

// DeleteRepo unregisters a repository: it stops the repo's supervised
// backends, removes its database rows, then deletes its mirror clone,
// artifacts, state directories, and build logs. On-disk cleanup failures
// after the rows are gone are ignored — leftovers are unreachable and only
// cost disk. Returns ErrNotFound if the name isn't registered.
func (o *Orchestrator) DeleteRepo(name string) error {
	repo, err := o.database.GetRepoByName(name)
	if errors.Is(err, db.ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	o.super.StopRepo(repo.ID, "repo deleted")
	o.super.PurgeRepoContainers(repo.Name)
	if err := o.database.DeleteRepo(repo.ID); err != nil {
		return err
	}
	o.git.Remove(repo.Name)
	o.files.RemoveRepo(repo.Name)
	os.RemoveAll(o.opts.DataDir + "/logs/" + repo.Name)
	return nil
}

// SetRepoWatch enables or disables watching for a registered repo: watched
// repos are polled every PollInterval and branch tips that move deploy
// automatically. branches narrows which branches with comma-separated globs
// ("" = all). Enabling records the tips that already exist without deploying
// them; SetRepoWatchBackfill deploys those too.
func (o *Orchestrator) SetRepoWatch(name string, watchOn bool, branches string) (Repo, error) {
	return o.setRepoWatch(name, watchOn, branches, false)
}

// SetRepoWatchBackfill is SetRepoWatch that also deploys every matching
// branch tip the repo already has, not just the ones that move from here.
func (o *Orchestrator) SetRepoWatchBackfill(name string, watchOn bool, branches string) (Repo, error) {
	return o.setRepoWatch(name, watchOn, branches, true)
}

func (o *Orchestrator) setRepoWatch(name string, watchOn bool, branches string, backfill bool) (Repo, error) {
	canon, err := watch.ValidatePatterns(branches)
	if err != nil {
		return Repo{}, err
	}
	repo, err := o.database.GetRepoByName(name)
	if errors.Is(err, db.ErrNotFound) {
		return Repo{}, fmt.Errorf("repo %q: %w", name, ErrNotFound)
	}
	if err != nil {
		return Repo{}, err
	}
	row, err := o.database.SetRepoWatch(repo.ID, watchOn, canon, backfill)
	if err != nil {
		return Repo{}, err
	}
	if watchOn {
		o.watcher.Kick()
	}
	return toRepo(row), nil
}

// Repos lists registered repositories.
func (o *Orchestrator) Repos() ([]Repo, error) {
	rows, err := o.database.ListRepos()
	if err != nil {
		return nil, err
	}
	out := make([]Repo, len(rows))
	for i, r := range rows {
		out[i] = toRepo(r)
	}
	return out, nil
}

// RequestDeploy resolves ref (branch, tag, or sha) in the named repo and
// queues a build if needed. Idempotent per commit; rebuild bypasses the
// artifact cache.
func (o *Orchestrator) RequestDeploy(ctx context.Context, repo, ref string, rebuild bool) (Deploy, error) {
	// The embedding-API façade has no notion of a triggering identity;
	// created_by stays empty (the HTTP layer records it for real callers).
	row, err := o.queue.RequestDeploy(ctx, repo, ref, rebuild, "")
	if errors.Is(err, db.ErrNotFound) {
		return Deploy{}, fmt.Errorf("repo %q: %w", repo, ErrNotFound)
	}
	if err != nil {
		return Deploy{}, err
	}
	return o.toDeploy(row), nil
}

// Deploy returns one deploy by ID.
func (o *Orchestrator) Deploy(id int64) (Deploy, error) {
	row, err := o.database.GetDeployByID(id)
	if errors.Is(err, db.ErrNotFound) {
		return Deploy{}, ErrNotFound
	}
	if err != nil {
		return Deploy{}, err
	}
	return o.toDeploy(row), nil
}

// StopDeploy stops the supervised processes backing a deploy without removing
// it; they cold-start again on the next request. Processes are shared per
// artifact hash, so sibling deploys on the same hash stop too. Returns
// ErrNotFound if the deploy doesn't exist.
func (o *Orchestrator) StopDeploy(id int64) error {
	row, err := o.database.GetDeployByID(id)
	if errors.Is(err, db.ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	o.super.StopDeploy(row, "stopped via API")
	return nil
}

// DeleteDeploy removes a deploy and reclaims any artifacts, backend state, and
// process bookkeeping that no surviving deploy still references — the
// content-addressed halves shared with another deploy are kept. Returns
// ErrNotFound if the deploy doesn't exist.
func (o *Orchestrator) DeleteDeploy(id int64) error {
	row, err := o.database.GetDeployByID(id)
	if errors.Is(err, db.ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if err := o.database.DeleteDeploy(row.ID); err != nil {
		return err
	}
	o.super.GCDeploy(row)
	return nil
}

// DeployQuery narrows and pages a deploy listing. The zero value matches
// every deploy.
type DeployQuery struct {
	// Repo matches a registered repo name exactly.
	Repo string
	// Status matches a build status exactly — one of the Status* constants.
	// Evicted deploys stay listed as history, so filtering them out is how a
	// caller shows only deploys that still have artifacts on disk.
	Status string
	// Query is free text from a search box: a case-insensitive prefix of the
	// commit sha, or a case-insensitive substring of the repo name, branch,
	// ref, author name, or author email. Wildcard characters match
	// themselves, so a query is safe to pass through unescaped.
	Query string
	// Limit caps the page size; 0 means every match.
	Limit int
	// Offset skips that many matches. Rows are ordered by descending id.
	Offset int
}

// DeployPage is one page of a listing plus the size of the whole result set,
// so a caller can render "showing 1-25 of 118" without a second query.
type DeployPage struct {
	Deploys []Deploy
	// Total counts every deploy matching the query, ignoring Limit/Offset.
	Total int
}

// DeploysPage lists deploys newest first, narrowed and paged by q. Prefer it
// over Deploys anywhere the deploy count grows without bound: evicted deploys
// are kept as history, so a long-lived instance accumulates rows even while
// retention holds bytes in check.
func (o *Orchestrator) DeploysPage(q DeployQuery) (DeployPage, error) {
	f := db.DeployFilter{
		Repo:   q.Repo,
		Status: q.Status,
		Query:  q.Query,
		Limit:  q.Limit,
		Offset: q.Offset,
	}
	total, err := o.database.CountDeploys(f)
	if err != nil {
		return DeployPage{}, err
	}
	rows, err := o.database.ListDeploys(f)
	if err != nil {
		return DeployPage{}, err
	}
	out := make([]Deploy, len(rows))
	for i, row := range rows {
		out[i] = o.toDeploy(row)
	}
	return DeployPage{Deploys: out, Total: total}, nil
}

// Deploys lists every deploy newest first, optionally filtered by repo name.
// DeploysPage is the paged form.
func (o *Orchestrator) Deploys(repo string) ([]Deploy, error) {
	rows, err := o.database.ListDeploys(db.DeployFilter{Repo: repo})
	if err != nil {
		return nil, err
	}
	out := make([]Deploy, len(rows))
	for i, row := range rows {
		out[i] = o.toDeploy(row)
	}
	return out, nil
}

// DeployLogs returns a plain-text snapshot of a deploy's build logs.
func (o *Orchestrator) DeployLogs(id int64) (string, error) {
	row, err := o.database.GetDeployByID(id)
	if errors.Is(err, db.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	var b strings.Builder
	parts := []struct{ title, path string }{
		{"frontend build", row.FeBuildLogPath},
		{"backend build", row.BeBuildLogPath},
	}
	for _, name := range slices.Sorted(maps.Keys(row.Artifacts)) {
		parts = append(parts, struct{ title, path string }{
			"artifacts." + name + " build", row.Artifacts[name].LogPath,
		})
	}
	for _, part := range parts {
		fmt.Fprintf(&b, "--- %s ---\n", part.title)
		if part.path == "" {
			b.WriteString("(not started)\n")
			continue
		}
		content, err := os.ReadFile(part.path)
		if err != nil {
			b.WriteString("(no log)\n")
			continue
		}
		b.Write(content)
		b.WriteString("\n")
	}
	return b.String(), nil
}

func toRepo(r db.Repo) Repo {
	return Repo{
		ID: r.ID, Name: r.Name, Source: r.Source,
		Watch: r.Watch, WatchBranches: r.WatchBranches,
		CreatedAt: r.CreatedAt,
	}
}

func (o *Orchestrator) toDeploy(row db.DeployRow) Deploy {
	d := Deploy{
		ID:           row.ID,
		Repo:         row.RepoName,
		SHA:          row.SHA,
		ShortSHA:     row.ShortSHA,
		Ref:          row.Ref,
		Branch:       row.Branch,
		AuthorName:   row.AuthorName,
		AuthorEmail:  row.AuthorEmail,
		FeHash:       row.FeHash,
		BeHash:       row.BeHash,
		Status:       row.Status,
		Error:        row.Error,
		AttemptCount: row.AttemptCount,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
	if row.Status == db.DeployReady {
		d.PreviewURL = o.previewURL(row)
		if row.BeHash != "" {
			d.Process, d.ProcessError = o.processState(supervise.BackendKey(row.RepoID, row.BeHash))
		}
		if row.FeHash != "" && row.HasFeProcess {
			d.FeProcess, d.FeProcessError = o.processState(
				supervise.FrontendKey(row.RepoID, row.FeHash, row.BeHash))
		}
		for _, name := range slices.Sorted(maps.Keys(row.Artifacts)) {
			art := DeployArtifact{Name: name, Hash: row.Artifacts[name].Hash, Files: []ArtifactFile{}}
			for _, f := range o.files.ListArtifactFiles(row.RepoName, art.Hash) {
				art.Files = append(art.Files, ArtifactFile{Name: f.Name, Size: f.Size})
			}
			d.Artifacts = append(d.Artifacts, art)
		}
	}
	return d
}

// ArtifactFilePath returns the on-disk path of one downloadable artifact
// file of a ready deploy, for the embedding application to serve (the file
// is immutable, so http.ServeFile is enough). ErrNotFound covers a missing
// deploy, artifact name, or file, and deploys that aren't ready.
func (o *Orchestrator) ArtifactFilePath(deployID int64, artifact, file string) (string, error) {
	row, err := o.database.GetDeployByID(deployID)
	if errors.Is(err, db.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	ref, ok := row.Artifacts[artifact]
	if row.Status != db.DeployReady || !ok {
		return "", ErrNotFound
	}
	if file == "" || file == "." || file == ".." || strings.ContainsAny(file, `/\`) {
		return "", ErrNotFound
	}
	path := filepath.Join(o.files.ArtifactDir(row.RepoName, ref.Hash), file)
	if st, err := os.Stat(path); err != nil || !st.Mode().IsRegular() {
		return "", ErrNotFound
	}
	return path, nil
}

func (o *Orchestrator) previewURL(row db.DeployRow) string {
	return o.previewBase.URL(row.ShortSHA, row.RepoName)
}

// processState reports one side's live state plus, when that state is
// "crashed", the failure detail behind it.
func (o *Orchestrator) processState(k supervise.Key) (state, detail string) {
	state = o.super.Status(k)
	if state != supervise.StatusCrashed {
		return state, ""
	}
	f, _ := o.super.LastFailure(k)
	return state, f.Detail
}
