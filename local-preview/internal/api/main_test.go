package api

import (
	"os"
	"testing"

	"github.com/jmelahman/local-preview/internal/gittest"
)

// TestMain points the API at a dead docker socket. Repo deletion runs
// PurgeRepoContainers, which force-removes every container labeled for the
// repo on the daemon — on a shared daemon that kills the supervise package's
// container tests (same "demo" repo) running concurrently. None of these
// tests need docker.
//
// It also isolates the git environment; see gittest.IsolateEnv.
func TestMain(m *testing.M) {
	gittest.IsolateEnv()
	os.Setenv("DOCKER_HOST", "unix:///nonexistent/docker.sock")
	os.Exit(m.Run())
}
