package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmelahman/git-orchard/git"
)

func isolateGit(t *testing.T) {
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

func run(t *testing.T, repo git.Repo, args ...string) string {
	t.Helper()
	out, err := repo.Output(args...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newWorktree makes a repository with a commit, an origin remote and a
// hook, and a worktree of it on branch feature.
func newWorktree(t *testing.T) (main, wt git.Repo) {
	t.Helper()
	isolateGit(t)
	dir := t.TempDir()
	main = git.Repo{Dir: filepath.Join(dir, "main")}
	wt = git.Repo{Dir: filepath.Join(dir, "feature")}
	run(t, git.Repo{Dir: dir}, "init", "--quiet", "--initial-branch=master", main.Dir)
	write(t, filepath.Join(main.Dir, "README"), "main\n")
	run(t, main, "add", "README")
	run(t, main, "commit", "--quiet", "-m", "Start")
	run(t, main, "remote", "add", "origin", "git@example.com:owner/repo.git")
	run(t, main, "config", "branch.master.remote", "origin")
	write(t, filepath.Join(main.Dir, ".git", "hooks", "pre-commit"), "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(main.Dir, ".git", "hooks", "pre-commit"), 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, main, "worktree", "add", "--quiet", "-b", "feature", wt.Dir)
	return main, wt
}

func TestDetach(t *testing.T) {
	main, wt := newWorktree(t)
	write(t, filepath.Join(wt.Dir, "staged"), "staged\n")
	run(t, wt, "add", "staged")
	write(t, filepath.Join(wt.Dir, "README"), "changed\n")
	status := run(t, wt, "status", "--porcelain")

	top, err := Detach(wt.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if top != wt.Dir {
		t.Errorf("top = %q, want %q", top, wt.Dir)
	}

	if info, err := os.Stat(filepath.Join(wt.Dir, ".git")); err != nil || !info.IsDir() {
		t.Fatalf(".git isn't a directory: %v", err)
	}
	if got := run(t, wt, "rev-parse", "--git-common-dir"); got != ".git" {
		t.Errorf("common dir = %q, want .git", got)
	}
	if got := run(t, wt, "status", "--porcelain"); got != status {
		t.Errorf("status = %q, want %q", got, status)
	}
	if got := run(t, wt, "symbolic-ref", "HEAD"); got != "refs/heads/feature" {
		t.Errorf("HEAD = %q", got)
	}
	if got := run(t, wt, "rev-parse", "master"); got != run(t, main, "rev-parse", "master") {
		t.Errorf("master = %q", got)
	}
	if got := run(t, wt, "remote", "get-url", "origin"); got != "git@example.com:owner/repo.git" {
		t.Errorf("origin = %q", got)
	}
	if got := run(t, wt, "config", "branch.master.remote"); got != "origin" {
		t.Errorf("branch.master.remote = %q", got)
	}
	if got := run(t, wt, "config", "core.bare"); got != "false" {
		t.Errorf("core.bare = %q", got)
	}
	if info, err := os.Stat(filepath.Join(wt.Dir, ".git", "hooks", "pre-commit")); err != nil || info.Mode().Perm()&0o100 == 0 {
		t.Errorf("pre-commit hook not copied: %v", err)
	}
	if list := run(t, main, "worktree", "list"); strings.Contains(list, wt.Dir) {
		t.Errorf("still a worktree of main:\n%s", list)
	}
	entries, err := os.ReadDir(wt.Dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".git-detach-") {
			t.Errorf("left %s behind", e.Name())
		}
	}
}

func TestDetachDetachedHead(t *testing.T) {
	main, wt := newWorktree(t)
	head := run(t, main, "rev-parse", "HEAD")
	run(t, wt, "checkout", "--quiet", "--detach")
	if _, err := Detach(wt.Dir); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Output("symbolic-ref", "--quiet", "HEAD"); err == nil {
		t.Error("HEAD is attached")
	}
	if got := run(t, wt, "rev-parse", "HEAD"); got != head {
		t.Errorf("HEAD = %q, want %q", got, head)
	}
}

func TestDetachWorktreeConfig(t *testing.T) {
	main, wt := newWorktree(t)
	run(t, main, "config", "extensions.worktreeConfig", "true")
	run(t, wt, "config", "--worktree", "user.name", "Feature")
	run(t, main, "config", "--worktree", "user.name", "Main")
	if _, err := Detach(wt.Dir); err != nil {
		t.Fatal(err)
	}
	if got := run(t, wt, "config", "user.name"); got != "Feature" {
		t.Errorf("user.name = %q, want Feature", got)
	}
}

func TestDetachRefuses(t *testing.T) {
	main, wt := newWorktree(t)
	if _, err := Detach(main.Dir); err == nil || !strings.Contains(err.Error(), "not a linked worktree") {
		t.Errorf("main: %v", err)
	}
	run(t, main, "worktree", "lock", wt.Dir)
	if _, err := Detach(wt.Dir); err == nil || !strings.Contains(err.Error(), "locked") {
		t.Errorf("locked: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(wt.Dir, ".git")); err != nil || !strings.HasPrefix(string(got), "gitdir:") {
		t.Errorf("worktree changed: %q, %v", got, err)
	}
}
