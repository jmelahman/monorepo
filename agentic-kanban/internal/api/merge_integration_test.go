package api_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmelahman/kanban/internal/db"
)

// mergeFixture is a ticket with a real worktree on its own branch.
type mergeFixture struct {
	e     *testEnv
	board *db.Board
	tk    *db.Ticket
	sess  db.Session
}

// newMergeFixture creates the ticket's session and commits the agent settings
// (.claude/) that session creation drops into the worktree, so every test
// starts from a clean worktree one commit ahead of the base.
func newMergeFixture(t *testing.T, title string) *mergeFixture {
	t.Helper()
	redirectUserConfig(t)
	e := newEnv(t)
	board := e.seedBoard("Merge " + title)
	tk := e.seedTicket(board, title)
	resp := e.post(fmt.Sprintf("/api/tickets/%d/session", tk.ID), nil)
	assertStatus(t, resp, 201)
	f := &mergeFixture{e: e, board: board, tk: tk, sess: decodeJSON[db.Session](t, resp)}
	mustGit(t, f.wt(), "add", "-A")
	mustGit(t, f.wt(), "commit", "-q", "-m", "session files")
	return f
}

func (f *mergeFixture) repo() string { return f.e.repoPath }
func (f *mergeFixture) wt() string   { return f.sess.WorktreePath }

// merge posts the ticket's merge with an explicit strategy.
func (f *mergeFixture) merge(strategy string) (path string, body map[string]any) {
	return fmt.Sprintf("/api/tickets/%d/merge", f.tk.ID), map[string]any{"strategy": strategy}
}

// sync is merge's counterpart for the sync endpoint.
func (f *mergeFixture) sync(strategy string) (path string, body map[string]any) {
	return fmt.Sprintf("/api/tickets/%d/sync", f.tk.ID), map[string]any{"strategy": strategy}
}

// commitFile writes path in dir and commits it.
func commitFile(t *testing.T, dir, path, content, message string) {
	t.Helper()
	writeFile(t, dir, path, content)
	mustGit(t, dir, "add", path)
	mustGit(t, dir, "commit", "-q", "-m", message)
}

func writeFile(t *testing.T, dir, path, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// gitOut runs git in dir and returns trimmed output, failing the test on error.
func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := runGit(dir, args...)
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(out)
}

func assertGitOut(t *testing.T, dir, want string, args ...string) {
	t.Helper()
	if got := gitOut(t, dir, args...); got != want {
		t.Fatalf("git %s in %s = %q, want %q", strings.Join(args, " "), dir, got, want)
	}
}

func assertFileExists(t *testing.T, dir, path string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, path)); err != nil {
		t.Fatalf("%s missing from %s: %v", path, dir, err)
	}
}

// assertTrackedClean fails when dir has staged or unstaged changes to tracked
// files — the state a half-finished merge leaves behind.
func assertTrackedClean(t *testing.T, dir string) {
	t.Helper()
	assertGitOut(t, dir, "", "status", "--porcelain", "--untracked-files=no")
}

// assertNoOperationInProgress fails when dir's git dir still holds the state
// of an unfinished merge or rebase.
func assertNoOperationInProgress(t *testing.T, dir string) {
	t.Helper()
	gitDir := gitOut(t, dir, "rev-parse", "--absolute-git-dir")
	for _, name := range []string{"MERGE_HEAD", "rebase-merge", "rebase-apply"} {
		if _, err := os.Stat(filepath.Join(gitDir, name)); err == nil {
			t.Fatalf("%s exists under %s, want no merge or rebase in progress", name, gitDir)
		}
	}
}

// conflictFixture is a mergeFixture whose branch and base both changed
// shared.txt, so every strategy that combines them hits a conflict.
func conflictFixture(t *testing.T) *mergeFixture {
	t.Helper()
	f := newMergeFixture(t, "Conflict")
	commitFile(t, f.wt(), "shared.txt", "ticket\n", "ticket edits shared")
	commitFile(t, f.repo(), "shared.txt", "base\n", "base edits shared")
	return f
}

// A merge-commit merge lands the ticket's work on the base branch under a
// real merge commit, and leaves the source repo clean.
func TestTicketMerge_MergeCommit(t *testing.T) {
	f := newMergeFixture(t, "AddGreeting")
	commitFile(t, f.wt(), "greeting.txt", "hello\n", "add greeting")

	path, body := f.merge("merge-commit")
	assertStatus(t, f.e.post(path, body), 204)

	assertFileExists(t, f.repo(), "greeting.txt")
	assertGitOut(t, f.repo(), "1", "rev-list", "--merges", "--count", "main")
	assertTrackedClean(t, f.repo())
}

// A squash merge collapses the branch into one commit on the base, titled
// after the ticket so the history reads as one change per ticket.
func TestTicketMerge_Squash(t *testing.T) {
	f := newMergeFixture(t, "SquashTwo")
	commitFile(t, f.wt(), "one.txt", "1\n", "add one")
	commitFile(t, f.wt(), "two.txt", "2\n", "add two")

	path, body := f.merge("squash")
	assertStatus(t, f.e.post(path, body), 204)

	assertFileExists(t, f.repo(), "one.txt")
	assertFileExists(t, f.repo(), "two.txt")
	// init + the one squash commit.
	assertGitOut(t, f.repo(), "2", "rev-list", "--count", "main")
	assertGitOut(t, f.repo(), fmt.Sprintf("%s (#%d)", f.tk.Title, f.tk.ID), "log", "-1", "--format=%s", "main")
	assertGitOut(t, f.repo(), "0", "rev-list", "--merges", "--count", "main")
	assertTrackedClean(t, f.repo())
}

// A rebase merge replays the ticket's commits on top of a base that moved
// underneath it, then fast-forwards: linear history, no merge commit.
func TestTicketMerge_Rebase(t *testing.T) {
	f := newMergeFixture(t, "RebaseOnto")
	commitFile(t, f.wt(), "a.txt", "a\n", "add a")
	commitFile(t, f.repo(), "b.txt", "b\n", "add b")

	path, body := f.merge("rebase")
	assertStatus(t, f.e.post(path, body), 204)

	assertFileExists(t, f.repo(), "a.txt")
	assertFileExists(t, f.repo(), "b.txt")
	assertGitOut(t, f.repo(), "0", "rev-list", "--merges", "--count", "main")
	// The worktree's commit was replayed on top of the base's.
	assertGitOut(t, f.repo(), "add a", "log", "-1", "--format=%s", "main")
	assertTrackedClean(t, f.repo())
}

// Uncommitted work in the worktree is committed under the ticket's title
// before merging, so merging never silently drops what the agent left behind.
func TestTicketMerge_CommitsDirtyWorktree(t *testing.T) {
	f := newMergeFixture(t, "WorkInProgress")
	writeFile(t, f.wt(), "wip.txt", "wip\n")

	path, body := f.merge("squash")
	assertStatus(t, f.e.post(path, body), 204)

	assertFileExists(t, f.repo(), "wip.txt")
	assertGitOut(t, f.wt(), "", "status", "--porcelain")
	assertGitOut(t, f.repo(), f.tk.Title, "log", "-1", "--format=%s", f.sess.BranchName)
	assertTrackedClean(t, f.repo())
}

// A conflicting merge must roll the user's repository back to exactly where
// it was: base branch unmoved, tree clean, nothing left mid-merge or
// mid-rebase in either the source repo or the worktree.
func TestTicketMerge_ConflictLeavesBaseUntouched(t *testing.T) {
	for _, strategy := range []string{"merge-commit", "squash", "rebase"} {
		t.Run(strategy, func(t *testing.T) {
			f := conflictFixture(t)
			before := gitOut(t, f.repo(), "rev-parse", "main")

			path, body := f.merge(strategy)
			if got := errorBody(t, f.e, path, body, 409); !strings.Contains(got, "aborted") {
				t.Fatalf("conflict error = %q, want it to report an aborted merge", got)
			}

			assertGitOut(t, f.repo(), before, "rev-parse", "main")
			assertTrackedClean(t, f.repo())
			got, err := os.ReadFile(filepath.Join(f.repo(), "shared.txt"))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != "base\n" {
				t.Fatalf("shared.txt in source repo = %q, want %q", got, "base\n")
			}
			assertNoOperationInProgress(t, f.repo())
			assertNoOperationInProgress(t, f.wt())
		})
	}
}

// Merging resets the source repo on failure, which would discard the user's
// uncommitted edits to tracked files — so it must refuse up front.
func TestTicketMerge_RefusesDirtySourceRepo(t *testing.T) {
	f := newMergeFixture(t, "DirtySource")
	commitFile(t, f.wt(), "feature.txt", "feature\n", "add feature")
	commitFile(t, f.repo(), "tracked.txt", "committed\n", "add tracked")
	writeFile(t, f.repo(), "tracked.txt", "edited\n")
	before := gitOut(t, f.repo(), "rev-parse", "main")

	path, body := f.merge("squash")
	got := errorBody(t, f.e, path, body, 409)
	if !strings.Contains(got, "source repo has uncommitted changes") {
		t.Fatalf("dirty source error = %q, want it to name the uncommitted changes", got)
	}
	assertGitOut(t, f.repo(), before, "rev-parse", "main")
}

// The merge lands on whatever the source repo has checked out, so anything
// other than the base branch must be refused rather than merged into.
func TestTicketMerge_RefusesWhenBaseNotCheckedOut(t *testing.T) {
	f := newMergeFixture(t, "WrongBranch")
	commitFile(t, f.wt(), "feature.txt", "feature\n", "add feature")
	mustGit(t, f.repo(), "checkout", "-q", "-b", "elsewhere")

	path, body := f.merge("squash")
	got := errorBody(t, f.e, path, body, 409)
	if !strings.Contains(got, "must have main checked out") {
		t.Fatalf("wrong branch error = %q, want it to name the base branch", got)
	}
}

// A rebase sync replays the ticket's commits onto the moved base: the base's
// work shows up in the worktree with no merge commit.
func TestTicketSync_Rebase(t *testing.T) {
	f := newMergeFixture(t, "SyncRebase")
	commitFile(t, f.wt(), "a.txt", "a\n", "add a")
	commitFile(t, f.repo(), "b.txt", "b\n", "add b")

	path, body := f.sync("rebase")
	assertStatus(t, f.e.post(path, body), 204)

	assertFileExists(t, f.wt(), "b.txt")
	assertGitOut(t, f.wt(), "0", "rev-list", "--merges", "--count", "HEAD")
	gitOut(t, f.wt(), "merge-base", "--is-ancestor", "main", "HEAD")
}

// A merge sync brings the base's work into the worktree under a merge commit.
func TestTicketSync_Merge(t *testing.T) {
	f := newMergeFixture(t, "SyncMerge")
	commitFile(t, f.wt(), "a.txt", "a\n", "add a")
	commitFile(t, f.repo(), "b.txt", "b\n", "add b")

	path, body := f.sync("merge")
	assertStatus(t, f.e.post(path, body), 204)

	assertFileExists(t, f.wt(), "b.txt")
	assertGitOut(t, f.wt(), "1", "rev-list", "--merges", "--count", "HEAD")
}

// A conflicting sync must hand the worktree back as it found it; an agent
// resuming in a half-rebased worktree would commit onto a detached HEAD.
func TestTicketSync_ConflictAbortsCleanly(t *testing.T) {
	for _, strategy := range []string{"rebase", "merge"} {
		t.Run(strategy, func(t *testing.T) {
			f := conflictFixture(t)
			before := gitOut(t, f.wt(), "rev-parse", "HEAD")

			path, body := f.sync(strategy)
			if got := errorBody(t, f.e, path, body, 409); !strings.Contains(got, "aborted") {
				t.Fatalf("conflict error = %q, want it to report an aborted sync", got)
			}

			assertGitOut(t, f.wt(), before, "rev-parse", "HEAD")
			assertGitOut(t, f.wt(), "", "status", "--porcelain")
			assertNoOperationInProgress(t, f.wt())
		})
	}
}

// Syncing rewrites the worktree, so uncommitted work there is a refusal, not
// something to carry through a rebase.
func TestTicketSync_RefusesDirtyWorktree(t *testing.T) {
	f := newMergeFixture(t, "SyncDirty")
	writeFile(t, f.wt(), "scratch.txt", "scratch\n")

	path, body := f.sync("rebase")
	got := errorBody(t, f.e, path, body, 409)
	if !strings.Contains(got, "worktree has uncommitted changes") {
		t.Fatalf("dirty worktree error = %q, want it to name the uncommitted changes", got)
	}
}

// Done files the ticket in the board's last column whether or not it ever had
// a session; a session that cannot be stopped must not block the move.
func TestTicketDone_MovesToLastColumn(t *testing.T) {
	f := newMergeFixture(t, "Finish")
	ctx := context.Background()
	cols, err := f.e.store.ListColumns(ctx, f.board.ID)
	if err != nil {
		t.Fatal(err)
	}
	first, last := cols[0].ID, cols[len(cols)-1].ID
	if first == last {
		t.Fatalf("board has %d column(s), want at least two", len(cols))
	}

	assertDone := func(t *testing.T, id int64) {
		t.Helper()
		assertStatus(t, f.e.post(fmt.Sprintf("/api/tickets/%d/done", id), nil), 204)
		got, err := f.e.store.GetTicket(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if got.ColumnID != last {
			t.Fatalf("ticket column = %d, want last column %d (first is %d)", got.ColumnID, last, first)
		}
	}

	t.Run("with session", func(t *testing.T) { assertDone(t, f.tk.ID) })
	t.Run("without session", func(t *testing.T) {
		assertDone(t, f.e.seedTicket(f.board, "NoSession").ID)
	})
}
