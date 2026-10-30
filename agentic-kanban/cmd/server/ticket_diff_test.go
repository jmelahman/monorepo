package server

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jmelahman/kanban/internal/db"
)

// diffTicket makes a ticket whose session has a real worktree on a branch off
// main, with no changes yet.
func diffTicket(t *testing.T) (srvURL string, id int64, worktree string) {
	t.Helper()
	srv, store, board := newKanbanCLITestServer(t)
	cols, _ := store.ListColumns(t.Context(), board.ID)
	tk := &db.Ticket{BoardID: board.ID, ColumnID: cols[0].ID, Title: "Diff me", Slug: "diff-me"}
	if err := store.CreateTicket(t.Context(), tk); err != nil {
		t.Fatal(err)
	}
	worktree = filepath.Join(t.TempDir(), "wt")
	mustGit(t, board.RepoPath, "worktree", "add", "-q", "-b", "kanban/diff-me", worktree)
	sess := &db.Session{TicketID: tk.ID, Status: db.SessionStatusIdle, BranchName: "kanban/diff-me", WorktreePath: worktree}
	if err := store.UpsertSession(t.Context(), sess); err != nil {
		t.Fatal(err)
	}
	return srv.URL, tk.ID, worktree
}

func TestRunTicketDiff(t *testing.T) {
	url, id, worktree := diffTicket(t)
	run := func(t *testing.T, asJSON bool) (string, string) {
		t.Helper()
		var out, errOut bytes.Buffer
		if err := runTicketDiff(t.Context(), url, &out, &errOut, id, false, asJSON); err != nil {
			t.Fatal(err)
		}
		return out.String(), errOut.String()
	}

	out, errOut := run(t, false)
	if out != "" || errOut != "ticket "+strconv.FormatInt(id, 10)+" has no changes against main\n" {
		t.Errorf("clean worktree: out = %q, errOut = %q", out, errOut)
	}

	// Committed and untracked work both belong in the patch, like the Diff tab.
	if err := os.WriteFile(filepath.Join(worktree, "committed.txt"), []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mustGit(t, worktree, "add", "committed.txt")
	mustGit(t, worktree, "commit", "-q", "-m", "work")
	if err := os.WriteFile(filepath.Join(worktree, "untracked.txt"), []byte("two\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	out, _ = run(t, false)
	for _, want := range []string{"+++ b/committed.txt", "+one", "+++ b/untracked.txt", "+two"} {
		if !strings.Contains(out, want) {
			t.Errorf("patch = %q, want it to contain %q", out, want)
		}
	}
	if strings.Contains(out, "\x1b[") {
		t.Errorf("non-interactive patch has colors, so it can't be applied: %q", out)
	}

	out, _ = run(t, true)
	var got struct{ Base, Patch string }
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("--json output %q: %v", out, err)
	}
	if got.Base != "main" || !strings.Contains(got.Patch, "+one") {
		t.Errorf("--json = %+v", got)
	}
}

func TestRunTicketDiffNoSession(t *testing.T) {
	srv, store, board := newKanbanCLITestServer(t)
	cols, _ := store.ListColumns(t.Context(), board.ID)
	tk := &db.Ticket{BoardID: board.ID, ColumnID: cols[0].ID, Title: "No session", Slug: "no-session"}
	if err := store.CreateTicket(t.Context(), tk); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err := runTicketDiff(t.Context(), srv.URL, &out, &out, tk.ID, false, false)
	if err == nil || !strings.Contains(err.Error(), "no session") {
		t.Fatalf("err = %v, want it to mention the missing session", err)
	}
}

func TestRunTicketDiffNoWorktree(t *testing.T) {
	srv, store, board := newKanbanCLITestServer(t)
	cols, _ := store.ListColumns(t.Context(), board.ID)
	tk := &db.Ticket{BoardID: board.ID, ColumnID: cols[0].ID, Title: "No worktree", Slug: "no-worktree"}
	if err := store.CreateTicket(t.Context(), tk); err != nil {
		t.Fatal(err)
	}
	sess := &db.Session{TicketID: tk.ID, Status: db.SessionStatusIdle, BranchName: "kanban/no-worktree"}
	if err := store.UpsertSession(t.Context(), sess); err != nil {
		t.Fatal(err)
	}
	for _, asJSON := range []bool{false, true} {
		var out bytes.Buffer
		err := runTicketDiff(t.Context(), srv.URL, &out, &out, tk.ID, false, asJSON)
		if err == nil || !strings.Contains(err.Error(), "no worktree") {
			t.Errorf("asJSON=%v: err = %v, want it to mention the missing worktree", asJSON, err)
		}
	}
}

func TestTicketDiffCommandPipedPrintsPatch(t *testing.T) {
	restore := stdinIsTerminal
	stdinIsTerminal = func() bool { return false }
	t.Cleanup(func() { stdinIsTerminal = restore })

	url, id, worktree := diffTicket(t)
	if err := os.WriteFile(filepath.Join(worktree, "new.txt"), []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := ticketCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"diff", strconv.FormatInt(id, 10), "--server", url})
	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "+hello") {
		t.Errorf("output = %q, want the patch", out.String())
	}
}

func TestEscapeControls(t *testing.T) {
	for in, want := range map[string]string{
		"plain\ttext\n":            "plain\ttext\n",
		"caf\u00e9 \u65e5\u672c\n": "caf\u00e9 \u65e5\u672c\n",
		"\x1b]52;c;aGk=\x07":       "^[]52;c;aGk=^G",
		"\x1b[31mfake\x1b[0m":      "^[[31mfake^[[0m",
		"hide\rover":               "hide^Mover",
		"del\x7f":                  "del^?",
		"\u009b31m":                "\uFFFD31m",
		"\x9b31m":                  "\uFFFD31m",
	} {
		if got := escapeControls(in); got != want {
			t.Errorf("escapeControls(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestColorizePatch(t *testing.T) {
	patch := "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1,2 +1,2 @@\n ctx\n--- removed\n+added\n"
	want := "\x1b[1mdiff --git a/x b/x\x1b[0m\n" +
		"\x1b[1m--- a/x\x1b[0m\n" +
		"\x1b[1m+++ b/x\x1b[0m\n" +
		"\x1b[36m@@ -1,2 +1,2 @@\x1b[0m\n" +
		" ctx\n" +
		"\x1b[31m--- removed\x1b[0m\n" +
		"\x1b[32m+added\x1b[0m\n"
	if got := colorizePatch(patch); got != want {
		t.Errorf("colorizePatch = %q, want %q", got, want)
	}
}
