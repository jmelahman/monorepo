package git

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMergeSquash_IdentityInjected reproduces the "Author identity unknown"
// failure that surfaced when kanban's container has no user.name/user.email
// configured: a clean git repo with no committer config should still be able
// to squash-merge as long as an Identity is passed.
func TestMergeSquash_IdentityInjected(t *testing.T) {
	// Isolate from the developer's global gitconfig so "no identity" really
	// means none — otherwise CI passes but workstations with ~/.gitconfig
	// silently skip the negative branch.
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("HOME", t.TempDir())
	for _, k := range []string{"EMAIL", "GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL"} {
		if _, ok := os.LookupEnv(k); ok {
			old := os.Getenv(k)
			os.Unsetenv(k)
			t.Cleanup(func() { os.Setenv(k, old) })
		}
	}

	repo := initBareishRepo(t)

	// Set identity *only* for the seed commit, then unset it so the merge has
	// to rely on the Identity argument.
	mustGit(t, repo, "config", "user.name", "Seed")
	mustGit(t, repo, "config", "user.email", "seed@example.com")
	writeAndCommit(t, repo, "main", "seed.txt", "seed", "init")
	mustGit(t, repo, "checkout", "-q", "-b", "feature")
	writeAndCommit(t, repo, "feature", "feature.txt", "x", "feature work")
	mustGit(t, repo, "checkout", "-q", "main")
	mustGit(t, repo, "config", "--unset", "user.name")
	mustGit(t, repo, "config", "--unset", "user.email")

	// Sanity: confirm an unconfigured commit fails the way the bug report shows.
	if err := MergeSquash(repo, "feature", "should fail", Identity{}); err == nil {
		t.Fatalf("expected MergeSquash without identity to fail in unconfigured repo")
	}
	// `merge --squash` leaves the index dirty without MERGE_HEAD, so reset
	// rather than `merge --abort`.
	mustGit(t, repo, "reset", "-q", "--hard", "HEAD")

	// With an identity, the same call should succeed.
	id := Identity{Name: "Ada Lovelace", Email: "ada@example.com"}
	if err := MergeSquash(repo, "feature", "squash: feature work", id); err != nil {
		t.Fatalf("MergeSquash with identity: %v", err)
	}

	out := mustGitOut(t, repo, "log", "-1", "--pretty=format:%an <%ae>")
	if got := strings.TrimSpace(out); got != "Ada Lovelace <ada@example.com>" {
		t.Errorf("commit author = %q; want %q", got, "Ada Lovelace <ada@example.com>")
	}
}

func TestIdentity_ConfigArgs(t *testing.T) {
	if got := (Identity{}).configArgs(); got != nil {
		t.Errorf("empty identity should produce no args, got %v", got)
	}
	if got := (Identity{Name: "x"}).configArgs(); got != nil {
		t.Errorf("partial identity should produce no args, got %v", got)
	}
	got := Identity{Name: "Ada", Email: "ada@example.com"}.configArgs()
	want := []string{"-c", "user.name=Ada", "-c", "user.email=ada@example.com"}
	if len(got) != len(want) {
		t.Fatalf("got %v; want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("arg %d: got %q; want %q", i, got[i], want[i])
		}
	}
}

func TestResolveLatestBase(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("HOME", t.TempDir())

	t.Run("origin_strictly_ahead", func(t *testing.T) {
		remote, local := seedRemoteAndClone(t)
		// Push a newer commit to the bare remote via a second clone.
		other := filepath.Join(t.TempDir(), "other")
		mustGit(t, "", "clone", "-q", remote, other)
		mustGit(t, other, "config", "user.name", "Other")
		mustGit(t, other, "config", "user.email", "other@example.com")
		writeAndCommit(t, other, "main", "new.txt", "new", "newer")
		mustGit(t, other, "push", "-q", "origin", "main")

		got := ResolveLatestBase(local, "main")
		if got != "origin/main" {
			t.Fatalf("got %q, want %q", got, "origin/main")
		}
		// Sanity: the fetch actually happened — origin/main now matches the new commit.
		remoteSha := strings.TrimSpace(mustGitOut(t, local, "rev-parse", "refs/remotes/origin/main"))
		otherSha := strings.TrimSpace(mustGitOut(t, other, "rev-parse", "HEAD"))
		if remoteSha != otherSha {
			t.Fatalf("origin/main = %q, want %q (fetch didn't run)", remoteSha, otherSha)
		}
	})

	t.Run("local_strictly_ahead", func(t *testing.T) {
		_, local := seedRemoteAndClone(t)
		writeAndCommit(t, local, "main", "unpushed.txt", "x", "unpushed")
		if got := ResolveLatestBase(local, "main"); got != "main" {
			t.Fatalf("got %q, want %q (must preserve unpushed work)", got, "main")
		}
	})

	t.Run("equal_tips", func(t *testing.T) {
		_, local := seedRemoteAndClone(t)
		if got := ResolveLatestBase(local, "main"); got != "main" {
			t.Fatalf("got %q, want %q", got, "main")
		}
	})

	t.Run("diverged", func(t *testing.T) {
		remote, local := seedRemoteAndClone(t)
		// Local commits one thing on main.
		writeAndCommit(t, local, "main", "local.txt", "L", "local-only")
		// Remote (via second clone) commits a different thing on main and pushes.
		other := filepath.Join(t.TempDir(), "other")
		mustGit(t, "", "clone", "-q", remote, other)
		mustGit(t, other, "config", "user.name", "Other")
		mustGit(t, other, "config", "user.email", "other@example.com")
		writeAndCommit(t, other, "main", "remote.txt", "R", "remote-only")
		mustGit(t, other, "push", "-q", "origin", "main")

		if got := ResolveLatestBase(local, "main"); got != "main" {
			t.Fatalf("got %q, want %q (diverged: prefer local)", got, "main")
		}
	})

	t.Run("no_remote", func(t *testing.T) {
		repo := initBareishRepo(t)
		mustGit(t, repo, "config", "user.name", "Solo")
		mustGit(t, repo, "config", "user.email", "solo@example.com")
		writeAndCommit(t, repo, "main", "seed.txt", "s", "seed")
		if got := ResolveLatestBase(repo, "main"); got != "main" {
			t.Fatalf("got %q, want %q", got, "main")
		}
	})

	t.Run("empty_base", func(t *testing.T) {
		repo := initBareishRepo(t)
		if got := ResolveLatestBase(repo, ""); got != "" {
			t.Fatalf("got %q, want empty", got)
		}
	})
}

// --- test helpers ---

// seedRemoteAndClone creates a bare "remote" with one commit on main, then
// clones it into a "local" working repo. Returns absolute paths to both.
func seedRemoteAndClone(t *testing.T) (remote, local string) {
	t.Helper()
	root := t.TempDir()
	remote = filepath.Join(root, "remote.git")
	mustGit(t, "", "init", "-q", "--bare", "-b", "main", remote)

	// Seed via a temporary clone so the bare repo gets an initial main branch.
	seed := filepath.Join(root, "seed")
	mustGit(t, "", "clone", "-q", remote, seed)
	mustGit(t, seed, "config", "user.name", "Seed")
	mustGit(t, seed, "config", "user.email", "seed@example.com")
	mustGit(t, seed, "checkout", "-q", "-b", "main")
	writeAndCommit(t, seed, "main", "seed.txt", "seed", "seed")
	mustGit(t, seed, "push", "-q", "-u", "origin", "main")

	local = filepath.Join(root, "local")
	mustGit(t, "", "clone", "-q", remote, local)
	mustGit(t, local, "config", "user.name", "Local")
	mustGit(t, local, "config", "user.email", "local@example.com")
	return remote, local
}

// TestDiffAgainstBase checks that the patch spans committed-on-branch changes,
// uncommitted edits to tracked files, and brand-new untracked files (merge-base
// → working tree), while excluding commits made on base after the branch
// diverged — and that computing it never mutates the real index.
func TestDiffAgainstBase(t *testing.T) {
	repo := initBareishRepo(t)
	mustGit(t, repo, "config", "user.name", "Seed")
	mustGit(t, repo, "config", "user.email", "seed@example.com")
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("ignored.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, repo, "add", ".gitignore")
	writeAndCommit(t, repo, "main", "seed.txt", "seed\n", "init")

	// Branch off and commit a change on the branch.
	mustGit(t, repo, "checkout", "-q", "-b", "feature")
	writeAndCommit(t, repo, "feature", "committed.txt", "committed change\n", "add committed")

	// Advance base after divergence — must not leak into the branch diff.
	mustGit(t, repo, "checkout", "-q", "main")
	writeAndCommit(t, repo, "main", "main-only.txt", "main only\n", "main moves on")
	mustGit(t, repo, "checkout", "-q", "feature")

	// Leave an uncommitted edit to a tracked file, a brand-new untracked file,
	// and an ignored file (which must not appear).
	if err := os.WriteFile(filepath.Join(repo, "seed.txt"), []byte("seed edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "untracked.txt"), []byte("brand new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "ignored.txt"), []byte("noise\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	statusBefore := mustGitOut(t, repo, "status", "--porcelain")

	patch, err := DiffAgainstBase(repo, "main")
	if err != nil {
		t.Fatalf("DiffAgainstBase: %v", err)
	}
	for _, want := range []string{
		"committed.txt", "committed change",
		"seed.txt", "seed edited",
		"untracked.txt", "brand new",
	} {
		if !strings.Contains(patch, want) {
			t.Errorf("patch missing %q\n---\n%s", want, patch)
		}
	}
	if strings.Contains(patch, "main-only.txt") {
		t.Errorf("patch should exclude post-divergence base changes:\n%s", patch)
	}
	if strings.Contains(patch, "ignored.txt") {
		t.Errorf("patch should exclude gitignored files:\n%s", patch)
	}

	// The throwaway index must not have leaked into the real one: untracked.txt
	// stays untracked, and nothing new is staged.
	if statusAfter := mustGitOut(t, repo, "status", "--porcelain"); statusAfter != statusBefore {
		t.Errorf("real index mutated by diff:\nbefore:\n%s\nafter:\n%s", statusBefore, statusAfter)
	}
}

// TestReadBaseFile checks that the "old" side of the diff is read from the
// merge-base — original content for a tracked file (ignoring both the worktree
// edit and post-divergence base movement), and "not found" for files that
// didn't exist at the merge-base (added-on-branch or untracked) so callers treat
// them as an empty old side.
func TestReadBaseFile(t *testing.T) {
	repo := initBareishRepo(t)
	mustGit(t, repo, "config", "user.name", "Seed")
	mustGit(t, repo, "config", "user.email", "seed@example.com")
	writeAndCommit(t, repo, "main", "seed.txt", "seed\n", "init")

	// Branch off (merge-base is this point), add a file on the branch, advance
	// base after divergence, then edit the tracked file in the worktree.
	mustGit(t, repo, "checkout", "-q", "-b", "feature")
	writeAndCommit(t, repo, "feature", "committed.txt", "committed change\n", "add committed")
	mustGit(t, repo, "checkout", "-q", "main")
	writeAndCommit(t, repo, "main", "seed.txt", "seed moved on main\n", "main moves on")
	mustGit(t, repo, "checkout", "-q", "feature")
	if err := os.WriteFile(filepath.Join(repo, "seed.txt"), []byte("seed edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Tracked file: base content is the merge-base version, not the worktree
	// edit and not main's post-divergence change.
	if got, err := ReadBaseFile(repo, "main", "seed.txt"); err != nil || got != "seed\n" {
		t.Errorf("ReadBaseFile(seed.txt) = %q, %v; want %q, nil", got, err, "seed\n")
	}
	// Added on the branch after divergence — absent at the merge-base.
	if _, err := ReadBaseFile(repo, "main", "committed.txt"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadBaseFile(committed.txt) err = %v; want fs.ErrNotExist", err)
	}
	// Path traversal is rejected before touching git.
	if _, err := ReadBaseFile(repo, "main", "../escape.txt"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ReadBaseFile(../escape.txt) err = %v; want fs.ErrNotExist", err)
	}
}

func TestReadWorktreeFile(t *testing.T) {
	repo := initBareishRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "hello.txt"), []byte("hi there\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "sub", "nested.txt"), []byte("nested\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A secret outside the worktree, plus a symlink to it from inside, to prove
	// neither "../" nor a symlink can escape the worktree.
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("top secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(repo, "link.txt")); err != nil {
		t.Fatal(err)
	}

	t.Run("reads a file", func(t *testing.T) {
		got, err := ReadWorktreeFile(repo, "hello.txt")
		if err != nil || got != "hi there\n" {
			t.Fatalf("ReadWorktreeFile = %q, %v; want %q, nil", got, err, "hi there\n")
		}
	})
	t.Run("reads a nested file", func(t *testing.T) {
		got, err := ReadWorktreeFile(repo, "sub/nested.txt")
		if err != nil || got != "nested\n" {
			t.Fatalf("ReadWorktreeFile = %q, %v; want %q, nil", got, err, "nested\n")
		}
	})
	for _, bad := range []string{
		"../secret.txt",        // parent escape
		"sub/../../secret.txt", // escape after a descent
		"/etc/hostname",        // absolute
		"link.txt",             // symlink pointing outside the tree
		"missing.txt",          // does not exist
	} {
		t.Run("rejects "+bad, func(t *testing.T) {
			if got, err := ReadWorktreeFile(repo, bad); err == nil {
				t.Fatalf("ReadWorktreeFile(%q) = %q, nil; want error", bad, got)
			}
		})
	}
}

// signingRepo seeds a repo whose config enables signing with a key that does
// not exist, so a commit attempt fails unless signing is suppressed.
func signingRepo(t *testing.T) string {
	t.Helper()
	repo := initBareishRepo(t)
	mustGit(t, repo, "config", "user.name", "Seed")
	mustGit(t, repo, "config", "user.email", "seed@example.com")
	mustGit(t, repo, "config", "commit.gpgsign", "true")
	mustGit(t, repo, "config", "gpg.format", "ssh")
	mustGit(t, repo, "config", "user.signingkey", filepath.Join(repo, "missing-key.pub"))
	if err := os.WriteFile(filepath.Join(repo, "a.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AddAll(repo); err != nil {
		t.Fatalf("AddAll: %v", err)
	}
	return repo
}

// TestCommit_SkipsSigning verifies that by default kanban's commit operations
// never attempt to sign, even when the repo config enables it — the kanban
// container has no signing key, so a mounted ~/.gitconfig with
// commit.gpgsign=true must not break merges.
func TestCommit_SkipsSigning(t *testing.T) {
	SetCommitSigning(false) // default; explicit so the test is order-independent.
	repo := signingRepo(t)
	if err := Commit(repo, "no sign", Identity{Name: "Ada", Email: "ada@example.com"}); err != nil {
		t.Fatalf("Commit should skip signing and succeed: %v", err)
	}
	if sig := strings.TrimSpace(mustGitOut(t, repo, "log", "-1", "--pretty=%G?")); sig != "N" {
		t.Errorf("commit signature status = %q, want N (unsigned)", sig)
	}
}

// TestCommit_SigningEnabled verifies the toggle: with signing enabled kanban no
// longer suppresses it, so the same repo (configured to sign with a missing
// key) now fails the commit instead of silently skipping the signature.
func TestCommit_SigningEnabled(t *testing.T) {
	SetCommitSigning(true)
	defer SetCommitSigning(false)
	repo := signingRepo(t)
	if err := Commit(repo, "sign", Identity{Name: "Ada", Email: "ada@example.com"}); err == nil {
		t.Fatal("Commit should attempt signing (and fail with a missing key) when signing is enabled")
	}
}

func initBareishRepo(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "repo")
	mustGit(t, "", "init", "-q", "-b", "main", dir)
	return dir
}

func writeAndCommit(t *testing.T, repo, branch, name, contents, msg string) {
	t.Helper()
	cur, _ := CurrentBranch(repo)
	if cur != branch {
		mustGit(t, repo, "checkout", "-q", branch)
	}
	path := filepath.Join(repo, name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, repo, "add", name)
	mustGit(t, repo, "commit", "-q", "-m", msg)
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, errb.String())
	}
}

func mustGitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, errb.String())
	}
	return out.String()
}

// An untracked file makes IsClean report dirty (the caller is about to
// AddAll it) but leaves IsCleanTracked clean — the merge gate only guards
// against losing *tracked* work, which a reset or aborted merge can discard.
func TestIsCleanTracked_IgnoresUntracked(t *testing.T) {
	repo := initBareishRepo(t)
	mustGit(t, repo, "config", "user.name", "Seed")
	mustGit(t, repo, "config", "user.email", "seed@example.com")
	writeAndCommit(t, repo, "main", "seed.txt", "seed", "init")

	assertClean := func(what string, fn func(string) (bool, error), want bool) {
		t.Helper()
		got, err := fn(repo)
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if got != want {
			t.Fatalf("%s = %v, want %v", what, got, want)
		}
	}

	assertClean("IsClean on pristine repo", IsClean, true)
	assertClean("IsCleanTracked on pristine repo", IsCleanTracked, true)

	// An untracked scratch dir — the real-world case was a stray .claude/
	// blocking every merge.
	if err := os.MkdirAll(filepath.Join(repo, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".claude", "settings.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	assertClean("IsClean with untracked file", IsClean, false)
	assertClean("IsCleanTracked with untracked file", IsCleanTracked, true)

	// A modified tracked file is real risk: both must report dirty.
	if err := os.WriteFile(filepath.Join(repo, "seed.txt"), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	assertClean("IsClean with modified file", IsClean, false)
	assertClean("IsCleanTracked with modified file", IsCleanTracked, false)
}
