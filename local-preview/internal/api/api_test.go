package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jmelahman/local-preview/internal/build"
	"github.com/jmelahman/local-preview/internal/clone"
	"github.com/jmelahman/local-preview/internal/config"
	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/gitrepo"
	"github.com/jmelahman/local-preview/internal/retain"
	"github.com/jmelahman/local-preview/internal/store"
	"github.com/jmelahman/local-preview/internal/supervise"
)

const fixtureManifest = `
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

// newSourceRepo builds a minimal deployable repo (trivial build commands).
func newSourceRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"preview.toml":   fixtureManifest,
		"web/index.html": "<html>hi</html>",
		"srv/main.txt":   "backend-ish",
	}
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runTestGit(t, dir, "init", "-q", "-b", "main")
	runTestGit(t, dir, "add", "-A")
	runTestGit(t, dir, "commit", "-qm", "initial")
	return dir
}

func newTestMux(t *testing.T) (*http.ServeMux, string) {
	t.Helper()
	mux, root, _ := newTestMuxSuper(t)
	return mux, root
}

// newTestMuxSuper is newTestMux plus the supervisor behind it, for tests
// that drive process state directly rather than through the proxy.
func newTestMuxSuper(t *testing.T) (*http.ServeMux, string, *supervise.Manager) {
	t.Helper()
	deps, root := newTestDeps(t)
	return NewMux(deps), root, deps.Super
}

// newTestDeps builds a fully wired Deps against in-memory/temp storage.
// Callers that need to tweak a field (e.g. UploadAuth) build the mux
// themselves with NewMux.
func newTestDeps(t *testing.T) (Deps, string) {
	t.Helper()
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
	super := supervise.New(database, files, filepath.Join(root, "logs"))
	t.Cleanup(super.StopAll)
	queue := build.NewQueue(database, gitMgr, files, super, filepath.Join(root, "logs"), nil)
	// These tests assert the cold path — never-started backends, empty run
	// logs, idle stats — so ready builds must not auto-start their processes.
	queue.SetAutoStart(false)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	queue.Start(ctx, 1)
	cloner := clone.New(database, gitMgr, nil)
	cloner.Start(ctx)

	return Deps{
		Store: database,
		Build: BuildInfo{Version: "test"},
		Config: config.Config{
			DataDir: root,
			Preview: config.PreviewBase{Domain: "preview.localhost", Scheme: "http", Port: "8080"},
		},
		Git:                 gitMgr,
		Queue:               queue,
		Super:               super,
		Cloner:              cloner,
		Files:               files,
		Sweeper:             retain.New(database, super, files),
		DBPath:              ":memory:",
		GitHubWebhookSecret: testWebhookSecret,
	}, root
}

func doJSON(t *testing.T, mux *http.ServeMux, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// repoResp captures the repo fields the registration tests assert on.
type repoResp struct {
	Status   string `json:"status"`
	Error    string `json:"error"`
	Progress string `json:"progress"`
	Watch    bool   `json:"watch"`
}

// waitRepoStatus polls the repo until its background clone reaches want
// (ready or failed).
func waitRepoStatus(t *testing.T, mux *http.ServeMux, name, want string) repoResp {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		rec := doJSON(t, mux, "GET", "/api/repos/"+name, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("get repo %s: %d %s", name, rec.Code, rec.Body)
		}
		var repo repoResp
		if err := json.Unmarshal(rec.Body.Bytes(), &repo); err != nil {
			t.Fatal(err)
		}
		if repo.Status == want {
			return repo
		}
		if repo.Status != "cloning" || time.Now().After(deadline) {
			t.Fatalf("repo %s status = %q (error %q), want %q", name, repo.Status, repo.Error, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// registerRepo registers a repo and waits out the background clone — the
// old synchronous flow, for tests that just need a deployable repo.
func registerRepo(t *testing.T, mux *http.ServeMux, name, src string) {
	t.Helper()
	body := `{"name":` + jsonQuote(name) + `,"source":` + jsonQuote(src) + `}`
	if rec := doJSON(t, mux, "POST", "/api/repos", body); rec.Code != http.StatusAccepted {
		t.Fatalf("create repo: %d %s", rec.Code, rec.Body)
	}
	waitRepoStatus(t, mux, name, "ready")
}

func TestHealth(t *testing.T) {
	mux, _ := newTestMux(t)
	rec := doJSON(t, mux, "GET", "/api/health", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["status"] != "ok" || resp["version"] != "test" {
		t.Fatalf("unexpected body: %v", resp)
	}
	// The dashboard reads the preview domain from here; without it the UI
	// falls back to guessing the default.
	if resp["preview_domain"] != "preview.localhost" {
		t.Fatalf("preview_domain = %q, want the configured domain", resp["preview_domain"])
	}
}

func TestRepoEndpoints(t *testing.T) {
	mux, _ := newTestMux(t)
	src := newSourceRepo(t)

	// Registration is asynchronous: the response is 202 with the row still
	// cloning; the repo turns ready in the background.
	rec := doJSON(t, mux, "POST", "/api/repos", `{"name":"demo","source":`+jsonQuote(src)+`}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create repo: %d %s", rec.Code, rec.Body)
	}
	var created repoResp
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Status != "cloning" {
		t.Fatalf("created status = %q, want cloning", created.Status)
	}
	waitRepoStatus(t, mux, "demo", "ready")

	for body, want := range map[string]int{
		`{"name":"demo","source":` + jsonQuote(src) + `}`: http.StatusConflict,
		`{"name":"Bad.Name","source":"/x"}`:               http.StatusBadRequest,
		`{`:                                               http.StatusBadRequest,
	} {
		if rec := doJSON(t, mux, "POST", "/api/repos", body); rec.Code != want {
			t.Errorf("POST %s: %d, want %d", body, rec.Code, want)
		}
	}

	rec = doJSON(t, mux, "GET", "/api/repos", "")
	var repos []db.Repo
	if err := json.Unmarshal(rec.Body.Bytes(), &repos); err != nil || len(repos) != 1 {
		t.Fatalf("list repos: %s (%v)", rec.Body, err)
	}
	if rec := doJSON(t, mux, "GET", "/api/repos/demo", ""); rec.Code != http.StatusOK {
		t.Fatalf("get repo: %d", rec.Code)
	}
	if rec := doJSON(t, mux, "GET", "/api/repos/nope", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("get missing repo: %d", rec.Code)
	}
}

// TestJSONBodySizeCap checks that a JSON handler rejects an oversized body
// with 413 before decoding, rather than allocating proportional to the input.
func TestJSONBodySizeCap(t *testing.T) {
	mux, _ := newTestMux(t)

	// A body just past maxJSONBody: valid JSON prefix followed by filler so
	// the decoder can't short-circuit before hitting the cap.
	big := `{"name":"demo","source":"` + strings.Repeat("a", (1<<20)+16) + `"}`
	if rec := doJSON(t, mux, "POST", "/api/repos", big); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized POST /api/repos: %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

// A source that can't be cloned surfaces asynchronously: the repo row turns
// failed with the clone error, deploys against it are refused, and deleting
// it frees the name for another attempt.
func TestRepoCloneFailure(t *testing.T) {
	mux, _ := newTestMux(t)

	bad := filepath.Join(t.TempDir(), "does-not-exist")
	rec := doJSON(t, mux, "POST", "/api/repos", `{"name":"demo","source":`+jsonQuote(bad)+`}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create repo: %d %s", rec.Code, rec.Body)
	}
	repo := waitRepoStatus(t, mux, "demo", "failed")
	if repo.Error == "" {
		t.Error("failed repo has no error message")
	}

	// Not ready → deploys are refused with a conflict, and the name stays
	// taken until the failed row is deleted.
	if rec := doJSON(t, mux, "POST", "/api/deploys", `{"repo":"demo","ref":"main"}`); rec.Code != http.StatusConflict {
		t.Fatalf("deploy on failed repo: %d, want 409", rec.Code)
	}
	if rec := doJSON(t, mux, "POST", "/api/repos", `{"name":"demo","source":`+jsonQuote(bad)+`}`); rec.Code != http.StatusConflict {
		t.Fatalf("re-create over failed repo: %d, want 409", rec.Code)
	}
	if rec := doJSON(t, mux, "DELETE", "/api/repos/demo", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete failed repo: %d %s", rec.Code, rec.Body)
	}
	registerRepo(t, mux, "demo", newSourceRepo(t))
}

// A deleted repo must be re-registrable even when its mirror clone survived
// deletion (cleanup is best-effort and can fail or race an in-flight fetch).
func TestRepoDeleteThenRecreate(t *testing.T) {
	mux, root := newTestMux(t)
	src := newSourceRepo(t)

	registerRepo(t, mux, "demo", src)
	if rec := doJSON(t, mux, "DELETE", "/api/repos/demo", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete repo: %d %s", rec.Code, rec.Body)
	}
	// Resurrect an orphaned mirror dir, as a fetch racing the delete would.
	if err := os.MkdirAll(filepath.Join(root, "repos", "demo.git", "refs"), 0o755); err != nil {
		t.Fatal(err)
	}
	registerRepo(t, mux, "demo", src)
}

func TestDeployEndpoints(t *testing.T) {
	mux, _ := newTestMux(t)
	src := newSourceRepo(t)
	registerRepo(t, mux, "demo", src)

	rec := doJSON(t, mux, "POST", "/api/deploys", `{"repo":"demo","ref":"main"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create deploy: %d %s", rec.Code, rec.Body)
	}
	var d struct {
		ID          int64  `json:"id"`
		Status      string `json:"status"`
		PreviewURL  string `json:"preview_url"`
		ShortSHA    string `json:"short_sha"`
		Branch      string `json:"branch"`
		AuthorName  string `json:"author_name"`
		AuthorEmail string `json:"author_email"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if d.Branch != "main" || d.AuthorName != "test" || d.AuthorEmail != "test@example.com" {
		t.Fatalf("commit metadata missing from response: %s", rec.Body)
	}

	deadline := time.Now().Add(30 * time.Second)
	for d.Status != "ready" {
		if d.Status == "failed" || time.Now().After(deadline) {
			logs := doJSON(t, mux, "GET", "/api/deploys/1/logs", "")
			t.Fatalf("deploy status = %s; logs:\n%s", d.Status, logs.Body)
		}
		time.Sleep(50 * time.Millisecond)
		rec = doJSON(t, mux, "GET", "/api/deploys/1", "")
		if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
			t.Fatal(err)
		}
	}
	wantURL := "http://" + d.ShortSHA + "-demo.preview.localhost:8080/"
	if d.PreviewURL != wantURL {
		t.Fatalf("preview_url = %q, want %q", d.PreviewURL, wantURL)
	}

	for query, want := range map[string]int{
		"?repo=demo":                         1,
		"?branch=main":                       1,
		"?author=test%40exam":                1,
		"?author=TEST":                       1,
		"?repo=demo&branch=main&author=test": 1,
		"?branch=other":                      0,
		"?author=someone-else":               0,
		"?status=ready":                      1,
		"?status=failed":                     0,
		"?q=" + d.ShortSHA:                   1,
		"?q=demo":                            1,
		"?q=MAIN":                            1,
		"?q=test%40example":                  1,
		"?q=" + d.ShortSHA + "&status=ready": 1,
		"?q=no-such-thing":                   0,
	} {
		rec = doJSON(t, mux, "GET", "/api/deploys"+query, "")
		var list []json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != want {
			t.Fatalf("list deploys %s: got %d (%v), want %d: %s", query, len(list), err, want, rec.Body)
		}
	}
	if rec := doJSON(t, mux, "GET", "/api/deploys?status=bogus", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("list deploys with unknown status: %d %s", rec.Code, rec.Body)
	}

	// X-Total-Count counts the filter's matches, not the page's rows: it is
	// what tells a pager another page exists.
	for _, tc := range []struct {
		query, total string
		rows         int
	}{
		{"", "1", 1},
		{"?limit=1", "1", 1},
		{"?offset=1", "1", 0},
		{"?status=failed", "0", 0},
	} {
		rec = doJSON(t, mux, "GET", "/api/deploys"+tc.query, "")
		var list []json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != tc.rows {
			t.Fatalf("list deploys %q: got %d rows (%v), want %d", tc.query, len(list), err, tc.rows)
		}
		if got := rec.Header().Get("X-Total-Count"); got != tc.total {
			t.Errorf("list deploys %q: X-Total-Count = %q, want %q", tc.query, got, tc.total)
		}
	}

	rec = doJSON(t, mux, "GET", "/api/deploys/1/logs", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "frontend build") {
		t.Fatalf("logs: %d %s", rec.Code, rec.Body)
	}

	if rec := doJSON(t, mux, "POST", "/api/deploys", `{"repo":"nope","ref":"main"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("deploy unknown repo: %d", rec.Code)
	}
	if rec := doJSON(t, mux, "POST", "/api/deploys", `{"repo":"demo","ref":"no-such-branch"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("deploy bad ref: %d", rec.Code)
	}
	if rec := doJSON(t, mux, "GET", "/api/deploys/99", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("get missing deploy: %d", rec.Code)
	}
}

func TestArtifactDownload(t *testing.T) {
	mux, _ := newTestMux(t)
	src := newSourceRepo(t)
	manifest := fixtureManifest + `
[artifacts.cli]
path  = "srv"
build = [["sh", "-c", "echo cli-binary > mycli && chmod +x mycli"]]
files = ["mycli"]
`
	if err := os.WriteFile(filepath.Join(src, "preview.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, src, "commit", "-qam", "add artifact")

	registerRepo(t, mux, "demo", src)
	if rec := doJSON(t, mux, "POST", "/api/deploys", `{"repo":"demo","ref":"main"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("create deploy: %d %s", rec.Code, rec.Body)
	}

	var d struct {
		Status    string `json:"status"`
		Artifacts []struct {
			Name   string `json:"name"`
			Hash   string `json:"hash"`
			Status string `json:"status"`
			Error  string `json:"error"`
			Files  []struct {
				Name string `json:"name"`
				Size int64  `json:"size"`
				URL  string `json:"url"`
			} `json:"files"`
		} `json:"artifacts"`
	}
	// Artifacts build after the deploy turns ready, so wait for both.
	deadline := time.Now().Add(30 * time.Second)
	for d.Status != "ready" || len(d.Artifacts) != 1 || d.Artifacts[0].Status == "building" {
		if d.Status == "failed" || time.Now().After(deadline) {
			logs := doJSON(t, mux, "GET", "/api/deploys/1/logs", "")
			t.Fatalf("deploy status = %s; logs:\n%s", d.Status, logs.Body)
		}
		time.Sleep(50 * time.Millisecond)
		rec := doJSON(t, mux, "GET", "/api/deploys/1", "")
		if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
			t.Fatal(err)
		}
	}
	if d.Artifacts[0].Status != "ready" || d.Artifacts[0].Error != "" {
		t.Fatalf("artifact = %+v", d.Artifacts[0])
	}

	if len(d.Artifacts) != 1 || d.Artifacts[0].Name != "cli" || d.Artifacts[0].Hash == "" {
		t.Fatalf("artifacts = %+v", d.Artifacts)
	}
	files := d.Artifacts[0].Files
	if len(files) != 1 || files[0].Name != "mycli" || files[0].Size == 0 {
		t.Fatalf("artifact files = %+v", files)
	}
	wantURL := "/api/deploys/1/artifacts/cli/mycli"
	if files[0].URL != wantURL {
		t.Fatalf("url = %q, want %q", files[0].URL, wantURL)
	}

	rec := doJSON(t, mux, "GET", wantURL, "")
	if rec.Code != http.StatusOK || rec.Body.String() != "cli-binary\n" {
		t.Fatalf("download: %d %q", rec.Code, rec.Body.String())
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, `attachment`) || !strings.Contains(cd, "mycli") {
		t.Fatalf("Content-Disposition = %q", cd)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/octet-stream" {
		t.Fatalf("Content-Type = %q", ct)
	}

	// The artifact build's log is part of the deploy's log snapshot.
	logs := doJSON(t, mux, "GET", "/api/deploys/1/logs", "")
	if !strings.Contains(logs.Body.String(), "artifacts.cli build") {
		t.Fatalf("logs missing artifact section:\n%s", logs.Body)
	}

	for _, path := range []string{
		"/api/deploys/1/artifacts/nope/mycli",         // unknown artifact
		"/api/deploys/1/artifacts/cli/nope",           // unknown file
		"/api/deploys/1/artifacts/cli/..%2Fmycli",     // encoded traversal
		"/api/deploys/1/artifacts/cli/%2E%2E%2Fmycli", // fully encoded traversal
		"/api/deploys/99/artifacts/cli/mycli",         // unknown deploy
	} {
		if rec := doJSON(t, mux, "GET", path, ""); rec.Code == http.StatusOK {
			t.Errorf("GET %s: %d, want an error", path, rec.Code)
		}
	}
}

// TestCrashedProcessSurfaces: a backend whose start attempt dies must not
// keep reading "idle" — the deploy reports "crashed" with the reason, both
// on the row and in its stats, and a stop clears it again.
func TestCrashedProcessSurfaces(t *testing.T) {
	mux, _, super := newTestMuxSuper(t)
	src := newSourceRepo(t)
	registerRepo(t, mux, "demo", src)
	if rec := doJSON(t, mux, "POST", "/api/deploys", `{"repo":"demo","ref":"main"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("create deploy: %d %s", rec.Code, rec.Body)
	}
	var d struct {
		Status       string `json:"status"`
		BeHash       string `json:"be_hash"`
		Process      string `json:"process"`
		ProcessError string `json:"process_error"`
	}
	deadline := time.Now().Add(30 * time.Second)
	for d.Status != "ready" {
		if d.Status == "failed" || time.Now().After(deadline) {
			t.Fatalf("deploy status = %s", d.Status)
		}
		time.Sleep(50 * time.Millisecond)
		rec := doJSON(t, mux, "GET", "/api/deploys/1", "")
		if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
			t.Fatal(err)
		}
	}
	if d.Process != "idle" {
		t.Fatalf("process before any start = %q, want idle", d.Process)
	}

	// The fixture manifest's run command doesn't exist, so the start fails
	// exactly like a backend that dies on boot.
	var repo struct {
		ID int64 `json:"id"`
	}
	rec := doJSON(t, mux, "GET", "/api/repos/demo", "")
	if err := json.Unmarshal(rec.Body.Bytes(), &repo); err != nil {
		t.Fatal(err)
	}
	key := supervise.BackendKey(repo.ID, d.BeHash)
	if _, err := super.EnsureRunning(context.Background(), key, "demo"); err == nil {
		t.Fatal("EnsureRunning succeeded; want a failed start")
	}

	rec = doJSON(t, mux, "GET", "/api/deploys/1", "")
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if d.Process != "crashed" || d.ProcessError == "" {
		t.Fatalf("process = %q, process_error = %q; want crashed with a reason", d.Process, d.ProcessError)
	}

	rec = doJSON(t, mux, "GET", "/api/deploys/1/stats", "")
	var stats struct {
		Backend struct {
			State string `json:"state"`
			Error string `json:"error"`
		} `json:"backend"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatal(err)
	}
	if stats.Backend.State != "crashed" || stats.Backend.Error != d.ProcessError {
		t.Fatalf("backend stats = %+v, want crashed with the same reason", stats.Backend)
	}

	// The crash is a runtime state, not a build outcome, so the listing has
	// to resolve it against the supervisor: the deploy answers to
	// status=crashed, and stays out of status=ready, which is what a user
	// filters by to see the previews that can still serve.
	if ids, total := listDeployIDs(t, mux, "crashed"); len(ids) != 1 || ids[0] != 1 || total != 1 {
		t.Fatalf("status=crashed listed %v (total %d), want deploy 1", ids, total)
	}
	if ids, total := listDeployIDs(t, mux, "ready"); len(ids) != 0 || total != 0 {
		t.Fatalf("status=ready listed %v (total %d), want nothing while it's crashed", ids, total)
	}

	// Stopping acknowledges the crash: nothing is running, and the deploy
	// reads idle again — with no leftover reason (decoded fresh, since an
	// empty process_error is omitted from the body).
	if rec := doJSON(t, mux, "POST", "/api/deploys/1/stop", ""); rec.Code != http.StatusOK {
		t.Fatalf("stop: %d %s", rec.Code, rec.Body)
	}
	var after struct {
		Process      string `json:"process"`
		ProcessError string `json:"process_error"`
	}
	rec = doJSON(t, mux, "GET", "/api/deploys/1", "")
	if err := json.Unmarshal(rec.Body.Bytes(), &after); err != nil {
		t.Fatal(err)
	}
	if after.Process != "idle" || after.ProcessError != "" {
		t.Fatalf("process after stop = %q (%q), want idle", after.Process, after.ProcessError)
	}
	// ...and the listing follows: no longer crashed, ready again.
	if ids, total := listDeployIDs(t, mux, "crashed"); len(ids) != 0 || total != 0 {
		t.Fatalf("status=crashed after stop listed %v (total %d), want nothing", ids, total)
	}
	if ids, total := listDeployIDs(t, mux, "ready"); len(ids) != 1 || ids[0] != 1 || total != 1 {
		t.Fatalf("status=ready after stop listed %v (total %d), want deploy 1", ids, total)
	}
}

// listDeployIDs returns the deploy ids a status filter matches, plus the
// X-Total-Count the pager reads — the two must agree about the filter.
func listDeployIDs(t *testing.T, mux *http.ServeMux, status string) ([]int64, int) {
	t.Helper()
	rec := doJSON(t, mux, "GET", "/api/deploys?status="+status, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list status=%s: %d %s", status, rec.Code, rec.Body)
	}
	var rows []struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	total, err := strconv.Atoi(rec.Header().Get("X-Total-Count"))
	if err != nil {
		t.Fatalf("X-Total-Count %q: %v", rec.Header().Get("X-Total-Count"), err)
	}
	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	return ids, total
}

// TestObservabilityEndpoints covers the run-log tail and stats endpoints
// against a ready (never-started) deploy: incremental offsets, restart
// (new-attempt) resets, side validation, and the idle stats shape.
func TestObservabilityEndpoints(t *testing.T) {
	mux, root := newTestMux(t)
	src := newSourceRepo(t)
	registerRepo(t, mux, "demo", src)
	if rec := doJSON(t, mux, "POST", "/api/deploys", `{"repo":"demo","ref":"main"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("create deploy: %d %s", rec.Code, rec.Body)
	}
	var d struct {
		Status string `json:"status"`
		BeHash string `json:"be_hash"`
	}
	deadline := time.Now().Add(30 * time.Second)
	for d.Status != "ready" {
		if d.Status == "failed" || time.Now().After(deadline) {
			t.Fatalf("deploy status = %s", d.Status)
		}
		time.Sleep(50 * time.Millisecond)
		rec := doJSON(t, mux, "GET", "/api/deploys/1", "")
		if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
			t.Fatal(err)
		}
	}

	// Stats: the backend was never started, so state only; the static
	// frontend has no process side.
	rec := doJSON(t, mux, "GET", "/api/deploys/1/stats", "")
	if rec.Code != 200 {
		t.Fatalf("stats: %d %s", rec.Code, rec.Body)
	}
	var stats struct {
		Backend *struct {
			State       string  `json:"state"`
			MemoryBytes *uint64 `json:"memory_bytes"`
		} `json:"backend"`
		Frontend json.RawMessage `json:"frontend"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatal(err)
	}
	if stats.Backend == nil || stats.Backend.State != "idle" || stats.Backend.MemoryBytes != nil {
		t.Fatalf("backend stats = %s", rec.Body)
	}
	if string(stats.Frontend) != "null" {
		t.Fatalf("frontend stats = %s, want null", stats.Frontend)
	}

	// Run log before any start: attempt 0, empty.
	var chunk struct {
		Side      string `json:"side"`
		Attempt   int    `json:"attempt"`
		Offset    int64  `json:"offset"`
		Content   string `json:"content"`
		Truncated bool   `json:"truncated"`
	}
	getChunk := func(query string) {
		t.Helper()
		rec := doJSON(t, mux, "GET", "/api/deploys/1/logs/run"+query, "")
		if rec.Code != 200 {
			t.Fatalf("run log %s: %d %s", query, rec.Code, rec.Body)
		}
		chunk = struct {
			Side      string `json:"side"`
			Attempt   int    `json:"attempt"`
			Offset    int64  `json:"offset"`
			Content   string `json:"content"`
			Truncated bool   `json:"truncated"`
		}{}
		if err := json.Unmarshal(rec.Body.Bytes(), &chunk); err != nil {
			t.Fatal(err)
		}
	}
	getChunk("")
	if chunk.Attempt != 0 || chunk.Content != "" {
		t.Fatalf("chunk before start = %+v", chunk)
	}

	// Simulate a start attempt by writing the file the supervisor would.
	dir := filepath.Join(root, "logs", "demo", "run", "be-"+d.BeHash)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "1.log")
	if err := os.WriteFile(logPath, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	getChunk("")
	if chunk.Attempt != 1 || chunk.Content != "hello\n" || chunk.Offset != 6 {
		t.Fatalf("first chunk = %+v", chunk)
	}

	// Appended bytes arrive incrementally from the echoed offset.
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("world\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	getChunk("?attempt=1&offset=6")
	if chunk.Content != "world\n" || chunk.Offset != 12 {
		t.Fatalf("incremental chunk = %+v", chunk)
	}
	getChunk("?attempt=1&offset=12")
	if chunk.Content != "" || chunk.Offset != 12 {
		t.Fatalf("caught-up chunk = %+v", chunk)
	}

	// A new attempt file resets the view to the new log.
	if err := os.WriteFile(filepath.Join(dir, "2.log"), []byte("restarted\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	getChunk("?attempt=1&offset=12")
	if chunk.Attempt != 2 || chunk.Content != "restarted\n" {
		t.Fatalf("restart chunk = %+v", chunk)
	}

	// Side validation: the fixture frontend is static.
	if rec := doJSON(t, mux, "GET", "/api/deploys/1/logs/run?side=fe", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("fe run log on static frontend: %d %s", rec.Code, rec.Body)
	}
	if rec := doJSON(t, mux, "GET", "/api/deploys/1/logs/run?side=nope", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("bogus side: %d", rec.Code)
	}
	if rec := doJSON(t, mux, "GET", "/api/deploys/99/logs/run", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("missing deploy run log: %d", rec.Code)
	}
	if rec := doJSON(t, mux, "GET", "/api/deploys/99/stats", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("missing deploy stats: %d", rec.Code)
	}
}

func TestDeleteRepo(t *testing.T) {
	mux, root := newTestMux(t)
	src := newSourceRepo(t)
	registerRepo(t, mux, "demo", src)
	// Build one deploy to ready so there are artifacts, state, and logs to
	// remove.
	if rec := doJSON(t, mux, "POST", "/api/deploys", `{"repo":"demo","ref":"main"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("create deploy: %d %s", rec.Code, rec.Body)
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		rec := doJSON(t, mux, "GET", "/api/deploys/1", "")
		var d struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
			t.Fatal(err)
		}
		if d.Status == "ready" {
			break
		}
		if d.Status == "failed" || time.Now().After(deadline) {
			t.Fatalf("deploy status = %s", d.Status)
		}
		time.Sleep(50 * time.Millisecond)
	}

	if rec := doJSON(t, mux, "DELETE", "/api/repos/demo", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete repo: %d %s", rec.Code, rec.Body)
	}
	if rec := doJSON(t, mux, "GET", "/api/repos/demo", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("get deleted repo: %d", rec.Code)
	}
	rec := doJSON(t, mux, "GET", "/api/deploys", "")
	var list []json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 0 {
		t.Fatalf("deploys after delete: %s (%v)", rec.Body, err)
	}
	for _, dir := range []string{
		filepath.Join(root, "artifacts", "demo"),
		filepath.Join(root, "state", "demo"),
		filepath.Join(root, "logs", "demo"),
		filepath.Join(root, "repos", "demo.git"),
	} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Errorf("%s still exists after delete (err=%v)", dir, err)
		}
	}
	if rec := doJSON(t, mux, "DELETE", "/api/repos/demo", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("delete missing repo: %d", rec.Code)
	}
	// The name is immediately reusable.
	registerRepo(t, mux, "demo", src)
}

// testDeploy captures the deploy fields the stop/delete tests assert on.
type testDeploy struct {
	ID       int64  `json:"id"`
	Status   string `json:"status"`
	FeHash   string `json:"fe_hash"`
	BeHash   string `json:"be_hash"`
	ShortSHA string `json:"short_sha"`
}

// deployAndWait deploys ref in the "demo" repo and blocks until it is ready.
func deployAndWait(t *testing.T, mux *http.ServeMux, ref string) testDeploy {
	t.Helper()
	rec := doJSON(t, mux, "POST", "/api/deploys", `{"repo":"demo","ref":`+jsonQuote(ref)+`}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create deploy %s: %d %s", ref, rec.Code, rec.Body)
	}
	var d testDeploy
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for d.Status != "ready" {
		if d.Status == "failed" || time.Now().After(deadline) {
			logs := doJSON(t, mux, "GET", fmt.Sprintf("/api/deploys/%d/logs", d.ID), "")
			t.Fatalf("deploy %d status = %s; logs:\n%s", d.ID, d.Status, logs.Body)
		}
		time.Sleep(50 * time.Millisecond)
		rec := doJSON(t, mux, "GET", fmt.Sprintf("/api/deploys/%d", d.ID), "")
		if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

func TestStopAndDeleteDeploy(t *testing.T) {
	mux, root := newTestMux(t)
	src := newSourceRepo(t)
	registerRepo(t, mux, "demo", src)
	d := deployAndWait(t, mux, "main")

	beDir := filepath.Join(root, "artifacts", "demo", "be", d.BeHash)
	feDir := filepath.Join(root, "artifacts", "demo", "fe", d.FeHash)
	stateDir := filepath.Join(root, "state", "demo", d.BeHash)

	// Stop quiesces processes but keeps the deploy row and its artifacts.
	if rec := doJSON(t, mux, "POST", fmt.Sprintf("/api/deploys/%d/stop", d.ID), ""); rec.Code != http.StatusOK {
		t.Fatalf("stop deploy: %d %s", rec.Code, rec.Body)
	}
	if rec := doJSON(t, mux, "GET", fmt.Sprintf("/api/deploys/%d", d.ID), ""); rec.Code != http.StatusOK {
		t.Fatalf("deploy gone after stop: %d", rec.Code)
	}
	for _, dir := range []string{beDir, feDir, stateDir} {
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("%s missing after stop (err=%v)", dir, err)
		}
	}

	// Delete removes the row and reclaims its now-orphaned artifacts/state.
	if rec := doJSON(t, mux, "DELETE", fmt.Sprintf("/api/deploys/%d", d.ID), ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete deploy: %d %s", rec.Code, rec.Body)
	}
	if rec := doJSON(t, mux, "GET", fmt.Sprintf("/api/deploys/%d", d.ID), ""); rec.Code != http.StatusNotFound {
		t.Fatalf("deploy still present after delete: %d", rec.Code)
	}
	rec := doJSON(t, mux, "GET", "/api/deploys", "")
	var list []json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 0 {
		t.Fatalf("deploys after delete: %s (%v)", rec.Body, err)
	}
	for _, dir := range []string{beDir, feDir, stateDir} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Errorf("%s still exists after delete (err=%v)", dir, err)
		}
	}

	// Both verbs 404 on a missing deploy.
	if rec := doJSON(t, mux, "POST", "/api/deploys/999/stop", ""); rec.Code != http.StatusNotFound {
		t.Errorf("stop missing deploy: %d", rec.Code)
	}
	if rec := doJSON(t, mux, "DELETE", "/api/deploys/999", ""); rec.Code != http.StatusNotFound {
		t.Errorf("delete missing deploy: %d", rec.Code)
	}
}

func TestDeleteDeployKeepsSharedArtifacts(t *testing.T) {
	mux, root := newTestMux(t)
	src := newSourceRepo(t)
	registerRepo(t, mux, "demo", src)
	first := deployAndWait(t, mux, "main")

	// A frontend-only change keeps the backend partition — and its hash —
	// identical, so the two deploys share a backend artifact and state dir.
	if err := os.WriteFile(filepath.Join(src, "web", "index.html"), []byte("<html>v2</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, src, "commit", "-qam", "frontend tweak")
	second := deployAndWait(t, mux, "main")

	if second.BeHash != first.BeHash {
		t.Fatalf("expected shared be_hash, got %q and %q", first.BeHash, second.BeHash)
	}
	if second.FeHash == first.FeHash {
		t.Fatalf("expected distinct fe_hash, both %q", first.FeHash)
	}

	// Deleting the second deploy drops its unique frontend artifact, but the
	// backend artifact and state shared with the first must survive.
	if rec := doJSON(t, mux, "DELETE", fmt.Sprintf("/api/deploys/%d", second.ID), ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete deploy: %d %s", rec.Code, rec.Body)
	}
	kept := []string{
		filepath.Join(root, "artifacts", "demo", "be", first.BeHash),
		filepath.Join(root, "state", "demo", first.BeHash),
		filepath.Join(root, "artifacts", "demo", "fe", first.FeHash),
	}
	for _, dir := range kept {
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("shared/kept %s removed (err=%v)", dir, err)
		}
	}
	orphanedFe := filepath.Join(root, "artifacts", "demo", "fe", second.FeHash)
	if _, err := os.Stat(orphanedFe); !os.IsNotExist(err) {
		t.Errorf("orphaned frontend %s still exists (err=%v)", orphanedFe, err)
	}
	if rec := doJSON(t, mux, "GET", fmt.Sprintf("/api/deploys/%d", first.ID), ""); rec.Code != http.StatusOK {
		t.Fatalf("surviving deploy gone: %d", rec.Code)
	}
}

func TestRepoWatchEndpoints(t *testing.T) {
	mux, _ := newTestMux(t)
	src := newSourceRepo(t)

	rec := doJSON(t, mux, "POST", "/api/repos",
		`{"name":"demo","source":`+jsonQuote(src)+`,"watch":true,"watch_branches":"main"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create watched repo: %d %s", rec.Code, rec.Body)
	}
	var repo struct {
		Watch         bool   `json:"watch"`
		WatchBranches string `json:"watch_branches"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &repo); err != nil {
		t.Fatal(err)
	}
	if !repo.Watch || repo.WatchBranches != "main" {
		t.Fatalf("created repo watch fields: %s", rec.Body)
	}
	// Let the background clone land so teardown doesn't race it.
	waitRepoStatus(t, mux, "demo", "ready")

	// Disable watching without touching the stored branch filter.
	rec = doJSON(t, mux, "PATCH", "/api/repos/demo", `{"watch":false}`)
	if err := json.Unmarshal(rec.Body.Bytes(), &repo); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("patch: %d %s (%v)", rec.Code, rec.Body, err)
	}
	if repo.Watch || repo.WatchBranches != "main" {
		t.Fatalf("watch off should keep branches: %s", rec.Body)
	}

	// Re-enable with a new filter; whitespace canonicalizes.
	rec = doJSON(t, mux, "PATCH", "/api/repos/demo", `{"watch":true,"watch_branches":" main , release/* "}`)
	if err := json.Unmarshal(rec.Body.Bytes(), &repo); err != nil {
		t.Fatal(err)
	}
	if !repo.Watch || repo.WatchBranches != "main,release/*" {
		t.Fatalf("re-enable: %s", rec.Body)
	}

	for body, want := range map[string]int{
		`{"watch_branches":"bad["}`: http.StatusBadRequest,
		`{}`:                        http.StatusBadRequest,
		`{bad json`:                 http.StatusBadRequest,
	} {
		if rec := doJSON(t, mux, "PATCH", "/api/repos/demo", body); rec.Code != want {
			t.Errorf("PATCH %s: %d, want %d", body, rec.Code, want)
		}
	}
	if rec := doJSON(t, mux, "PATCH", "/api/repos/nope", `{"watch":true}`); rec.Code != http.StatusNotFound {
		t.Fatalf("patch missing repo: %d", rec.Code)
	}
	if rec := doJSON(t, mux, "POST", "/api/repos",
		`{"name":"bad","source":"/x","watch_branches":"oops["}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("create with bad pattern: %d", rec.Code)
	}
}

func TestListDeploysLimitParam(t *testing.T) {
	mux, _ := newTestMux(t)
	for _, bad := range []string{"abc", "0", "-1", "1.5"} {
		if rec := doJSON(t, mux, "GET", "/api/deploys?limit="+bad, ""); rec.Code != http.StatusBadRequest {
			t.Errorf("limit=%s: %d, want 400", bad, rec.Code)
		}
	}
	for _, bad := range []string{"abc", "-1", "1.5"} {
		if rec := doJSON(t, mux, "GET", "/api/deploys?offset="+bad, ""); rec.Code != http.StatusBadRequest {
			t.Errorf("offset=%s: %d, want 400", bad, rec.Code)
		}
	}
	rec := doJSON(t, mux, "GET", "/api/deploys?limit=5&offset=0", "")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("limit=5 on empty instance: %d %s, want 200 []", rec.Code, rec.Body)
	}
}

func TestUnknownAPIPathIs404(t *testing.T) {
	mux, _ := newTestMux(t)
	rec := doJSON(t, mux, "GET", "/api/nope", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// jsonQuote JSON-quotes a string (paths may contain characters needing
// escaping).
func jsonQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
