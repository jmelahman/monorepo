package github

import (
	"os"
	"testing"
)

// TestMain drops the git environment the test process may have inherited:
// git hands its hooks GIT_INDEX_FILE and friends, so `go test` run from a
// pre-commit hook would otherwise point the fixture repo's git commands at
// the caller's repository.
func TestMain(m *testing.M) {
	for _, k := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_OBJECT_DIRECTORY", "GIT_COMMON_DIR"} {
		os.Unsetenv(k)
	}
	os.Exit(m.Run())
}
