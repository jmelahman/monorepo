// Package gittest isolates a test binary's git subprocesses from the
// environment and config of whoever runs it.
package gittest

import (
	"errors"
	"os"
	"os/exec"
	"strings"
)

// IsolateEnv is for TestMain, before m.Run. It does two things.
//
// It drops the git environment a hook hands down, every variable git lists
// in `git rev-parse --local-env-vars`. Git gives its hooks GIT_INDEX_FILE: a
// plain commit's is the repo-relative ".git/index", which every git command
// would resolve against whichever temp repo it points at, and in a linked
// worktree ".git" is a file, not a directory. `git commit -a` makes it the
// absolute path of the real index.lock, so a temp repo's `git add` would write
// to the commit in progress.
//
// It also hides the developer's global and system gitconfig. A global
// core.hooksPath (a prek or pre-commit shim, say) would otherwise run on every
// test commit and fail outside a configured repo. A global commit.gpgsign
// or similar would do the same. Tests set any identity they need on the repo
// or with -c. See REGRESSIONS.md: "Tests inherit the caller's git environment
// and gitconfig".
//
// Hiding the gitconfig is not enough for ignore and attribute rules: with
// core.excludesFile unset git still reads $XDG_CONFIG_HOME/git/ignore (or
// ~/.config/git/ignore), and likewise git/attributes. A developer who ignores
// **/.claude/settings.local.json there leaves the session's agent settings
// invisible to `git add -A`, so both are pinned to the null device.
func IsolateEnv() {
	out, err := exec.Command("git", "rev-parse", "--local-env-vars").Output()
	if err != nil && !errors.Is(err, exec.ErrNotFound) {
		panic(err)
	}
	for k := range strings.FieldsSeq(string(out)) {
		os.Unsetenv(k)
	}
	os.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	os.Setenv("GIT_CONFIG_COUNT", "2")
	os.Setenv("GIT_CONFIG_KEY_0", "core.excludesFile")
	os.Setenv("GIT_CONFIG_VALUE_0", os.DevNull)
	os.Setenv("GIT_CONFIG_KEY_1", "core.attributesFile")
	os.Setenv("GIT_CONFIG_VALUE_1", os.DevNull)
}
