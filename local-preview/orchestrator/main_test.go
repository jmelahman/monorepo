package orchestrator

import (
	"os"
	"testing"
)

// TestMain points the orchestrator at a dead docker socket. New runs
// ReclaimOrphans, which force-removes every managed container and network on
// the daemon — on a shared daemon that kills the supervise package's
// container tests running concurrently. None of these tests need docker.
func TestMain(m *testing.M) {
	os.Setenv("DOCKER_HOST", "unix:///nonexistent/docker.sock")
	os.Exit(m.Run())
}
