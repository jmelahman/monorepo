package mcp

import (
	"os"
	"testing"
)

// TestMain drops the git environment the test process may have inherited.
// Git hands its hooks GIT_INDEX_FILE and friends as *repo-relative* paths, so
// `go test` run from a pre-commit hook would otherwise have every git command
// resolve ".git/index" against whichever temp repo it is pointed at — and in a
// linked worktree ".git" is a file, not a directory.
func TestMain(m *testing.M) {
	for _, k := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_OBJECT_DIRECTORY", "GIT_COMMON_DIR"} {
		os.Unsetenv(k)
	}
	os.Exit(m.Run())
}
