package api_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmelahman/kanban/internal/db"
)

// ---------- Health ----------

func TestHealth(t *testing.T) {
	e := newEnv(t)
	resp := e.get("/api/health")
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 503 {
		t.Fatalf("status = %d; want 200 or 503", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.StatusCode == 200 && body["status"] != "ok" {
		t.Errorf("status field = %q; want 'ok'", body["status"])
	}
	if resp.StatusCode == 503 && body["docker"] == "" {
		t.Errorf("503 should report a docker error reason")
	}
}

// ---------- Boards ----------

func TestBoards_Lifecycle(t *testing.T) {
	e := newEnv(t)

	t.Run("list_empty_returns_array", func(t *testing.T) {
		resp := e.get("/api/boards")
		assertStatus(t, resp, 200)
		body := readBody(t, resp)
		if strings.TrimSpace(string(body)) != "[]" {
			t.Errorf("empty list = %s; want []", body)
		}
	})

	t.Run("create_happy_path", func(t *testing.T) {
		resp := e.post("/api/boards", map[string]any{
			"name":             "My Project",
			"source_repo_path": e.repoPath,
			"base_branch":      "main",
		})
		assertStatus(t, resp, 201)
		b := decodeJSON[db.Board](t, resp)
		if b.ID == 0 || b.Slug != "my-project" || b.Name != "My Project" {
			t.Errorf("unexpected board: %+v", b)
		}
		if b.WorktreeRoot == "" {
			t.Errorf("worktree_root should default when blank")
		}
	})

	t.Run("create_missing_name_returns_400", func(t *testing.T) {
		resp := e.post("/api/boards", map[string]any{"source_repo_path": e.repoPath})
		assertStatus(t, resp, 400)
	})

	t.Run("create_invalid_json_returns_400", func(t *testing.T) {
		resp := e.post("/api/boards", "not json")
		assertStatus(t, resp, 400)
	})

	t.Run("get_404", func(t *testing.T) {
		resp := e.get("/api/boards/9999")
		assertStatus(t, resp, 404)
	})

	t.Run("get_happy_path", func(t *testing.T) {
		b := e.seedBoard("Onyx")
		resp := e.get(fmt.Sprintf("/api/boards/%d", b.ID))
		assertStatus(t, resp, 200)
		got := decodeJSON[db.Board](t, resp)
		if got.ID != b.ID || got.Name != "Onyx" {
			t.Errorf("got = %+v; want id=%d name=Onyx", got, b.ID)
		}
	})

	t.Run("state_includes_default_columns", func(t *testing.T) {
		b := e.seedBoard("WithColumns")
		resp := e.get(fmt.Sprintf("/api/boards/%d/state", b.ID))
		assertStatus(t, resp, 200)
		state := decodeJSON[map[string]json.RawMessage](t, resp)
		var cols []db.Column
		_ = json.Unmarshal(state["columns"], &cols)
		if len(cols) != 4 {
			t.Errorf("default columns = %d; want 4 (Backlog/In Progress/Review/Done)", len(cols))
		}
	})

	t.Run("state_404_when_board_missing", func(t *testing.T) {
		resp := e.get("/api/boards/9999/state")
		assertStatus(t, resp, 404)
	})
}

// ---------- Tickets ----------

func TestTickets_Lifecycle(t *testing.T) {
	e := newEnv(t)
	board := e.seedBoard("B")
	cols, _ := e.store.ListColumns(t.Context(), board.ID)

	t.Run("create_happy_path", func(t *testing.T) {
		resp := e.post(fmt.Sprintf("/api/boards/%d/tickets", board.ID), map[string]any{
			"column_id": cols[0].ID,
			"title":     "Add login",
			"body":      "details",
		})
		assertStatus(t, resp, 201)
		tk := decodeJSON[db.Ticket](t, resp)
		if tk.Slug != "add-login" || tk.BoardID != board.ID || tk.ColumnID != cols[0].ID {
			t.Errorf("ticket = %+v", tk)
		}
		if tk.Position == 0 {
			t.Errorf("position should auto-increment, got 0")
		}
	})

	t.Run("create_missing_title_400", func(t *testing.T) {
		resp := e.post(fmt.Sprintf("/api/boards/%d/tickets", board.ID),
			map[string]any{"column_id": cols[0].ID})
		assertStatus(t, resp, 400)
	})

	t.Run("create_unknown_board_404", func(t *testing.T) {
		resp := e.post("/api/boards/9999/tickets", map[string]any{
			"column_id": cols[0].ID, "title": "x",
		})
		assertStatus(t, resp, 404)
	})

	t.Run("move_happy_path", func(t *testing.T) {
		tk := e.seedTicket(board, "ToMove")
		resp := e.patch(fmt.Sprintf("/api/tickets/%d/move", tk.ID), map[string]any{
			"column_id": cols[1].ID, "position": 0,
		})
		assertStatus(t, resp, 204)
		got, _ := e.store.GetTicket(t.Context(), tk.ID)
		if got == nil || got.ColumnID != cols[1].ID {
			t.Errorf("ticket not moved: %+v", got)
		}
	})

	t.Run("move_invalid_json_400", func(t *testing.T) {
		resp := e.patch("/api/tickets/1/move", "{bad")
		assertStatus(t, resp, 400)
	})

	t.Run("archive_happy_path", func(t *testing.T) {
		tk := e.seedTicket(board, "ToArchive")
		resp := e.post(fmt.Sprintf("/api/tickets/%d/archive", tk.ID), nil)
		assertStatus(t, resp, 204)
		// Archived tickets disappear from board state
		stResp := e.get(fmt.Sprintf("/api/boards/%d/state", board.ID))
		st := decodeJSON[map[string]json.RawMessage](t, stResp)
		var tickets []db.Ticket
		_ = json.Unmarshal(st["tickets"], &tickets)
		for _, x := range tickets {
			if x.ID == tk.ID {
				t.Errorf("archived ticket %d still in state", tk.ID)
			}
		}
	})

	t.Run("archive_unknown_404", func(t *testing.T) {
		resp := e.post("/api/tickets/9999/archive", nil)
		assertStatus(t, resp, 404)
	})
}

// ---------- Sessions ----------

func TestSessions(t *testing.T) {
	e := newEnv(t)
	board := e.seedBoard("Sessions")
	tk := e.seedTicket(board, "ImplementX")

	t.Run("ensure_creates_worktree_and_row", func(t *testing.T) {
		resp := e.post(fmt.Sprintf("/api/tickets/%d/session", tk.ID), nil)
		assertStatus(t, resp, 201)
		sess := decodeJSON[db.Session](t, resp)
		if sess.TicketID != tk.ID || sess.Status != db.SessionStatusStopped {
			t.Errorf("unexpected session: %+v", sess)
		}
		if _, err := os.Stat(sess.WorktreePath); err != nil {
			t.Errorf("worktree dir missing: %v", err)
		}
		// Branch should exist in source repo
		out, err := runGit(e.repoPath, "branch", "--list", sess.BranchName)
		if err != nil || !strings.Contains(out, sess.BranchName) {
			t.Errorf("branch %q not in repo: out=%q err=%v", sess.BranchName, out, err)
		}
	})

	t.Run("ensure_idempotent", func(t *testing.T) {
		first := e.post(fmt.Sprintf("/api/tickets/%d/session", tk.ID), nil)
		second := e.post(fmt.Sprintf("/api/tickets/%d/session", tk.ID), nil)
		assertStatus(t, first, 201)
		assertStatus(t, second, 201)
		s1 := decodeJSON[db.Session](t, first)
		s2 := decodeJSON[db.Session](t, second)
		if s1.ID != s2.ID {
			t.Errorf("ensure not idempotent: %d != %d", s1.ID, s2.ID)
		}
	})

	t.Run("ensure_unknown_ticket_404", func(t *testing.T) {
		resp := e.post("/api/tickets/9999/session", nil)
		assertStatus(t, resp, 404)
	})

	t.Run("start_without_devcontainer_500", func(t *testing.T) {
		// Seed a session row pointing at a worktree without a .devcontainer.
		other := e.seedTicket(board, "NoDevcontainer")
		sess := e.seedSession(other)
		_ = os.MkdirAll(sess.WorktreePath, 0o755)
		resp := e.post(fmt.Sprintf("/api/sessions/%d/start", sess.ID), nil)
		assertStatus(t, resp, 500)
		var errBody map[string]string
		_ = json.Unmarshal(readBody(t, resp), &errBody)
		if errBody["error"] == "" {
			t.Errorf("expected error message in body")
		}
	})

	t.Run("stop_no_container_204", func(t *testing.T) {
		other := e.seedTicket(board, "Stopper")
		sess := e.seedSession(other)
		resp := e.post(fmt.Sprintf("/api/sessions/%d/stop", sess.ID), nil)
		assertStatus(t, resp, 204)
	})

	t.Run("stop_unknown_500", func(t *testing.T) {
		resp := e.post("/api/sessions/9999/stop", nil)
		// no session row → store returns ErrNotFound; handler maps to 500.
		assertStatus(t, resp, 500)
	})
}

// ---------- Tasks ----------

func TestTasks(t *testing.T) {
	e := newEnv(t)
	board := e.seedBoard("Tasks")
	tk := e.seedTicket(board, "DoTasks")
	sess := e.seedSession(tk)

	t.Run("discover_empty_worktree_returns_array", func(t *testing.T) {
		_ = os.MkdirAll(sess.WorktreePath, 0o755)
		resp := e.get(fmt.Sprintf("/api/sessions/%d/discover-tasks", sess.ID))
		assertStatus(t, resp, 200)
		body := readBody(t, resp)
		if strings.TrimSpace(string(body)) != "[]" {
			t.Errorf("empty discover = %s; want []", body)
		}
	})

	t.Run("discover_parses_tasks_json", func(t *testing.T) {
		_ = os.MkdirAll(filepath.Join(sess.WorktreePath, ".vscode"), 0o755)
		tasksJSON := `{
			"version": "2.0.0",
			"tasks": [
				{"label": "Start Frontend", "type": "shell", "command": "npm", "args": ["run", "dev"]}
			]
		}`
		if err := os.WriteFile(filepath.Join(sess.WorktreePath, ".vscode", "tasks.json"), []byte(tasksJSON), 0o644); err != nil {
			t.Fatal(err)
		}
		// Pair with .kanban.toml to verify port association
		toml := `[[task]]
label = "Start Frontend"
container_port = 3000
`
		if err := os.WriteFile(filepath.Join(sess.WorktreePath, ".kanban.toml"), []byte(toml), 0o644); err != nil {
			t.Fatal(err)
		}

		resp := e.get(fmt.Sprintf("/api/sessions/%d/discover-tasks", sess.ID))
		assertStatus(t, resp, 200)
		var found []map[string]any
		if err := json.Unmarshal(readBody(t, resp), &found); err != nil {
			t.Fatal(err)
		}
		if len(found) != 1 {
			t.Fatalf("found = %d tasks; want 1: %+v", len(found), found)
		}
		if found[0]["label"] != "Start Frontend" {
			t.Errorf("label = %v; want 'Start Frontend'", found[0]["label"])
		}
		if found[0]["has_port"] != true {
			t.Errorf("has_port = %v; want true", found[0]["has_port"])
		}
		if cp, _ := found[0]["container_port"].(float64); int(cp) != 3000 {
			t.Errorf("container_port = %v; want 3000", found[0]["container_port"])
		}
	})

	t.Run("discover_unknown_session_404", func(t *testing.T) {
		resp := e.get("/api/sessions/9999/discover-tasks")
		assertStatus(t, resp, 404)
	})

	t.Run("list_runs_empty", func(t *testing.T) {
		resp := e.get(fmt.Sprintf("/api/sessions/%d/task-runs", sess.ID))
		assertStatus(t, resp, 200)
		body := readBody(t, resp)
		if strings.TrimSpace(string(body)) != "[]" {
			t.Errorf("empty runs = %s; want []", body)
		}
	})

	t.Run("create_run_unknown_session_404", func(t *testing.T) {
		resp := e.post("/api/sessions/9999/task-runs", map[string]any{"label": "x"})
		assertStatus(t, resp, 404)
	})

	t.Run("create_run_unknown_label_404", func(t *testing.T) {
		// Worktree has tasks.json from previous subtest.
		resp := e.post(fmt.Sprintf("/api/sessions/%d/task-runs", sess.ID),
			map[string]any{"label": "Does Not Exist"})
		assertStatus(t, resp, 404)
	})

	t.Run("stop_run_unknown_404", func(t *testing.T) {
		resp := e.delete("/api/task-runs/9999")
		assertStatus(t, resp, 404)
	})
}

// ---------- Ports ----------

func TestPorts(t *testing.T) {
	e := newEnv(t)
	board := e.seedBoard("Ports")
	tk := e.seedTicket(board, "P")
	sess := e.seedSession(tk)

	t.Run("list_empty", func(t *testing.T) {
		resp := e.get(fmt.Sprintf("/api/sessions/%d/ports", sess.ID))
		assertStatus(t, resp, 200)
		body := readBody(t, resp)
		if strings.TrimSpace(string(body)) != "[]" {
			t.Errorf("empty ports = %s; want []", body)
		}
	})

	t.Run("list_with_seeded_port", func(t *testing.T) {
		e.seedPort(sess, "frontend", 3000, 13000)
		resp := e.get(fmt.Sprintf("/api/sessions/%d/ports", sess.ID))
		assertStatus(t, resp, 200)
		var ports []db.PortAllocation
		if err := json.Unmarshal(readBody(t, resp), &ports); err != nil {
			t.Fatal(err)
		}
		if len(ports) != 1 || ports[0].HostPort != 13000 {
			t.Errorf("ports = %+v", ports)
		}
	})

	t.Run("create_port_session_not_running_500", func(t *testing.T) {
		// Session has no bridge IP registered → startProxy fails.
		other := e.seedTicket(board, "P2")
		sess2 := e.seedSession(other)
		resp := e.post(fmt.Sprintf("/api/sessions/%d/ports", sess2.ID),
			map[string]any{"label": "x", "container_port": 8080})
		assertStatus(t, resp, 500)
	})

	t.Run("create_port_missing_field_400", func(t *testing.T) {
		resp := e.post(fmt.Sprintf("/api/sessions/%d/ports", sess.ID),
			map[string]any{"label": "x"})
		assertStatus(t, resp, 400)
	})

	t.Run("create_port_unknown_session_404", func(t *testing.T) {
		resp := e.post("/api/sessions/9999/ports",
			map[string]any{"label": "x", "container_port": 8080})
		assertStatus(t, resp, 404)
	})

	t.Run("delete_port_204", func(t *testing.T) {
		other := e.seedTicket(board, "P3")
		sess3 := e.seedSession(other)
		p := e.seedPort(sess3, "x", 4000, 13050)
		resp := e.delete(fmt.Sprintf("/api/ports/%d", p.ID))
		assertStatus(t, resp, 204)
		left, _ := e.store.ListPorts(t.Context(), sess3.ID)
		if len(left) != 0 {
			t.Errorf("port not deleted: %+v", left)
		}
	})
}

// ---------- Static frontend ----------

func TestStatic(t *testing.T) {
	e := newEnv(t)

	t.Run("unknown_api_path_404", func(t *testing.T) {
		// Any /api/* route not in the mux should 404, not fall through to index.html.
		resp := e.get("/api/no-such-route")
		assertStatus(t, resp, 404)
	})
}

// ---------- helpers ----------

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}
