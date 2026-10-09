// Package gittest keeps tests that run git away from the developer's
// configuration and from the environment of a hook that may be running them.
package gittest

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Isolate keeps the user's git configuration (signing, hooks, URL rewrites)
// out of the test.
func Isolate(t testing.TB) {
	// git exports these to hooks, e.g. GIT_INDEX_FILE to the pre-commit hook
	// that runs these tests, and they'd point every git command here at the
	// repository being committed to.
	out, err := exec.Command("git", "rev-parse", "--local-env-vars").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range strings.Fields(string(out)) {
		t.Setenv(k, "")
		if err := os.Unsetenv(k); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, k := range []string{"GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME"} {
		t.Setenv(k, "Test")
	}
	for _, k := range []string{"GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(k, "test@example.com")
	}
}
