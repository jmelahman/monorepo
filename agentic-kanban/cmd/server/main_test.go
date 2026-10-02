package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmelahman/local-preview/orchestrator"

	"github.com/jmelahman/kanban/internal/config"
	"github.com/jmelahman/kanban/internal/gittest"
	"github.com/jmelahman/kanban/internal/previews"
)

// TestMain isolates git from the caller's environment and gitconfig; see
// gittest.IsolateEnv.
func TestMain(m *testing.M) {
	gittest.IsolateEnv()
	os.Exit(m.Run())
}

func TestPreviewBaseURLOverridesDomain(t *testing.T) {
	t.Setenv(previews.BaseURLEnv, "https://preview.example.com")
	t.Setenv("KANBAN_PREVIEW_DOMAIN", "legacy.example.net")
	t.Setenv(previews.ManifestDirEnv, t.TempDir())

	orch := newPreviewOrchestrator(&config.Config{DataDir: t.TempDir()}, ":7474", true, nil)
	if orch == nil {
		t.Fatal("preview orchestrator should accept the public base URL over the old domain")
	}
	defer orch.Close()

	handler := orch.WrapHost(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := io.WriteString(w, "kanban"); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	req := httptest.NewRequest("GET", "http://abc1234-demo.preview.example.com/", nil)
	req.Host = "abc1234-demo.preview.example.com"
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusNotFound || !strings.Contains(resp.Body.String(), "Unknown preview") {
		t.Fatalf("custom preview host was not routed: status=%d body=%q", resp.Code, resp.Body.String())
	}

	// Deploy links carry the public scheme and host, not the listen port.
	repo := t.TempDir()
	files := map[string]string{
		"preview.toml": `
[frontend]
path  = "web"
build = [["true"]]
dist  = "."

[backend]
path        = "srv"
build       = [["true"]]
run         = ["./never-started"]
health_path = "/health"
`,
		"web/index.html": "<html>hi</html>",
		"srv/main.txt":   "backend-ish",
	}
	for name, content := range files {
		p := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"add", "-A"},
		{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false", "commit", "-qm", "init"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	ctx := context.Background()
	if _, err := orch.RegisterRepo(ctx, "demo", repo); err != nil {
		t.Fatal(err)
	}
	d, err := orch.RequestDeploy(ctx, "demo", "main", false)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for d.Status != orchestrator.StatusReady {
		if d.Status == orchestrator.StatusFailed || time.Now().After(deadline) {
			t.Fatalf("deploy not ready: %+v", d)
		}
		time.Sleep(50 * time.Millisecond)
		if d, err = orch.Deploy(d.ID); err != nil {
			t.Fatal(err)
		}
	}
	if !strings.HasPrefix(d.PreviewURL, "https://") || !strings.Contains(d.PreviewURL, ".preview.example.com") ||
		strings.Contains(d.PreviewURL, ":7474") {
		t.Fatalf("preview_url = %q, want an https URL on preview.example.com without the listen port", d.PreviewURL)
	}
}
