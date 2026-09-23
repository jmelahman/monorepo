package git

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jmelahman/kanban/internal/metrics"
)

// fetchTimeout caps best-effort `git fetch origin <base>` calls done while
// resolving the start-point for a new worktree. Short enough that an
// unreachable remote doesn't block session creation; long enough for a
// normal small fetch over commercial broadband.
const fetchTimeout = 10 * time.Second

// Identity is an optional commit author/committer override. When both fields
// are non-empty, callers can pass it to commit-creating helpers to inject
// `-c user.name=... -c user.email=...` flags so the commit doesn't depend on
// whatever identity the kanban container's git can auto-detect.
type Identity struct {
	Name  string
	Email string
}

func (i Identity) configArgs() []string {
	if i.Name == "" || i.Email == "" {
		return nil
	}
	return []string{"-c", "user.name=" + i.Name, "-c", "user.email=" + i.Email}
}

// AddWorktree creates a new worktree at path, creating a new branch from base.
// Passes --no-track so the new branch never inherits a remote upstream when
// base is a remote-tracking ref (e.g. "origin/main"); session branches aren't
// meant to push back to the base.
func AddWorktree(repoPath, branch, worktreePath, base string) error {
	start := time.Now()
	args := []string{"-C", repoPath, "worktree", "add", "--no-track", "-b", branch, worktreePath}
	if base != "" {
		args = append(args, base)
	}
	err := run("git", args...)
	metrics.ObserveGitCommand("worktree_add", start, err)
	return err
}

// ResolveLatestBase returns the ref to use as the start-point for a new
// worktree based on `base`. It best-effort fetches `origin/<base>` with a
// short timeout, then returns "origin/<base>" iff that remote-tracking ref
// is strictly ahead of local `<base>`. In every other case — no remote,
// fetch failure, ref missing, equal tips, local ahead, or diverged — it
// returns `<base>` unchanged. Returns "" when base is "" so callers preserve
// the no-start-point `git worktree add` behavior.
func ResolveLatestBase(repoPath, base string) string {
	if base == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()
	start := time.Now()
	err := exec.CommandContext(ctx, "git", "-C", repoPath, "fetch", "--quiet", "origin", base).Run()
	metrics.ObserveGitCommand("fetch", start, err)
	if err != nil {
		log.Printf("git fetch origin %s in %s: %v (falling back to local %s)", base, repoPath, err, base)
	}
	remote := "refs/remotes/origin/" + base
	if exec.Command("git", "-C", repoPath, "rev-parse", "--verify", "--quiet", remote).Run() != nil {
		return base
	}
	localSha, err1 := revParse(repoPath, base)
	remoteSha, err2 := revParse(repoPath, remote)
	if err1 != nil || err2 != nil || localSha == remoteSha {
		return base
	}
	// origin strictly ahead iff local is an ancestor of origin AND they differ.
	if exec.Command("git", "-C", repoPath, "merge-base", "--is-ancestor", base, remote).Run() == nil {
		return "origin/" + base
	}
	return base
}

func revParse(repoPath, ref string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--verify", "--quiet", ref)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

// AddWorktreeFromExisting checks out an existing branch into a new worktree.
func AddWorktreeFromExisting(repoPath, branch, worktreePath string) error {
	start := time.Now()
	err := run("git", "-C", repoPath, "worktree", "add", worktreePath, branch)
	metrics.ObserveGitCommand("worktree_add", start, err)
	return err
}

// RemoveWorktree removes a worktree (force).
func RemoveWorktree(repoPath, worktreePath string) error {
	start := time.Now()
	err := run("git", "-C", repoPath, "worktree", "remove", "--force", worktreePath)
	metrics.ObserveGitCommand("worktree_remove", start, err)
	return err
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

// MergeNoFF merges branch into the currently checked-out branch in repoPath
// always creating a merge commit, even when fast-forward is possible. When
// id is set, its name/email override the commit author/committer.
func MergeNoFF(repoPath, branch string, id Identity) error {
	args := append([]string{}, id.configArgs()...)
	args = append(args, "-C", repoPath, "merge", "--no-ff", "--no-edit", branch)
	return run("git", args...)
}

// MergeSquash squashes branch into the index of repoPath without committing,
// then creates a single commit with message. When id is set, its name/email
// override the commit author/committer.
func MergeSquash(repoPath, branch, message string, id Identity) error {
	if err := run("git", "-C", repoPath, "merge", "--squash", branch); err != nil {
		return err
	}
	args := append([]string{}, id.configArgs()...)
	args = append(args, "-C", repoPath, "commit", "-m", message)
	return run("git", args...)
}

// MergeFFOnly fast-forwards the currently checked-out branch in repoPath to
// branch. Fails if a fast-forward isn't possible.
func MergeFFOnly(repoPath, branch string) error {
	return run("git", "-C", repoPath, "merge", "--ff-only", branch)
}

// DefaultBranch returns the repo's default branch name. It prefers what
// origin/HEAD points to (set by `git clone` or `git remote set-head`), and
// falls back to the currently checked-out branch when no remote default is
// configured. Returns ("", nil) if neither is available (e.g. detached HEAD
// in a repo with no remote).
func DefaultBranch(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err == nil {
		return strings.TrimPrefix(strings.TrimSpace(out.String()), "origin/"), nil
	}
	return CurrentBranch(repoPath)
}

// CurrentBranch returns the short branch name checked out in repoPath, or an
// empty string if HEAD is detached.
func CurrentBranch(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "symbolic-ref", "--quiet", "--short", "HEAD")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		// Non-zero exit on detached HEAD; surface as empty string, not error.
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

// RebaseAbort aborts an in-progress rebase. Errors are swallowed.
func RebaseAbort(worktreePath string) {
	_ = exec.Command("git", "-C", worktreePath, "rebase", "--abort").Run()
}

// MergeAbort aborts an in-progress merge. Errors are swallowed.
func MergeAbort(worktreePath string) {
	_ = exec.Command("git", "-C", worktreePath, "merge", "--abort").Run()
}

// ResetHard resets the currently checked-out branch in repoPath to ref.
// Errors are swallowed; used to recover from a failed squash commit.
func ResetHard(repoPath, ref string) {
	_ = exec.Command("git", "-C", repoPath, "reset", "--hard", ref).Run()
}

// AddAll stages every change in the worktree.
func AddAll(worktreePath string) error {
	return run("git", "-C", worktreePath, "add", "-A")
}

// Commit creates a commit with the given message in the worktree. When id is
// set, its name/email override the commit author/committer.
func Commit(worktreePath, message string, id Identity) error {
	args := append([]string{}, id.configArgs()...)
	args = append(args, "-C", worktreePath, "commit", "-m", message)
	return run("git", args...)
}

// IsClean reports whether the worktree has no uncommitted changes, counting
// untracked files as dirty. Use it when the caller is about to stage
// everything (git.AddAll), where an untracked file is work that would be
// swept into the commit.
func IsClean(worktreePath string) (bool, error) {
	return isClean(worktreePath, true)
}

// IsCleanTracked is IsClean but ignores untracked files. Use it for gates
// that only protect against *losing* work — a reset or an aborted merge
// leaves untracked files untouched, so they are no reason to refuse.
func IsCleanTracked(worktreePath string) (bool, error) {
	return isClean(worktreePath, false)
}

func isClean(worktreePath string, untrackedIsDirty bool) (bool, error) {
	args := []string{"-C", worktreePath, "status", "--porcelain"}
	if !untrackedIsDirty {
		args = append(args, "--untracked-files=no")
	}
	cmd := exec.Command("git", args...)
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

// DiffAgainstBase returns the unified diff (patch) of the worktree's full
// uncommitted state — committed-on-branch changes plus every uncommitted
// modification, including brand-new untracked files (honoring .gitignore) —
// against the point where the session branch diverged from base. It resolves
// base to whichever ref exists locally (`base`, then `origin/<base>`), finds
// the merge-base with HEAD, and diffs from there to the working tree (`-M`
// enables rename detection). When no merge-base can be computed (e.g. an
// orphan/empty branch) it diffs against the resolved base, and when base
// resolves to nothing it diffs against the empty tree so a brand-new branch
// still shows its work. An empty string with a nil error means "no changes".
//
// Untracked files have no git object to diff, so to surface them in one pass —
// without mutating the agent's real index — everything is staged into a
// throwaway index (seeded from the real one so git's stat-cache keeps `add -A`
// fast), then a single `diff --cached` reports the lot. The real index and
// working tree are left untouched. .gitignore is honored, so anything the repo
// ignores (e.g. a gitignored `.claude/settings.local.json`) never appears.
func DiffAgainstBase(worktreePath, base string) (diff string, err error) {
	defer func(start time.Time) { metrics.ObserveGitCommand("diff", start, err) }(time.Now())
	diffTarget := resolveDiffTarget(worktreePath, base)

	tmpDir, err := os.MkdirTemp("", "kanban-diff-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()
	tmpIndex := filepath.Join(tmpDir, "index")
	seedTempIndex(worktreePath, tmpIndex)

	env := append(os.Environ(), "GIT_INDEX_FILE="+tmpIndex)
	if _, err := captureGitEnv(env, "-C", worktreePath, "add", "-A"); err != nil {
		return "", err
	}
	args := []string{"-C", worktreePath, "diff", "--no-color", "-M", "--cached"}
	if diffTarget != "" {
		args = append(args, diffTarget)
	}
	return captureGitEnv(env, args...)
}

// ReadWorktreeFile returns the current working-tree contents of relPath inside
// worktreePath — the "new" side of DiffAgainstBase (working tree, including
// uncommitted edits). relPath is validated to stay within the worktree, both
// lexically and after resolving symlinks, so a hostile path (e.g. "../" or a
// symlink pointing out of the tree) can't read arbitrary files. A missing file,
// or one resolving outside the worktree, returns fs.ErrNotExist.
func ReadWorktreeFile(worktreePath, relPath string) (string, error) {
	clean, err := cleanRelPath(relPath)
	if err != nil {
		return "", err
	}
	root, err := filepath.EvalSymlinks(worktreePath)
	if err != nil {
		return "", err
	}
	full, err := filepath.EvalSymlinks(filepath.Join(root, clean))
	if err != nil {
		return "", err
	}
	if full != root && !strings.HasPrefix(full, root+string(filepath.Separator)) {
		return "", fs.ErrNotExist
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ReadBaseFile returns the contents of relPath as of the point the session
// branch diverged from base — the "old" side of DiffAgainstBase. It resolves
// the same diff target DiffAgainstBase does (the merge-base with HEAD, falling
// back to the resolved base ref) and reads the blob with `git show <ref>:<path>`,
// so the lines it returns line up exactly with the patch's hunks. A file absent
// at that point (e.g. one the session newly created) or a branch with no base to
// compare against returns fs.ErrNotExist, so callers can treat it as an empty
// old side. Unlike ReadWorktreeFile this reads from git's object store rather
// than the filesystem, so only the lexical ".."-escape guard applies.
func ReadBaseFile(worktreePath, base, relPath string) (string, error) {
	clean, err := cleanRelPath(relPath)
	if err != nil {
		return "", err
	}
	diffTarget := resolveDiffTarget(worktreePath, base)
	if diffTarget == "" {
		return "", fs.ErrNotExist
	}
	out, err := captureGit("-C", worktreePath, "show", diffTarget+":"+filepath.ToSlash(clean))
	if err != nil {
		// Most often the path doesn't exist at this ref (a new file); git also
		// errors on a bad ref. Either way there's no old side to show.
		return "", fs.ErrNotExist
	}
	return out, nil
}

// cleanRelPath validates that relPath is a non-empty, non-absolute path that
// can't escape its root via "..", returning the lexically-cleaned form. Shared
// by the worktree- and git-object readers as their first-line path guard.
func cleanRelPath(relPath string) (string, error) {
	if relPath == "" || filepath.IsAbs(relPath) {
		return "", fs.ErrNotExist
	}
	clean := filepath.Clean(relPath)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fs.ErrNotExist
	}
	return clean, nil
}

// seedTempIndex best-effort copies the worktree's real index into dst so that
// `git add -A` against dst reuses git's stat-cache and skips re-hashing
// unchanged files. Failures are ignored — an empty index still yields a
// correct (if slower) diff.
func seedTempIndex(worktreePath, dst string) {
	out, err := captureGit("-C", worktreePath, "rev-parse", "--git-path", "index")
	if err != nil {
		return
	}
	src := strings.TrimSpace(out)
	if !filepath.IsAbs(src) {
		src = filepath.Join(worktreePath, src)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return
	}
	_ = os.WriteFile(dst, data, 0o600)
}

// resolveDiffBase returns the first of `base` or `origin/<base>` that names an
// existing ref reachable from the worktree, or "" when neither does (or base
// is empty).
func resolveDiffBase(worktreePath, base string) string {
	if base == "" {
		return ""
	}
	if _, err := revParse(worktreePath, base); err == nil {
		return base
	}
	remote := "origin/" + base
	if _, err := revParse(worktreePath, remote); err == nil {
		return remote
	}
	return ""
}

// resolveDiffTarget returns the ref DiffAgainstBase diffs the working tree
// against: the merge-base of the resolved base ref and HEAD, falling back to the
// resolved base ref when no merge-base exists (e.g. an orphan branch), and ""
// when base resolves to nothing (a brand-new branch is then diffed against the
// empty tree). Shared with ReadBaseFile so the "old" side of a single file is
// read from the exact same point the patch is computed against.
func resolveDiffTarget(worktreePath, base string) string {
	ref := resolveDiffBase(worktreePath, base)
	if ref == "" {
		return ""
	}
	if mb, err := mergeBase(worktreePath, ref, "HEAD"); err == nil && mb != "" {
		return mb
	}
	return ref
}

// mergeBase returns the best common ancestor SHA of refs a and b.
func mergeBase(worktreePath, a, b string) (string, error) {
	out, err := captureGit("-C", worktreePath, "merge-base", a, b)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// captureGit runs git with args and returns its stdout. Unlike run(), it keeps
// stdout for the caller; the wrapped error includes stderr for context.
func captureGit(args ...string) (string, error) {
	return captureGitEnv(nil, args...)
}

// captureGitEnv is captureGit with an explicit environment (e.g. a
// GIT_INDEX_FILE override). A nil env inherits the current process environment.
func captureGitEnv(env []string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if env != nil {
		cmd.Env = env
	}
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, errb.String())
	}
	return out.String(), nil
}

// noSignArgs disables commit/tag signing for the git operations kanban runs on
// the user's behalf (merge, squash, rebase, commit). Injected as `-c` overrides
// (highest precedence), mirroring how Identity injects user.name/email.
var noSignArgs = []string{"-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"}

// signCommits gates whether kanban lets the ambient git config sign its
// commits. False (the default) forces signing off via noSignArgs, since the
// kanban container usually has no signing key and a mounted ~/.gitconfig with
// commit.gpgsign=true would otherwise make merges fail. Toggled at startup and
// from the App Settings handler via SetCommitSigning.
var signCommits atomic.Bool

// SetCommitSigning configures whether kanban's own commit/merge/squash/rebase
// operations may be signed. When enabled, kanban stops forcing signing off and
// defers to the git config's commit.gpgsign — the deployment is then
// responsible for providing a signing key/agent inside the container.
func SetCommitSigning(enabled bool) { signCommits.Store(enabled) }

func run(name string, args ...string) error {
	if name == "git" && !signCommits.Load() {
		args = append(append([]string{}, noSignArgs...), args...)
	}
	cmd := exec.Command(name, args...)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, errb.String())
	}
	return nil
}
