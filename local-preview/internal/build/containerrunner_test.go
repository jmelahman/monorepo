package build

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmelahman/local-preview/internal/devcontainer"
	"github.com/jmelahman/local-preview/internal/dockerapi"
)

func specFor(t *testing.T, image string, argv ...string) RunSpec {
	t.Helper()
	scratch := t.TempDir()
	if err := os.MkdirAll(filepath.Join(scratch, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	return RunSpec{
		RepoName:   "demo",
		SHA:        "deadbeef",
		ScratchDir: scratch,
		Dir:        "web",
		Argv:       argv,
		Image:      image,
	}
}

func TestAutoRunnerHostPath(t *testing.T) {
	r := &autoRunner{}
	spec := specFor(t, "", "sh", "-c", "echo host-built > out.txt")
	var out bytes.Buffer
	if err := r.Run(context.Background(), spec, &out); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(spec.ScratchDir, "web", "out.txt"))
	if err != nil || strings.TrimSpace(string(b)) != "host-built" {
		t.Fatalf("host output: %q, %v", b, err)
	}
}

func TestAutoRunnerFallsBackWithoutDocker(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///nonexistent/docker.sock")
	r := &autoRunner{}
	spec := specFor(t, "alpine:3.20", "sh", "-c", "echo fallback > out.txt")
	var out bytes.Buffer
	if err := r.Run(context.Background(), spec, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "warning: build image") {
		t.Fatalf("missing fallback warning: %q", out.String())
	}
	if b, _ := os.ReadFile(filepath.Join(spec.ScratchDir, "web", "out.txt")); strings.TrimSpace(string(b)) != "fallback" {
		t.Fatalf("fallback did not run on host: %q", b)
	}
}

// TestAutoRunnerContainer runs a real one-shot build container. Skipped when
// no daemon is reachable, or when the daemon doesn't share this process's
// filesystem (CI job containers talk to the runner host's daemon, so bind
// mounts resolve on the host and build outputs never appear here).
func TestAutoRunnerContainer(t *testing.T) {
	ctx := context.Background()
	cli, err := dockerapi.Connect(ctx)
	if err != nil {
		t.Skipf("docker unavailable: %v", err)
	}
	r := &autoRunner{}
	spec := specFor(t, "alpine:3.20", "sh", "-c", "pwd && echo containerized > out.txt")

	marker := filepath.Join(spec.ScratchDir, "marker")
	if err := os.WriteFile(marker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var probe bytes.Buffer
	if err := cli.RunStep(ctx, dockerapi.Step{
		Image: "alpine:3.20",
		Cmd:   []string{"sh", "-c", "test -f /probe/marker"},
		Binds: []string{spec.ScratchDir + ":/probe"},
	}, &probe); err != nil {
		t.Skipf("daemon does not share this filesystem (sibling daemon?): %v", err)
	}
	var out bytes.Buffer
	if err := r.Run(ctx, spec, &out); err != nil {
		t.Fatalf("Run: %v\noutput: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "/preview-build/web") {
		t.Fatalf("workdir missing from output: %q", out.String())
	}
	b, err := os.ReadFile(filepath.Join(spec.ScratchDir, "web", "out.txt"))
	if err != nil || strings.TrimSpace(string(b)) != "containerized" {
		t.Fatalf("container output on host: %q, %v", b, err)
	}

	// Failures propagate with output.
	out.Reset()
	err = r.Run(ctx, specFor(t, "alpine:3.20", "sh", "-c", "echo boom >&2; exit 7"), &out)
	if err == nil || !strings.Contains(err.Error(), "status 7") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(out.String(), "boom") {
		t.Fatalf("stderr missing: %q", out.String())
	}
}

// TestAutoRunnerDevcontainerFallsBackWithoutDocker: the devcontainer default
// is soft — no daemon means a host build with a warning, not a failure.
func TestAutoRunnerDevcontainerFallsBackWithoutDocker(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///nonexistent/docker.sock")
	r := &autoRunner{}
	spec := specFor(t, "", "sh", "-c", "echo devc-fallback > out.txt")
	spec.Devcontainer = devcontainer.Config{Image: "alpine:3.20", Home: "/home/dev"}
	var out bytes.Buffer
	if err := r.Run(context.Background(), spec, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "warning: devcontainer") {
		t.Fatalf("missing fallback warning: %q", out.String())
	}
	if b, _ := os.ReadFile(filepath.Join(spec.ScratchDir, "web", "out.txt")); strings.TrimSpace(string(b)) != "devc-fallback" {
		t.Fatalf("fallback did not run on host: %q", b)
	}
}

// TestAutoRunnerDevcontainer runs a step in the devcontainer default: its
// cache volume is mounted and HOME points at the remote-user home. Skipped
// without a filesystem-sharing daemon, like TestAutoRunnerContainer.
func TestAutoRunnerDevcontainer(t *testing.T) {
	ctx := context.Background()
	cli, err := dockerapi.Connect(ctx)
	if err != nil {
		t.Skipf("docker unavailable: %v", err)
	}
	r := &autoRunner{}
	spec := specFor(t, "", "sh", "-c", "echo home=$HOME && test -d /devc-cache && echo devc-built > out.txt")
	spec.Devcontainer = devcontainer.Config{
		Image:  "alpine:3.20",
		Mounts: []devcontainer.Mount{{Source: "local-preview-test-devc-build", Target: "/devc-cache"}},
		Home:   "/home/dev",
	}

	marker := filepath.Join(spec.ScratchDir, "marker")
	if err := os.WriteFile(marker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var probe bytes.Buffer
	if err := cli.RunStep(ctx, dockerapi.Step{
		Image: "alpine:3.20",
		Cmd:   []string{"sh", "-c", "test -f /probe/marker"},
		Binds: []string{spec.ScratchDir + ":/probe"},
	}, &probe); err != nil {
		t.Skipf("daemon does not share this filesystem (sibling daemon?): %v", err)
	}
	var out bytes.Buffer
	if err := r.Run(ctx, spec, &out); err != nil {
		t.Fatalf("Run: %v\noutput: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "[build image: alpine:3.20 (devcontainer)]") {
		t.Fatalf("missing devcontainer banner: %q", out.String())
	}
	if !strings.Contains(out.String(), "home=/home/dev") {
		t.Fatalf("HOME not set to the devcontainer home: %q", out.String())
	}
	if b, err := os.ReadFile(filepath.Join(spec.ScratchDir, "web", "out.txt")); err != nil || strings.TrimSpace(string(b)) != "devc-built" {
		t.Fatalf("container output on host: %q, %v", b, err)
	}

	// An explicit manifest image beats the devcontainer default.
	spec2 := specFor(t, "alpine:3.20", "sh", "-c", "true")
	spec2.Devcontainer = devcontainer.Config{Image: "example/never-pulled:1"}
	out.Reset()
	if err := r.Run(ctx, spec2, &out); err != nil {
		t.Fatalf("explicit-image run: %v\noutput: %s", err, out.String())
	}
	if strings.Contains(out.String(), "devcontainer") {
		t.Fatalf("explicit image should win over the devcontainer: %q", out.String())
	}
}
