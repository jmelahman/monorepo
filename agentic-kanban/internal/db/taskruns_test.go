package db_test

import (
	"testing"

	"github.com/jmelahman/kanban/internal/db"
)

// TestSetTaskRunExecID guards the stop path: the runner creates the row
// before the docker exec exists, so the exec id has to be written back or
// DELETE /api/task-runs/{id} reads a nil id and silently signals nothing.
func TestSetTaskRunExecID(t *testing.T) {
	ctx := t.Context()
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	board := &db.Board{Name: "B", Slug: "b", RepoPath: "/r", WorktreeRoot: "/w", BaseBranch: "main"}
	if err := store.CreateBoard(ctx, board); err != nil {
		t.Fatal(err)
	}
	cols, err := store.ListColumns(ctx, board.ID)
	if err != nil || len(cols) == 0 {
		t.Fatalf("columns: %v %v", cols, err)
	}
	tk := &db.Ticket{BoardID: board.ID, ColumnID: cols[0].ID, Title: "T", Slug: "t"}
	if err := store.CreateTicket(ctx, tk); err != nil {
		t.Fatal(err)
	}
	sess := &db.Session{TicketID: tk.ID, WorktreePath: "/w/t", BranchName: "t", Status: db.SessionStatusIdle}
	if err := store.UpsertSession(ctx, sess); err != nil {
		t.Fatal(err)
	}

	tr := &db.TaskRun{SessionID: sess.ID, TaskLabel: "web", Command: "npm run dev", Status: db.TaskRunStatusRunning}
	if err := store.CreateTaskRun(ctx, tr); err != nil {
		t.Fatal(err)
	}
	if err := store.SetTaskRunExecID(ctx, tr.ID, "exec-123"); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetTaskRun(ctx, tr.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ExecID == nil || *got.ExecID != "exec-123" {
		t.Fatalf("exec id = %v, want exec-123", got.ExecID)
	}
}
