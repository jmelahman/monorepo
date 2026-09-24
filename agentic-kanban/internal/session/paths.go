package session

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

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
	// ProjectDir scopes the session to a subdirectory of the repo: the whole
	// worktree is still mounted, but the agent works from here. Repo-relative
	// and slash-separated, or "" for a whole-repo session. Always normalized,
	// so it never escapes the worktree.
	ProjectDir string
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

	projectDir := ""
	if board != nil {
		// Clamp rather than propagate: a row written by an older binary or
		// edited by hand must not be able to point the agent's cwd — or the
		// ${localWorkspaceFolder} a devcontainer.json can use as a bind
		// source — outside the worktree.
		if normalized, err := NormalizeProjectDir(board.ProjectDir); err == nil {
			projectDir = normalized
		}
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

	return ResolvedPaths{MountPath: mount, RepoPath: repo, HasRepo: hasRepo, ProjectDir: projectDir}
}

// ProjectRoot returns the host directory the agent should work from: the
// session's worktree, descended into ProjectDir. Returns worktreePath
// unchanged for a whole-repo session.
func (p ResolvedPaths) ProjectRoot(worktreePath string) string {
	if p.ProjectDir == "" {
		return worktreePath
	}
	return filepath.Join(worktreePath, filepath.FromSlash(p.ProjectDir))
}

// NormalizeProjectDir cleans a repo-relative subproject path into the form
// stored in boards.project_dir: slash-separated, no leading "./", no trailing
// slash. The empty string means "the whole repo" and is returned as-is.
//
// Absolute paths and anything escaping the repo via ".." are rejected. That
// is a security boundary, not just tidiness: project_dir becomes the
// container's working directory *and* the ${localWorkspaceFolder} that a
// repo-supplied devcontainer.json may use as a bind mount source.
func NormalizeProjectDir(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	s = filepath.ToSlash(s)
	if path.IsAbs(s) || filepath.IsAbs(s) {
		return "", fmt.Errorf("project_dir must be relative to the repository root, got %q", s)
	}
	cleaned := path.Clean(s)
	if cleaned == "." {
		// "." and "./" name the repo root, which is what "" already means.
		return "", nil
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("project_dir must stay inside the repository, got %q", s)
	}
	return cleaned, nil
}

// ValidateProjectDir normalizes a board's project_dir and enforces the two
// rules that make it meaningful. It needs a repo_path, because it is relative
// to one. And it cannot coexist with mount_path: mount_path makes the bind
// source the *main checkout* (or a parent of it), so descending into
// project_dir from there lands outside the per-ticket worktree and silently
// drops branch isolation. Every write path — HTTP, MCP, CLI — runs this.
func ValidateProjectDir(projectDir, repoPath, mountPath string) (string, error) {
	normalized, err := NormalizeProjectDir(projectDir)
	if err != nil {
		return "", err
	}
	if normalized == "" {
		return "", nil
	}
	if strings.TrimSpace(repoPath) == "" {
		return "", fmt.Errorf("project_dir requires repo_path: it names a subdirectory of the repository")
	}
	if strings.TrimSpace(mountPath) != "" {
		return "", fmt.Errorf("project_dir and mount_path are mutually exclusive: mount_path binds the main checkout, so a project_dir under it would bypass the per-ticket worktree")
	}
	return normalized, nil
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
