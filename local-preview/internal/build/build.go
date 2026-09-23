// Package build turns deploy requests into published artifacts. A small
// worker pool drains a queue of deploy IDs; each build extracts the commit's
// tree, reads preview.toml from that commit, computes the partition hashes,
// and builds only the sides whose artifacts don't exist yet. The frontend
// and backend gate readiness; downloadable artifacts build afterwards, from
// their own extraction, so they never delay the preview. Concurrent deploys
// that share a hash are deduplicated with singleflight.
//
// Build logs are keyed by artifact hash, not deploy ID — that's what a build
// execution actually is, and deploys sharing a hash share its log. Deploy
// rows carry the paths.
package build

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/devcontainer"
	"github.com/jmelahman/local-preview/internal/fleet"
	"github.com/jmelahman/local-preview/internal/gitrepo"
	"github.com/jmelahman/local-preview/internal/hashkey"
	"github.com/jmelahman/local-preview/internal/manifest"
	"github.com/jmelahman/local-preview/internal/store"
	"github.com/jmelahman/local-preview/internal/supervise"
)

// DefaultBuildTimeout bounds a single build step.
const DefaultBuildTimeout = 10 * time.Minute

// ErrRepoNotReady marks deploy requests against a repo whose mirror clone
// hasn't finished (or failed); the API maps it to 409.
var ErrRepoNotReady = errors.New("repo is not ready")

// ManifestName is the default contract file read from the committed tree.
const ManifestName = "preview.toml"

// devcontainerPaths are the in-tree locations tried, in order, for the
// devcontainer config at the deployed commit — the two standard spec
// locations (nested .devcontainer/<folder>/ variants are not searched).
var devcontainerPaths = []string{".devcontainer/devcontainer.json", ".devcontainer.json"}

// ManifestRef locates a preview manifest in a target repo: a TOML file at
// the repo root, optionally rooted at a top-level table (so embedders can
// host the manifest inside a larger config file).
type ManifestRef struct {
	Path  string
	Table string
}

func (r ManifestRef) String() string {
	if r.Table == "" {
		return r.Path
	}
	return fmt.Sprintf("%s [%s]", r.Path, r.Table)
}

// Queue coordinates deploys: request → queued row → worker → frontend and
// backend builds → state provisioning → ready → downloadable artifacts.
type Queue struct {
	db      *db.Store
	git     *gitrepo.Manager
	files   *store.Store
	super   *supervise.Manager
	logsDir string
	runner  Runner

	manifestRefs     []ManifestRef
	localManifestDir string
	autoStart        bool
	starter          DeployStarter
	buildTimeout     time.Duration
	sf               singleflight.Group
	work             chan int64

	mu      sync.Mutex
	rebuild map[int64]bool

	// Optional durable artifact tier (S3/MinIO), owned by the store. nil
	// disables persist/hydrate. Cached here from q.files at Start so the
	// persist pool's nil checks and enqueue guards don't reach through the
	// store on every published side.
	tier        store.ArtifactTier
	persistJobs chan persistJob
	persistQuit chan struct{}
	persistWG   sync.WaitGroup
}

// NewQueue wires the pipeline. runner may be nil for the default runner.
// Call Start to launch workers.
func NewQueue(database *db.Store, git *gitrepo.Manager, files *store.Store, super *supervise.Manager, logsDir string, runner Runner) *Queue {
	if runner == nil {
		// The default routes image-declared steps into containers, steps of
		// image-less sides into the repo's devcontainer when one resolves,
		// and everything else to the host.
		runner = &autoRunner{}
	}
	return &Queue{
		db:           database,
		git:          git,
		files:        files,
		super:        super,
		logsDir:      logsDir,
		runner:       runner,
		manifestRefs: []ManifestRef{{Path: ManifestName}},
		autoStart:    true,
		buildTimeout: DefaultBuildTimeout,
		work:         make(chan int64, 256),
		rebuild:      make(map[int64]bool),
	}
}

// SetManifestRefs replaces the manifest locations tried (in order) at each
// deployed commit. Call before Start.
func (q *Queue) SetManifestRefs(refs []ManifestRef) {
	if len(refs) > 0 {
		q.manifestRefs = refs
	}
}

// SetLocalManifestDir sets a directory searched for out-of-repo manifests
// (<dir>/<repo>.toml, plain preview.toml schema) when no in-repo source
// matches — onboarding a repo whose upstream can't carry a manifest. Call
// before Start.
func (q *Queue) SetLocalManifestDir(dir string) {
	q.localManifestDir = dir
}

// SetAutoStart controls whether a deploy's processes start as soon as its
// build turns ready (the default), rather than waiting for the first
// request. Call before Start.
func (q *Queue) SetAutoStart(v bool) {
	q.autoStart = v
}

// DeployStarter warm-starts a freshly built deploy's processes. On a single
// node it is the local *supervise.Manager; on a control node it is the fleet
// registry, so the pre-warm lands on the worker user traffic will route to —
// warming the control node's own Manager there would burn its memory on a
// process the proxy never dials.
type DeployStarter interface {
	StartDeploy(ctx context.Context, row db.DeployRow) error
}

// SetStarter overrides where auto-started deploys warm up (default: the
// local supervise.Manager). Call before Start.
func (q *Queue) SetStarter(s DeployStarter) {
	q.starter = s
}

// deployStarter returns the warm-start target, defaulting to the local manager.
func (q *Queue) deployStarter() DeployStarter {
	if q.starter != nil {
		return q.starter
	}
	return q.super
}

// Start launches n build workers and re-enqueues deploys interrupted by a
// previous shutdown.
func (q *Queue) Start(ctx context.Context, n int) {
	for range n {
		go q.worker(ctx)
	}
	q.startPersist()
	// A backlog larger than the work buffer would block the caller — which at
	// startup is the goroutine that still has to bring up the HTTP server — so
	// hand the resume to a goroutine the workers can drain behind.
	go func() {
		ids, err := q.db.ListUnfinishedDeployIDs()
		if err != nil {
			return
		}
		for _, id := range ids {
			select {
			case q.work <- id:
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (q *Queue) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case id := <-q.work:
			q.process(ctx, id)
		}
	}
}

func (q *Queue) enqueue(id int64) {
	q.work <- id
}

// RequestDeploy is the single entry point for every trigger adapter (CLI,
// hook, API, later poller/webhook): resolve ref → upsert deploy row →
// enqueue. Idempotent per (repo, sha): an in-flight or ready deploy is
// returned as-is unless rebuild is set.
// createdBy records the identity that triggered the deploy for the audit
// trail (SSO login, GitHub Actions actor, or "" for the poller); it is stored
// only on a freshly created row, not on a rebuild of an existing one.
func (q *Queue) RequestDeploy(ctx context.Context, repoName, ref string, rebuild bool, createdBy string) (db.DeployRow, error) {
	repo, gr, sha, err := q.resolveRepoRef(ctx, repoName, ref)
	if err != nil {
		return db.DeployRow{}, err
	}

	d, err := q.db.GetDeployBySHA(repo.ID, sha)
	switch {
	case err == nil:
		switch {
		case d.Status == db.DeployQueued || d.Status == db.DeployBuilding:
			// Already in flight.
		case d.Status == db.DeployReady && !rebuild:
			// Nothing to do.
		default:
			// failed, evicted, or an explicit rebuild of a ready deploy.
			if err := q.db.ResetDeploy(d.ID); err != nil {
				return db.DeployRow{}, err
			}
			q.setRebuild(d.ID, rebuild)
			q.enqueue(d.ID)
		}
	case errors.Is(err, db.ErrNotFound):
		meta := commitMeta(ctx, gr, ref, sha)
		meta.CreatedBy = createdBy
		d, err = q.db.CreateDeploy(repo.ID, sha, meta)
		if err != nil {
			return db.DeployRow{}, err
		}
		q.setRebuild(d.ID, rebuild)
		q.enqueue(d.ID)
	default:
		return db.DeployRow{}, err
	}
	return q.db.GetDeployByID(d.ID)
}

// resolveRepoRef looks up a ready repo and resolves ref to a commit sha. Branch
// and tag names resolve against local refs, which go stale, so it fetches
// first; full or abbreviated shas rely on ResolveRef's fetch-on-miss retry.
// Shared by RequestDeploy and Upload so a deploy and an upload of the same ref
// always resolve to the same commit. A repo that isn't ready (still cloning, or
// its clone failed) returns ErrRepoNotReady.
func (q *Queue) resolveRepoRef(ctx context.Context, repoName, ref string) (db.Repo, gitrepo.Repo, string, error) {
	repo, err := q.db.GetRepoByName(repoName)
	if err != nil {
		return db.Repo{}, gitrepo.Repo{}, "", err
	}
	switch repo.Status {
	case db.RepoReady:
	case db.RepoCloning:
		return db.Repo{}, gitrepo.Repo{}, "", fmt.Errorf("repo %q is still cloning: %w", repoName, ErrRepoNotReady)
	default:
		return db.Repo{}, gitrepo.Repo{}, "", fmt.Errorf("clone of repo %q failed (%s): %w", repoName, repo.Error, ErrRepoNotReady)
	}
	gr := q.git.Open(repo.Name)
	if !looksLikeSHA(ref) {
		if _, err := gr.Fetch(ctx); err != nil {
			return db.Repo{}, gitrepo.Repo{}, "", fmt.Errorf("fetch %s: %w", repoName, err)
		}
	}
	sha, err := gr.ResolveRef(ctx, ref)
	if err != nil {
		return db.Repo{}, gitrepo.Repo{}, "", err
	}
	return repo, gr, sha, nil
}

// EvictUnreachable reclaims deploys of repo whose commit is no longer
// reachable from any surviving branch tip — the cleanup that deleting an
// unmerged branch (or force-pushing past a commit) should trigger. tips are
// the repo's current branch-head shas, gathered by the caller after a
// fetch --prune; reachability is judged against every branch, not just the
// watched subset, so a commit still living on some other branch is never
// evicted. Returns the number of deploys evicted.
//
// Only pending and serving deploys are candidates. A queued deploy is
// cancelled before it wastes a build — process() skips evicted rows. A ready
// deploy's processes and content-addressed artifacts are released via
// GCDeploy, which spares any hash a surviving deploy still shares. Building
// deploys are left to finish and are caught on a later poll; failed and
// already-evicted rows own nothing to reclaim and stand as the historical
// record. Each deploy is marked evicted before its GC sweep so the
// shared-hash check observes the correct surviving set. Eviction is
// reversible: the proxy serves a "cleaned up" page and RequestDeploy rebuilds
// the deploy on demand.
func (q *Queue) EvictUnreachable(ctx context.Context, repo db.Repo, tips []string) (int, error) {
	rows, err := q.db.ListDeploys(db.DeployFilter{Repo: repo.Name})
	if err != nil {
		return 0, err
	}
	bySHA := make(map[string]db.DeployRow, len(rows))
	var candidates []string
	for _, row := range rows {
		if row.Status == db.DeployReady || row.Status == db.DeployQueued {
			bySHA[row.SHA] = row
			candidates = append(candidates, row.SHA)
		}
	}
	if len(candidates) == 0 {
		return 0, nil
	}
	stale, err := q.git.Open(repo.Name).UnreachableSHAs(ctx, candidates, tips)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, sha := range stale {
		row := bySHA[sha]
		if err := q.db.SetDeployEvicted(row.ID); err != nil {
			log.Printf("evict %s@%.7s: %v", repo.Name, sha, err)
			continue
		}
		q.super.GCDeploy(row)
		n++
	}
	return n, nil
}

func (q *Queue) setRebuild(id int64, v bool) {
	q.mu.Lock()
	if v {
		q.rebuild[id] = true
	} else {
		delete(q.rebuild, id)
	}
	q.mu.Unlock()
}

func (q *Queue) takeRebuild(id int64) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	v := q.rebuild[id]
	delete(q.rebuild, id)
	return v
}

func (q *Queue) process(ctx context.Context, id int64) {
	row, err := q.db.GetDeployByID(id)
	if err != nil {
		log.Printf("build: load deploy %d: %v", id, err)
		return
	}
	if row.Status == db.DeployEvicted {
		return
	}
	if row.Status == db.DeployReady {
		// Re-enqueued at startup: the preview already serves; only artifact
		// builds a shutdown interrupted remain.
		q.buildArtifacts(ctx, row, false)
		return
	}
	if err := q.db.SetDeployBuilding(id); err != nil {
		log.Printf("build: mark building %d: %v", id, err)
		return
	}
	rebuild := q.takeRebuild(id)
	if err := q.buildDeploy(ctx, row, rebuild); err != nil {
		log.Printf("build: deploy %d (%s@%s) failed: %v", id, row.RepoName, row.ShortSHA, err)
		q.db.SetDeployFailed(id, truncate(err.Error(), 500))
		return
	}
	// The hashes landed during the build; a worker serves by hydrating them
	// from the durable tier, so a deploy is not truly ready until its sides
	// are durable. Block here (Save is skip-if-exists, so this only waits out
	// the tail of the async persist) rather than let auto-start or a first
	// visit race the upload and 502 with "not present in durable tier".
	built, err := q.db.GetDeployByID(id)
	if err != nil {
		log.Printf("build: deploy %d: reload before ready: %v", id, err)
		q.db.SetDeployFailed(id, truncate(err.Error(), 500))
		return
	}
	if err := q.ensureDurable(built.RepoName, built.FeHash, built.BeHash); err != nil {
		log.Printf("build: deploy %d (%s@%s) persist: %v", id, built.RepoName, built.ShortSHA, err)
		q.db.SetDeployFailed(id, truncate("persist to durable tier: "+err.Error(), 500))
		return
	}
	q.db.SetDeployReady(id)
	log.Printf("build: deploy %d (%s@%s) ready", id, row.RepoName, row.ShortSHA)
	if q.autoStart {
		q.autoStartDeploy(ctx, id)
	}
	// Downloadable artifacts build only now, after the deploy went ready:
	// they can take a while and must never gate the preview.
	if row, err = q.db.GetDeployByID(id); err == nil {
		q.buildArtifacts(ctx, row, rebuild)
	}
}

// autoStartDeploy warm-starts a freshly built deploy's processes in the
// background so its first visit skips the cold start. Failures are logged
// only: the deploy is ready and start-on-request still applies; the idle
// reaper and warm cap govern the processes from here.
//
// ErrNoWorker is retried, and the retry is what makes scale-from-zero work
// for pushes: the failed placement registers unserved demand, the autoscaler
// launches a worker off it, and the retry loop is the only caller that will
// come back to actually warm the process once that worker registers
// (~2–4 min). Give up fire-once here and the summoned node boots to nothing —
// it idles until scale-in reclaims it, and the pusher still pays the cold
// start. Any other error is terminal: a worker existed and the start failed
// there, so retrying would just re-run a broken start against a live worker.
func (q *Queue) autoStartDeploy(ctx context.Context, id int64) {
	row, err := q.db.GetDeployByID(id) // re-read: the hashes landed during the build
	if err != nil {
		log.Printf("build: auto-start deploy %d: %v", id, err)
		return
	}
	go func() {
		// ~8 min of 30s attempts: comfortably past spot fulfillment + boot +
		// image pull + self-registration, bounded so a fleet that never gets a
		// worker (max_size reached, spot drought) doesn't retry forever.
		err := startWithRetry(ctx, 16, 30*time.Second, func() error {
			return q.deployStarter().StartDeploy(ctx, row)
		})
		if err != nil && ctx.Err() == nil {
			log.Printf("build: auto-start deploy %d (%s@%s): %v", id, row.RepoName, row.ShortSHA, err)
		}
	}()
}

// startWithRetry runs start, retrying only fleet.ErrNoWorker (a fleet at zero
// whose demand signal is busy launching a node) every gap, up to attempts. Any
// other error returns immediately — a worker existed and the start failed
// there, so retrying would re-run a broken start against a live worker.
func startWithRetry(ctx context.Context, attempts int, gap time.Duration, start func() error) error {
	for i := 0; ; i++ {
		err := start()
		if err == nil || !errors.Is(err, fleet.ErrNoWorker) || i >= attempts-1 {
			return err
		}
		select {
		case <-ctx.Done():
			return err
		case <-time.After(gap):
		}
	}
}

func (q *Queue) buildDeploy(ctx context.Context, row db.DeployRow, rebuild bool) error {
	gr := q.git.Open(row.RepoName)

	m, err := q.loadManifest(ctx, gr, row)
	if err != nil {
		return err
	}
	hs, err := q.resolveHashes(ctx, gr, row.SHA, m)
	if err != nil {
		return err
	}
	feHash, beHash, env := hs.fe, hs.be, hs.env
	artRefs := make(map[string]db.ArtifactRef, len(hs.art))
	for _, name := range slices.Sorted(maps.Keys(hs.art)) {
		h := hs.art[name]
		// Artifacts build after the deploy turns ready (see buildArtifacts);
		// a cache hit is ready the moment its hash is known.
		status := db.ArtifactBuilding
		if !rebuild && q.files.HasArtifact(row.RepoName, h) {
			status = db.ArtifactReady
		}
		artRefs[name] = db.ArtifactRef{Hash: h, LogPath: q.logPath(row.RepoName, "dl", h), Status: status}
	}
	feLog := q.logPath(row.RepoName, "fe", feHash)
	beLog := q.logPath(row.RepoName, "be", beHash)
	if err := q.db.SetDeployHashes(row.ID, feHash, beHash, feLog, beLog); err != nil {
		return err
	}
	if err := q.db.SetDeployArtifacts(row.ID, artRefs); err != nil {
		return err
	}

	// Try the durable tier before deciding to build: a hydrated artifact makes
	// HasFrontend/HasBackend true, so the build is skipped. Never on an explicit
	// rebuild, which must re-run the build. Downloadable artifacts hydrate later
	// (buildArtifacts), off the readiness-gating path.
	if !rebuild {
		q.hydrate(ctx, row.RepoName, "fe", feHash)
		q.hydrate(ctx, row.RepoName, "be", beHash)
	}

	needFe := rebuild || !q.files.HasFrontend(row.RepoName, feHash)
	needBe := rebuild || !q.files.HasBackend(row.RepoName, beHash)

	// One extraction serves both sides; skipped entirely on full cache hits.
	var scratch string
	if needFe || needBe {
		dir, cleanup, err := q.files.NewScratchDir("build")
		if err != nil {
			return err
		}
		defer cleanup()
		if err := gr.Archive(ctx, row.SHA, dir); err != nil {
			return err
		}
		scratch = dir
	}

	if needFe {
		key := row.RepoName + ":fe:" + feHash
		if _, err, _ := q.sf.Do(key, func() (any, error) {
			return nil, q.buildFrontend(ctx, row, m.Frontend, env, scratch, feHash, feLog, rebuild)
		}); err != nil {
			return fmt.Errorf("frontend build: %w (log: %s)", err, feLog)
		}
	}
	if needBe {
		key := row.RepoName + ":be:" + beHash
		if _, err, _ := q.sf.Do(key, func() (any, error) {
			return nil, q.buildBackend(ctx, row, m.Backend, env, scratch, beHash, beLog, rebuild)
		}); err != nil {
			return fmt.Errorf("backend build: %w (log: %s)", err, beLog)
		}
	}

	// Provision state before ready: "ready" must imply the state dir exists
	// so the proxy's hot path never provisions anything. Run configs carry
	// the top-level networks list — and the resolved devcontainer, the
	// side's default runtime when it pins no run_image — alongside the
	// section: run-time context the supervisor needs. Networks stay out of
	// the artifact hash; the devcontainer already fed it via devcExtra.
	runCfg, err := json.Marshal(struct {
		manifest.Backend
		Networks     []string             `json:"networks,omitempty"`
		Devcontainer *devcontainer.Config `json:"devcontainer,omitempty"`
	}{m.Backend, m.Networks, env.runDefault(m.Backend.RunImage)})
	if err != nil {
		return err
	}
	repo, err := q.db.GetRepoByName(row.RepoName)
	if err != nil {
		return err
	}
	if len(m.Frontend.Run) > 0 {
		feCfg, err := json.Marshal(struct {
			manifest.Frontend
			Networks     []string             `json:"networks,omitempty"`
			Devcontainer *devcontainer.Config `json:"devcontainer,omitempty"`
		}{m.Frontend, m.Networks, env.runDefault(m.Frontend.RunImage)})
		if err != nil {
			return err
		}
		if err := q.db.CreateFrontendArtifact(db.FrontendArtifact{
			RepoID: repo.ID, FeHash: feHash, RunConfig: string(feCfg),
		}); err != nil {
			return err
		}
	}
	return q.super.ForkOrInitStateDir(ctx, gr, repo.ID, row.RepoName, beHash, row.SHA, string(runCfg))
}

// hashSet is a commit's resolved content-addresses: the frontend and backend
// hashes plus one per declared downloadable artifact (keyed by name). env is
// the resolved build environment, carried alongside so a caller that goes on to
// build a side doesn't resolve the devcontainer twice.
type hashSet struct {
	fe  string
	be  string
	art map[string]string
	env buildEnv
}

// hashInputs resolves the two per-commit inputs every side's content-address
// shares: the build environment (the commit's devcontainer, when a side would
// use it) and the filtered git tree. Splitting this out lets Upload hash a
// single side without hashing — and thus without requiring valid partitions
// for — the others.
func (q *Queue) hashInputs(ctx context.Context, gr gitrepo.Repo, sha string, m manifest.Manifest) (buildEnv, []gitrepo.TreeEntry, error) {
	env := q.loadDevcontainer(ctx, gr, sha, m)
	entries, err := gr.LsTree(ctx, sha, "")
	return env, entries, err
}

// feHashOf, beHashOf, and artHashOf are the per-side content-address
// computations. They are the single source of truth shared by resolveHashes
// (the build) and Upload: an uploaded side must land in the exact slot a build
// would target, which holds only because both feed hashkey identical inputs
// (manifest section, resolved devcontainer via devcExtra, filtered tree).
func feHashOf(fe manifest.Frontend, env buildEnv, entries []gitrepo.TreeEntry) (string, error) {
	return hashkey.Frontend(fe, devcExtra(env.devc, fe.Image, fe.RunImage, len(fe.Run) > 0), entries)
}

func beHashOf(m manifest.Manifest, env buildEnv, entries []gitrepo.TreeEntry) (string, error) {
	return hashkey.Backend(m.Backend, m.Frontend.Path,
		devcExtra(env.devc, m.Backend.Image, m.Backend.RunImage, true), entries)
}

func artHashOf(m manifest.Manifest, spec manifest.Artifact, env buildEnv, entries []gitrepo.TreeEntry) (string, error) {
	return hashkey.Artifact(spec, m.Frontend.Path, devcExtra(env.devc, spec.Image, "", false), entries)
}

// resolveHashes computes the content-address of every side of a commit — what
// buildDeploy needs, since it hashes all sides to decide what to build.
func (q *Queue) resolveHashes(ctx context.Context, gr gitrepo.Repo, sha string, m manifest.Manifest) (hashSet, error) {
	env, entries, err := q.hashInputs(ctx, gr, sha, m)
	if err != nil {
		return hashSet{}, err
	}
	feHash, err := feHashOf(m.Frontend, env, entries)
	if err != nil {
		return hashSet{}, err
	}
	beHash, err := beHashOf(m, env, entries)
	if err != nil {
		return hashSet{}, err
	}
	art := make(map[string]string, len(m.Artifacts))
	for _, name := range slices.Sorted(maps.Keys(m.Artifacts)) {
		h, err := artHashOf(m, m.Artifacts[name], env, entries)
		if err != nil {
			return hashSet{}, fmt.Errorf("artifacts.%s: %w", name, err)
		}
		art[name] = h
	}
	return hashSet{fe: feHash, be: beHash, art: art, env: env}, nil
}

// buildArtifacts builds the deploy's still-pending downloadable artifacts.
// It runs after the deploy turned ready — artifact builds can take a while
// and must never gate the preview — so failures land on the artifact's ref
// (status + error) and leave the deploy ready. The side publishes renamed
// their subtrees out of buildDeploy's scratch tree (see REGRESSIONS.md), so
// this phase takes its own extraction of the same commit.
func (q *Queue) buildArtifacts(ctx context.Context, row db.DeployRow, rebuild bool) {
	var pending []string
	for _, name := range slices.Sorted(maps.Keys(row.Artifacts)) {
		if row.Artifacts[name].Status == db.ArtifactBuilding {
			pending = append(pending, name)
		}
	}
	if len(pending) == 0 {
		return
	}
	fail := func(name string, err error) {
		// A cancelled build is an interrupted one, not a failed one: leave
		// the ref building so the startup resume finishes it.
		if ctx.Err() != nil {
			return
		}
		log.Printf("build: deploy %d (%s@%s) artifacts.%s failed: %v",
			row.ID, row.RepoName, row.ShortSHA, name, err)
		if err := q.db.SetDeployArtifactStatus(row.ID, name, db.ArtifactFailed, truncate(err.Error(), 500)); err != nil {
			log.Printf("build: mark artifacts.%s failed for deploy %d: %v", name, row.ID, err)
		}
	}
	gr := q.git.Open(row.RepoName)
	m, err := q.loadManifest(ctx, gr, row)
	if err != nil {
		for _, name := range pending {
			fail(name, err)
		}
		return
	}
	env := q.loadDevcontainer(ctx, gr, row.SHA, m)

	var scratch string
	for _, name := range pending {
		ref := row.Artifacts[name]
		// A sibling deploy sharing the hash may have published it meanwhile, or
		// the durable tier may hold it from a prior build — either way, skip the
		// build.
		if !rebuild {
			q.hydrate(ctx, row.RepoName, "dl", ref.Hash)
		}
		if !rebuild && q.files.HasArtifact(row.RepoName, ref.Hash) {
			q.db.SetDeployArtifactStatus(row.ID, name, db.ArtifactReady, "")
			continue
		}
		spec, ok := m.Artifacts[name]
		if !ok {
			fail(name, fmt.Errorf("artifact is no longer declared by the manifest"))
			continue
		}
		if scratch == "" {
			dir, cleanup, err := q.files.NewScratchDir("artifacts")
			if err != nil {
				fail(name, err)
				continue
			}
			defer cleanup()
			if err := gr.Archive(ctx, row.SHA, dir); err != nil {
				fail(name, err)
				continue
			}
			scratch = dir
		}
		key := row.RepoName + ":dl:" + ref.Hash
		if _, err, _ := q.sf.Do(key, func() (any, error) {
			return nil, q.buildArtifact(ctx, row, spec, env, scratch, ref.Hash, ref.LogPath, rebuild)
		}); err != nil {
			fail(name, fmt.Errorf("%w (log: %s)", err, ref.LogPath))
			continue
		}
		q.db.SetDeployArtifactStatus(row.ID, name, db.ArtifactReady, "")
	}
}

// loadManifest reads the first present manifest source at the deployed
// commit, then falls back to the local manifest dir (<dir>/<repo>.toml,
// read from the server's disk rather than the committed tree). A missing
// file or table means "try the next source"; a present but invalid manifest
// fails the deploy with its parse error.
func (q *Queue) loadManifest(ctx context.Context, gr gitrepo.Repo, row db.DeployRow) (manifest.Manifest, error) {
	tried := make([]string, 0, len(q.manifestRefs)+1)
	for _, ref := range q.manifestRefs {
		tried = append(tried, ref.String())
		raw, err := gr.ReadFile(ctx, row.SHA, ref.Path)
		if err != nil {
			continue
		}
		m, err := manifest.ParseAt(raw, ref.Table)
		if errors.Is(err, manifest.ErrNoManifest) {
			continue
		}
		if err != nil {
			return manifest.Manifest{}, fmt.Errorf("%s: %w", ref, err)
		}
		return m, nil
	}
	if q.localManifestDir != "" {
		local := filepath.Join(q.localManifestDir, row.RepoName+".toml")
		tried = append(tried, local)
		if raw, err := os.ReadFile(local); err == nil {
			m, err := manifest.Parse(raw)
			if err != nil {
				return manifest.Manifest{}, fmt.Errorf("%s: %w", local, err)
			}
			return m, nil
		}
	}
	return manifest.Manifest{}, fmt.Errorf(
		"no preview manifest at %s (looked for %s) — is the repo onboarded?",
		row.ShortSHA, strings.Join(tried, ", "))
}

func (q *Queue) buildFrontend(ctx context.Context, row db.DeployRow, fe manifest.Frontend, env buildEnv, scratch, hash, logPath string, overwrite bool) error {
	logF, err := openLog(logPath)
	if err != nil {
		return err
	}
	defer logF.Close()
	env.noteSkipped(fe.Image, logF)
	for _, step := range fe.Build {
		if err := q.runStep(ctx, row, scratch, fe.Path, fe.Image, env.devc, step, logF); err != nil {
			return err
		}
	}
	if len(fe.Run) > 0 {
		// Process-mode frontend: the whole built tree is the artifact — the
		// server runs from it the way backends do; there is no static dist.
		if err := q.files.PublishFrontend(row.RepoName, hash, filepath.Join(scratch, fe.Path), overwrite); err != nil {
			return err
		}
		q.enqueuePersist(row.RepoName, "fe", hash)
		return nil
	}
	dist := filepath.Join(scratch, fe.Path, fe.Dist)
	if st, err := os.Stat(dist); err != nil || !st.IsDir() {
		return fmt.Errorf("frontend.dist %q was not produced by the build", fe.Dist)
	}
	if err := q.files.PublishFrontend(row.RepoName, hash, dist, overwrite); err != nil {
		return err
	}
	q.enqueuePersist(row.RepoName, "fe", hash)
	return nil
}

func (q *Queue) buildBackend(ctx context.Context, row db.DeployRow, be manifest.Backend, env buildEnv, scratch, hash, logPath string, overwrite bool) error {
	logF, err := openLog(logPath)
	if err != nil {
		return err
	}
	defer logF.Close()
	env.noteSkipped(be.Image, logF)
	for _, step := range be.Build {
		if err := q.runStep(ctx, row, scratch, be.Path, be.Image, env.devc, step, logF); err != nil {
			return err
		}
	}
	if err := q.files.PublishBackend(row.RepoName, hash, filepath.Join(scratch, be.Path), overwrite); err != nil {
		return err
	}
	q.enqueuePersist(row.RepoName, "be", hash)
	return nil
}

func (q *Queue) buildArtifact(ctx context.Context, row db.DeployRow, a manifest.Artifact, env buildEnv, scratch, hash, logPath string, overwrite bool) error {
	logF, err := openLog(logPath)
	if err != nil {
		return err
	}
	defer logF.Close()
	env.noteSkipped(a.Image, logF)
	for _, step := range a.Build {
		if err := q.runStep(ctx, row, scratch, a.Path, a.Image, env.devc, step, logF); err != nil {
			return err
		}
	}
	if err := q.files.PublishArtifactFiles(row.RepoName, hash, filepath.Join(scratch, a.Path), a.Files, overwrite); err != nil {
		return err
	}
	q.enqueuePersist(row.RepoName, "dl", hash)
	return nil
}

func (q *Queue) runStep(ctx context.Context, row db.DeployRow, scratch, dir, image string, devc devcontainer.Config, argv []string, logF io.Writer) error {
	cctx, cancel := context.WithTimeout(ctx, q.buildTimeout)
	defer cancel()
	fmt.Fprintf(logF, "$ %s\n", strings.Join(argv, " "))
	return q.runner.Run(cctx, RunSpec{
		RepoName:     row.RepoName,
		SHA:          row.SHA,
		ScratchDir:   scratch,
		Dir:          dir,
		Argv:         argv,
		Image:        image,
		Devcontainer: devc,
	}, logF)
}

// buildEnv is the deploy-wide default build/run environment resolved from
// the commit's devcontainer: the config itself, or a note explaining why a
// present devcontainer was skipped (surfaced in the build logs of sides
// that would have used it).
type buildEnv struct {
	devc devcontainer.Config
	note string
}

// noteSkipped writes the skip note to a side's build log when the side has
// no explicit image — the sides an unusable devcontainer actually affects.
func (e buildEnv) noteSkipped(sideImage string, logF io.Writer) {
	if e.note != "" && sideImage == "" {
		fmt.Fprintln(logF, e.note)
	}
}

// runDefault is the devcontainer stored in a side's run config as its
// default runtime — nil when the side pins an explicit run_image or no
// devcontainer resolved.
func (e buildEnv) runDefault(runImage string) *devcontainer.Config {
	if runImage != "" || e.devc.Image == "" {
		return nil
	}
	d := e.devc
	return &d
}

// loadDevcontainer resolves the deployed commit's devcontainer as the
// default build/run environment for sides without an explicit image. An
// unusable config (parse error, no image — e.g. a Dockerfile build) never
// fails the deploy — the file isn't part of the preview contract; it falls
// back to the host with the reason in buildEnv.note.
func (q *Queue) loadDevcontainer(ctx context.Context, gr gitrepo.Repo, sha string, m manifest.Manifest) buildEnv {
	if !m.DevcontainerEnabled() {
		return buildEnv{}
	}
	for _, p := range devcontainerPaths {
		raw, err := gr.ReadFile(ctx, sha, p)
		if err != nil {
			continue
		}
		cfg, err := devcontainer.Parse(raw)
		if err != nil {
			return buildEnv{note: fmt.Sprintf("warning: %s: %v; building on the host", p, err)}
		}
		if cfg.Image == "" {
			return buildEnv{note: fmt.Sprintf("warning: %s declares no usable image (Dockerfile-built devcontainers are not supported); building on the host", p)}
		}
		return buildEnv{devc: cfg}
	}
	return buildEnv{}
}

// devcExtra is a side's hash contribution from the resolved devcontainer:
// non-nil only when the side would actually use it — for build steps (no
// explicit image) or for its run (a run command without run_image). The
// canonical JSON covers exactly the honored subset, so edits to unrelated
// devcontainer.json keys don't rebuild anything.
func devcExtra(devc devcontainer.Config, buildImage, runImage string, hasRun bool) []byte {
	if devc.Image == "" {
		return nil
	}
	if buildImage != "" && (!hasRun || runImage != "") {
		return nil
	}
	b, err := json.Marshal(devc)
	if err != nil {
		return nil
	}
	return b
}

func (q *Queue) logPath(repoName, kind, hash string) string {
	return filepath.Join(q.logsDir, repoName, kind, hash+".log")
}

// openLog truncates: a (re)build execution owns its per-hash log.
func openLog(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(f, "# build started %s\n", time.Now().UTC().Format(time.RFC3339))
	return f, nil
}

// commitMeta gathers the display metadata stored on a new deploy: the
// requested ref, the branch the commit is attributed to, and the commit
// author. Branch and author are best-effort — a deploy is never failed over
// metadata. When the ref itself isn't a branch (a sha or tag), the branch is
// derived from the branch tips pointing at the commit, so the common
// deploy-HEAD-by-sha flow still gets one.
func commitMeta(ctx context.Context, gr gitrepo.Repo, ref, sha string) db.DeployMeta {
	meta := db.DeployMeta{}
	if !looksLikeSHA(ref) {
		meta.Ref = ref
	}
	if meta.Ref != "" && gr.IsBranch(ctx, meta.Ref) {
		meta.Branch = meta.Ref
	} else if branches, err := gr.BranchesPointingAt(ctx, sha); err == nil && len(branches) > 0 {
		meta.Branch = branches[0]
	}
	if info, err := gr.CommitMeta(ctx, sha); err == nil {
		meta.AuthorName = info.AuthorName
		meta.AuthorEmail = info.AuthorEmail
	} else {
		log.Printf("build: commit metadata for %s@%s: %v", gr.Name, sha, err)
	}
	return meta
}

// looksLikeSHA reports whether ref is plausibly an (abbreviated) commit sha
// rather than a branch or tag name.
func looksLikeSHA(ref string) bool {
	if len(ref) < 7 || len(ref) > 64 {
		return false
	}
	for _, c := range ref {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
