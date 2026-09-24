package api

import (
	"os"
	"testing"
)

// TestMain points the API at a dead docker socket. Repo deletion runs
// PurgeRepoContainers, which force-removes every container labeled for the
// repo on the daemon — on a shared daemon that kills the supervise package's
// container tests (same "demo" repo) running concurrently. None of these
// tests need docker.
func TestMain(m *testing.M) {
	os.Setenv("DOCKER_HOST", "unix:///nonexistent/docker.sock")
	os.Exit(m.Run())
}
