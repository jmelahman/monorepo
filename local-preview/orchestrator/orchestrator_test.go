package orchestrator

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jmelahman/local-preview/internal/db"
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

func newSourceRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"preview.toml":   fixtureManifest,
		"web/index.html": "<html>embedded preview</html>",
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

// recordingRunner delegates to the default host behavior while recording
// every spec it sees.
type recordingRunner struct {
	mu    sync.Mutex
	specs []RunSpec
}

func (r *recordingRunner) Run(ctx context.Context, spec RunSpec, output io.Writer) error {
	r.mu.Lock()
	r.specs = append(r.specs, spec)
	r.mu.Unlock()
	cmd := exec.CommandContext(ctx, spec.Argv[0], spec.Argv[1:]...)
	cmd.Dir = filepath.Join(spec.ScratchDir, spec.Dir)
	cmd.Stdout = output
	cmd.Stderr = output
	return cmd.Run()
}

func waitReady(t *testing.T, o *Orchestrator, id int64) Deploy {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		d, err := o.Deploy(id)
		if err != nil {
			t.Fatal(err)
		}
		switch d.Status {
		case StatusReady:
			return d
		case StatusFailed:
			logs, _ := o.DeployLogs(id)
			t.Fatalf("deploy failed: %s\n%s", d.Error, logs)
		}
		if time.Now().After(deadline) {
			t.Fatalf("deploy did not finish; status=%s", d.Status)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func TestEmbeddedLifecycle(t *testing.T) {
	src := newSourceRepo(t)
	runner := &recordingRunner{}
	o, err := New(Options{
		DataDir: filepath.Join(t.TempDir(), "previews"),
		Addr:    ":8080",
		Runner:  runner,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { o.Close() })
	ctx := context.Background()

	// Register: idempotent on same source, conflict on different source.
	repo, err := o.RegisterRepo(ctx, "demo", src)
	if err != nil {
		t.Fatal(err)
	}
	again, err := o.RegisterRepo(ctx, "demo", src)
	if err != nil || again.ID != repo.ID {
		t.Fatalf("re-register: %+v, %v", again, err)
	}
	if _, err := o.RegisterRepo(ctx, "demo", "/elsewhere"); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflicting source err = %v, want ErrConflict", err)
	}
	if _, err := o.RegisterRepo(ctx, "Bad.Name", src); err == nil {
		t.Fatal("invalid name should be rejected")
	}
	repos, err := o.Repos()
	if err != nil || len(repos) != 1 {
		t.Fatalf("Repos = %+v, %v", repos, err)
	}

	// Deploy by branch name.
	d, err := o.RequestDeploy(ctx, "demo", "main", false)
	if err != nil {
		t.Fatal(err)
	}
	d = waitReady(t, o, d.ID)
	wantURL := "http://" + d.ShortSHA + "-demo.preview.localhost:8080/"
	if d.PreviewURL != wantURL {
		t.Fatalf("PreviewURL = %q, want %q", d.PreviewURL, wantURL)
	}
	if d.Ref != "main" {
		t.Fatalf("Ref = %q", d.Ref)
	}

	// The injected runner saw both sides' steps with repo context.
	runner.mu.Lock()
	specs := append([]RunSpec(nil), runner.specs...)
	runner.mu.Unlock()
	if len(specs) != 2 {
		t.Fatalf("runner saw %d specs, want 2: %+v", len(specs), specs)
	}
	dirs := map[string]bool{}
	for _, s := range specs {
		if s.RepoName != "demo" || s.SHA != d.SHA || s.ScratchDir == "" {
			t.Fatalf("bad spec: %+v", s)
		}
		dirs[s.Dir] = true
	}
	if !dirs["web"] || !dirs["srv"] {
		t.Fatalf("runner dirs = %v", dirs)
	}

	// Listing and logs.
	list, err := o.Deploys("demo")
	if err != nil || len(list) != 1 || list[0].ID != d.ID {
		t.Fatalf("Deploys = %+v, %v", list, err)
	}
	logs, err := o.DeployLogs(d.ID)
	if err != nil || !strings.Contains(logs, "frontend build") {
		t.Fatalf("DeployLogs = %q, %v", logs, err)
	}
	if _, err := o.Deploy(999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing deploy err = %v", err)
	}
	if _, err := o.RequestDeploy(ctx, "nope", "main", false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown repo err = %v", err)
	}

	// WrapHost: preview subdomains are served, everything else falls
	// through to the embedding application.
	fallback := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "host-app")
	})
	handler := o.WrapHost(fallback)

	req := httptest.NewRequest("GET", "http://x/", nil)
	req.Host = d.ShortSHA + "-demo.preview.localhost:8080"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "embedded preview") {
		t.Fatalf("preview host: %d %q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest("GET", "http://x/anything", nil)
	req.Host = "localhost:8080"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Body.String() != "host-app" {
		t.Fatalf("fallback host: %q", rec.Body.String())
	}
}

func TestNewValidation(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Fatal("missing DataDir should error")
	}
	// Two different answers to "what host are previews on?".
	_, err := New(Options{
		DataDir:        t.TempDir(),
		DBPath:         ":memory:",
		PreviewDomain:  "preview.example.com",
		PreviewBaseURL: "https://previews.other.com",
	})
	if err == nil {
		t.Fatal("conflicting PreviewDomain and PreviewBaseURL should error")
	}
}

// PreviewBaseURL replaces the scheme and port that Addr implies — the case
// of an embedding server behind a TLS-terminating proxy.
func TestPreviewBaseURLOption(t *testing.T) {
	o, err := New(Options{
		DataDir:           filepath.Join(t.TempDir(), "previews"),
		DBPath:            ":memory:",
		Addr:              ":8080",
		PreviewBaseURL:    "https://preview.example.com",
		PollInterval:      -1,
		RetentionInterval: -1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { o.Close() })

	got := o.previewURL(db.DeployRow{Deploy: db.Deploy{ShortSHA: "abc1234"}, RepoName: "demo"})
	if want := "https://abc1234-demo.preview.example.com/"; got != want {
		t.Errorf("previewURL = %q, want %q", got, want)
	}
	// The same host must route: WrapHost matches on the derived domain.
	if o.opts.PreviewDomain != "preview.example.com" {
		t.Errorf("PreviewDomain = %q, want the base URL's host", o.opts.PreviewDomain)
	}
}
