package api

import (
	"os"
	"strings"
	"testing"
)

// TestMain points the API at a dead docker socket. Repo deletion runs
// PurgeRepoContainers, which force-removes every container labeled for the
// repo on the daemon — on a shared daemon that kills the supervise package's
// container tests (same "demo" repo) running concurrently. None of these
// tests need docker.
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
