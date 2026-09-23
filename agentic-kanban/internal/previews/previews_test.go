package previews

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmelahman/local-preview/orchestrator"

	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/docker"
)

func TestRepoName(t *testing.T) {
	cases := map[string]string{
		"demo-board":  "demo-board",
		"My Board":    "myboard", // invalid chars dropped
		"-lead-trail-": "lead-trail",
	}
	for slug, want := range cases {
		if got := RepoName(&db.Board{ID: 7, Slug: slug}); got != want {
			t.Errorf("RepoName(%q) = %q, want %q", slug, got, want)
		}
	}
	if got := RepoName(&db.Board{ID: 7, Slug: "!!!"}); got != "board-7" {
		t.Errorf("RepoName fallback = %q", got)
	}
}

func TestAutoDeployEnabled(t *testing.T) {
	t.Setenv(AutoDeployEnv, "")
	if !AutoDeployEnabled() {
		t.Fatal("default should be enabled")
	}
	t.Setenv(AutoDeployEnv, "0")
	if AutoDeployEnabled() {
		t.Fatal("0 should disable")
	}
	t.Setenv(AutoDeployEnv, "false")
	if AutoDeployEnabled() {
		t.Fatal("false should disable")
	}
}

// TestDockerRunner runs a real one-shot build container. Skipped when no
// Docker daemon is reachable (e.g. CI).
func TestDockerRunner(t *testing.T) {
	cli, err := docker.NewClient()
	if err != nil {
		t.Skipf("docker client: %v", err)
	}
	t.Cleanup(func() { cli.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if err := cli.Ping(ctx); err != nil {
		t.Skipf("docker daemon unavailable: %v", err)
	}

	scratch := t.TempDir()
	if err := os.MkdirAll(filepath.Join(scratch, "web"), 0o755); err != nil {
		t.Fatal(err)
	}

	r := NewDockerRunner(cli)
	var out bytes.Buffer
	// A manifest-declared image is honored over devcontainer discovery.
	err = r.Run(ctx, orchestrator.RunSpec{
		RepoName:   "runner-test",
		SHA:        "deadbeef",
		ScratchDir: scratch,
		Dir:        "web",
		Argv:       []string{"sh", "-c", "pwd && echo built > out.txt"},
		Image:      "alpine:3.20",
	}, &out)
	if err != nil {
		t.Fatalf("Run: %v\noutput: %s", err, out.String())
	}
	// The step ran in the mounted workdir and its output landed on the host.
	if !strings.Contains(out.String(), "/preview-build/web") {
		t.Fatalf("workdir missing from output: %q", out.String())
	}
	if !strings.Contains(out.String(), "alpine:3.20") {
		t.Fatalf("declared image not honored: %q", out.String())
	}
	content, err := os.ReadFile(filepath.Join(scratch, "web", "out.txt"))
	if err != nil || strings.TrimSpace(string(content)) != "built" {
		t.Fatalf("host file: %q, %v", content, err)
	}

	// Failure propagates with output.
	out.Reset()
	err = r.Run(ctx, orchestrator.RunSpec{
		RepoName:   "runner-test",
		ScratchDir: scratch,
		Dir:        ".",
		Argv:       []string{"sh", "-c", "echo boom >&2; exit 3"},
	}, &out)
	if err == nil || !strings.Contains(err.Error(), "status 3") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(out.String(), "boom") {
		t.Fatalf("stderr missing from output: %q", out.String())
	}
}
