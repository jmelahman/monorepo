package orchestrator

import (
	"os"
	"testing"

	"github.com/jmelahman/local-preview/internal/gittest"
)

// TestMain points the orchestrator at a dead docker socket. New runs
// ReclaimOrphans, which force-removes every managed container and network on
// the daemon — on a shared daemon that kills the supervise package's
// container tests running concurrently. None of these tests need docker.
//
// It also isolates the git environment; see gittest.IsolateEnv.
func TestMain(m *testing.M) {
	gittest.IsolateEnv()
	os.Setenv("DOCKER_HOST", "unix:///nonexistent/docker.sock")
	os.Exit(m.Run())
}
