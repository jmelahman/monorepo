package api_test

import (
	"os"
	"testing"
)

// TestMain drops the git environment the test process may have inherited.
// Git hands its hooks GIT_INDEX_FILE and friends as *repo-relative* paths, so
// `go test` run from a pre-commit hook would otherwise have every git command
// in the server under test resolve ".git/index" against whichever temp repo it is
// pointed at — and in a linked worktree ".git" is a file, not a directory.
//
// It also points every docker client at a dead socket. newEnv builds a real
// local-preview orchestrator, whose New runs ReclaimOrphans: that
// force-removes every managed container and network on the daemon, killing
// local-preview's supervise container tests when prek runs both projects'
// go-test hooks concurrently. Sessions must not reach a daemon either, or
// they really spawn (and leak) containers. None of these tests need docker.
func TestMain(m *testing.M) {
	unsetGitEnv()
	os.Setenv("DOCKER_HOST", "unix:///nonexistent/docker.sock")
	os.Exit(m.Run())
}

func unsetGitEnv() {
	for _, k := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_OBJECT_DIRECTORY", "GIT_COMMON_DIR"} {
		os.Unsetenv(k)
	}
}
