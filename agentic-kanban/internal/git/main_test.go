package git

import (
	"os"
	"testing"

	"github.com/jmelahman/kanban/internal/sshsig"
)

// TestMain drops the git environment the test process may have inherited.
// Git hands its hooks GIT_INDEX_FILE and friends as *repo-relative* paths, so
// `go test` run from a pre-commit hook would otherwise have every git command
// resolve ".git/index" against whichever temp repo it is pointed at — and in a
// linked worktree ".git" is a file, not a directory.
//
// It also answers for kanban as gpg.ssh.program, as main.go does: with signing
// on, run() points git at os.Executable(), which under `go test` is this
// binary.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "-Y" {
		os.Exit(sshsig.Main(os.Args[1:]))
	}
	for _, k := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_OBJECT_DIRECTORY", "GIT_COMMON_DIR"} {
		os.Unsetenv(k)
	}
	os.Exit(m.Run())
}
