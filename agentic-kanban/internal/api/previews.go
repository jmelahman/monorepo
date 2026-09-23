package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/jmelahman/local-preview/orchestrator"

	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/previews"
	"github.com/jmelahman/kanban/internal/session"
)

// Preview deployments: each ticket branch can be deployed as a live preview
// served at <sha>.<board>.<preview-domain> by the embedded local-preview
// orchestrator. Deploys build from the committed tree of the board repo's
// branch (worktree branches share the repo's object store, so agent commits
// are visible with no extra plumbing).

// previewContext resolves the session-scoped preview inputs: the session,
// its board, the orchestrator repo name, and the host repo path. Writes the
// error response and returns ok=false on failure.
func (h *handlers) previewContext(w http.ResponseWriter, r *http.Request) (sess *db.Session, repoName, repoPath string, ok bool) {
	if h.previews == nil {
		h.httpError(w, fmt.Errorf("preview orchestrator unavailable (see server logs)"), 503)
		return nil, "", "", false
	}
	id := pathID(r, "id")
	sess, err := h.store.GetSession(r.Context(), id)
	if err != nil {
		h.httpError(w, err, 404)
		return nil, "", "", false
	}
	board, err := h.boardForSession(r.Context(), sess)
	if err != nil {
		h.httpError(w, err, 500)
		return nil, "", "", false
	}
	paths := session.ResolvePaths(board, sess)
	if !paths.HasRepo {
		h.httpError(w, fmt.Errorf("board has no git repo to deploy"), 400)
		return nil, "", "", false
	}
	if sess.BranchName == "" {
		h.httpError(w, fmt.Errorf("session has no branch to deploy"), 400)
		return nil, "", "", false
	}
	return sess, previews.RepoName(board), paths.RepoPath, true
}

// maybeAutoDeployPreview requests a preview of the session branch's current
// tip after an agent finishes a work burst (status → idle). Fire-and-forget:
// deploys are idempotent per commit, so an unchanged tip is a no-op. Gated
// on the board being onboarded — a manifest in the worktree, or an
// out-of-repo one on the server — so boards that haven't onboarded never
// accumulate failed deploys, and on KANBAN_PREVIEW_AUTO_DEPLOY.
func (h *handlers) maybeAutoDeployPreview(sess *db.Session) {
	if h.previews == nil || sess == nil || sess.BranchName == "" || sess.WorktreePath == "" {
		return
	}
	if !previews.AutoDeployEnabled() {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		board, err := h.boardForSession(ctx, sess)
		if err != nil {
			log.Printf("preview auto-deploy: board for session %d: %v", sess.ID, err)
			return
		}
		paths := session.ResolvePaths(board, sess)
		if !paths.HasRepo {
			return
		}
		name := previews.RepoName(board)
		// The onboarding gate needs the repo name (out-of-repo manifests
		// are keyed by it), hence resolving the board first.
		if !previews.Onboarded(sess.WorktreePath, name) {
			return
		}
		if _, err := h.previews.RegisterRepo(ctx, name, paths.RepoPath); err != nil {
			log.Printf("preview auto-deploy: register %s: %v", name, err)
			return
		}
		d, err := h.previews.RequestDeploy(ctx, name, sess.BranchName, false)
		if err != nil {
			log.Printf("preview auto-deploy: deploy %s@%s: %v", name, sess.BranchName, err)
			return
		}
		log.Printf("preview auto-deploy: %s@%s → deploy %d (%s)", name, sess.BranchName, d.ID, d.Status)
	}()
}

// listSessionPreviews returns the session branch's preview deploys, newest
// first. A board never deployed before simply has none.
func (h *handlers) listSessionPreviews(w http.ResponseWriter, r *http.Request) {
	sess, repoName, _, ok := h.previewContext(w, r)
	if !ok {
		return
	}
	all, err := h.previews.Deploys(repoName)
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	deploys := []orchestrator.Deploy{}
	for _, d := range all {
		if d.Ref == sess.BranchName {
			deploys = append(deploys, d)
		}
	}
	writeJSON(w, 200, deploys)
}

// createSessionPreview registers the board repo with the orchestrator
// (idempotent) and requests a deploy of the session branch's current tip.
func (h *handlers) createSessionPreview(w http.ResponseWriter, r *http.Request) {
	sess, repoName, repoPath, ok := h.previewContext(w, r)
	if !ok {
		return
	}
	if _, err := h.previews.RegisterRepo(r.Context(), repoName, repoPath); err != nil {
		h.httpError(w, fmt.Errorf("register repo for previews: %w", err), 500)
		return
	}
	deploy, err := h.previews.RequestDeploy(r.Context(), repoName, sess.BranchName, false)
	if err != nil {
		h.httpError(w, err, 400)
		return
	}
	writeJSON(w, 202, deploy)
}

// dashboardDeploy is one deploy plus the board it belongs to. The
// orchestrator only knows repo names (derived from board slugs), so kanban
// maps them back to boards here — the previews dashboard spans every board,
// unlike the session-scoped endpoints above. Board fields are empty for a
// deploy whose board has since been renamed or deleted.
type dashboardDeploy struct {
	orchestrator.Deploy
	BoardID   int64  `json:"board_id,omitempty"`
	BoardName string `json:"board_name,omitempty"`
	BoardSlug string `json:"board_slug,omitempty"`
}

// listPreviews returns every preview deploy across all boards, newest
// first — the previews dashboard's data source.
func (h *handlers) listPreviews(w http.ResponseWriter, r *http.Request) {
	if h.previews == nil {
		h.httpError(w, fmt.Errorf("preview orchestrator unavailable (see server logs)"), 503)
		return
	}
	deploys, err := h.previews.Deploys("")
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	boards, err := h.store.ListBoards(r.Context())
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	byRepo := make(map[string]db.Board, len(boards))
	for _, b := range boards {
		byRepo[previews.RepoName(&b)] = b
	}
	out := make([]dashboardDeploy, 0, len(deploys))
	for _, d := range deploys {
		row := dashboardDeploy{Deploy: d}
		if b, ok := byRepo[d.Repo]; ok {
			row.BoardID, row.BoardName, row.BoardSlug = b.ID, b.Name, b.Slug
		}
		out = append(out, row)
	}
	writeJSON(w, 200, out)
}

// createBoardPreview deploys an arbitrary ref of a board's repo. The
// session endpoint only ever deploys that session's branch; the dashboard
// needs to deploy anything the repo has — a base branch, a tag, a SHA.
func (h *handlers) createBoardPreview(w http.ResponseWriter, r *http.Request) {
	if h.previews == nil {
		h.httpError(w, fmt.Errorf("preview orchestrator unavailable (see server logs)"), 503)
		return
	}
	board, err := h.store.GetBoard(r.Context(), pathID(r, "id"))
	if err != nil {
		h.httpError(w, err, 404)
		return
	}
	body, err := decodeBody[struct {
		Ref string `json:"ref"`
	}](r)
	if err != nil && !errors.Is(err, io.EOF) {
		h.httpError(w, err, 400)
		return
	}
	ref := strings.TrimSpace(body.Ref)
	if ref == "" {
		ref = board.BaseBranch
	}
	if ref == "" {
		h.httpError(w, fmt.Errorf("ref required (board has no base branch)"), 400)
		return
	}
	paths := session.ResolvePaths(board, nil)
	if !paths.HasRepo {
		h.httpError(w, fmt.Errorf("board has no git repo to deploy"), 400)
		return
	}
	name := previews.RepoName(board)
	if _, err := h.previews.RegisterRepo(r.Context(), name, paths.RepoPath); err != nil {
		h.httpError(w, fmt.Errorf("register repo for previews: %w", err), 500)
		return
	}
	deploy, err := h.previews.RequestDeploy(r.Context(), name, ref, false)
	if err != nil {
		h.httpError(w, err, 400)
		return
	}
	writeJSON(w, 202, deploy)
}

// previewArtifact serves one downloadable file from a ready deploy's
// [artifacts.<name>] section — a CLI binary or other per-commit build
// output the manifest publishes instead of running. The orchestrator
// resolves the on-disk path and rejects anything that isn't a regular file
// inside the artifact dir, so this handler only has to stream it. Artifacts
// are content-addressed and immutable, hence the long-lived cache header.
func (h *handlers) previewArtifact(w http.ResponseWriter, r *http.Request) {
	if h.previews == nil {
		h.httpError(w, fmt.Errorf("preview orchestrator unavailable (see server logs)"), 503)
		return
	}
	name := r.PathValue("artifact")
	file := r.PathValue("file")
	path, err := h.previews.ArtifactFilePath(pathID(r, "id"), name, file)
	if errors.Is(err, orchestrator.ErrNotFound) {
		h.httpError(w, fmt.Errorf("artifact %q file %q not found", name, file), 404)
		return
	}
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(file)))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, path)
}

// stopPreview stops the supervised processes backing a deploy without
// removing it: the deploy stays listed and cold-starts again on the next
// request to its URL. Processes are shared per artifact hash, so sibling
// deploys built to the same output stop with it.
func (h *handlers) stopPreview(w http.ResponseWriter, r *http.Request) {
	if h.previews == nil {
		h.httpError(w, fmt.Errorf("preview orchestrator unavailable (see server logs)"), 503)
		return
	}
	err := h.previews.StopDeploy(pathID(r, "id"))
	if errors.Is(err, orchestrator.ErrNotFound) {
		h.httpError(w, fmt.Errorf("preview not found"), 404)
		return
	}
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	w.WriteHeader(204)
}

// deletePreview removes a deploy and reclaims the artifacts and process
// state no surviving deploy still references. Halves shared with another
// deploy (previews are content-addressed per side) are kept.
func (h *handlers) deletePreview(w http.ResponseWriter, r *http.Request) {
	if h.previews == nil {
		h.httpError(w, fmt.Errorf("preview orchestrator unavailable (see server logs)"), 503)
		return
	}
	err := h.previews.DeleteDeploy(pathID(r, "id"))
	if errors.Is(err, orchestrator.ErrNotFound) {
		h.httpError(w, fmt.Errorf("preview not found"), 404)
		return
	}
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	w.WriteHeader(204)
}

// storageRepo is one repo's slice of the storage report, resolved back to
// the board that owns it (orchestrator repos are named after boards).
type storageRepo struct {
	Repo           string `json:"repo"`
	ArtifactsBytes int64  `json:"artifacts_bytes"`
	StateBytes     int64  `json:"state_bytes"`
	LogsBytes      int64  `json:"logs_bytes"`
	MirrorBytes    int64  `json:"mirror_bytes"`
	TotalBytes     int64  `json:"total_bytes"`
	Deploys        int    `json:"deploys"`
	EvictedDeploys int    `json:"evicted_deploys"`
	BoardID        int64  `json:"board_id,omitempty"`
	BoardName      string `json:"board_name,omitempty"`
	BoardSlug      string `json:"board_slug,omitempty"`
}

// storageReport is the GET /api/previews/storage response.
type storageReport struct {
	TotalBytes     int64         `json:"total_bytes"`
	ArtifactsBytes int64         `json:"artifacts_bytes"`
	StateBytes     int64         `json:"state_bytes"`
	LogsBytes      int64         `json:"logs_bytes"`
	MirrorBytes    int64         `json:"mirror_bytes"`
	TmpBytes       int64         `json:"tmp_bytes"`
	DBBytes        int64         `json:"db_bytes"`
	Repos          []storageRepo `json:"repos"`
}

// previewStorage reports how much disk the preview orchestrator uses,
// broken down by category and by board. The orchestrator walks its data dir
// to answer, so this is a live number — call it from a user action, not a
// poll.
func (h *handlers) previewStorage(w http.ResponseWriter, r *http.Request) {
	if h.previews == nil {
		h.httpError(w, fmt.Errorf("preview orchestrator unavailable (see server logs)"), 503)
		return
	}
	rep, err := h.previews.Storage()
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	boards, err := h.store.ListBoards(r.Context())
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	byRepo := make(map[string]db.Board, len(boards))
	for _, b := range boards {
		byRepo[previews.RepoName(&b)] = b
	}
	out := storageReport{
		TotalBytes:     rep.TotalBytes,
		ArtifactsBytes: rep.ArtifactsBytes,
		StateBytes:     rep.StateBytes,
		LogsBytes:      rep.LogsBytes,
		MirrorBytes:    rep.MirrorBytes,
		TmpBytes:       rep.TmpBytes,
		DBBytes:        rep.DBBytes,
		Repos:          make([]storageRepo, 0, len(rep.Repos)),
	}
	for _, u := range rep.Repos {
		row := storageRepo{
			Repo:           u.Repo,
			ArtifactsBytes: u.ArtifactsBytes,
			StateBytes:     u.StateBytes,
			LogsBytes:      u.LogsBytes,
			MirrorBytes:    u.MirrorBytes,
			TotalBytes:     u.TotalBytes,
			Deploys:        u.Deploys,
			EvictedDeploys: u.EvictedDeploys,
		}
		if b, ok := byRepo[u.Repo]; ok {
			row.BoardID, row.BoardName, row.BoardSlug = b.ID, b.Name, b.Slug
		}
		out.Repos = append(out.Repos, row)
	}
	writeJSON(w, 200, out)
}

// previewRetention returns the retention policy bounding how many preview
// deploys are kept.
func (h *handlers) previewRetention(w http.ResponseWriter, r *http.Request) {
	if h.previews == nil {
		h.httpError(w, fmt.Errorf("preview orchestrator unavailable (see server logs)"), 503)
		return
	}
	policy, err := h.previews.RetentionPolicy()
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	writeJSON(w, 200, policy)
}

// updatePreviewRetention replaces the retention policy. It takes effect on
// the next sweep — hourly, or immediately via POST /api/previews/gc — so
// tightening limits never surprise-evicts on save. Either limit at 0 means
// unlimited; both at 0 disables eviction.
func (h *handlers) updatePreviewRetention(w http.ResponseWriter, r *http.Request) {
	if h.previews == nil {
		h.httpError(w, fmt.Errorf("preview orchestrator unavailable (see server logs)"), 503)
		return
	}
	policy, err := decodeBody[orchestrator.RetentionPolicy](r)
	if err != nil {
		h.httpError(w, err, 400)
		return
	}
	if policy.MaxDeploysPerRepo < 0 || policy.MaxAgeDays < 0 {
		h.httpError(w, fmt.Errorf("retention limits must be >= 0 (0 = unlimited)"), 400)
		return
	}
	if err := h.previews.SetRetentionPolicy(policy); err != nil {
		h.httpError(w, err, 500)
		return
	}
	writeJSON(w, 200, policy)
}

// collectPreviewGarbage runs one retention sweep immediately and reports
// what it evicted. With retention disabled it still collects stale staging
// leftovers, so it always has something to do.
func (h *handlers) collectPreviewGarbage(w http.ResponseWriter, r *http.Request) {
	if h.previews == nil {
		h.httpError(w, fmt.Errorf("preview orchestrator unavailable (see server logs)"), 503)
		return
	}
	res, err := h.previews.CollectGarbage()
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	writeJSON(w, 200, res)
}

// previewLogs returns the build-log snapshot for one preview deploy.
func (h *handlers) previewLogs(w http.ResponseWriter, r *http.Request) {
	if h.previews == nil {
		h.httpError(w, fmt.Errorf("preview orchestrator unavailable (see server logs)"), 503)
		return
	}
	logs, err := h.previews.DeployLogs(pathID(r, "id"))
	if errors.Is(err, orchestrator.ErrNotFound) {
		h.httpError(w, fmt.Errorf("preview not found"), 404)
		return
	}
	if err != nil {
		h.httpError(w, err, 500)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, logs)
}
