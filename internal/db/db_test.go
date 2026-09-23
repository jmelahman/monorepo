package db_test

import (
	"errors"
	"path/filepath"
	"sort"
	"testing"

	"github.com/jmelahman/kanban/internal/db"
)

// expectedTables is the set of tables schema.sql is supposed to create on a
// fresh open. If schema.sql gains a table, this list goes with it; the test
// then guards against accidental schema drift.
var expectedTables = []string{
	"board_env_vars",
	"boards",
	"columns",
	"hook_configs",
	"port_allocations",
	"sessions",
	"task_runs",
	"tickets",
}

func TestOpen_InMemory(t *testing.T) {
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open(:memory:): %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	assertSchema(t, store)
	assertBoardLifecycle(t, store)
}

func TestOpen_FreshFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kanban.db")
	store, err := db.Open(path)
	if err != nil {
		t.Fatalf("db.Open(%q): %v", path, err)
	}
	t.Cleanup(func() { _ = store.Close() })

	assertSchema(t, store)
	assertBoardLifecycle(t, store)
}

// TestOpen_Reopen opens an already-populated DB a second time — the path
// production hits on every startup after the first one.
func TestOpen_Reopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kanban.db")
	first, err := db.Open(path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	second, err := db.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })
	assertSchema(t, second)
}

func assertSchema(t *testing.T, store *db.Store) {
	t.Helper()
	rows, err := store.DB().Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		t.Fatalf("query tables: %v", err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	sort.Strings(got)
	if len(got) != len(expectedTables) {
		t.Fatalf("tables = %v; want %v", got, expectedTables)
	}
	for i, name := range expectedTables {
		if got[i] != name {
			t.Errorf("tables[%d] = %q; want %q (full: %v)", i, got[i], name, got)
		}
	}
}

func TestCountSessionsByStatus(t *testing.T) {
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open(:memory:): %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	ctx := t.Context()

	// Empty DB → empty map.
	got, err := store.CountSessionsByStatus(ctx)
	if err != nil {
		t.Fatalf("CountSessionsByStatus (empty): %v", err)
	}
	if len(got) != 0 {
		t.Errorf("empty count = %v; want empty map", got)
	}

	b := &db.Board{Name: "Counts", Slug: "counts", BaseBranch: "main", RepoPath: "/tmp/x"}
	if err := store.CreateBoard(ctx, b); err != nil {
		t.Fatalf("CreateBoard: %v", err)
	}
	cols, err := store.ListColumns(ctx, b.ID)
	if err != nil {
		t.Fatalf("ListColumns: %v", err)
	}

	// One session per ticket (ticket_id is UNIQUE), spread across statuses.
	statuses := []string{
		db.SessionStatusWorking,
		db.SessionStatusWorking,
		db.SessionStatusIdle,
		db.SessionStatusAwaitingPerm,
		db.SessionStatusStarting,
		db.SessionStatusStopped,
		db.SessionStatusError,
	}
	for i, st := range statuses {
		tk := &db.Ticket{BoardID: b.ID, ColumnID: cols[0].ID, Title: "t", Slug: "t"}
		if err := store.CreateTicket(ctx, tk); err != nil {
			t.Fatalf("CreateTicket[%d]: %v", i, err)
		}
		sess := &db.Session{
			TicketID:     tk.ID,
			WorktreePath: "/tmp/wt",
			BranchName:   "kanban/test",
			Status:       st,
		}
		if err := store.UpsertSession(ctx, sess); err != nil {
			t.Fatalf("UpsertSession[%d]: %v", i, err)
		}
	}

	got, err = store.CountSessionsByStatus(ctx)
	if err != nil {
		t.Fatalf("CountSessionsByStatus: %v", err)
	}
	want := map[string]int{
		db.SessionStatusWorking:      2,
		db.SessionStatusIdle:         1,
		db.SessionStatusAwaitingPerm: 1,
		db.SessionStatusStarting:     1,
		db.SessionStatusStopped:      1,
		db.SessionStatusError:        1,
	}
	if len(got) != len(want) {
		t.Fatalf("count = %v; want %v", got, want)
	}
	for status, n := range want {
		if got[status] != n {
			t.Errorf("count[%q] = %d; want %d (full: %v)", status, got[status], n, got)
		}
	}
}

func TestRepointSessionBranch(t *testing.T) {
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open(:memory:): %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	ctx := t.Context()

	b := &db.Board{Name: "Repoint", Slug: "repoint", BaseBranch: "main", RepoPath: "/tmp/x"}
	if err := store.CreateBoard(ctx, b); err != nil {
		t.Fatalf("CreateBoard: %v", err)
	}
	cols, err := store.ListColumns(ctx, b.ID)
	if err != nil {
		t.Fatalf("ListColumns: %v", err)
	}
	tk := &db.Ticket{BoardID: b.ID, ColumnID: cols[0].ID, Title: "t", Slug: "t"}
	if err := store.CreateTicket(ctx, tk); err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}

	// Seed a session with the pr_* fields populated so we can assert the
	// re-point clears them (the poller will repopulate for the new branch).
	prNum := int64(42)
	sess := &db.Session{
		TicketID:     tk.ID,
		WorktreePath: "/tmp/wt",
		BranchName:   "kanban/old",
		Status:       db.SessionStatusStopped,
		PRState:      "open",
		PRNumber:     &prNum,
		PRURL:        "https://github.com/x/y/pull/42",
		PRTitle:      "old pr",
	}
	if err := store.UpsertSession(ctx, sess); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}

	if err := store.RepointSessionBranch(ctx, sess.ID, "kanban/new"); err != nil {
		t.Fatalf("RepointSessionBranch: %v", err)
	}

	got, err := store.GetSession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.BranchName != "kanban/new" {
		t.Errorf("branch_name = %q; want %q", got.BranchName, "kanban/new")
	}
	if got.PRState != "" || got.PRNumber != nil || got.PRURL != "" || got.PRTitle != "" {
		t.Errorf("pr_* not cleared: state=%q number=%v url=%q title=%q",
			got.PRState, got.PRNumber, got.PRURL, got.PRTitle)
	}

	// Unknown id → ErrNotFound.
	if err := store.RepointSessionBranch(ctx, 9999, "kanban/whatever"); !errors.Is(err, db.ErrNotFound) {
		t.Errorf("RepointSessionBranch(unknown) error = %v; want ErrNotFound", err)
	}
}

func assertBoardLifecycle(t *testing.T, store *db.Store) {
	t.Helper()
	ctx := t.Context()

	b := &db.Board{Name: "Smoke", Slug: "smoke", BaseBranch: "main", RepoPath: "/tmp/x"}
	if err := store.CreateBoard(ctx, b); err != nil {
		t.Fatalf("CreateBoard: %v", err)
	}
	if b.ID == 0 {
		t.Fatal("CreateBoard did not assign id")
	}

	cols, err := store.ListColumns(ctx, b.ID)
	if err != nil {
		t.Fatalf("ListColumns: %v", err)
	}
	wantCols := []string{"Backlog", "In Progress", "Review", "Done"}
	if len(cols) != len(wantCols) {
		t.Fatalf("default columns = %d; want %d (%v)", len(cols), len(wantCols), wantCols)
	}
	for i, c := range cols {
		if c.Name != wantCols[i] || c.Position != i {
			t.Errorf("col[%d] = {%q, pos %d}; want {%q, pos %d}", i, c.Name, c.Position, wantCols[i], i)
		}
	}

	tk := &db.Ticket{BoardID: b.ID, ColumnID: cols[0].ID, Title: "first", Slug: "first"}
	if err := store.CreateTicket(ctx, tk); err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}
	if tk.ID == 0 || tk.Position != 1 {
		t.Errorf("ticket = %+v; want id != 0 and position 1", tk)
	}

	got, err := store.ListTickets(ctx, b.ID)
	if err != nil {
		t.Fatalf("ListTickets: %v", err)
	}
	if len(got) != 1 || got[0].ID != tk.ID {
		t.Errorf("ListTickets = %+v; want one row with id %d", got, tk.ID)
	}
}

func TestUpdateSessionHarness(t *testing.T) {
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open(:memory:): %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	ctx := t.Context()

	b := &db.Board{Name: "Harness", Slug: "harness", BaseBranch: "main", RepoPath: "/tmp/x"}
	if err := store.CreateBoard(ctx, b); err != nil {
		t.Fatalf("CreateBoard: %v", err)
	}
	cols, err := store.ListColumns(ctx, b.ID)
	if err != nil {
		t.Fatalf("ListColumns: %v", err)
	}
	tk := &db.Ticket{BoardID: b.ID, ColumnID: cols[0].ID, Title: "t", Slug: "t"}
	if err := store.CreateTicket(ctx, tk); err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}
	sess := &db.Session{TicketID: tk.ID, WorktreePath: "/tmp/wt", BranchName: "kanban/t", Status: db.SessionStatusStopped}
	if err := store.UpsertSession(ctx, sess); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}

	if err := store.UpdateSessionHarness(ctx, sess.ID, "pi"); err != nil {
		t.Fatalf("UpdateSessionHarness: %v", err)
	}
	// A whole-row write from a stale snapshot must not clobber the harness.
	sess.Status = db.SessionStatusIdle
	if err := store.UpsertSession(ctx, sess); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}
	got, err := store.GetSession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.Harness != "pi" {
		t.Errorf("harness = %q; want %q", got.Harness, "pi")
	}
	byBoard, err := store.ListSessionsByBoard(ctx, b.ID)
	if err != nil || len(byBoard) != 1 || byBoard[0].Harness != "pi" {
		t.Errorf("ListSessionsByBoard = %+v, %v; want one session with harness pi", byBoard, err)
	}

	if err := store.UpdateSessionHarness(ctx, sess.ID, ""); err != nil {
		t.Fatalf("UpdateSessionHarness(clear): %v", err)
	}
	if got, _ := store.GetSessionByTicket(ctx, tk.ID); got == nil || got.Harness != "" {
		t.Errorf("harness after clear = %+v; want empty", got)
	}

	if err := store.UpdateSessionHarness(ctx, 9999, "pi"); !errors.Is(err, db.ErrNotFound) {
		t.Errorf("UpdateSessionHarness(unknown) error = %v; want ErrNotFound", err)
	}
}

// TestResetSessionActivity checks only hook-reported activity is reset.
func TestResetSessionActivity(t *testing.T) {
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open(:memory:): %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	ctx := t.Context()

	b := &db.Board{Name: "Activity", Slug: "activity", BaseBranch: "main", RepoPath: "/tmp/x"}
	if err := store.CreateBoard(ctx, b); err != nil {
		t.Fatalf("CreateBoard: %v", err)
	}
	cols, err := store.ListColumns(ctx, b.ID)
	if err != nil {
		t.Fatalf("ListColumns: %v", err)
	}
	for _, tc := range []struct {
		status string
		want   string
	}{
		{db.SessionStatusWorking, db.SessionStatusIdle},
		{db.SessionStatusAwaitingPerm, db.SessionStatusIdle},
		{db.SessionStatusIdle, db.SessionStatusIdle},
		{db.SessionStatusStopped, db.SessionStatusStopped},
		{db.SessionStatusStarting, db.SessionStatusStarting},
		{db.SessionStatusError, db.SessionStatusError},
	} {
		tk := &db.Ticket{BoardID: b.ID, ColumnID: cols[0].ID, Title: tc.status, Slug: tc.status}
		if err := store.CreateTicket(ctx, tk); err != nil {
			t.Fatalf("CreateTicket: %v", err)
		}
		sess := &db.Session{TicketID: tk.ID, WorktreePath: "/tmp/wt-" + tc.status, Status: tc.status}
		if err := store.UpsertSession(ctx, sess); err != nil {
			t.Fatalf("UpsertSession: %v", err)
		}
		changed, err := store.ResetSessionActivity(ctx, sess.ID)
		if err != nil {
			t.Fatalf("ResetSessionActivity(%s): %v", tc.status, err)
		}
		got, err := store.GetSession(ctx, sess.ID)
		if err != nil {
			t.Fatalf("GetSession: %v", err)
		}
		if got.Status != tc.want || changed != (tc.status != tc.want) {
			t.Errorf("%s: status %q, changed %v; want %q, changed %v", tc.status, got.Status, changed, tc.want, tc.status != tc.want)
		}
	}
}

// TestMigrateAddsSessionHarness opens a database whose sessions table
// predates the harness column and checks Open adds it.
func TestMigrateAddsSessionHarness(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kanban.db")
	store, err := db.Open(path)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	if _, err := store.DB().Exec(`ALTER TABLE sessions DROP COLUMN harness`); err != nil {
		t.Fatalf("drop harness: %v", err)
	}
	_ = store.Close()

	store, err = db.Open(path)
	if err != nil {
		t.Fatalf("db.Open (migrate): %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if _, err := store.DB().Exec(`SELECT harness FROM sessions`); err != nil {
		t.Errorf("sessions.harness missing after migrate: %v", err)
	}
}
