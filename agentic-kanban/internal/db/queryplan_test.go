package db

import (
	"context"
	"strings"
	"testing"
)

// TestQueryPlansUseIndexes asserts that each hot query is index-backed by
// inspecting EXPLAIN QUERY PLAN output for the SCAN keyword (SQLite's marker
// for a full-table scan). It runs against an empty in-memory DB; the planner
// still emits accurate plan choices because it relies on schema heuristics,
// not just row counts.
//
// If this fails, someone added a query path that bypasses an index, or
// dropped an index from schema.sql. The benchmark in perfbench_test.go
// quantifies the impact; this test gates it.
func TestQueryPlansUseIndexes(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	cases := []struct {
		name  string
		query string
		args  []any
	}{
		{
			"MaxTicketPosition",
			`SELECT MAX(position) FROM tickets WHERE column_id=? AND archived_at IS NULL`,
			[]any{1},
		},
		{
			"ListTickets",
			`SELECT id FROM tickets WHERE board_id=? AND archived_at IS NULL ORDER BY column_id, position`,
			[]any{1},
		},
		{
			"ListArchivedTickets",
			`SELECT id FROM tickets WHERE board_id=? AND archived_at IS NOT NULL ORDER BY archived_at DESC`,
			[]any{1},
		},
		{
			"FindOpenTicketByFingerprint",
			`SELECT id FROM tickets WHERE board_id=? AND fingerprint=? AND archived_at IS NULL LIMIT 1`,
			[]any{1, "fp"},
		},
		{
			"ListTaskRuns",
			`SELECT id FROM task_runs WHERE session_id=? ORDER BY id DESC`,
			[]any{1},
		},
		{
			"ListHooks",
			`SELECT id FROM hook_configs WHERE enabled=1 AND event=? AND (board_id IS NULL OR board_id=?)`,
			[]any{"x", 1},
		},
		{
			"ListPorts",
			`SELECT id FROM port_allocations WHERE session_id=?`,
			[]any{1},
		},
		{
			"AllocateHostPort",
			`SELECT host_port FROM port_allocations WHERE host_port BETWEEN ? AND ?`,
			[]any{13000, 13099},
		},
		{
			"GetSessionByTicket",
			`SELECT id FROM sessions WHERE ticket_id=?`,
			[]any{1},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := explain(t, store, tc.query, tc.args...)
			for _, line := range plan {
				if strings.HasPrefix(line, "SCAN ") {
					t.Errorf("query plan does a full table SCAN — missing index?\n  query: %s\n  plan:\n    %s",
						tc.query, strings.Join(plan, "\n    "))
					return
				}
			}
		})
	}
}

func explain(t *testing.T, store *Store, query string, args ...any) []string {
	t.Helper()
	rows, err := store.DB().QueryContext(context.Background(), "EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var id, parent, notUsed int
		var detail string
		if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
			t.Fatal(err)
		}
		lines = append(lines, detail)
	}
	return lines
}
