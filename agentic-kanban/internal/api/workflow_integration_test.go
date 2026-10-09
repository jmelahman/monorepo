package api_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/jmelahman/kanban/internal/db"
)

// TestFreshInstall_BoardAndTicketWorkflow walks the full happy-path flow a
// user hits the first time they launch the app: empty boards list, create a
// board, observe the four default columns, create three tickets, move one
// across columns, archive another, and verify the resulting board state.
//
// The point is to exercise the empty-DB code paths together, in order, so a
// regression in any of them shows up here rather than on a real fresh
// install.
func TestFreshInstall_BoardAndTicketWorkflow(t *testing.T) {
	e := newEnv(t)

	// 1. Empty boards list serializes as [].
	resp := e.get("/api/boards")
	assertStatus(t, resp, 200)
	if got := strings.TrimSpace(string(readBody(t, resp))); got != "[]" {
		t.Fatalf("empty /api/boards = %q; want []", got)
	}

	// 2. Create a board.
	resp = e.post("/api/boards", map[string]any{
		"name":        "Project Alpha",
		"repo_path":   e.repoPath,
		"base_branch": "main",
	})
	assertStatus(t, resp, 201)
	board := decodeJSON[db.Board](t, resp)
	if board.ID == 0 || board.Slug != "project-alpha" {
		t.Fatalf("created board = %+v; want id != 0 and slug 'project-alpha'", board)
	}

	// 3. /state shows the four default columns and zero tickets.
	state := decodeJSON[map[string]json.RawMessage](t,
		e.get(fmt.Sprintf("/api/boards/%d/state", board.ID)))
	var cols []db.Column
	if err := json.Unmarshal(state["columns"], &cols); err != nil {
		t.Fatalf("decode columns: %v", err)
	}
	wantCols := []string{"Backlog", "In Progress", "Review", "Done"}
	if len(cols) != len(wantCols) {
		t.Fatalf("default columns = %d; want %d (%v)", len(cols), len(wantCols), wantCols)
	}
	for i, c := range cols {
		if c.Name != wantCols[i] {
			t.Errorf("column[%d] = %q; want %q", i, c.Name, wantCols[i])
		}
	}
	var tickets []db.Ticket
	if err := json.Unmarshal(state["tickets"], &tickets); err != nil {
		t.Fatalf("decode tickets: %v", err)
	}
	if len(tickets) != 0 {
		t.Errorf("fresh board tickets = %d; want 0", len(tickets))
	}

	// 4. Create three tickets in the leftmost (Backlog) column.
	titles := []string{"Setup CI", "Add login", "Wire metrics"}
	created := make([]db.Ticket, 0, len(titles))
	for _, title := range titles {
		r := e.post(fmt.Sprintf("/api/boards/%d/tickets", board.ID),
			map[string]any{"title": title})
		assertStatus(t, r, 201)
		tk := decodeJSON[db.Ticket](t, r)
		if tk.ColumnID != cols[0].ID {
			t.Errorf("ticket %q column = %d; want leftmost %d", title, tk.ColumnID, cols[0].ID)
		}
		created = append(created, tk)
	}

	// 5. Move the first ticket from Backlog → In Progress.
	inProgress := cols[1]
	r := e.patch(fmt.Sprintf("/api/tickets/%d/move", created[0].ID),
		map[string]any{"column_id": inProgress.ID, "position": 0})
	assertStatus(t, r, 204)

	// 6. Archive the third ticket.
	r = e.post(fmt.Sprintf("/api/tickets/%d/archive", created[2].ID), nil)
	assertStatus(t, r, 204)

	// 7. Final state: two active tickets, one in Backlog, one in In Progress;
	//    archived ticket is excluded from /state.
	state = decodeJSON[map[string]json.RawMessage](t,
		e.get(fmt.Sprintf("/api/boards/%d/state", board.ID)))
	var got []db.Ticket
	if err := json.Unmarshal(state["tickets"], &got); err != nil {
		t.Fatalf("decode final tickets: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("active tickets = %d; want 2 (%+v)", len(got), got)
	}
	byCol := map[int64]int{}
	for _, tk := range got {
		byCol[tk.ColumnID]++
		if tk.ID == created[2].ID {
			t.Errorf("archived ticket %d still in /state", tk.ID)
		}
	}
	if byCol[cols[0].ID] != 1 {
		t.Errorf("Backlog tickets = %d; want 1", byCol[cols[0].ID])
	}
	if byCol[inProgress.ID] != 1 {
		t.Errorf("In Progress tickets = %d; want 1", byCol[inProgress.ID])
	}

	// 8. /api/boards now lists the one we created.
	listed := decodeJSON[[]db.Board](t, e.get("/api/boards"))
	if len(listed) != 1 || listed[0].ID != board.ID {
		t.Errorf("/api/boards = %+v; want single board id %d", listed, board.ID)
	}
}
