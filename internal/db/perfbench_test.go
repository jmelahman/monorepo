//go:build perfbench

// Run:
//
//	go test -tags=perfbench -run TestPerfReport -v ./internal/db/
//
// Seeds an in-memory SQLite at three sizes, times each hot query, then drops
// the idx_* indexes added by schema.sql and re-times to get the no-index
// baseline. Prints a side-by-side table.
package db

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestPerfReport(t *testing.T) {
	for _, n := range []int{100, 1000, 5000} {
		withIdx := timeQueries(t, n, false)
		noIdx := timeQueries(t, n, true)

		fmt.Printf("\n=== n=%d tickets ===\n", n)
		fmt.Printf("%-15s %12s %12s %10s\n", "query", "no-index", "indexed", "speedup")
		for _, q := range hotQueries() {
			a, b := noIdx[q.name], withIdx[q.name]
			fmt.Printf("%-15s %12s %12s %9.1fx\n", q.name, a, b, float64(a)/float64(b))
		}
	}
}

func timeQueries(t *testing.T, tickets int, dropIdx bool) map[string]time.Duration {
	t.Helper()
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	boardID, sessionID := seedBench(t, store, tickets)
	if dropIdx {
		dropIdxIndexes(t, store)
	}

	ctx := context.Background()
	const iters = 200
	out := map[string]time.Duration{}
	for _, q := range hotQueries() {
		for i := 0; i < 3; i++ {
			_ = q.run(ctx, store, boardID, sessionID)
		}
		start := time.Now()
		for i := 0; i < iters; i++ {
			if err := q.run(ctx, store, boardID, sessionID); err != nil {
				t.Fatalf("%s: %v", q.name, err)
			}
		}
		out[q.name] = time.Since(start) / iters
	}
	return out
}

type hotQuery struct {
	name string
	run  func(ctx context.Context, store *Store, boardID, sessionID int64) error
}

func hotQueries() []hotQuery {
	return []hotQuery{
		{"MaxPosition", func(ctx context.Context, s *Store, _, _ int64) error {
			_, err := s.MaxTicketPosition(ctx, 1)
			return err
		}},
		{"ListTickets", func(ctx context.Context, s *Store, b, _ int64) error {
			_, err := s.ListTickets(ctx, b)
			return err
		}},
		{"ListArchived", func(ctx context.Context, s *Store, b, _ int64) error {
			_, err := s.ListArchivedTickets(ctx, b)
			return err
		}},
		{"ListTaskRuns", func(ctx context.Context, s *Store, _, sess int64) error {
			_, err := s.ListTaskRuns(ctx, sess)
			return err
		}},
		{"ListHooks", func(ctx context.Context, s *Store, b, _ int64) error {
			_, err := s.ListHooks(ctx, &b, "event-1")
			return err
		}},
		{"ListPorts", func(ctx context.Context, s *Store, _, sess int64) error {
			_, err := s.ListPorts(ctx, sess)
			return err
		}},
		{"AllocPort", func(ctx context.Context, s *Store, _, _ int64) error {
			_, err := s.AllocateHostPort(ctx, 13000, 13099)
			return err
		}},
	}
}

func seedBench(t *testing.T, store *Store, tickets int) (boardID, sessionID int64) {
	t.Helper()
	ctx := context.Background()
	b := &Board{Name: "bench", Slug: "bench", BaseBranch: "main"}
	if err := store.CreateBoard(ctx, b); err != nil {
		t.Fatal(err)
	}
	cols, err := store.ListColumns(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < tickets; i++ {
		col := cols[i%len(cols)]
		tk := &Ticket{BoardID: b.ID, ColumnID: col.ID, Title: fmt.Sprintf("t-%d", i), Slug: fmt.Sprintf("t-%d", i)}
		if err := store.CreateTicket(ctx, tk); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			s := &Session{TicketID: tk.ID, WorktreePath: "/tmp/x", BranchName: "x", Status: SessionStatusStopped}
			if err := store.UpsertSession(ctx, s); err != nil {
				t.Fatal(err)
			}
			sessionID = s.ID
			for j := 0; j < 200; j++ {
				tr := &TaskRun{SessionID: s.ID, TaskLabel: "t", Command: "c", Status: TaskRunStatusExited}
				if err := store.CreateTaskRun(ctx, tr); err != nil {
					t.Fatal(err)
				}
			}
			for j := 0; j < 30; j++ {
				p := &PortAllocation{SessionID: s.ID, Label: fmt.Sprintf("l%d", j), ContainerPort: 3000 + j, HostPort: 13000 + j}
				if err := store.CreatePort(ctx, p); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	for i := 0; i < 50; i++ {
		_, err := store.DB().ExecContext(ctx,
			`INSERT INTO hook_configs (board_id, event, command, enabled) VALUES (?, ?, ?, 1)`,
			b.ID, fmt.Sprintf("event-%d", i%10), fmt.Sprintf("noop-%d", i))
		if err != nil {
			t.Fatal(err)
		}
	}
	return b.ID, sessionID
}

func dropIdxIndexes(t *testing.T, store *Store) {
	t.Helper()
	ctx := context.Background()
	rows, err := store.DB().QueryContext(ctx,
		`SELECT name FROM sqlite_master WHERE type='index' AND name LIKE 'idx_%'`)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		names = append(names, n)
	}
	rows.Close()
	for _, n := range names {
		if _, err := store.DB().ExecContext(ctx, `DROP INDEX `+n); err != nil {
			t.Fatal(err)
		}
	}
}
