package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jmelahman/kanban/internal/config"
	"github.com/jmelahman/kanban/internal/previews"
)

// TestMain drops the git environment the test process may have inherited.
// Git hands its hooks GIT_INDEX_FILE and friends as *repo-relative* paths, so
// `go test` run from a pre-commit hook would otherwise have every git command
// in these tests resolve ".git/index" against whichever temp repo it is
// pointed at — and in a linked worktree ".git" is a file, not a directory.
func TestMain(m *testing.M) {
	unsetGitEnv()
	os.Exit(m.Run())
}

func unsetGitEnv() {
	for _, k := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_OBJECT_DIRECTORY", "GIT_COMMON_DIR"} {
		os.Unsetenv(k)
	}
}

func TestPreviewBaseURLOverridesDomain(t *testing.T) {
	t.Setenv(previews.BaseURLEnv, "https://preview.example.com")
	t.Setenv("KANBAN_PREVIEW_DOMAIN", "legacy.example.net")

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
}
