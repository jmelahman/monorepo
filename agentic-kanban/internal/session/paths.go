package session

import (
	"os"
	"path/filepath"

	"github.com/jmelahman/kanban/internal/db"
)

// ResolvedPaths is the per-session result of layering session-level overrides
// over board defaults to determine where things live on the host and which
// of those locations is a real git repo we can run worktree/sync/merge against.
type ResolvedPaths struct {
	// MountPath is the host directory bind-mounted at the devcontainer's
	// workspaceFolder. May be a parent of multiple repos.
	MountPath string
	// RepoPath is the host path of the git repo this session operates on.
	// Empty when the board/session is not associated with a repo.
	RepoPath string
	// HasRepo is true when RepoPath points at a directory containing a .git
	// entry — git operations (worktree/sync/merge/PR poll) are only valid then.
	HasRepo bool
}

// ResolvePaths layers session-level overrides over board-level defaults. When
// MountPath is unset on both, it falls back to the session's worktree (today's
// behavior: mount the worktree at /workspace) and finally to the repo path.
func ResolvePaths(board *db.Board, sess *db.Session) ResolvedPaths {
	repo := ""
	if sess != nil && sess.RepoPath != "" {
		repo = sess.RepoPath
	} else if board != nil {
		repo = board.RepoPath
	}

	mount := ""
	if sess != nil && sess.MountPath != "" {
		mount = sess.MountPath
	} else if board != nil {
		mount = board.MountPath
	}

	hasRepo := repo != "" && isGitRepo(repo)

	if mount == "" {
		// MountPath is optional: when unset, mount the session's worktree so
		// /workspace points at the branch-isolated checkout, or fall back to
		// the bare repo path.
		if hasRepo && sess != nil && sess.WorktreePath != "" {
			mount = sess.WorktreePath
		} else if repo != "" {
			mount = repo
		}
	}

	return ResolvedPaths{MountPath: mount, RepoPath: repo, HasRepo: hasRepo}
}

// isGitRepo reports whether path contains a `.git` entry — either a directory
// (primary repo) or a regular file (linked worktree pointing at a parent
// gitdir). Used both to tell whether a board's repo_path is something git can
// run against, and to tell a real worktree from an empty directory dockerd
// auto-created on a prior failed bind-mount.
func isGitRepo(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(path, ".git"))
	if err != nil {
		return false
	}
	return info.IsDir() || info.Mode().IsRegular()
}
