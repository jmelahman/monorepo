package tasks

import (
	"context"
	"errors"
	"testing"

	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/hooks"
)

// newOrphanEnv seeds a session and returns a runner with no docker client,
// so every container call goes through the stubs the test installs.
func newOrphanEnv(t *testing.T, containerID string) (*Runner, *db.Store, *db.Session) {
	t.Helper()
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
	sess := &db.Session{TicketID: tk.ID, WorktreePath: "/w/t", BranchName: "t", Status: db.SessionStatusIdle, ContainerID: &containerID}
	if err := store.UpsertSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	return NewRunner(store, nil, hooks.NewRunner(store)), store, sess
}

// seedRun inserts a running row the way an earlier server process would
// have left it: in the DB, but not watched by this runner.
func seedRun(t *testing.T, store *db.Store, sess *db.Session, execID string) *db.TaskRun {
	t.Helper()
	tr := &db.TaskRun{SessionID: sess.ID, TaskLabel: "web", Command: "npm run dev", Status: db.TaskRunStatusRunning}
	if execID != "" {
		tr.ExecID = &execID
	}
	if err := store.CreateTaskRun(t.Context(), tr); err != nil {
		t.Fatal(err)
	}
	return tr
}

func runStatus(t *testing.T, store *db.Store, id int64) (string, *int) {
	t.Helper()
	got, err := store.GetTaskRun(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	return got.Status, got.ExitCode
}

// TestReconcileOrphanedRuns: a run left running by a previous server process
// has no goroutine to record its exit, so the list has to ask docker.
func TestReconcileOrphanedRuns(t *testing.T) {
	three := 3
	cases := []struct {
		name      string
		container string
		running   bool
		code      *int
		err       error
		want      string
		wantCode  *int
	}{
		{name: "exec_gone", container: "c1", want: db.TaskRunStatusStopped},
		{name: "exec_finished", container: "c1", code: &three, want: db.TaskRunStatusExited, wantCode: &three},
		{name: "exec_still_running", container: "c1", running: true, want: db.TaskRunStatusRunning},
		{name: "inspect_error_is_not_evidence", container: "c1", err: errors.New("daemon unreachable"), want: db.TaskRunStatusRunning},
		{name: "no_container", container: "", want: db.TaskRunStatusStopped},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, store, sess := newOrphanEnv(t, tc.container)
			r.execState = func(context.Context, string) (bool, *int, error) { return tc.running, tc.code, tc.err }
			tr := seedRun(t, store, sess, "exec-1")
			if err := r.Reconcile(t.Context(), sess); err != nil {
				t.Fatal(err)
			}
			status, code := runStatus(t, store, tr.ID)
			if status != tc.want || (code == nil) != (tc.wantCode == nil) || (code != nil && *code != *tc.wantCode) {
				t.Errorf("run = %s %v, want %s %v", status, code, tc.want, tc.wantCode)
			}
		})
	}

	t.Run("live_run_is_left_to_its_goroutine", func(t *testing.T) {
		r, store, sess := newOrphanEnv(t, "c1")
		r.execState = func(context.Context, string) (bool, *int, error) {
			t.Error("inspected a run this process is watching")
			return false, nil, nil
		}
		tr := seedRun(t, store, sess, "exec-1")
		r.live[tr.ID] = true
		if err := r.Reconcile(t.Context(), sess); err != nil {
			t.Fatal(err)
		}
		if status, _ := runStatus(t, store, tr.ID); status != db.TaskRunStatusRunning {
			t.Errorf("status = %s, want running", status)
		}
	})
}

// TestStopOrphanedRun: stopping a run no goroutine is watching has to record
// the stop itself, or the run reads as running forever.
func TestStopOrphanedRun(t *testing.T) {
	t.Run("orphan_in_live_container", func(t *testing.T) {
		r, store, sess := newOrphanEnv(t, "c1")
		var stopped []int64
		r.stopExec = func(_ context.Context, _ string, id int64) error { stopped = append(stopped, id); return nil }
		tr := seedRun(t, store, sess, "exec-1")
		if err := r.Stop(t.Context(), sess, tr); err != nil {
			t.Fatal(err)
		}
		if len(stopped) != 1 || stopped[0] != tr.ID {
			t.Errorf("stop script ran for %v, want [%d]", stopped, tr.ID)
		}
		if status, _ := runStatus(t, store, tr.ID); status != db.TaskRunStatusStopped {
			t.Errorf("status = %s, want stopped", status)
		}
	})

	t.Run("live_run_waits_for_its_goroutine", func(t *testing.T) {
		r, store, sess := newOrphanEnv(t, "c1")
		r.stopExec = func(context.Context, string, int64) error { return nil }
		tr := seedRun(t, store, sess, "exec-1")
		r.live[tr.ID] = true
		if err := r.Stop(t.Context(), sess, tr); err != nil {
			t.Fatal(err)
		}
		if status, _ := runStatus(t, store, tr.ID); status != db.TaskRunStatusRunning {
			t.Errorf("status = %s, want running until the exec's output ends", status)
		}
	})

	t.Run("no_container", func(t *testing.T) {
		r, store, sess := newOrphanEnv(t, "")
		r.stopExec = func(context.Context, string, int64) error {
			t.Error("ran the stop script without a container")
			return nil
		}
		tr := seedRun(t, store, sess, "exec-1")
		if err := r.Stop(t.Context(), sess, tr); err != nil {
			t.Fatal(err)
		}
		if status, _ := runStatus(t, store, tr.ID); status != db.TaskRunStatusStopped {
			t.Errorf("status = %s, want stopped", status)
		}
	})

	t.Run("orphan_in_removed_container", func(t *testing.T) {
		r, store, sess := newOrphanEnv(t, "c1")
		r.stopExec = func(context.Context, string, int64) error { return errors.New("no such container") }
		r.containerRunning = func(context.Context, string) (bool, error) { return false, nil }
		tr := seedRun(t, store, sess, "exec-1")
		if err := r.Stop(t.Context(), sess, tr); err != nil {
			t.Fatal(err)
		}
		if status, _ := runStatus(t, store, tr.ID); status != db.TaskRunStatusStopped {
			t.Errorf("status = %s, want stopped", status)
		}
	})

	t.Run("live_run_in_removed_container_still_errors", func(t *testing.T) {
		r, store, sess := newOrphanEnv(t, "c1")
		r.stopExec = func(context.Context, string, int64) error { return errors.New("no such container") }
		r.containerRunning = func(context.Context, string) (bool, error) { return false, nil }
		tr := seedRun(t, store, sess, "exec-1")
		r.live[tr.ID] = true
		if err := r.Stop(t.Context(), sess, tr); err == nil {
			t.Error("Stop succeeded for a live run whose stop script failed")
		}
	})

	t.Run("stop_script_failure_leaves_run", func(t *testing.T) {
		r, store, sess := newOrphanEnv(t, "c1")
		r.stopExec = func(context.Context, string, int64) error { return errors.New("exec failed") }
		r.containerRunning = func(context.Context, string) (bool, error) { return true, nil }
		tr := seedRun(t, store, sess, "exec-1")
		if err := r.Stop(t.Context(), sess, tr); err == nil {
			t.Error("Stop succeeded despite the stop script failing")
		}
		if status, _ := runStatus(t, store, tr.ID); status != db.TaskRunStatusRunning {
			t.Errorf("status = %s, want running", status)
		}
	})
}
