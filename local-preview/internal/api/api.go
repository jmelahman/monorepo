// Package api defines the HTTP surface served at the apex host: JSON
// endpoints under /api/ plus the embedded dashboard SPA at /.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jmelahman/local-preview/internal/build"
	"github.com/jmelahman/local-preview/internal/clone"
	"github.com/jmelahman/local-preview/internal/config"
	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/githuboidc"
	"github.com/jmelahman/local-preview/internal/githubsso"
	"github.com/jmelahman/local-preview/internal/gitrepo"
	"github.com/jmelahman/local-preview/internal/retain"
	"github.com/jmelahman/local-preview/internal/store"
	"github.com/jmelahman/local-preview/internal/supervise"
	"github.com/jmelahman/local-preview/internal/watch"
	"github.com/jmelahman/local-preview/web"
)

// BuildInfo describes the running binary; mirrored from cmd/server so the
// api package doesn't import it.
type BuildInfo struct {
	Version string `json:"version"`
}

// RuntimeView is where the dashboard's live process state comes from: status,
// crash detail, resource samples, run logs, and stop. On a single node it is
// the local *supervise.Manager; on a control node whose previews run on
// workers it is the fleet registry, which merges the workers' reports — the
// local Manager tracks nothing there, so reading it would render every
// preview "idle" with no stats and empty run logs.
type RuntimeView interface {
	Status(k supervise.Key) string
	LastFailure(k supervise.Key) (supervise.Failure, bool)
	CrashedKeys() []supervise.Key
	Stats(ctx context.Context, k supervise.Key) *supervise.ProcessStats
	RunLog(repoName, side, hash string, attempt int, offset int64) (supervise.RunLog, error)
	Stop(k supervise.Key, reason string)
	StopRepo(repoID int64, reason string)
	// Exec bridges an interactive exec session (execstream frames on stream)
	// into the container backing k — directly on a single node, forwarded to
	// the worker holding the process on a control node.
	Exec(ctx context.Context, k supervise.Key, opts supervise.ExecOptions, stream io.ReadWriter) error
}

// Deps carries the dependencies handlers need.
type Deps struct {
	Store  *db.Store
	Build  BuildInfo
	Config config.Config
	Git    *gitrepo.Manager
	Queue  *build.Queue
	Super  *supervise.Manager
	// Runtime is the live-process view handlers render from; nil falls back
	// to Super (single-node). Super stays alongside it for control-node-local
	// lifecycle work (artifact GC, container purges) that never moves to
	// workers.
	Runtime RuntimeView
	// Cloner runs registrations' mirror clones in the background and holds
	// their live progress.
	Cloner *clone.Cloner
	// Watcher, when set, is kicked after watch settings change so a newly
	// watched repo polls immediately instead of waiting an interval.
	Watcher *watch.Watcher
	// Files locates published artifacts on disk (downloadable-artifact
	// listings and downloads).
	Files *store.Store
	// Sweeper runs retention sweeps (POST /api/gc).
	Sweeper *retain.Sweeper
	// DBPath is the SQLite location actually opened (":memory:" for
	// --in-memory), not Config's default — storage reporting sizes the real
	// database, never a stale file from a previous on-disk run.
	DBPath string
	// GitHubWebhookSecret validates X-Hub-Signature-256 on webhook
	// deliveries; empty disables POST /api/webhooks/github.
	GitHubWebhookSecret string
	// UploadAuth authenticates upload requests against GitHub Actions OIDC.
	// When non-nil, every upload must present a valid token whose repository
	// claim matches the target repo's source; nil leaves uploads
	// unauthenticated (the default, matching the rest of the API).
	UploadAuth UploadVerifier
	// MaxUploadBytes caps the compressed request body an upload may stream
	// before the server rejects it with 413, bounding the bytes an untrusted
	// (auth-exempt by default) client can push before extraction. 0 disables
	// the wire cap; the store's decompression cap still bounds expansion.
	MaxUploadBytes int64
	// SSO authenticates interactive dashboard logins (browser session cookie)
	// and programmatic callers (GitHub personal-access token as a bearer),
	// both against one allowlist. nil disables authentication entirely — the
	// API stays open, as it was before SSO existed.
	SSO SSOProvider
	// DashboardOrigin is the scheme://host the dashboard is served from,
	// derived from --sso-callback-url. State-changing API requests must carry
	// this Origin (CSRF defense), and preview-auth redirects target it.
	DashboardOrigin string
	// CookiesSecure marks session cookies Secure (HTTPS-only). Derived from the
	// callback URL: true for https and for localhost (a secure context even
	// over plain http).
	CookiesSecure bool
	// WarmPolicy reads the effective warm policy and SetWarmPolicy persists a
	// new one and applies it everywhere processes run (the local manager, and
	// every worker via the fleet's reconcile loop). Both nil disables the
	// endpoints (tests that don't care).
	WarmPolicy    func() WarmPolicy
	SetWarmPolicy func(p WarmPolicy) error
	// FleetStats returns the control node's fleet runtime rollup for the
	// statistics view (worker count, capacity, warm/cold totals). nil on a
	// single node, where handleStats falls back to the local Manager.
	FleetStats func() FleetSummary
}

// WarmPolicy shapes the warm-process footprint at runtime, per serving node:
// MaxWarm is the soft *target* — beyond it the reaper prunes the
// least-recently-used processes that are actually idle, but never
// actively-used ones, so bursts are served in full and pruned back once
// quiet (0 = unlimited). MinWarm is the floor — that many most-recent
// processes are exempt from idle timeouts, keeping the previews a developer
// is most likely to revisit hot indefinitely. IdleTimeoutSeconds overrides
// every manifest's idle_timeout when > 0 (0 = per-manifest values, default
// 30m).
type WarmPolicy struct {
	MaxWarm            int `json:"max_warm"`
	MinWarm            int `json:"min_warm"`
	IdleTimeoutSeconds int `json:"idle_timeout_seconds"`
}

// SSOProvider runs the GitHub OAuth web flow and resolves a token (an OAuth
// code or a personal-access token) to an allowlisted identity.
// *githubsso.Provider implements it; tests substitute a fake so no network or
// GitHub app is needed.
type SSOProvider interface {
	AuthCodeURL(state string) string
	Exchange(ctx context.Context, code string) (githubsso.Identity, error)
	VerifyToken(ctx context.Context, token string) (githubsso.Identity, error)
}

// UploadVerifier verifies an upload's bearer token and returns the GitHub
// Actions OIDC claims it carries. *githuboidc.Verifier implements it; tests
// substitute a fake so verification needs no network.
type UploadVerifier interface {
	Verify(ctx context.Context, rawToken string) (githuboidc.Claims, error)
}

// NewMux returns the apex-host handler: API routes plus the dashboard SPA.
func NewMux(d Deps) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", d.handleHealth)
	mux.HandleFunc("GET /api/auth/login", d.handleAuthLogin)
	mux.HandleFunc("GET /api/auth/callback", d.handleAuthCallback)
	mux.HandleFunc("POST /api/auth/logout", d.handleAuthLogout)
	mux.HandleFunc("GET /api/auth/me", d.handleAuthMe)
	mux.HandleFunc("GET /api/auth/preview-grant", d.handleAuthPreviewGrant)
	mux.HandleFunc("POST /api/repos", d.handleCreateRepo)
	mux.HandleFunc("GET /api/repos", d.handleListRepos)
	mux.HandleFunc("GET /api/repos/{name}", d.handleGetRepo)
	mux.HandleFunc("PATCH /api/repos/{name}", d.handleUpdateRepo)
	mux.HandleFunc("DELETE /api/repos/{name}", d.handleDeleteRepo)
	mux.HandleFunc("POST /api/webhooks/github", d.handleGitHubWebhook)
	mux.HandleFunc("POST /api/repos/{repo}/uploads/frontend", d.handleUploadFrontend)
	mux.HandleFunc("POST /api/repos/{repo}/uploads/backend", d.handleUploadBackend)
	mux.HandleFunc("POST /api/repos/{repo}/uploads/artifacts/{name}", d.handleUploadArtifact)
	mux.HandleFunc("POST /api/deploys", d.handleCreateDeploy)
	mux.HandleFunc("GET /api/deploys", d.handleListDeploys)
	mux.HandleFunc("GET /api/deploys/{id}", d.handleGetDeploy)
	mux.HandleFunc("POST /api/deploys/{id}/stop", d.handleStopDeploy)
	mux.HandleFunc("DELETE /api/deploys/{id}", d.handleDeleteDeploy)
	mux.HandleFunc("GET /api/deploys/{id}/logs", d.handleDeployLogs)
	mux.HandleFunc("GET /api/deploys/{id}/logs/run", d.handleDeployRunLog)
	mux.HandleFunc("GET /api/deploys/{id}/stats", d.handleDeployStats)
	mux.HandleFunc("GET /api/deploys/{id}/exec", d.handleDeployExec)
	mux.HandleFunc("GET /api/deploys/{id}/artifacts/{name}/{file}", d.handleArtifactDownload)
	mux.HandleFunc("GET /api/storage", d.handleStorage)
	mux.HandleFunc("GET /api/retention", d.handleGetRetention)
	mux.HandleFunc("PUT /api/retention", d.handlePutRetention)
	mux.HandleFunc("GET /api/warm", d.handleGetWarm)
	mux.HandleFunc("PUT /api/warm", d.handlePutWarm)
	mux.HandleFunc("GET /api/stats", d.handleStats)
	mux.HandleFunc("POST /api/gc", d.handleRunGC)
	mux.Handle("/", web.Handler())
	return mux
}

// handleHealth reports liveness plus the runtime facts the dashboard can't
// know on its own — notably the preview base domain, which is fixed at
// startup by --preview-domain/--preview-base-url.
func (d Deps) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":         "ok",
		"version":        d.Build.Version,
		"preview_domain": d.Config.Preview.Domain,
	})
}

// repoJSON augments a repo row with the clone's live progress line, present
// only while the repo is still cloning.
type repoJSON struct {
	db.Repo
	Progress string `json:"progress,omitempty"`
}

func (d Deps) repoJSON(r db.Repo) repoJSON {
	out := repoJSON{Repo: r}
	if r.Status == db.RepoCloning && d.Cloner != nil {
		out.Progress = d.Cloner.Progress(r.ID)
	}
	return out
}

// handleCreateRepo registers a repo. The mirror clone runs in the
// background — the response is 202 with the row in status "cloning"; poll
// GET /api/repos/{name} until it turns ready (or failed, with error set).
func (d Deps) handleCreateRepo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name          string `json:"name"`
		Source        string `json:"source"`
		Watch         bool   `json:"watch"`
		WatchBranches string `json:"watch_branches"`
		// Deploy the branch tips that already exist, rather than only what
		// changes from now on.
		Backfill bool `json:"backfill"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Source = strings.TrimSpace(req.Source)
	if err := gitrepo.ValidateName(req.Name); err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Source == "" {
		httpError(w, http.StatusBadRequest, "source is required (a local path or clone URL)")
		return
	}
	branches, err := watch.ValidatePatterns(req.WatchBranches)
	if err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}
	repo, err := d.Store.CreateRepo(req.Name, req.Source, d.Git.Open(req.Name).Path, db.RepoCloning)
	if errors.Is(err, db.ErrConflict) {
		httpError(w, http.StatusConflict, fmt.Sprintf("repo %q already exists", req.Name))
		return
	}
	if err != nil {
		internalError(w, "create repo", err)
		return
	}
	if req.Watch || branches != "" {
		// Stored before the clone starts; the watcher skips non-ready repos
		// and is kicked by the cloner once this one turns ready.
		if repo, err = d.Store.SetRepoWatch(repo.ID, req.Watch, branches, req.Backfill); err != nil {
			internalError(w, "set repo watch", err)
			return
		}
	}
	d.Cloner.Begin(repo)
	writeJSON(w, http.StatusAccepted, d.repoJSON(repo))
}

// handleUpdateRepo changes a repo's watch settings. Fields absent from the
// PATCH body keep their current value.
func (d Deps) handleUpdateRepo(w http.ResponseWriter, r *http.Request) {
	repo, err := d.Store.GetRepoByName(r.PathValue("name"))
	if errors.Is(err, db.ErrNotFound) {
		httpError(w, http.StatusNotFound, "repo not found")
		return
	}
	if err != nil {
		internalError(w, "get repo", err)
		return
	}
	var req struct {
		Watch         *bool   `json:"watch"`
		WatchBranches *string `json:"watch_branches"`
		Backfill      bool    `json:"backfill"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Watch == nil && req.WatchBranches == nil {
		httpError(w, http.StatusBadRequest, "nothing to update: set watch and/or watch_branches")
		return
	}
	watchOn, branches := repo.Watch, repo.WatchBranches
	if req.Watch != nil {
		watchOn = *req.Watch
	}
	if req.WatchBranches != nil {
		if branches, err = watch.ValidatePatterns(*req.WatchBranches); err != nil {
			httpError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	repo, err = d.Store.SetRepoWatch(repo.ID, watchOn, branches, req.Backfill)
	if err != nil {
		internalError(w, "set repo watch", err)
		return
	}
	if watchOn {
		d.Watcher.Kick()
	}
	writeJSON(w, http.StatusOK, d.repoJSON(repo))
}

func (d Deps) handleListRepos(w http.ResponseWriter, r *http.Request) {
	repos, err := d.Store.ListRepos()
	if err != nil {
		internalError(w, "list repos", err)
		return
	}
	out := make([]repoJSON, len(repos))
	for i, repo := range repos {
		out[i] = d.repoJSON(repo)
	}
	writeJSON(w, http.StatusOK, out)
}

func (d Deps) handleGetRepo(w http.ResponseWriter, r *http.Request) {
	repo, err := d.Store.GetRepoByName(r.PathValue("name"))
	if errors.Is(err, db.ErrNotFound) {
		httpError(w, http.StatusNotFound, "repo not found")
		return
	}
	if err != nil {
		internalError(w, "get repo", err)
		return
	}
	writeJSON(w, http.StatusOK, d.repoJSON(repo))
}

// handleDeleteRepo unregisters a repo: stops its backends, removes its DB
// rows, then deletes its mirror clone, artifacts, state, and logs. On-disk
// cleanup is best-effort once the rows are gone — leftovers are unreachable
// and only cost disk, so failures are logged rather than surfaced.
func (d Deps) handleDeleteRepo(w http.ResponseWriter, r *http.Request) {
	repo, err := d.Store.GetRepoByName(r.PathValue("name"))
	if errors.Is(err, db.ErrNotFound) {
		httpError(w, http.StatusNotFound, "repo not found")
		return
	}
	if err != nil {
		internalError(w, "get repo", err)
		return
	}
	d.Cloner.Cancel(repo.ID)
	d.runtime().StopRepo(repo.ID, "repo deleted")
	d.Super.PurgeRepoContainers(repo.Name)
	if err := d.Store.DeleteRepo(repo.ID); err != nil {
		internalError(w, "delete repo", err)
		return
	}
	if err := d.Git.Remove(repo.Name); err != nil {
		log.Printf("delete repo %s: remove mirror: %v", repo.Name, err)
	}
	for _, dir := range []string{
		filepath.Join(d.Config.ArtifactsDir(), repo.Name),
		filepath.Join(d.Config.StateDir(), repo.Name),
		filepath.Join(d.Config.LogsDir(), repo.Name),
	} {
		if err := os.RemoveAll(dir); err != nil {
			log.Printf("delete repo %s: %v", repo.Name, err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// deployJSON augments a deploy row with its preview URL and the
// supervisor's live process statuses. FeProcess is present only for
// process-mode frontends; Artifacts only for manifests that declare
// downloadable artifacts. The *Error fields carry the exit or start-failure
// detail behind a "crashed" side, so a caller never has to dig through the
// run log to learn why a preview stopped answering.
type deployJSON struct {
	db.DeployRow
	PreviewURL     string         `json:"preview_url,omitempty"`
	Process        string         `json:"process,omitempty"`
	ProcessError   string         `json:"process_error,omitempty"`
	FeProcess      string         `json:"fe_process,omitempty"`
	FeProcessError string         `json:"fe_process_error,omitempty"`
	Artifacts      []artifactJSON `json:"artifacts,omitempty"`
}

// artifactJSON is one named downloadable artifact on a ready deploy.
// Artifacts build after the deploy itself turns ready, so Status can lag
// the deploy's: building (files still empty), ready, or failed (Error holds
// the build failure summary).
type artifactJSON struct {
	Name   string             `json:"name"`
	Hash   string             `json:"hash"`
	Status string             `json:"status"`
	Error  string             `json:"error,omitempty"`
	Files  []artifactFileJSON `json:"files"`
}

type artifactFileJSON struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	// URL is the download path on the apex host.
	URL string `json:"url"`
}

func (d Deps) deployJSON(row db.DeployRow) deployJSON {
	out := deployJSON{DeployRow: row}
	if row.Status == db.DeployReady {
		out.PreviewURL = d.previewURL(row)
		if row.BeHash != "" {
			out.Process, out.ProcessError = d.processState(supervise.BackendKey(row.RepoID, row.BeHash))
		}
		if row.FeHash != "" && row.HasFeProcess {
			out.FeProcess, out.FeProcessError = d.processState(
				supervise.FrontendKey(row.RepoID, row.FeHash, row.BeHash))
		}
		for _, name := range slices.Sorted(maps.Keys(row.Artifacts)) {
			ref := row.Artifacts[name]
			status := ref.Status
			if status == "" {
				// Rows written before per-artifact statuses were built in full.
				status = db.ArtifactReady
			}
			art := artifactJSON{Name: name, Hash: ref.Hash, Status: status,
				Error: ref.Error, Files: []artifactFileJSON{}}
			for _, f := range d.Files.ListArtifactFiles(row.RepoName, ref.Hash) {
				art.Files = append(art.Files, artifactFileJSON{
					Name: f.Name,
					Size: f.Size,
					URL:  fmt.Sprintf("/api/deploys/%d/artifacts/%s/%s", row.ID, name, url.PathEscape(f.Name)),
				})
			}
			out.Artifacts = append(out.Artifacts, art)
		}
	}
	return out
}

// runtime returns the live-process view, defaulting to the local manager.
func (d Deps) runtime() RuntimeView {
	if d.Runtime != nil {
		return d.Runtime
	}
	return d.Super
}

// processState reports one side's live state plus, when that state is
// "crashed", the failure detail behind it.
func (d Deps) processState(k supervise.Key) (state, detail string) {
	rt := d.runtime()
	state = rt.Status(k)
	if state != supervise.StatusCrashed {
		return state, ""
	}
	f, _ := rt.LastFailure(k)
	return state, f.Detail
}

// crashedProcs translates the supervisor's crashed keys into the deploy
// columns that identify them, so a listing can filter on runtime state
// without the DB knowing anything about processes.
func (d Deps) crashedProcs() []db.ProcKey {
	keys := d.runtime().CrashedKeys()
	procs := make([]db.ProcKey, 0, len(keys))
	for _, k := range keys {
		if k.Side == supervise.SideFrontend {
			procs = append(procs, db.ProcKey{RepoID: k.RepoID, FeHash: k.Hash, BeHash: k.Peer})
			continue
		}
		procs = append(procs, db.ProcKey{RepoID: k.RepoID, BeHash: k.Hash})
	}
	return procs
}

// previewURL builds the public URL of a deploy's preview.
func (d Deps) previewURL(row db.DeployRow) string {
	return d.Config.Preview.URL(row.ShortSHA, row.RepoName)
}

func (d Deps) handleCreateDeploy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Repo    string `json:"repo"`
		Ref     string `json:"ref"`
		Rebuild bool   `json:"rebuild"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Repo == "" || req.Ref == "" {
		httpError(w, http.StatusBadRequest, "repo and ref are required")
		return
	}
	// A CI caller authenticates with a GitHub Actions OIDC token instead of a
	// session — `preview upload ... --oidc --deploy` deploys what it just
	// uploaded. The token is already verified; what it does not yet establish
	// is that its workflow owns this repo.
	// createdBy is the audit trail for who triggered this deploy: the GitHub
	// Actions actor for a CI/OIDC caller, otherwise the signed-in GitHub login
	// for a session caller. A PAT-bearer caller currently attaches no identity
	// (AuthMiddleware verifies the token but discards the resolved identity),
	// so it records empty rather than a wrong value.
	var createdBy string
	if claims, ok := oidcClaimsFrom(r.Context()); ok {
		if !d.oidcMayActOn(w, claims, req.Repo, "deploy") {
			return
		}
		createdBy = claims.Actor
	} else if sess, ok := d.apexSession(r); ok {
		createdBy = sess.GitHubLogin
	}
	row, err := d.Queue.RequestDeploy(r.Context(), req.Repo, req.Ref, req.Rebuild, createdBy)
	if errors.Is(err, db.ErrNotFound) {
		httpError(w, http.StatusNotFound, fmt.Sprintf("repo %q is not registered", req.Repo))
		return
	}
	if errors.Is(err, build.ErrRepoNotReady) {
		httpError(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, d.deployJSON(row))
}

func (d Deps) handleListDeploys(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status, crashed := q.Get("status"), db.CrashedAny
	switch {
	case status == supervise.StatusCrashed:
		// A crashed deploy is a ready one whose process died, so it answers
		// to its own status rather than hiding under "ready".
		status, crashed = db.DeployReady, db.CrashedOnly
	case status == db.DeployReady:
		crashed = db.CrashedNone
	case status != "" && !db.IsDeployStatus(status):
		httpError(w, http.StatusBadRequest, fmt.Sprintf(
			"unknown status %q (one of: queued, building, ready, crashed, failed, evicted)", status))
		return
	}
	limit := 0
	if s := q.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			httpError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		limit = n
	}
	offset := 0
	if s := q.Get("offset"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			httpError(w, http.StatusBadRequest, "offset must be a non-negative integer")
			return
		}
		offset = n
	}
	filter := db.DeployFilter{
		Repo:   q.Get("repo"),
		Branch: q.Get("branch"),
		Author: q.Get("author"),
		Status: status,
		Query:  q.Get("q"),
		Limit:  limit,
		Offset: offset,
	}
	if crashed != db.CrashedAny {
		filter.Crashed, filter.CrashedProcs = crashed, d.crashedProcs()
	}
	// The body stays a plain array, so the match count — what a pager needs to
	// know how far it can go — rides along in a header.
	total, err := d.Store.CountDeploys(filter)
	if err != nil {
		internalError(w, "count deploys", err)
		return
	}
	rows, err := d.Store.ListDeploys(filter)
	if err != nil {
		internalError(w, "list deploys", err)
		return
	}
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	out := make([]deployJSON, len(rows))
	for i, row := range rows {
		out[i] = d.deployJSON(row)
	}
	writeJSON(w, http.StatusOK, out)
}

func (d Deps) deployFromPath(w http.ResponseWriter, r *http.Request) (db.DeployRow, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpError(w, http.StatusBadRequest, "invalid deploy id")
		return db.DeployRow{}, false
	}
	row, err := d.Store.GetDeployByID(id)
	if errors.Is(err, db.ErrNotFound) {
		httpError(w, http.StatusNotFound, "deploy not found")
		return db.DeployRow{}, false
	}
	if err != nil {
		internalError(w, "get deploy", err)
		return db.DeployRow{}, false
	}
	return row, true
}

func (d Deps) handleGetDeploy(w http.ResponseWriter, r *http.Request) {
	row, ok := d.deployFromPath(w, r)
	if !ok {
		return
	}
	// A CI caller polls this until the deploy it just created settles, so it
	// may read only its own repo's deploys. Answering "not found" rather than
	// "forbidden" keeps the sequential IDs from being enumerable.
	if claims, isOIDC := oidcClaimsFrom(r.Context()); isOIDC && !d.oidcOwnsRepo(claims, row.RepoName) {
		httpError(w, http.StatusNotFound, "deploy not found")
		return
	}
	writeJSON(w, http.StatusOK, d.deployJSON(row))
}

// handleStopDeploy stops the deploy's supervised processes without removing
// it; they cold-start again on the next request. Because processes are shared
// per artifact hash, sibling deploys on the same hash stop too. Returns the
// updated deploy (its process state now reads idle).
func (d Deps) handleStopDeploy(w http.ResponseWriter, r *http.Request) {
	row, ok := d.deployFromPath(w, r)
	if !ok {
		return
	}
	d.stopDeploy(row, "stopped via API")
	writeJSON(w, http.StatusOK, d.deployJSON(row))
}

// handleDeleteDeploy hard-deletes a deploy: it removes the row, then stops and
// garbage-collects any artifacts, backend state, and process bookkeeping no
// surviving deploy still references (see supervise.Manager.GCDeploy). On-disk
// cleanup is best-effort — orphans only cost disk — so it never fails the
// request once the row is gone.
func (d Deps) handleDeleteDeploy(w http.ResponseWriter, r *http.Request) {
	row, ok := d.deployFromPath(w, r)
	if !ok {
		return
	}
	if err := d.Store.DeleteDeploy(row.ID); err != nil {
		internalError(w, "delete deploy", err)
		return
	}
	d.Super.GCDeploy(row)
	w.WriteHeader(http.StatusNoContent)
}

// handleArtifactDownload serves one file of a named downloadable artifact.
func (d Deps) handleArtifactDownload(w http.ResponseWriter, r *http.Request) {
	row, ok := d.deployFromPath(w, r)
	if !ok {
		return
	}
	if row.Status != db.DeployReady {
		httpError(w, http.StatusConflict, fmt.Sprintf("deploy is %s, not ready", row.Status))
		return
	}
	ref, ok := row.Artifacts[r.PathValue("name")]
	if !ok {
		httpError(w, http.StatusNotFound, "no such artifact")
		return
	}
	switch ref.Status {
	case db.ArtifactBuilding:
		httpError(w, http.StatusConflict, "artifact is still building")
		return
	case db.ArtifactFailed:
		httpError(w, http.StatusConflict, fmt.Sprintf("artifact build failed: %s", ref.Error))
		return
	}
	// Path values are decoded, so a segment can smuggle separators
	// (%2F, %5C); published files are flat base names, never nested.
	file := r.PathValue("file")
	if file == "" || file == "." || file == ".." || strings.ContainsAny(file, `/\`) {
		httpError(w, http.StatusNotFound, "no such file")
		return
	}
	path := filepath.Join(d.Files.ArtifactDir(row.RepoName, ref.Hash), file)
	if st, err := os.Stat(path); err != nil || !st.Mode().IsRegular() {
		httpError(w, http.StatusNotFound, "no such file")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", file))
	http.ServeFile(w, r, path)
}

// handleDeployLogs returns a plain-text snapshot of every build log.
// (?follow=1 streaming arrives in M2.)
func (d Deps) handleDeployLogs(w http.ResponseWriter, r *http.Request) {
	row, ok := d.deployFromPath(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
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
		fmt.Fprintf(w, "--- %s ---\n", part.title)
		if part.path == "" {
			fmt.Fprintln(w, "(not started)")
			continue
		}
		b, err := os.ReadFile(part.path)
		if err != nil {
			fmt.Fprintln(w, "(no log)")
			continue
		}
		w.Write(b)
		fmt.Fprintln(w)
	}
}

// runLogChunk is one incremental slice of a process run log.
type runLogChunk struct {
	Side    string `json:"side"`
	Attempt int    `json:"attempt"` // Nth start of the artifact; 0 = never started
	Offset  int64  `json:"offset"`  // echo back to receive only new bytes
	Content string `json:"content"`
	// Truncated marks a fresh view that skipped history beyond the tail cap.
	Truncated bool `json:"truncated,omitempty"`
	// Process is the side's live state, so a log view can label itself.
	Process string `json:"process,omitempty"`
}

// sideKey resolves the ?side= query to the deploy's supervisor key and
// artifact hash. ok=false means the error response was already written.
func (d Deps) sideKey(w http.ResponseWriter, r *http.Request, row db.DeployRow) (side, hash string, key supervise.Key, ok bool) {
	side = r.URL.Query().Get("side")
	switch side {
	case "", "be":
		side = "be"
		return side, row.BeHash, supervise.BackendKey(row.RepoID, row.BeHash), true
	case "fe":
		if row.FeHash != "" && !row.HasFeProcess {
			httpError(w, http.StatusNotFound, "deploy has no frontend process (static frontend)")
			return "", "", key, false
		}
		return side, row.FeHash, supervise.FrontendKey(row.RepoID, row.FeHash, row.BeHash), true
	default:
		httpError(w, http.StatusBadRequest, `side must be "be" or "fe"`)
		return "", "", key, false
	}
}

// handleDeployRunLog returns an incremental slice of a run log — the
// supervised process's combined stdout+stderr (init output included), the
// docker-logs view of a preview. Each call returns the latest start
// attempt's log from the requested offset; echoing attempt and offset back
// yields only new bytes, and a restart (new attempt) resets the view to a
// tail of the new file. Run logs outlive their process, so crash output is
// readable after the exit.
func (d Deps) handleDeployRunLog(w http.ResponseWriter, r *http.Request) {
	row, ok := d.deployFromPath(w, r)
	if !ok {
		return
	}
	side, hash, key, ok := d.sideKey(w, r, row)
	if !ok {
		return
	}
	if hash == "" {
		// No artifact yet (still building) or the deploy has no such side.
		writeJSON(w, http.StatusOK, runLogChunk{Side: side})
		return
	}
	q := r.URL.Query()
	attempt, _ := strconv.Atoi(q.Get("attempt"))
	offset, _ := strconv.ParseInt(q.Get("offset"), 10, 64)
	// The log lives wherever the process runs (or last ran): this node's disk
	// on a single node, a worker's via the fleet view on a control node.
	rl, err := d.runtime().RunLog(row.RepoName, side, hash, attempt, offset)
	if err != nil {
		internalError(w, "read run log", err)
		return
	}
	writeJSON(w, http.StatusOK, runLogChunk{
		Side:      side,
		Process:   d.runtime().Status(key),
		Attempt:   rl.Attempt,
		Offset:    rl.Offset,
		Content:   rl.Content,
		Truncated: rl.Truncated,
	})
}

// stopDeploy stops the deploy's supervised processes through the runtime
// view, so a preview served by a worker actually stops rather than only the
// (empty) local table being consulted.
func (d Deps) stopDeploy(row db.DeployRow, reason string) {
	rt := d.runtime()
	if row.BeHash != "" {
		rt.Stop(supervise.BackendKey(row.RepoID, row.BeHash), reason)
	}
	if row.FeHash != "" && row.HasFeProcess {
		rt.Stop(supervise.FrontendKey(row.RepoID, row.FeHash, row.BeHash), reason)
	}
}

// sideStats is one side's slice of a deploy stats response. Sampled fields
// are absent while the process isn't running (or can't be sampled);
// cpu_percent additionally needs two samples, so it appears from the second
// poll onward. Error accompanies a "crashed" state.
type sideStats struct {
	State            string   `json:"state"`
	Error            string   `json:"error,omitempty"`
	Runtime          string   `json:"runtime,omitempty"`
	CPUPercent       *float64 `json:"cpu_percent,omitempty"`
	MemoryBytes      *uint64  `json:"memory_bytes,omitempty"`
	MemoryLimitBytes uint64   `json:"memory_limit_bytes,omitempty"`
	StartedAt        string   `json:"started_at,omitempty"`
}

// handleDeployStats reports live resource usage — the docker-stats view of
// a preview — for the deploy's supervised processes. A side the deploy
// doesn't have is null.
func (d Deps) handleDeployStats(w http.ResponseWriter, r *http.Request) {
	row, ok := d.deployFromPath(w, r)
	if !ok {
		return
	}
	sample := func(k supervise.Key) *sideStats {
		state, detail := d.processState(k)
		s := &sideStats{State: state, Error: detail}
		if ps := d.runtime().Stats(r.Context(), k); ps != nil {
			s.Runtime = ps.Runtime
			s.CPUPercent = ps.CPUPercent
			s.MemoryBytes = &ps.MemoryBytes
			s.MemoryLimitBytes = ps.MemoryLimitBytes
			if !ps.StartedAt.IsZero() {
				s.StartedAt = ps.StartedAt.UTC().Format(time.RFC3339)
			}
		}
		return s
	}
	resp := struct {
		Backend  *sideStats `json:"backend"`
		Frontend *sideStats `json:"frontend"`
	}{}
	if row.BeHash != "" {
		resp.Backend = sample(supervise.BackendKey(row.RepoID, row.BeHash))
	}
	if row.FeHash != "" && row.HasFeProcess {
		resp.Frontend = sample(supervise.FrontendKey(row.RepoID, row.FeHash, row.BeHash))
	}
	writeJSON(w, http.StatusOK, resp)
}

// maxJSONBody caps a JSON request body before decoding. The API's payloads
// are all small (repo/deploy/retention descriptors), so 1 MiB is generous
// while still denying an unauthenticated caller an unbounded-allocation lever.
const maxJSONBody = 1 << 20

// decodeJSON reads and decodes a JSON request body under a fixed size cap.
// It mirrors the webhook handler's MaxBytesReader guard so no handler decodes
// an unbounded stream; an oversized body returns 413, malformed JSON 400. On
// error it has already written the response — the caller just returns.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			httpError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return false
		}
		httpError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}

// httpError writes a JSON error body so API clients never have to parse
// plain-text errors.
func httpError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// internalError logs the underlying error and hides it from the client.
func internalError(w http.ResponseWriter, op string, err error) {
	log.Printf("%s: %v", op, err)
	httpError(w, http.StatusInternalServerError, "internal server error")
}
