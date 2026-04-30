package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// AddWorktree creates a new worktree at path, creating a new branch from base.
func AddWorktree(repoPath, branch, worktreePath, base string) error {
	args := []string{"-C", repoPath, "worktree", "add", "-b", branch, worktreePath}
	if base != "" {
		args = append(args, base)
	}
	return run("git", args...)
}

// AddWorktreeFromExisting checks out an existing branch into a new worktree.
func AddWorktreeFromExisting(repoPath, branch, worktreePath string) error {
	return run("git", "-C", repoPath, "worktree", "add", worktreePath, branch)
}

// RemoveWorktree removes a worktree (force).
func RemoveWorktree(repoPath, worktreePath string) error {
	return run("git", "-C", repoPath, "worktree", "remove", "--force", worktreePath)
}

// DeleteBranch deletes a local branch.
func DeleteBranch(repoPath, branch string) error {
	return run("git", "-C", repoPath, "branch", "-D", branch)
}

// Rebase rebases the current branch in worktreePath onto ref.
func Rebase(worktreePath, ref string) error {
	return run("git", "-C", worktreePath, "rebase", ref)
}

// Merge merges ref into the current branch in worktreePath (no edit).
func Merge(worktreePath, ref string) error {
	return run("git", "-C", worktreePath, "merge", "--no-edit", ref)
}

// RebaseAbort aborts an in-progress rebase. Errors are swallowed.
func RebaseAbort(worktreePath string) {
	_ = exec.Command("git", "-C", worktreePath, "rebase", "--abort").Run()
}

// MergeAbort aborts an in-progress merge. Errors are swallowed.
func MergeAbort(worktreePath string) {
	_ = exec.Command("git", "-C", worktreePath, "merge", "--abort").Run()
}

// IsClean reports whether the worktree has no uncommitted changes.
func IsClean(worktreePath string) (bool, error) {
	cmd := exec.Command("git", "-C", worktreePath, "status", "--porcelain")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return false, err
	}
	return strings.TrimSpace(out.String()) == "", nil
}

// CurrentHead returns the abbreviated SHA at HEAD.
func CurrentHead(repoPath, ref string) (string, error) {
	if ref == "" {
		ref = "HEAD"
	}
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--short", ref)
	var out bytes.Buffer
	var errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git rev-parse: %w: %s", err, errb.String())
	}
	return strings.TrimSpace(out.String()), nil
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, errb.String())
	}
	return nil
}
