// Package gittest isolates a test binary's git subprocesses from the
// environment and config of whoever runs it.
package gittest

import "os"

// IsolateEnv is for TestMain, before m.Run. It does two things.
//
// It drops the git environment a hook hands down. Git gives its hooks
// GIT_INDEX_FILE and similar variables as *repo-relative* paths. So `go
// test` run from a pre-commit hook would have every git command resolve
// ".git/index" against whichever temp repo it points at, and in a linked
// worktree ".git" is a file, not a directory.
//
// It also hides the developer's global and system gitconfig. A global
// core.hooksPath (a prek or pre-commit shim, say) would otherwise run on every
// test commit and fail outside a configured repo. A global commit.gpgsign
// or similar would do the same. Tests set any identity they need on the repo
// or with -c. See REGRESSIONS.md: "Tests inherit the caller's git environment
// and gitconfig".
func IsolateEnv() {
	for _, k := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_OBJECT_DIRECTORY", "GIT_COMMON_DIR"} {
		os.Unsetenv(k)
	}
	os.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}
