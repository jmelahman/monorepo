package cli

import (
	"solod.dev/so/errors"
	"solod.dev/so/os"
)

var errNotRepo = errors.New("undot: not inside a git repository (no .git found)")

// chdirRepoRoot walks up from the current directory until it finds a .git
// entry (directory, or file for worktrees/submodules) and makes that
// directory the working directory. Every other path in the program is
// relative to the repo root.
func chdirRepoRoot() error {
	var wdBuf [os.MaxPathLen]byte
	for range 256 {
		if _, err := os.Lstat(".git"); err == nil {
			return nil
		}
		wd, err := os.Getwd(wdBuf[:])
		if err != nil || wd == "/" {
			return errNotRepo
		}
		if err2 := os.Chdir(".."); err2 != nil {
			return errNotRepo
		}
	}
	return errNotRepo
}
