package api

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/errreport"
)

// TestHTTPError_SkipsCanceledContext pins that a request whose context was
// canceled (client disconnect) does not file an error ticket and does not
// write a response body. context.Canceled is client-side noise, not a 5xx
// the operator needs to see.
func TestHTTPError_SkipsCanceledContext(t *testing.T) {
	store, err := db.Open(filepath.Join(t.TempDir(), "kanban.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	reporter := errreport.New(store, errreport.Config{Enabled: true, BoardName: "Errors"})
	h := &handlers{store: store, reporter: reporter}

	rec := httptest.NewRecorder()
	h.httpError(rec, fmt.Errorf("archive: %w", context.Canceled), 500)

	if rec.Code != 200 {
		t.Fatalf("status = %d; want 200 (untouched recorder default)", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("body should be empty; got %q", rec.Body.String())
	}

	board, err := store.GetBoardBySlug(context.Background(), "errors")
	if err == nil && board != nil {
		tickets, _ := store.ListTickets(context.Background(), board.ID)
		if len(tickets) != 0 {
			t.Fatalf("context.Canceled must not file a ticket; got %d", len(tickets))
		}
	}
}

// TestHTTPError_ReportsNonCanceled is the inverse: a real 5xx error must
// still get reported, so the canceled-context short-circuit doesn't silence
// genuine server bugs.
func TestHTTPError_ReportsNonCanceled(t *testing.T) {
	store, err := db.Open(filepath.Join(t.TempDir(), "kanban.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	reporter := errreport.New(store, errreport.Config{Enabled: true, BoardName: "Errors"})
	h := &handlers{store: store, reporter: reporter}

	rec := httptest.NewRecorder()
	h.httpError(rec, errors.New("disk full"), 500)

	if rec.Code != 500 {
		t.Fatalf("status = %d; want 500", rec.Code)
	}
	board, err := store.GetBoardBySlug(context.Background(), "errors")
	if err != nil {
		t.Fatalf("expected errors board: %v", err)
	}
	tickets, _ := store.ListTickets(context.Background(), board.ID)
	if len(tickets) != 1 {
		t.Fatalf("expected 1 ticket from real 5xx; got %d", len(tickets))
	}
}

// TestHTTPError_SkipsBadGateway pins that a 502 (the code prDetail returns when
// the GitHub API is unreachable) still writes its error response but does not
// file a ticket. A GitHub outage or the operator going offline is a transient
// upstream failure, not a server bug — reporting it just spams the errors board.
func TestHTTPError_SkipsBadGateway(t *testing.T) {
	store, err := db.Open(filepath.Join(t.TempDir(), "kanban.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	reporter := errreport.New(store, errreport.Config{Enabled: true, BoardName: "Errors"})
	h := &handlers{store: store, reporter: reporter}

	rec := httptest.NewRecorder()
	h.httpError(rec, errors.New("github unreachable: dial tcp: no route to host"), 502)

	if rec.Code != 502 {
		t.Fatalf("status = %d; want 502 (client still gets the error)", rec.Code)
	}
	board, err := store.GetBoardBySlug(context.Background(), "errors")
	if err == nil && board != nil {
		tickets, _ := store.ListTickets(context.Background(), board.ID)
		if len(tickets) != 0 {
			t.Fatalf("502 must not file a ticket; got %d", len(tickets))
		}
	}
}
