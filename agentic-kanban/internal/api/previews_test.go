package api_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmelahman/local-preview/orchestrator"

	"github.com/jmelahman/kanban/internal/db"
)

// previewManifest is a minimal preview.toml whose builds need only a shell.
const previewManifest = `
[frontend]
path  = "web"
build = [["sh", "-c", "mkdir -p dist && cp index.html dist/"]]
dist  = "dist"

[backend]
path        = "srv"
build       = [["true"]]
run         = ["./never-started"]
health_path = "/api/health"
`

// seedPreviewableRepo makes the env's board repo deployable: preview.toml
// plus trivial frontend/backend files, committed on main.
func seedPreviewableRepo(t *testing.T, e *testEnv) {
	t.Helper()
	files := map[string]string{
		"preview.toml":   previewManifest,
		"web/index.html": "<html>ticket preview</html>",
		"srv/main.txt":   "backend-ish",
	}
	for name, content := range files {
		p := filepath.Join(e.repoPath, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustGit(t, e.repoPath, "add", "-A")
	mustGit(t, e.repoPath, "commit", "-qm", "add preview manifest")
}

func TestSessionPreviews(t *testing.T) {
	e := newEnv(t)
	seedPreviewableRepo(t, e)
	board := e.seedBoard("Demo Board")
	ticket := e.seedTicket(board, "Add feature")
	sess := e.seedSession(ticket)

	// The session branch exists in the board repo (normally created by the
	// worktree machinery).
	mustGit(t, e.repoPath, "branch", sess.BranchName)

	// No deploys yet — but the endpoint works before anything is registered.
	resp := e.get(fmt.Sprintf("/api/sessions/%d/previews", sess.ID))
	assertStatus(t, resp, 200)
	if got := decodeJSON[[]orchestrator.Deploy](t, resp); len(got) != 0 {
		t.Fatalf("expected no deploys, got %+v", got)
	}

	// Deploy the branch tip.
	resp = e.post(fmt.Sprintf("/api/sessions/%d/previews", sess.ID), nil)
	assertStatus(t, resp, 202)
	deploy := decodeJSON[orchestrator.Deploy](t, resp)
	if deploy.Ref != sess.BranchName || deploy.Repo != "demo-board" {
		t.Fatalf("unexpected deploy: %+v", deploy)
	}

	// Poll the session-scoped list until the deploy is ready.
	deadline := time.Now().Add(30 * time.Second)
	for {
		resp = e.get(fmt.Sprintf("/api/sessions/%d/previews", sess.ID))
		assertStatus(t, resp, 200)
		deploys := decodeJSON[[]orchestrator.Deploy](t, resp)
		if len(deploys) == 1 && deploys[0].Status == orchestrator.StatusReady {
			if deploys[0].PreviewURL == "" {
				t.Fatalf("ready deploy missing preview_url: %+v", deploys[0])
			}
			break
		}
		if len(deploys) == 1 && deploys[0].Status == orchestrator.StatusFailed {
			t.Fatalf("deploy failed: %s", deploys[0].Error)
		}
		if time.Now().After(deadline) {
			t.Fatalf("deploy never became ready: %+v", deploys)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Build logs are exposed.
	resp = e.get(fmt.Sprintf("/api/previews/%d/logs", deploy.ID))
	assertStatus(t, resp, 200)
	if body := string(readBody(t, resp)); !strings.Contains(body, "frontend build") || !strings.Contains(body, "backend build") {
		t.Fatalf("logs = %q", body)
	}

	resp = e.get("/api/previews/999/logs")
	assertStatus(t, resp, 404)
}

// TestSessionPreviewsViaKanbanToml: the manifest can live as a [previews]
// table inside .kanban.toml instead of a dedicated preview.toml.
func TestSessionPreviewsViaKanbanToml(t *testing.T) {
	e := newEnv(t)
	hosted := "[sync]\nallow_rebase = true\n" +
		strings.ReplaceAll(previewManifest, "\n[", "\n[previews.")
	files := map[string]string{
		".kanban.toml":   hosted,
		"web/index.html": "<html>via kanban toml</html>",
		"srv/main.txt":   "backend-ish",
	}
	for name, content := range files {
		p := filepath.Join(e.repoPath, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustGit(t, e.repoPath, "add", "-A")
	mustGit(t, e.repoPath, "commit", "-qm", "onboard via kanban toml")

	board := e.seedBoard("Toml Board")
	ticket := e.seedTicket(board, "Feature")
	sess := e.seedSession(ticket)
	mustGit(t, e.repoPath, "branch", sess.BranchName)

	resp := e.post(fmt.Sprintf("/api/sessions/%d/previews", sess.ID), nil)
	assertStatus(t, resp, 202)
	deploy := decodeJSON[orchestrator.Deploy](t, resp)

	deadline := time.Now().Add(30 * time.Second)
	for {
		got, err := e.previews.Deploy(deploy.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status == orchestrator.StatusReady {
			break
		}
		if got.Status == orchestrator.StatusFailed {
			logs, _ := e.previews.DeployLogs(deploy.ID)
			t.Fatalf("deploy failed: %s\n%s", got.Error, logs)
		}
		if time.Now().After(deadline) {
			t.Fatalf("deploy never became ready: %+v", got)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// seedUnonboardableRepo commits a deployable tree that carries no manifest
// at all — the case an out-of-repo manifest exists for (an upstream that
// won't take a preview.toml).
func seedUnonboardableRepo(t *testing.T, e *testEnv) {
	t.Helper()
	files := map[string]string{
		"web/index.html": "<html>out-of-repo manifest</html>",
		"srv/main.txt":   "backend-ish",
	}
	for name, content := range files {
		p := filepath.Join(e.repoPath, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustGit(t, e.repoPath, "add", "-A")
	mustGit(t, e.repoPath, "commit", "-qm", "no manifest in here")
}

// TestSessionPreviewsViaLocalManifest: a repo that can't carry a manifest
// upstream is onboarded by a server-side <board-slug>.toml instead.
func TestSessionPreviewsViaLocalManifest(t *testing.T) {
	e := newEnv(t)
	seedUnonboardableRepo(t, e)
	board := e.seedBoard("Vendored Board")
	ticket := e.seedTicket(board, "Feature")
	sess := e.seedSession(ticket)
	mustGit(t, e.repoPath, "branch", sess.BranchName)

	// Without the manifest the deploy fails with the orchestrator's
	// "is the repo onboarded?" error rather than building anything.
	resp := e.post(fmt.Sprintf("/api/sessions/%d/previews", sess.ID), nil)
	assertStatus(t, resp, 202)
	first := decodeJSON[orchestrator.Deploy](t, resp)
	if got := awaitTerminal(t, e, first.ID); got.Status != orchestrator.StatusFailed {
		t.Fatalf("expected failure without a manifest, got %+v", got)
	}

	// The manifest is named for the orchestrator repo name — the board slug.
	e.writeLocalManifest("vendored-board", previewManifest)

	mustGit(t, e.repoPath, "commit", "--allow-empty", "-qm", "next tip")
	mustGit(t, e.repoPath, "branch", "-f", sess.BranchName, "HEAD")
	resp = e.post(fmt.Sprintf("/api/sessions/%d/previews", sess.ID), nil)
	assertStatus(t, resp, 202)
	deploy := decodeJSON[orchestrator.Deploy](t, resp)
	if got := awaitTerminal(t, e, deploy.ID); got.Status != orchestrator.StatusReady {
		logs, _ := e.previews.DeployLogs(deploy.ID)
		t.Fatalf("deploy with local manifest: %+v\n%s", got, logs)
	}
}

// TestAutoDeployWithLocalManifest: the deploy-on-idle gate opens for a repo
// onboarded only by a server-side manifest — the worktree has nothing to
// probe, so a worktree-only gate would silently never deploy.
func TestAutoDeployWithLocalManifest(t *testing.T) {
	e := newEnv(t)
	seedUnonboardableRepo(t, e)
	board := e.seedBoard("Vendored Board")
	ticket := e.seedTicket(board, "Auto feature")
	sess := e.seedSession(ticket)
	mustGit(t, e.repoPath, "branch", sess.BranchName)
	if err := os.MkdirAll(sess.WorktreePath, 0o755); err != nil {
		t.Fatal(err)
	}

	// No manifest anywhere yet → idle must not deploy.
	resp := e.patch(fmt.Sprintf("/api/sessions/%d/status", sess.ID), map[string]string{"status": "idle"})
	assertStatus(t, resp, 204)
	readBody(t, resp)
	time.Sleep(300 * time.Millisecond)
	resp = e.get(fmt.Sprintf("/api/sessions/%d/previews", sess.ID))
	if got := decodeJSON[[]orchestrator.Deploy](t, resp); len(got) != 0 {
		t.Fatalf("deployed without any manifest: %+v", got)
	}

	e.writeLocalManifest("vendored-board", previewManifest)
	resp = e.patch(fmt.Sprintf("/api/sessions/%d/status", sess.ID), map[string]string{"status": "idle"})
	assertStatus(t, resp, 204)
	readBody(t, resp)

	deadline := time.Now().Add(30 * time.Second)
	for {
		resp = e.get(fmt.Sprintf("/api/sessions/%d/previews", sess.ID))
		deploys := decodeJSON[[]orchestrator.Deploy](t, resp)
		if len(deploys) == 1 && deploys[0].Status == orchestrator.StatusReady {
			break
		}
		if len(deploys) == 1 && deploys[0].Status == orchestrator.StatusFailed {
			logs, _ := e.previews.DeployLogs(deploys[0].ID)
			t.Fatalf("auto-deploy failed: %s\n%s", deploys[0].Error, logs)
		}
		if time.Now().After(deadline) {
			t.Fatalf("auto-deploy never became ready: %+v", deploys)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// awaitTerminal polls a deploy until it stops being queued/building.
func awaitTerminal(t *testing.T, e *testEnv, id int64) orchestrator.Deploy {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		got, err := e.previews.Deploy(id)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != orchestrator.StatusQueued && got.Status != orchestrator.StatusBuilding {
			return got
		}
		if time.Now().After(deadline) {
			t.Fatalf("deploy %d stuck in %s", id, got.Status)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestAutoDeployOnIdle: an agent reporting idle (finished a work burst)
// triggers a deploy of the branch tip — gated on the worktree carrying a
// preview manifest.
func TestAutoDeployOnIdle(t *testing.T) {
	e := newEnv(t)
	seedPreviewableRepo(t, e)
	board := e.seedBoard("Demo Board")
	ticket := e.seedTicket(board, "Auto feature")
	sess := e.seedSession(ticket)
	mustGit(t, e.repoPath, "branch", sess.BranchName)

	// The gate: no preview.toml in the worktree → idle must NOT deploy.
	if err := os.MkdirAll(sess.WorktreePath, 0o755); err != nil {
		t.Fatal(err)
	}
	resp := e.patch(fmt.Sprintf("/api/sessions/%d/status", sess.ID), map[string]string{"status": "idle"})
	assertStatus(t, resp, 204)
	readBody(t, resp)
	time.Sleep(300 * time.Millisecond)
	resp = e.get(fmt.Sprintf("/api/sessions/%d/previews", sess.ID))
	assertStatus(t, resp, 200)
	if got := decodeJSON[[]orchestrator.Deploy](t, resp); len(got) != 0 {
		t.Fatalf("deploy without preview.toml gate: %+v", got)
	}

	// With preview.toml present, idle deploys the tip.
	if err := os.WriteFile(filepath.Join(sess.WorktreePath, "preview.toml"), []byte(previewManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	resp = e.patch(fmt.Sprintf("/api/sessions/%d/status", sess.ID), map[string]string{"status": "idle"})
	assertStatus(t, resp, 204)
	readBody(t, resp)

	deadline := time.Now().Add(30 * time.Second)
	for {
		resp = e.get(fmt.Sprintf("/api/sessions/%d/previews", sess.ID))
		assertStatus(t, resp, 200)
		deploys := decodeJSON[[]orchestrator.Deploy](t, resp)
		if len(deploys) == 1 && deploys[0].Status == orchestrator.StatusReady {
			break
		}
		if len(deploys) == 1 && deploys[0].Status == orchestrator.StatusFailed {
			t.Fatalf("auto-deploy failed: %s", deploys[0].Error)
		}
		if time.Now().After(deadline) {
			t.Fatalf("auto-deploy never became ready: %+v", deploys)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Re-reporting idle with an unchanged tip is a no-op (idempotent per
	// commit — still exactly one deploy).
	resp = e.patch(fmt.Sprintf("/api/sessions/%d/status", sess.ID), map[string]string{"status": "idle"})
	assertStatus(t, resp, 204)
	readBody(t, resp)
	time.Sleep(300 * time.Millisecond)
	resp = e.get(fmt.Sprintf("/api/sessions/%d/previews", sess.ID))
	deploys := decodeJSON[[]orchestrator.Deploy](t, resp)
	if len(deploys) != 1 {
		t.Fatalf("idempotency: %d deploys", len(deploys))
	}

	// The env kill-switch works.
	t.Setenv("KANBAN_PREVIEW_AUTO_DEPLOY", "0")
	mustGit(t, e.repoPath, "commit", "--allow-empty", "-qm", "new tip")
	mustGit(t, e.repoPath, "branch", "-f", sess.BranchName, "HEAD")
	resp = e.patch(fmt.Sprintf("/api/sessions/%d/status", sess.ID), map[string]string{"status": "idle"})
	assertStatus(t, resp, 204)
	readBody(t, resp)
	time.Sleep(300 * time.Millisecond)
	resp = e.get(fmt.Sprintf("/api/sessions/%d/previews", sess.ID))
	if deploys := decodeJSON[[]orchestrator.Deploy](t, resp); len(deploys) != 1 {
		t.Fatalf("kill-switch ignored: %d deploys", len(deploys))
	}
}

// TestSessionPreviewArtifacts: a manifest declaring [artifacts.<name>]
// builds downloadable per-commit outputs alongside the preview, and each
// file is served by base name. Regression guard for the whole table being
// rejected as an unknown key (local-preview < v0.1.2), which failed every
// deploy of a repo that published a CLI.
func TestSessionPreviewArtifacts(t *testing.T) {
	e := newEnv(t)
	manifest := previewManifest + `
[artifacts.cli]
path  = "srv"
build = [["sh", "-c", "mkdir -p bin && printf 'binary-ish' > bin/tool-linux-amd64"]]
files = ["bin/tool-linux-amd64"]
`
	files := map[string]string{
		"preview.toml":   manifest,
		"web/index.html": "<html>with artifacts</html>",
		"srv/main.txt":   "backend-ish",
	}
	for name, content := range files {
		p := filepath.Join(e.repoPath, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustGit(t, e.repoPath, "add", "-A")
	mustGit(t, e.repoPath, "commit", "-qm", "publish a cli artifact")

	board := e.seedBoard("Artifact Board")
	ticket := e.seedTicket(board, "Ship a CLI")
	sess := e.seedSession(ticket)
	mustGit(t, e.repoPath, "branch", sess.BranchName)

	resp := e.post(fmt.Sprintf("/api/sessions/%d/previews", sess.ID), nil)
	assertStatus(t, resp, 202)
	deploy := decodeJSON[orchestrator.Deploy](t, resp)

	var ready orchestrator.Deploy
	deadline := time.Now().Add(30 * time.Second)
	for {
		got, err := e.previews.Deploy(deploy.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status == orchestrator.StatusReady {
			ready = got
			break
		}
		if got.Status == orchestrator.StatusFailed {
			logs, _ := e.previews.DeployLogs(deploy.ID)
			t.Fatalf("deploy failed: %s\n%s", got.Error, logs)
		}
		if time.Now().After(deadline) {
			t.Fatalf("deploy never became ready: %+v", got)
		}
		time.Sleep(50 * time.Millisecond)
	}

	if len(ready.Artifacts) != 1 || ready.Artifacts[0].Name != "cli" {
		t.Fatalf("expected one 'cli' artifact, got %+v", ready.Artifacts)
	}
	// Files are addressed by base name, not the manifest's relative path.
	if fs := ready.Artifacts[0].Files; len(fs) != 1 || fs[0].Name != "tool-linux-amd64" {
		t.Fatalf("unexpected artifact files: %+v", fs)
	}

	resp = e.get(fmt.Sprintf("/api/previews/%d/artifacts/cli/tool-linux-amd64", deploy.ID))
	assertStatus(t, resp, 200)
	if body := string(readBody(t, resp)); body != "binary-ish" {
		t.Fatalf("artifact body = %q", body)
	}

	// Unknown artifact names and files are 404s, not 500s.
	resp = e.get(fmt.Sprintf("/api/previews/%d/artifacts/nope/tool-linux-amd64", deploy.ID))
	assertStatus(t, resp, 404)
	readBody(t, resp)
	resp = e.get(fmt.Sprintf("/api/previews/%d/artifacts/cli/missing", deploy.ID))
	assertStatus(t, resp, 404)
	readBody(t, resp)

	// The artifact's build log is included in the deploy log snapshot.
	logs, err := e.previews.DeployLogs(deploy.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs, "artifacts.cli build") {
		t.Fatalf("artifact build log missing from snapshot:\n%s", logs)
	}
}

// TestDeleteBoardRemovesPreviewRepo: deleting a board unregisters its repo
// with the orchestrator, so deployments don't outlive the board that owns
// them.
func TestDeleteBoardRemovesPreviewRepo(t *testing.T) {
	e := newEnv(t)
	seedPreviewableRepo(t, e)
	board := e.seedBoard("Doomed Board")
	ticket := e.seedTicket(board, "Feature")
	sess := e.seedSession(ticket)
	mustGit(t, e.repoPath, "branch", sess.BranchName)

	resp := e.post(fmt.Sprintf("/api/sessions/%d/previews", sess.ID), nil)
	assertStatus(t, resp, 202)
	readBody(t, resp)

	repos, err := e.previews.Repos()
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || repos[0].Name != "doomed-board" {
		t.Fatalf("expected the board's repo registered, got %+v", repos)
	}

	resp = e.delete(fmt.Sprintf("/api/boards/%d", board.ID))
	assertStatus(t, resp, 204)
	readBody(t, resp)

	repos, err = e.previews.Repos()
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 0 {
		t.Fatalf("preview repo outlived its board: %+v", repos)
	}

	// A board that never deployed deletes cleanly too (no repo to remove).
	other := e.seedBoard("Never Deployed")
	resp = e.delete(fmt.Sprintf("/api/boards/%d", other.ID))
	assertStatus(t, resp, 204)
	readBody(t, resp)
}

func TestSessionPreviewsRequireRepoAndBranch(t *testing.T) {
	e := newEnv(t)

	// A board without a git repo can't deploy.
	noRepo := &db.Board{Name: "No Repo", Slug: "no-repo"}
	if err := e.store.CreateBoard(t.Context(), noRepo); err != nil {
		t.Fatal(err)
	}
	ticket := e.seedTicket(noRepo, "Task")
	sess := e.seedSession(ticket)

	resp := e.post(fmt.Sprintf("/api/sessions/%d/previews", sess.ID), nil)
	assertStatus(t, resp, 400)

	resp = e.get("/api/sessions/999/previews")
	assertStatus(t, resp, 404)
}

// TestDashboardPreviews: the cross-board feed lists every deploy joined to
// its owning board, and the board endpoint deploys an arbitrary ref.
func TestDashboardPreviews(t *testing.T) {
	e := newEnv(t)
	seedPreviewableRepo(t, e)
	board := e.seedBoard("Demo Board")

	// Empty before anything deploys — the dashboard renders on a fresh install.
	resp := e.get("/api/previews")
	assertStatus(t, resp, 200)
	if got := decodeJSON[[]map[string]any](t, resp); len(got) != 0 {
		t.Fatalf("expected no deploys, got %+v", got)
	}

	// No ref in the body → the board's base branch.
	resp = e.post(fmt.Sprintf("/api/boards/%d/previews", board.ID), map[string]string{})
	assertStatus(t, resp, 202)
	deploy := decodeJSON[orchestrator.Deploy](t, resp)
	if deploy.Ref != "main" || deploy.Repo != "demo-board" {
		t.Fatalf("unexpected deploy: %+v", deploy)
	}
	if d := awaitTerminal(t, e, deploy.ID); d.Status != orchestrator.StatusReady {
		logs, _ := e.previews.DeployLogs(deploy.ID)
		t.Fatalf("deploy failed: %s\n%s", d.Error, logs)
	}

	// An explicit ref deploys that instead. A second commit on a tag keeps
	// the two deploys distinct.
	if err := os.WriteFile(filepath.Join(e.repoPath, "web/index.html"), []byte("<html>v2</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, e.repoPath, "commit", "-qam", "v2")
	mustGit(t, e.repoPath, "tag", "v2")

	resp = e.post(fmt.Sprintf("/api/boards/%d/previews", board.ID), map[string]string{"ref": "v2"})
	assertStatus(t, resp, 202)
	tagged := decodeJSON[orchestrator.Deploy](t, resp)
	if tagged.Ref != "v2" {
		t.Fatalf("ref = %q, want v2", tagged.Ref)
	}

	// Both show up on the dashboard, each carrying its board.
	resp = e.get("/api/previews")
	assertStatus(t, resp, 200)
	rows := decodeJSON[[]struct {
		orchestrator.Deploy
		BoardID   int64  `json:"board_id"`
		BoardName string `json:"board_name"`
		BoardSlug string `json:"board_slug"`
	}](t, resp)
	if len(rows) != 2 {
		t.Fatalf("expected 2 deploys, got %d: %+v", len(rows), rows)
	}
	for _, row := range rows {
		if row.BoardID != board.ID || row.BoardName != "Demo Board" || row.BoardSlug != "demo-board" {
			t.Fatalf("deploy %d not joined to its board: %+v", row.ID, row)
		}
	}

	resp = e.post("/api/boards/999/previews", map[string]string{"ref": "main"})
	assertStatus(t, resp, 404)
}

// TestBoardPreviewWithoutRepo: a board with no linked git repo has nothing
// to deploy, and says so rather than registering an empty path.
func TestBoardPreviewWithoutRepo(t *testing.T) {
	e := newEnv(t)
	board := &db.Board{Name: "No Repo", Slug: "no-repo", BaseBranch: "main"}
	if err := e.store.CreateBoard(context.Background(), board); err != nil {
		t.Fatal(err)
	}

	resp := e.post(fmt.Sprintf("/api/boards/%d/previews", board.ID), map[string]string{"ref": "main"})
	assertStatus(t, resp, 400)
	readBody(t, resp)
}

// TestStopAndDeletePreview: the dashboard's per-row lifecycle controls. Stop
// leaves the deploy listed (it cold-starts again on the next request);
// delete removes it and reclaims what nothing else references.
func TestStopAndDeletePreview(t *testing.T) {
	e := newEnv(t)
	seedPreviewableRepo(t, e)
	board := e.seedBoard("Demo Board")

	resp := e.post(fmt.Sprintf("/api/boards/%d/previews", board.ID), map[string]string{})
	assertStatus(t, resp, 202)
	deploy := decodeJSON[orchestrator.Deploy](t, resp)
	if d := awaitTerminal(t, e, deploy.ID); d.Status != orchestrator.StatusReady {
		logs, _ := e.previews.DeployLogs(deploy.ID)
		t.Fatalf("deploy failed: %s\n%s", d.Error, logs)
	}

	// Stopping a deploy that isn't running is a no-op, not an error — the
	// UI only offers it while something is up, but the endpoint is idempotent.
	resp = e.post(fmt.Sprintf("/api/previews/%d/stop", deploy.ID), nil)
	assertStatus(t, resp, 204)
	if _, err := e.previews.Deploy(deploy.ID); err != nil {
		t.Fatalf("stopped deploy should still exist: %v", err)
	}

	resp = e.delete(fmt.Sprintf("/api/previews/%d", deploy.ID))
	assertStatus(t, resp, 204)
	if _, err := e.previews.Deploy(deploy.ID); !errors.Is(err, orchestrator.ErrNotFound) {
		t.Fatalf("deploy %d survived delete: %v", deploy.ID, err)
	}
	rows := decodeJSON[[]map[string]any](t, e.get("/api/previews"))
	if len(rows) != 0 {
		t.Fatalf("expected an empty dashboard after delete, got %+v", rows)
	}

	// Unknown ids are 404s, not silent successes.
	assertStatus(t, e.post("/api/previews/9999/stop", nil), 404)
	assertStatus(t, e.delete("/api/previews/9999"), 404)
}

func TestPreviewStorageAndRetention(t *testing.T) {
	e := newEnv(t)
	seedPreviewableRepo(t, e)
	board := e.seedBoard("Demo Board")

	resp := e.post(fmt.Sprintf("/api/boards/%d/previews", board.ID), map[string]string{})
	assertStatus(t, resp, 202)
	deploy := decodeJSON[orchestrator.Deploy](t, resp)
	if d := awaitTerminal(t, e, deploy.ID); d.Status != orchestrator.StatusReady {
		logs, _ := e.previews.DeployLogs(deploy.ID)
		t.Fatalf("deploy failed: %s\n%s", d.Error, logs)
	}

	// Storage resolves the orchestrator's repo back to the board that owns it.
	type storageRow struct {
		Repo           string `json:"repo"`
		TotalBytes     int64  `json:"total_bytes"`
		Deploys        int    `json:"deploys"`
		EvictedDeploys int    `json:"evicted_deploys"`
		BoardID        int64  `json:"board_id"`
		BoardName      string `json:"board_name"`
	}
	report := decodeJSON[struct {
		TotalBytes int64        `json:"total_bytes"`
		Repos      []storageRow `json:"repos"`
	}](t, e.get("/api/previews/storage"))
	if report.TotalBytes <= 0 {
		t.Fatalf("total_bytes = %d, want > 0 after a ready deploy", report.TotalBytes)
	}
	if len(report.Repos) != 1 {
		t.Fatalf("repos = %+v, want one entry", report.Repos)
	}
	row := report.Repos[0]
	if row.BoardID != board.ID || row.BoardName != board.Name {
		t.Fatalf("repo row not attributed to its board: %+v", row)
	}
	if row.Deploys != 1 || row.EvictedDeploys != 0 {
		t.Fatalf("deploy counts = %d active / %d evicted, want 1 / 0", row.Deploys, row.EvictedDeploys)
	}

	// Retention defaults to unlimited and round-trips.
	policy := decodeJSON[orchestrator.RetentionPolicy](t, e.get("/api/previews/retention"))
	if policy != (orchestrator.RetentionPolicy{}) {
		t.Fatalf("default policy = %+v, want unlimited", policy)
	}
	assertStatus(t, e.put("/api/previews/retention",
		orchestrator.RetentionPolicy{MaxDeploysPerRepo: -1}), 400)
	assertStatus(t, e.put("/api/previews/retention",
		orchestrator.RetentionPolicy{MaxDeploysPerRepo: 5, MaxAgeDays: 30}), 200)
	policy = decodeJSON[orchestrator.RetentionPolicy](t, e.get("/api/previews/retention"))
	if policy != (orchestrator.RetentionPolicy{MaxDeploysPerRepo: 5, MaxAgeDays: 30}) {
		t.Fatalf("policy after save = %+v", policy)
	}

	// A board's newest ready deploy is protected, so a sweep under this
	// policy evicts nothing and the preview stays listed.
	gc := decodeJSON[orchestrator.GCResult](t, e.post("/api/previews/gc", nil))
	if len(gc.Evicted) != 0 {
		t.Fatalf("evicted = %+v, want the sole ready deploy protected", gc.Evicted)
	}
	if gc.Policy != policy {
		t.Fatalf("gc policy = %+v, want %+v", gc.Policy, policy)
	}
	if _, err := e.previews.Deploy(deploy.ID); err != nil {
		t.Fatalf("deploy should survive the sweep: %v", err)
	}
}
