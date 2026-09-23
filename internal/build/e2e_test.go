package build

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/gitrepo"
	"github.com/jmelahman/local-preview/internal/store"
	"github.com/jmelahman/local-preview/internal/supervise"
)

func runTestGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// newFixtureRepo copies testdata/fixture-repo into a temp dir and commits it.
func newFixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	copyTree(t, "testdata/fixture-repo", dir)
	runTestGit(t, dir, "init", "-q", "-b", "main")
	runTestGit(t, dir, "add", "-A")
	runTestGit(t, dir, "commit", "-qm", "initial")
	return dir
}

type env struct {
	q      *Queue
	db     *db.Store
	files  *store.Store
	super  *supervise.Manager
	git    *gitrepo.Manager
	repoID int64
}

func newEnv(t *testing.T, srcRepo string, configure ...func(*Queue)) *env {
	t.Helper()
	ctx := context.Background()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	root := t.TempDir()
	files := store.New(
		filepath.Join(root, "artifacts"),
		filepath.Join(root, "state"),
		filepath.Join(root, "tmp"),
	)
	gitMgr := gitrepo.NewManager(filepath.Join(root, "repos"))
	repo, err := gitMgr.Add(ctx, "demo", srcRepo, nil)
	if err != nil {
		t.Fatal(err)
	}
	dbRepo, err := database.CreateRepo("demo", srcRepo, repo.Path, db.RepoReady)
	if err != nil {
		t.Fatal(err)
	}
	super := supervise.New(database, files, filepath.Join(root, "logs"))
	t.Cleanup(super.StopAll)
	q := NewQueue(database, gitMgr, files, super, filepath.Join(root, "logs"), nil)
	for _, fn := range configure {
		fn(q)
	}
	qctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	q.Start(qctx, 2)
	return &env{q: q, db: database, files: files, super: super, git: gitMgr, repoID: dbRepo.ID}
}

// deployAndWait requests a deploy and polls until it reaches a terminal
// status.
func (e *env) deployAndWait(t *testing.T, ref string) db.DeployRow {
	t.Helper()
	row, err := e.q.RequestDeploy(context.Background(), "demo", ref, false, "")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(90 * time.Second)
	for {
		got, err := e.db.GetDeployByID(row.ID)
		if err != nil {
			t.Fatal(err)
		}
		switch got.Status {
		case db.DeployReady:
			return got
		case db.DeployFailed:
			log := readLogTail(t, got.FeBuildLogPath) + readLogTail(t, got.BeBuildLogPath)
			t.Fatalf("deploy failed: %s\n%s", got.Error, log)
		}
		if time.Now().After(deadline) {
			t.Fatalf("deploy did not finish; status=%s", got.Status)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// waitArtifact polls until the deploy's named artifact reaches a terminal
// build status — artifacts build on after the deploy turns ready — and
// returns its ref.
func (e *env) waitArtifact(t *testing.T, id int64, name string) db.ArtifactRef {
	t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for {
		row, err := e.db.GetDeployByID(id)
		if err != nil {
			t.Fatal(err)
		}
		ref, ok := row.Artifacts[name]
		if ok && (ref.Status == db.ArtifactReady || ref.Status == db.ArtifactFailed) {
			return ref
		}
		if time.Now().After(deadline) {
			t.Fatalf("artifact %s did not finish; ref=%+v", name, ref)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func readLogTail(t *testing.T, path string) string {
	t.Helper()
	if path == "" {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	s := string(b)
	if len(s) > 2000 {
		s = s[len(s)-2000:]
	}
	return s
}

func httpGet(t *testing.T, port int, path string) string {
	t.Helper()
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d%s", port, path))
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Fatalf("GET %s: %d %s", path, resp.StatusCode, b)
	}
	return string(b)
}

func TestVerticalSlice(t *testing.T) {
	src := newFixtureRepo(t)
	e := newEnv(t, src)
	ctx := context.Background()

	// Commit A, deployed by branch name.
	a := e.deployAndWait(t, "main")
	if a.FeHash == "" || a.BeHash == "" || a.Ref != "main" {
		t.Fatalf("deploy A: %+v", a)
	}
	if a.Branch != "main" || a.AuthorName != "test" || a.AuthorEmail != "test@example.com" {
		t.Fatalf("deploy A commit metadata: %+v", a)
	}
	if !e.files.HasFrontend("demo", a.FeHash) || !e.files.HasBackend("demo", a.BeHash) {
		t.Fatal("artifacts not published")
	}
	if b, err := os.ReadFile(filepath.Join(e.files.FrontendDir("demo", a.FeHash), "index.html")); err != nil || !strings.Contains(string(b), "fixture frontend v1") {
		t.Fatalf("frontend artifact content: %v %q", err, b)
	}

	portA, err := e.super.EnsureRunning(ctx, supervise.BackendKey(e.repoID, a.BeHash), "demo")
	if err != nil {
		t.Fatal(err)
	}
	if got := httpGet(t, portA, "/api/counter"); got != "1" {
		t.Fatalf("counter = %s, want 1", got)
	}

	// Commit B: frontend-only change → same backend hash, same process.
	if err := os.WriteFile(filepath.Join(src, "web", "src", "index.html"), []byte("<html>v2</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, src, "commit", "-qam", "fe change")
	shaB := runTestGit(t, src, "rev-parse", "HEAD")

	b := e.deployAndWait(t, shaB)
	if b.Ref != "" || b.Branch != "main" {
		t.Fatalf("sha deploy should derive its branch: %+v", b)
	}
	if b.BeHash != a.BeHash {
		t.Fatalf("frontend-only commit changed be_hash: %s → %s", a.BeHash, b.BeHash)
	}
	if b.FeHash == a.FeHash {
		t.Fatal("frontend change did not change fe_hash")
	}
	portB, err := e.super.EnsureRunning(ctx, supervise.BackendKey(e.repoID, b.BeHash), "demo")
	if err != nil {
		t.Fatal(err)
	}
	if portB != portA {
		t.Fatalf("frontend-only deploy restarted the backend (port %d → %d)", portA, portB)
	}

	// Commit C: backend-only change → new hash, new process, forked state.
	mainGo := filepath.Join(src, "backend", "main.go")
	code, err := os.ReadFile(mainGo)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mainGo, []byte(strings.Replace(string(code), `"ok"`, `"ok-v2"`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, src, "commit", "-qam", "be change")
	shaC := runTestGit(t, src, "rev-parse", "HEAD")

	c := e.deployAndWait(t, shaC)
	if c.FeHash != b.FeHash {
		t.Fatalf("backend-only commit changed fe_hash")
	}
	if c.BeHash == b.BeHash {
		t.Fatal("backend change did not change be_hash")
	}
	art, err := e.db.GetBackendArtifact(e.repoID, c.BeHash)
	if err != nil || art.ForkedFrom != a.BeHash {
		t.Fatalf("lineage: artifact = %+v, %v (want forked_from %s)", art, err, a.BeHash)
	}

	portC, err := e.super.EnsureRunning(ctx, supervise.BackendKey(e.repoID, c.BeHash), "demo")
	if err != nil {
		t.Fatal(err)
	}
	if got := httpGet(t, portC, "/api/counter"); got != "2" {
		t.Fatalf("forked counter = %s, want 2 (state continuity from commit A)", got)
	}
	if got := httpGet(t, portC, "/api/health"); got != "ok-v2" {
		t.Fatalf("health = %q, want new backend code", got)
	}

	// Redeploy of the same sha is idempotent (no rebuild).
	again, err := e.q.RequestDeploy(ctx, "demo", shaC, false, "")
	if err != nil || again.ID != c.ID || again.Status != db.DeployReady {
		t.Fatalf("redeploy: %+v, %v", again, err)
	}
}

func TestFailedBuildSurfacesInLog(t *testing.T) {
	src := newFixtureRepo(t)
	// Break the backend build.
	toml, err := os.ReadFile(filepath.Join(src, "preview.toml"))
	if err != nil {
		t.Fatal(err)
	}
	broken := strings.Replace(string(toml),
		`[["go", "build", "-o", "bin/fixture-server", "."]]`,
		`[["sh", "-c", "echo compile exploded >&2; exit 1"]]`, 1)
	if err := os.WriteFile(filepath.Join(src, "preview.toml"), []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, src, "commit", "-qam", "break build")

	e := newEnv(t, src)
	row, err := e.q.RequestDeploy(context.Background(), "demo", "main", false, "")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(60 * time.Second)
	var got db.DeployRow
	for {
		got, err = e.db.GetDeployByID(row.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status == db.DeployFailed {
			break
		}
		if got.Status == db.DeployReady || time.Now().After(deadline) {
			t.Fatalf("expected failure, status=%s", got.Status)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.Contains(got.Error, "backend build") {
		t.Fatalf("error = %q", got.Error)
	}
	logContent, err := os.ReadFile(got.BeBuildLogPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logContent), "compile exploded") {
		t.Fatalf("stderr missing from log: %s", logContent)
	}

	// A failed deploy can be re-requested (reset + attempt_count bump).
	again, err := e.q.RequestDeploy(context.Background(), "demo", got.SHA, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if again.AttemptCount != 1 {
		t.Fatalf("attempt_count = %d, want 1", again.AttemptCount)
	}
}

// TestProcessModeFrontendBuild covers the build half of frontend.run: the
// artifact is the whole built tree (not a dist), and a frontend_artifacts
// row records the run contract. Auto-start is off so the bogus run command
// is never attempted — the run half lives in the supervise tests.
func TestProcessModeFrontendBuild(t *testing.T) {
	src := newFixtureRepo(t)
	toml, err := os.ReadFile(filepath.Join(src, "preview.toml"))
	if err != nil {
		t.Fatal(err)
	}
	proc := strings.Replace(string(toml), `dist  = "dist"`,
		"run         = [\"./never-started\"]\nhealth_path = \"/\"", 1)
	if err := os.WriteFile(filepath.Join(src, "preview.toml"), []byte(proc), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, src, "commit", "-qam", "frontend as process")

	e := newEnv(t, src, func(q *Queue) { q.SetAutoStart(false) })
	d := e.deployAndWait(t, "main")

	// Whole tree published: sources present, not just a dist.
	if _, err := os.Stat(filepath.Join(e.files.FrontendDir("demo", d.FeHash), "src", "index.html")); err != nil {
		t.Fatalf("whole-tree artifact missing source file: %v", err)
	}
	art, err := e.db.GetFrontendArtifact(e.repoID, d.FeHash)
	if err != nil {
		t.Fatalf("frontend_artifacts row: %v", err)
	}
	if !strings.Contains(art.RunConfig, "never-started") {
		t.Fatalf("run_config = %s", art.RunConfig)
	}
}

const fixtureArtifactSection = `
[artifacts.cli]
path  = "backend"
build = [["sh", "-c", "mkdir -p bin && echo fixture-cli-v1 > bin/fixture-cli && chmod +x bin/fixture-cli"]]
files = ["bin/fixture-cli"]
`

// TestArtifactBuildAndReuse covers the downloadable-artifact side: the
// declared files are published flat under dl/<hash>, the deploy row records
// the name → hash mapping, and — like the other sides — an unrelated commit
// reuses the artifact while a partition change rebuilds it.
func TestArtifactBuildAndReuse(t *testing.T) {
	src := newFixtureRepo(t)
	f, err := os.OpenFile(filepath.Join(src, "preview.toml"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(fixtureArtifactSection); err != nil {
		t.Fatal(err)
	}
	f.Close()
	runTestGit(t, src, "commit", "-qam", "declare cli artifact")

	e := newEnv(t, src)
	a := e.deployAndWait(t, "main")
	ref, ok := a.Artifacts["cli"]
	if !ok || ref.Hash == "" || ref.LogPath == "" {
		t.Fatalf("deploy artifacts = %+v", a.Artifacts)
	}
	if ref = e.waitArtifact(t, a.ID, "cli"); ref.Status != db.ArtifactReady {
		t.Fatalf("artifact ref = %+v", ref)
	}
	if !e.files.HasArtifact("demo", ref.Hash) {
		t.Fatal("artifact not published")
	}
	bin := filepath.Join(e.files.ArtifactDir("demo", ref.Hash), "fixture-cli")
	st, err := os.Stat(bin)
	if err != nil {
		t.Fatalf("published file: %v", err)
	}
	if st.Mode().Perm()&0o111 == 0 {
		t.Fatalf("executable bit lost: %v", st.Mode())
	}
	if b, _ := os.ReadFile(bin); string(b) != "fixture-cli-v1\n" {
		t.Fatalf("published content = %q", b)
	}
	if got := e.files.ListArtifactFiles("demo", ref.Hash); len(got) != 1 || got[0].Name != "fixture-cli" {
		t.Fatalf("listed files = %+v", got)
	}

	// A frontend-only commit reuses the artifact hash.
	if err := os.WriteFile(filepath.Join(src, "web", "src", "index.html"), []byte("<html>v2</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, src, "commit", "-qam", "fe change")
	b := e.deployAndWait(t, runTestGit(t, src, "rev-parse", "HEAD"))
	if b.Artifacts["cli"].Hash != ref.Hash {
		t.Fatalf("frontend-only commit changed the artifact hash: %s → %s", ref.Hash, b.Artifacts["cli"].Hash)
	}
	// A cache hit is ready the moment its hash is known — no artifact phase.
	if b.Artifacts["cli"].Status != db.ArtifactReady {
		t.Fatalf("cached artifact ref = %+v", b.Artifacts["cli"])
	}

	// A commit inside the partition rebuilds under a new hash.
	if err := os.WriteFile(filepath.Join(src, "backend", "note.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, src, "add", "-A")
	runTestGit(t, src, "commit", "-qm", "be change")
	c := e.deployAndWait(t, runTestGit(t, src, "rev-parse", "HEAD"))
	if c.Artifacts["cli"].Hash == ref.Hash {
		t.Fatal("partition change did not change the artifact hash")
	}
	e.waitArtifact(t, c.ID, "cli")
	if !e.files.HasArtifact("demo", c.Artifacts["cli"].Hash) {
		t.Fatal("rebuilt artifact not published")
	}
}

// TestArtifactsBuildAfterReady: the frontend and backend gate readiness while
// downloadable artifacts build afterwards — a slow artifact never delays the
// preview. The artifact build blocks on a gate file the test only creates
// once it has observed the deploy ready with the artifact unbuilt.
func TestArtifactsBuildAfterReady(t *testing.T) {
	src := newFixtureRepo(t)
	gate := filepath.Join(t.TempDir(), "gate")
	section := fmt.Sprintf(`
[artifacts.cli]
path  = "backend"
build = [["sh", "-c", "until [ -f %s ]; do sleep 0.05; done; mkdir -p bin && echo gated > bin/fixture-cli"]]
files = ["bin/fixture-cli"]
`, gate)
	f, err := os.OpenFile(filepath.Join(src, "preview.toml"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(section); err != nil {
		t.Fatal(err)
	}
	f.Close()
	runTestGit(t, src, "commit", "-qam", "declare gated artifact")

	e := newEnv(t, src, func(q *Queue) { q.SetAutoStart(false) })
	d := e.deployAndWait(t, "main")

	// Ready arrived while the artifact build is still blocked: the sides are
	// published and servable, the artifact is not.
	if !e.files.HasFrontend("demo", d.FeHash) || !e.files.HasBackend("demo", d.BeHash) {
		t.Fatal("sides not published at ready")
	}
	ref := d.Artifacts["cli"]
	if ref.Status != db.ArtifactBuilding {
		t.Fatalf("artifact ref at ready = %+v, want status building", ref)
	}
	if e.files.HasArtifact("demo", ref.Hash) {
		t.Fatal("artifact published before its build was allowed to finish")
	}

	if err := os.WriteFile(gate, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got := e.waitArtifact(t, d.ID, "cli")
	if got.Status != db.ArtifactReady || got.Error != "" {
		t.Fatalf("artifact ref = %+v", got)
	}
	if !e.files.HasArtifact("demo", got.Hash) {
		t.Fatal("artifact not published after the gate opened")
	}
}

// TestArtifactBuildFailureLeavesDeployReady: declaring a file the build
// doesn't produce fails only that artifact — the error (naming the file)
// lands on its ref, nothing is published for it, and the deploy itself
// stays ready.
func TestArtifactBuildFailureLeavesDeployReady(t *testing.T) {
	src := newFixtureRepo(t)
	section := strings.Replace(fixtureArtifactSection,
		`files = ["bin/fixture-cli"]`, `files = ["bin/nope"]`, 1)
	f, err := os.OpenFile(filepath.Join(src, "preview.toml"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(section); err != nil {
		t.Fatal(err)
	}
	f.Close()
	runTestGit(t, src, "commit", "-qam", "declare bogus artifact file")

	e := newEnv(t, src)
	d := e.deployAndWait(t, "main")
	ref := e.waitArtifact(t, d.ID, "cli")
	if ref.Status != db.ArtifactFailed {
		t.Fatalf("artifact ref = %+v, want status failed", ref)
	}
	if !strings.Contains(ref.Error, "bin/nope") {
		t.Fatalf("artifact error = %q", ref.Error)
	}
	if e.files.HasArtifact("demo", ref.Hash) {
		t.Fatal("failed artifact build must not publish")
	}
	got, err := e.db.GetDeployByID(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != db.DeployReady || got.Error != "" {
		t.Fatalf("deploy = %s %q, want ready with no error", got.Status, got.Error)
	}
}

// removeCommittedManifest deletes preview.toml from the fixture repo and
// returns its content for re-hosting in another source.
func removeCommittedManifest(t *testing.T, src string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(src, "preview.toml"))
	if err != nil {
		t.Fatal(err)
	}
	runTestGit(t, src, "rm", "-q", "preview.toml")
	runTestGit(t, src, "commit", "-qm", "drop preview.toml")
	return content
}

func TestKanbanTableManifest(t *testing.T) {
	src := newFixtureRepo(t)
	content := removeCommittedManifest(t, src)
	kanban := strings.NewReplacer(
		"[frontend]", "[previews.frontend]",
		"[backend]", "[previews.backend]",
	).Replace(string(content))
	if err := os.WriteFile(filepath.Join(src, ".kanban.toml"), []byte(kanban), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, src, "add", ".kanban.toml")
	runTestGit(t, src, "commit", "-qm", "host manifest in .kanban.toml")

	e := newEnv(t, src, func(q *Queue) {
		q.SetManifestRefs([]ManifestRef{
			{Path: ManifestName},
			{Path: ".kanban.toml", Table: "previews"},
		})
	})
	d := e.deployAndWait(t, "main")
	if d.FeHash == "" || d.BeHash == "" {
		t.Fatalf("deploy from .kanban.toml [previews]: %+v", d)
	}
}

func TestLocalManifestFallback(t *testing.T) {
	src := newFixtureRepo(t)
	content := removeCommittedManifest(t, src)
	localDir := t.TempDir()

	e := newEnv(t, src, func(q *Queue) {
		q.SetManifestRefs([]ManifestRef{
			{Path: ManifestName},
			{Path: ".kanban.toml", Table: "previews"},
		})
		q.SetLocalManifestDir(localDir)
	})

	// No source anywhere yet: the deploy fails and the error names every
	// location tried, including the local path.
	row, err := e.q.RequestDeploy(context.Background(), "demo", "main", false, "")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(60 * time.Second)
	var got db.DeployRow
	for {
		got, err = e.db.GetDeployByID(row.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status == db.DeployFailed {
			break
		}
		if got.Status == db.DeployReady || time.Now().After(deadline) {
			t.Fatalf("expected failure, status=%s", got.Status)
		}
		time.Sleep(50 * time.Millisecond)
	}
	localPath := filepath.Join(localDir, "demo.toml")
	for _, want := range []string{"preview.toml", ".kanban.toml [previews]", localPath} {
		if !strings.Contains(got.Error, want) {
			t.Fatalf("error %q does not mention %s", got.Error, want)
		}
	}

	// Drop the manifest into the local dir and retry: the failed deploy
	// resets and builds from the out-of-repo manifest.
	if err := os.WriteFile(localPath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	d := e.deployAndWait(t, "main")
	if d.FeHash == "" || d.BeHash == "" {
		t.Fatalf("deploy from local manifest: %+v", d)
	}
}

// TestReadyDeployStartsAutomatically: a new deploy's backend comes up on
// its own right after the build turns ready — no request triggers it.
func TestReadyDeployStartsAutomatically(t *testing.T) {
	src := newFixtureRepo(t)
	e := newEnv(t, src)
	d := e.deployAndWait(t, "main")

	key := supervise.BackendKey(e.repoID, d.BeHash)
	deadline := time.Now().Add(30 * time.Second)
	for e.super.Status(key) != "running" {
		if time.Now().After(deadline) {
			t.Fatalf("backend did not auto-start; status=%q", e.super.Status(key))
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestRequestDeployUnknownRepo(t *testing.T) {
	src := newFixtureRepo(t)
	e := newEnv(t, src)
	_, err := e.q.RequestDeploy(context.Background(), "nope", "main", false, "")
	if !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// TestDevcontainerDefaultDiscovery: a commit carrying
// .devcontainer/devcontainer.json feeds both sides' hashes (the resolved
// config is part of their content address), stores the devcontainer as the
// backend's default runtime, and — docker being unreachable here — falls
// back to host builds with the reason in the build log. Opting out via the
// manifest restores the exact pre-devcontainer hashes: the top-level key
// lives outside the hashed sections.
func TestDevcontainerDefaultDiscovery(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///nonexistent/docker.sock")
	src := newFixtureRepo(t)
	e := newEnv(t, src, func(q *Queue) { q.SetAutoStart(false) })

	base := e.deployAndWait(t, "main")

	// Commit B: add a devcontainer. It lives outside both partitions, so
	// only the devcontainer machinery can move the hashes.
	if err := os.MkdirAll(filepath.Join(src, ".devcontainer"), 0o755); err != nil {
		t.Fatal(err)
	}
	devcJSON := `{
	  // JSONC on purpose
	  "image": "example/devc:1",
	  "mounts": ["source=devc-cache,target=/home/dev/.cache,type=volume",],
	  "remoteUser": "dev",
	}`
	if err := os.WriteFile(filepath.Join(src, ".devcontainer", "devcontainer.json"), []byte(devcJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, src, "add", "-A")
	runTestGit(t, src, "commit", "-qm", "add devcontainer")
	b := e.deployAndWait(t, runTestGit(t, src, "rev-parse", "HEAD"))
	if b.FeHash == base.FeHash || b.BeHash == base.BeHash {
		t.Fatalf("devcontainer did not feed the hashes: fe %s→%s be %s→%s",
			base.FeHash, b.FeHash, base.BeHash, b.BeHash)
	}
	beLog, err := os.ReadFile(b.BeBuildLogPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(beLog), `warning: devcontainer "example/devc:1"`) {
		t.Fatalf("backend log missing host-fallback warning: %s", beLog)
	}
	art, err := e.db.GetBackendArtifact(e.repoID, b.BeHash)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"devcontainer":{"image":"example/devc:1"`, `"home":"/home/dev"`, `"devc-cache"`} {
		if !strings.Contains(art.RunConfig, want) {
			t.Fatalf("run config missing %s: %s", want, art.RunConfig)
		}
	}

	// Commit C: opt out via the manifest. The sections are untouched, so
	// the hashes revert to the pre-devcontainer builds — full cache hit.
	f, err := os.OpenFile(filepath.Join(src, "preview.toml"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	toml, err := os.ReadFile(filepath.Join(src, "preview.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "preview.toml"), append([]byte("devcontainer = false\n"), toml...), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, src, "commit", "-qam", "opt out of devcontainer")
	c := e.deployAndWait(t, runTestGit(t, src, "rev-parse", "HEAD"))
	if c.FeHash != base.FeHash || c.BeHash != base.BeHash {
		t.Fatalf("opt-out did not restore hashes: fe %s vs %s, be %s vs %s",
			c.FeHash, base.FeHash, c.BeHash, base.BeHash)
	}
}

// TestDevcontainerUnusableNote: a devcontainer without an image (Dockerfile
// build) never fails the deploy; the sides that would have used it note the
// fallback in their build logs.
func TestDevcontainerUnusableNote(t *testing.T) {
	src := newFixtureRepo(t)
	if err := os.MkdirAll(filepath.Join(src, ".devcontainer"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, ".devcontainer", "devcontainer.json"),
		[]byte(`{"build": {"dockerfile": "Dockerfile"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, src, "add", "-A")
	runTestGit(t, src, "commit", "-qm", "dockerfile devcontainer")

	e := newEnv(t, src, func(q *Queue) { q.SetAutoStart(false) })
	d := e.deployAndWait(t, "main")
	beLog, err := os.ReadFile(d.BeBuildLogPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(beLog), "declares no usable image") {
		t.Fatalf("backend log missing unusable-devcontainer note: %s", beLog)
	}
}
