package gitrepo

import (
	"os"
	"strings"
	"testing"
)

// TestMain drops the git environment the test process may have inherited.
// Git hands its hooks GIT_INDEX_FILE and friends as *repo-relative* paths, so
// `go test` run from a pre-commit hook would otherwise have every git command
// resolve ".git/index" against whichever temp repo it is pointed at — and in a
// linked worktree ".git" is a file, not a directory.
func TestMain(m *testing.M) {
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "GIT_") {
			_ = os.Unsetenv(key)
		}
	}
	_ = os.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	_ = os.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	os.Exit(m.Run())
}
