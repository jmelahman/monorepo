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
