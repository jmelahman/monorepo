package api

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/jmelahman/kanban/internal/db"
)

// TestPublishSessionUpdatedRefetchesFromDB pins the refetch contract:
// publishSessionUpdated must read the current row from the DB and publish
// that, not whatever in-memory snapshot a caller happens to be holding.
//
// Regression target: the GitHub PR poller persists pr_number / pr_url to
// the sessions row while other handlers may have an older snapshot of the
// same row in scope. Pre-fix the publish accepted a *db.Session and
// emitted the caller's stale copy, so the frontend would overwrite a
// session that previously had a PR with one that has none, hiding the
// header link until a hard refresh. Post-fix it accepts the session id
// and refetches.
func TestPublishSessionUpdatedRefetchesFromDB(t *testing.T) {
	store, err := db.Open(filepath.Join(t.TempDir(), "kanban.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	ctx := context.Background()

	board := &db.Board{Name: "t", Slug: "t", RepoPath: "/tmp/r", BaseBranch: "main"}
	if err := store.CreateBoard(ctx, board); err != nil {
		t.Fatal(err)
	}
	cols, err := store.ListColumns(ctx, board.ID)
	if err != nil {
		t.Fatal(err)
	}
	tk := &db.Ticket{BoardID: board.ID, ColumnID: cols[0].ID, Title: "t", Slug: "t"}
	if err := store.CreateTicket(ctx, tk); err != nil {
		t.Fatal(err)
	}
	sess := &db.Session{TicketID: tk.ID, BranchName: "kanban/test/t", Status: db.SessionStatusStopped}
	if err := store.UpsertSession(ctx, sess); err != nil {
		t.Fatal(err)
	}

	// Simulate the github poller's writes landing in the DB after a
	// handler has already loaded its in-memory copy of sess.
	prNum := int64(42)
	if err := store.UpdateSessionPR(ctx, sess.ID, "open", &prNum, "https://github.com/x/y/pull/42", "Fix"); err != nil {
		t.Fatal(err)
	}

	bus := NewEventBus()
	ch, cancel := bus.subscribe(board.ID)
	defer cancel()

	h := &handlers{store: store, bus: bus}
	h.publishSessionUpdated(ctx, sess.ID)

	select {
	case ev := <-ch:
		if ev.Type != "session_updated" {
			t.Fatalf("event type = %q; want session_updated", ev.Type)
		}
		// Marshal through JSON to assert the wire shape (the SSE handler
		// serializes the same way).
		payload, err := json.Marshal(ev.Data)
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]any
		if err := json.Unmarshal(payload, &got); err != nil {
			t.Fatal(err)
		}
		if got["pr_number"] == nil {
			t.Errorf("payload missing pr_number; got %s", payload)
		}
		if got["pr_url"] != "https://github.com/x/y/pull/42" {
			t.Errorf("payload pr_url = %v; want the PR url", got["pr_url"])
		}
		if got["pr_state"] != "open" {
			t.Errorf("payload pr_state = %v; want open", got["pr_state"])
		}
	default:
		t.Fatalf("no event published")
	}
}
