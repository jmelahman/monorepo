// Package dockertest gates the tests that drive the real Docker daemon. They
// create containers, networks and volumes on whatever daemon the host points
// at, and host state (a port in use, a leftover container) can fail them, so
// they run only when asked: set LOCAL_PREVIEW_DOCKER_TESTS=1, as the
// integration workflow does. Plain `go test ./...` and its pre-commit hook
// skip them.
package dockertest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmelahman/local-preview/internal/dockerapi"
)

// EnvVar opts in to the tests that use the real Docker daemon.
const EnvVar = "LOCAL_PREVIEW_DOCKER_TESTS"

// Connect returns a client for the host's daemon, or skips the test. It skips
// unless EnvVar is set, when no daemon is reachable, and when the daemon
// doesn't share this process's filesystem: CI job containers talk to the
// runner host's daemon, so bind mounts of probeDir would silently resolve on
// the wrong host.
func Connect(t *testing.T, probeDir string) *dockerapi.Client {
	t.Helper()
	if os.Getenv(EnvVar) == "" {
		t.Skipf("set %s=1 to run tests against the real Docker daemon", EnvVar)
	}
	ctx := context.Background()
	cli, err := dockerapi.Connect(ctx)
	if err != nil {
		t.Skipf("docker unavailable: %v", err)
	}
	marker := filepath.Join(probeDir, "marker")
	if err := os.WriteFile(marker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var sink strings.Builder
	if err := cli.RunStep(ctx, dockerapi.Step{
		Image: "busybox:1",
		Cmd:   []string{"sh", "-c", "test -f /probe/marker"},
		Binds: []string{probeDir + ":/probe"},
	}, &sink); err != nil {
		t.Skipf("daemon does not share this filesystem (sibling daemon?): %v", err)
	}
	return cli
}
