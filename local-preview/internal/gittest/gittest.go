// Package gittest is the git plumbing the tests share. It imports nothing
// from this module, so any package's tests can use it.
package gittest

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// IsolateEnv is for TestMain. It drops the git environment the test process
// may have inherited: git hands its hooks GIT_INDEX_FILE and friends, so
// `go test` run from a pre-commit hook would otherwise point every git
// command at the wrong index. It also keeps the developer's gitconfig
// (signing, hooks, URL rewrites) out of the repositories the tests build.
func IsolateEnv() {
	for _, k := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "GIT_OBJECT_DIRECTORY", "GIT_COMMON_DIR"} {
		os.Unsetenv(k)
	}
	os.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

// Run runs git in dir as a fixed test identity and returns its trimmed
// output, failing the test on error.
func Run(t testing.TB, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
