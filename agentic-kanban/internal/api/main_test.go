package api_test

import (
	"os"
	"testing"

	"github.com/jmelahman/kanban/internal/gittest"
)

// TestMain isolates git from the caller's environment and gitconfig; see
// gittest.IsolateEnv.
//
// It also points every docker client at a dead socket. newEnv builds a real
// local-preview orchestrator, whose New runs ReclaimOrphans: that
// force-removes every managed container and network on the daemon, killing
// local-preview's supervise container tests when prek runs both projects'
// go-test hooks concurrently. Sessions must not reach a daemon either, or
// they really spawn (and leak) containers. None of these tests need docker.
func TestMain(m *testing.M) {
	gittest.IsolateEnv()
	os.Setenv("DOCKER_HOST", "unix:///nonexistent/docker.sock")
	os.Exit(m.Run())
}
