package server

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// gitOut runs the real git binary in dir and returns its trimmed stdout.
// Preferred over go-git for reading local checkout state: go-git refuses
// repositories using config it doesn't implement (repositoryformatversion 1
// with extensions like worktreeConfig — routine wherever `git worktree` or
// sparse checkouts are in play), while git itself reads them fine. go-git
// remains the fallback so the CLI still works on a machine without git.
func gitOut(dir string, args ...string) (string, bool) {
	gitBin, err := exec.LookPath("git")
	if err != nil {
		return "", false
	}
	cmd := exec.Command(gitBin, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

// findWorktreeRoot walks up from dir to the first directory containing a
// .git entry — the equivalent of `git rev-parse --show-toplevel`.
func findWorktreeRoot(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(abs, ".git")); err == nil {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", fmt.Errorf("no .git found in %s or any parent", dir)
		}
		abs = parent
	}
}

// localGitDir returns the git directory for the working tree at root — the
// equivalent of `git rev-parse --git-dir`. For a linked worktree (.git is a
// file), it follows the gitdir pointer.
func localGitDir(root string) (string, error) {
	dotGit := filepath.Join(root, ".git")
	fi, err := os.Stat(dotGit)
	if err != nil {
		return "", err
	}
	if fi.IsDir() {
		return dotGit, nil
	}
	raw, err := os.ReadFile(dotGit)
	if err != nil {
		return "", err
	}
	target := strings.TrimSpace(strings.TrimPrefix(string(raw), "gitdir:"))
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	return target, nil
}

func openLocalRepo(dir string) (*git.Repository, error) {
	return git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{
		DetectDotGit:          true,
		EnableDotGitCommonDir: true,
	})
}

// localHeadSHA returns the commit sha of HEAD for the working tree
// containing dir — the equivalent of `git rev-parse HEAD`.
func localHeadSHA(dir string) (string, error) {
	if sha, ok := gitOut(dir, "rev-parse", "HEAD"); ok {
		return sha, nil
	}
	repo, err := openLocalRepo(dir)
	if err != nil {
		return "", err
	}
	head, err := repo.Head()
	if err != nil {
		return "", err
	}
	return head.Hash().String(), nil
}

// localResolveRevision resolves rev (a branch, tag, sha, HEAD~1, …) to a
// full commit sha in the working tree containing dir — the equivalent of
// `git rev-parse <rev>`.
func localResolveRevision(dir, rev string) (string, error) {
	if sha, ok := gitOut(dir, "rev-parse", "--verify", rev+"^{commit}"); ok {
		return sha, nil
	}
	repo, err := openLocalRepo(dir)
	if err != nil {
		return "", err
	}
	h, err := repo.ResolveRevision(plumbing.Revision(rev))
	if err != nil {
		return "", err
	}
	return h.String(), nil
}

// localHeadBranch returns the branch HEAD is on for the working tree
// containing dir, or "" when detached or not a git repo.
func localHeadBranch(dir string) string {
	if branch, ok := gitOut(dir, "rev-parse", "--abbrev-ref", "HEAD"); ok {
		if branch == "HEAD" {
			return "" // detached
		}
		return branch
	}
	repo, err := openLocalRepo(dir)
	if err != nil {
		return ""
	}
	head, err := repo.Head()
	if err != nil || !head.Name().IsBranch() {
		return ""
	}
	return head.Name().Short()
}

// localOriginURL returns the origin remote's URL for the working tree
// containing dir, or "" if there is no origin remote.
func localOriginURL(dir string) string {
	if url, ok := gitOut(dir, "remote", "get-url", "origin"); ok {
		return url
	}
	repo, err := openLocalRepo(dir)
	if err != nil {
		return ""
	}
	remote, err := repo.Remote("origin")
	if err != nil || len(remote.Config().URLs) == 0 {
		return ""
	}
	return remote.Config().URLs[0]
}
