package orchestrator

import (
	"os"
	"strings"
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
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "GIT_") {
			_ = os.Unsetenv(key)
		}
	}
	_ = os.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	_ = os.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	os.Setenv("DOCKER_HOST", "unix:///nonexistent/docker.sock")
	os.Exit(m.Run())
}
