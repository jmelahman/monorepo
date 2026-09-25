package orchestrator

import (
	"os"
	"testing"
)

// TestMain points the orchestrator at a dead docker socket. New runs
// ReclaimOrphans, which force-removes every managed container and network on
// the daemon — on a shared daemon that kills the supervise package's
// container tests running concurrently. None of these tests need docker.
//
// It also drops the git environment the test process may have inherited: git
// hands its hooks GIT_INDEX_FILE and friends, so `go test` run from a
// pre-commit hook would otherwise point every git command at the wrong index.
func TestMain(m *testing.M) {
	for _, k := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_OBJECT_DIRECTORY", "GIT_COMMON_DIR"} {
		os.Unsetenv(k)
	}
	os.Setenv("DOCKER_HOST", "unix:///nonexistent/docker.sock")
	os.Exit(m.Run())
}
