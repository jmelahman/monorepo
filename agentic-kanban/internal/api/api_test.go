package api_test

import (
	"context"
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
	if resp.StatusCode == 503 && body["docker"] == "" && body["db"] == "" {
		t.Errorf("503 should report a docker or db error reason")
	}
}

func TestHealth_FailsWhenDBClosed(t *testing.T) {
	e := newEnv(t)
	// Closing the DB simulates a wedged/unreachable database — Ping should fail.
	if err := e.store.DB().Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	resp := e.get("/api/health")
	defer resp.Body.Close()
	if resp.StatusCode != 503 {
		t.Fatalf("status = %d; want 503", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["db"] == "" {
		t.Errorf("503 should report a db error reason; got %+v", body)
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
			"name":        "My Project",
			"repo_path":   e.repoPath,
			"base_branch": "main",
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
		resp := e.post("/api/boards", map[string]any{"repo_path": e.repoPath})
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

	t.Run("git_author_round_trip", func(t *testing.T) {
		resp := e.post("/api/boards", map[string]any{
			"name":             "Identity Project",
			"repo_path":        e.repoPath,
			"git_author_name":  "Ada Lovelace",
			"git_author_email": "ada@example.com",
		})
		assertStatus(t, resp, 201)
		b := decodeJSON[db.Board](t, resp)
		if b.GitAuthorName != "Ada Lovelace" || b.GitAuthorEmail != "ada@example.com" {
			t.Fatalf("create: got name=%q email=%q", b.GitAuthorName, b.GitAuthorEmail)
		}

		resp = e.patch(fmt.Sprintf("/api/boards/%d", b.ID), map[string]any{
			"git_author_name":  "Grace Hopper",
			"git_author_email": "grace@example.com",
		})
		assertStatus(t, resp, 200)
		patched := decodeJSON[db.Board](t, resp)
		if patched.GitAuthorName != "Grace Hopper" || patched.GitAuthorEmail != "grace@example.com" {
			t.Fatalf("patch: got name=%q email=%q", patched.GitAuthorName, patched.GitAuthorEmail)
		}

		resp = e.get(fmt.Sprintf("/api/boards/%d", b.ID))
		assertStatus(t, resp, 200)
		got := decodeJSON[db.Board](t, resp)
		if got.GitAuthorName != "Grace Hopper" || got.GitAuthorEmail != "grace@example.com" {
			t.Fatalf("get: got name=%q email=%q", got.GitAuthorName, got.GitAuthorEmail)
		}
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

	t.Run("create_by_slug", func(t *testing.T) {
		resp := e.post(fmt.Sprintf("/api/boards/%s/tickets", board.Slug), map[string]any{
			"title": "via slug",
		})
		assertStatus(t, resp, 201)
		tk := decodeJSON[db.Ticket](t, resp)
		if tk.BoardID != board.ID {
			t.Errorf("BoardID = %d, want %d", tk.BoardID, board.ID)
		}
	})

	t.Run("create_default_column", func(t *testing.T) {
		resp := e.post(fmt.Sprintf("/api/boards/%d/tickets", board.ID), map[string]any{
			"title": "no column specified",
		})
		assertStatus(t, resp, 201)
		tk := decodeJSON[db.Ticket](t, resp)
		if tk.ColumnID != cols[0].ID {
			t.Errorf("default column = %d, want leftmost %d", tk.ColumnID, cols[0].ID)
		}
	})

	t.Run("create_column_by_name", func(t *testing.T) {
		resp := e.post(fmt.Sprintf("/api/boards/%d/tickets", board.ID), map[string]any{
			"title":  "named column",
			"column": "in progress",
		})
		assertStatus(t, resp, 201)
		tk := decodeJSON[db.Ticket](t, resp)
		if tk.ColumnID != cols[1].ID {
			t.Errorf("ColumnID = %d, want %d (In Progress)", tk.ColumnID, cols[1].ID)
		}
	})

	t.Run("create_unknown_column_400", func(t *testing.T) {
		resp := e.post(fmt.Sprintf("/api/boards/%d/tickets", board.ID), map[string]any{
			"title":  "bad column",
			"column": "Nope",
		})
		assertStatus(t, resp, 400)
	})

	t.Run("create_unknown_slug_404", func(t *testing.T) {
		resp := e.post("/api/boards/no-such-slug/tickets", map[string]any{
			"title": "x",
		})
		assertStatus(t, resp, 404)
	})

	// GET /api/tickets/{id} is the one ticket read that needs no board
	// context, so a caller holding a bare id (the CLI's `ticket info`) can
	// find the board it lives on.
	t.Run("get_by_id", func(t *testing.T) {
		tk := e.seedTicket(board, "Fetch me")
		resp := e.get(fmt.Sprintf("/api/tickets/%d", tk.ID))
		assertStatus(t, resp, 200)
		got := decodeJSON[db.Ticket](t, resp)
		if got.ID != tk.ID || got.BoardID != board.ID || got.Title != "Fetch me" {
			t.Errorf("ticket = %+v", got)
		}
	})

	t.Run("get_unknown_404", func(t *testing.T) {
		assertStatus(t, e.get("/api/tickets/999999"), 404)
	})

	t.Run("update_rename", func(t *testing.T) {
		tk := e.seedTicket(board, "OldName")
		resp := e.patch(fmt.Sprintf("/api/tickets/%d", tk.ID), map[string]any{
			"title": "  New Name  ",
		})
		assertStatus(t, resp, 200)
		got := decodeJSON[db.Ticket](t, resp)
		if got.Title != "New Name" {
			t.Errorf("title = %q, want %q", got.Title, "New Name")
		}
		if got.Slug != tk.Slug {
			t.Errorf("slug should be stable on rename: got %q, want %q", got.Slug, tk.Slug)
		}
		stored, _ := e.store.GetTicket(t.Context(), tk.ID)
		if stored.Title != "New Name" {
			t.Errorf("stored title = %q", stored.Title)
		}
	})

	t.Run("update_body_only", func(t *testing.T) {
		tk := e.seedTicket(board, "BodyEdit")
		resp := e.patch(fmt.Sprintf("/api/tickets/%d", tk.ID), map[string]any{
			"body": "new body",
		})
		assertStatus(t, resp, 200)
		got := decodeJSON[db.Ticket](t, resp)
		if got.Title != "BodyEdit" {
			t.Errorf("title should be unchanged, got %q", got.Title)
		}
		if got.Body != "new body" {
			t.Errorf("body = %q", got.Body)
		}
	})

	t.Run("update_empty_title_400", func(t *testing.T) {
		tk := e.seedTicket(board, "X")
		resp := e.patch(fmt.Sprintf("/api/tickets/%d", tk.ID), map[string]any{
			"title": "   ",
		})
		assertStatus(t, resp, 400)
	})

	t.Run("update_unknown_404", func(t *testing.T) {
		resp := e.patch("/api/tickets/9999", map[string]any{"title": "x"})
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

func TestSessionSummary(t *testing.T) {
	e := newEnv(t)

	t.Run("empty_is_zero", func(t *testing.T) {
		resp := e.get("/api/sessions/summary")
		assertStatus(t, resp, 200)
		got := decodeJSON[map[string]int](t, resp)
		for _, k := range []string{"running", "working", "awaiting_perm", "idle", "starting"} {
			if got[k] != 0 {
				t.Errorf("%s = %d; want 0 (full: %v)", k, got[k], got)
			}
		}
	})

	t.Run("counts_active_states_instance_wide", func(t *testing.T) {
		board := e.seedBoard("SummaryCounts")
		statuses := []string{
			db.SessionStatusWorking,
			db.SessionStatusWorking,
			db.SessionStatusIdle,
			db.SessionStatusAwaitingPerm,
			db.SessionStatusStarting,
			db.SessionStatusStopped, // excluded from running
			db.SessionStatusError,   // excluded from running
		}
		for i, st := range statuses {
			tk := e.seedTicket(board, fmt.Sprintf("summary-%d", i))
			sess := e.seedSession(tk)
			if err := e.store.UpdateSessionStatus(context.Background(), sess.ID, st); err != nil {
				t.Fatalf("UpdateSessionStatus[%d]: %v", i, err)
			}
		}
		resp := e.get("/api/sessions/summary")
		assertStatus(t, resp, 200)
		got := decodeJSON[map[string]int](t, resp)
		want := map[string]int{
			"working":       2,
			"awaiting_perm": 1,
			"idle":          1,
			"starting":      1,
			"running":       5, // working*2 + awaiting + idle + starting; stopped/error excluded
		}
		for k, v := range want {
			if got[k] != v {
				t.Errorf("%s = %d; want %d (full: %v)", k, got[k], v, got)
			}
		}
	})
}

func TestSessions(t *testing.T) {
	// Sessions must not reach a real daemon (TestMain sets a dead
	// DOCKER_HOST): otherwise, on hosts where docker is up and the built-in
	// image is cached, start_without_devcontainer_500 would really spawn (and
	// leak) a session container and get 200 instead of 500.
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

	// seedDeadSession is a session row that still claims a live container
	// after the container died underneath kanban (host reboot, docker
	// restart, OOM kill). The probe stands in for the daemon's answer.
	seedDeadSession := func(t *testing.T, title string, running bool) (*db.Session, *[]string) {
		t.Helper()
		other := e.seedTicket(board, title)
		container := "dead-" + strings.ToLower(title)
		sess := &db.Session{
			TicketID:     other.ID,
			WorktreePath: filepath.Join(e.dir, "wt", fmt.Sprintf("ticket-%d", other.ID)),
			BranchName:   fmt.Sprintf("kanban/test/%d", other.ID),
			Status:       db.SessionStatusIdle,
			ContainerID:  &container,
		}
		if err := e.store.UpsertSession(context.Background(), sess); err != nil {
			t.Fatal(err)
		}
		probed := &[]string{}
		e.sessions.SetContainerProbe(func(_ context.Context, id string) (bool, error) {
			*probed = append(*probed, id)
			return running, nil
		})
		t.Cleanup(func() { e.sessions.SetContainerProbe(nil) })
		return sess, probed
	}
	containerOf := func(s db.Session) string {
		if s.ContainerID == nil {
			return ""
		}
		return *s.ContainerID
	}

	t.Run("ensure_reconciles_dead_container", func(t *testing.T) {
		sess, probed := seedDeadSession(t, "DeadOnEnsure", false)
		resp := e.post(fmt.Sprintf("/api/tickets/%d/session", sess.TicketID), nil)
		assertStatus(t, resp, 201)
		got := decodeJSON[db.Session](t, resp)
		if got.ID != sess.ID || got.Status != db.SessionStatusStopped || containerOf(got) != "" {
			t.Errorf("ensured session = %+v, want the same row stopped with no container", got)
		}
		if len(*probed) != 1 || (*probed)[0] != "dead-deadonensure" {
			t.Errorf("probed %v, want the recorded container once", *probed)
		}
	})

	t.Run("ensure_keeps_running_container", func(t *testing.T) {
		sess, _ := seedDeadSession(t, "AliveOnEnsure", true)
		resp := e.post(fmt.Sprintf("/api/tickets/%d/session", sess.TicketID), nil)
		assertStatus(t, resp, 201)
		got := decodeJSON[db.Session](t, resp)
		if got.Status != db.SessionStatusIdle || containerOf(got) != "dead-aliveonensure" {
			t.Errorf("ensured session = %+v, want untouched", got)
		}
	})

	t.Run("start_reconciles_dead_container", func(t *testing.T) {
		sess, probed := seedDeadSession(t, "DeadOnStart", false)
		_ = os.MkdirAll(sess.WorktreePath, 0o755)
		// Without a daemon the restart fails at spawn; the point is that
		// start no longer returns the stale idle row as a 200 no-op.
		resp := e.post(fmt.Sprintf("/api/sessions/%d/start", sess.ID), nil)
		assertStatus(t, resp, 500)
		if len(*probed) != 1 {
			t.Errorf("probed %v, want once", *probed)
		}
		fresh, err := e.store.GetSession(context.Background(), sess.ID)
		if err != nil {
			t.Fatal(err)
		}
		if fresh.Status != db.SessionStatusError || containerOf(*fresh) != "" {
			t.Errorf("session after failed restart = %+v, want error with no container", fresh)
		}
	})

	t.Run("stop_unknown_500", func(t *testing.T) {
		resp := e.post("/api/sessions/9999/stop", nil)
		// no session row → store returns ErrNotFound; handler maps to 500.
		assertStatus(t, resp, 500)
	})

	t.Run("claude_session_id_round_trip", func(t *testing.T) {
		other := e.seedTicket(board, "ClaudeResume")
		sess := e.seedSession(other)

		// Subscribe to board events before the PATCH so we can assert the
		// handler broadcasts session_updated. Without that broadcast, the
		// frontend InfoPanel never reflects the captured UUID until a hard
		// refetch — which is how the resume feature looks broken on views
		// (e.g. the overview canvas) that keep panels open across sessions.
		events := e.subscribeBoardEvents(board.ID)
		defer events.close()
		events.waitReady(t)

		uuid := "abcdef01-2345-6789-abcd-0123456789ab"
		resp := e.patch(fmt.Sprintf("/api/sessions/%d/claude-session", sess.ID),
			map[string]any{"claude_session_id": uuid})
		assertStatus(t, resp, 204)

		got, err := e.store.GetSession(context.Background(), sess.ID)
		if err != nil {
			t.Fatalf("GetSession: %v", err)
		}
		if got.ClaudeSessionID != uuid {
			t.Errorf("ClaudeSessionID = %q; want %q", got.ClaudeSessionID, uuid)
		}

		ev := events.next(t)
		if ev.Type != "session_updated" {
			t.Fatalf("expected session_updated event, got %q", ev.Type)
		}
		payload, _ := ev.Data["data"].(map[string]any)
		if payload == nil {
			t.Fatalf("event has no data payload: %#v", ev.Data)
		}
		if payload["id"] != float64(sess.ID) {
			t.Errorf("event session id = %v; want %d", payload["id"], sess.ID)
		}
		if payload["claude_session_id"] != uuid {
			t.Errorf("event claude_session_id = %v; want %q", payload["claude_session_id"], uuid)
		}
	})

	t.Run("claude_session_id_rejects_bad_uuid", func(t *testing.T) {
		other := e.seedTicket(board, "ClaudeBadUUID")
		sess := e.seedSession(other)

		for _, bad := range []string{"", "not-a-uuid", "abc"} {
			resp := e.patch(fmt.Sprintf("/api/sessions/%d/claude-session", sess.ID),
				map[string]any{"claude_session_id": bad})
			assertStatus(t, resp, 400)
		}
	})

	t.Run("claude_session_id_unknown_session_404", func(t *testing.T) {
		resp := e.patch("/api/sessions/9999/claude-session",
			map[string]any{"claude_session_id": "abcdef01-2345-6789-abcd-0123456789ab"})
		assertStatus(t, resp, 404)
	})

	t.Run("branch_repoint_clears_pr_and_broadcasts", func(t *testing.T) {
		other := e.seedTicket(board, "Repoint")
		sess := e.seedSession(other)
		// Populate pr_* so we can assert the re-point clears them.
		prNum := int64(7)
		if err := e.store.UpdateSessionPR(context.Background(), sess.ID, "open", &prNum,
			"https://example.com/pull/7", "old pr"); err != nil {
			t.Fatalf("UpdateSessionPR: %v", err)
		}

		events := e.subscribeBoardEvents(board.ID)
		defer events.close()
		events.waitReady(t)

		resp := e.patch(fmt.Sprintf("/api/sessions/%d/branch", sess.ID),
			map[string]any{"branch_name": "kanban/pivoted"})
		assertStatus(t, resp, 200)
		updated := decodeJSON[db.Session](t, resp)
		if updated.BranchName != "kanban/pivoted" {
			t.Errorf("response branch_name = %q; want %q", updated.BranchName, "kanban/pivoted")
		}
		if updated.PRState != "" || updated.PRNumber != nil || updated.PRURL != "" || updated.PRTitle != "" {
			t.Errorf("response pr_* not cleared: %+v", updated)
		}

		got, err := e.store.GetSession(context.Background(), sess.ID)
		if err != nil {
			t.Fatalf("GetSession: %v", err)
		}
		if got.BranchName != "kanban/pivoted" {
			t.Errorf("persisted branch_name = %q; want %q", got.BranchName, "kanban/pivoted")
		}
		if got.PRNumber != nil {
			t.Errorf("persisted pr_number = %v; want nil", got.PRNumber)
		}

		ev := events.next(t)
		if ev.Type != "session_updated" {
			t.Fatalf("expected session_updated event, got %q", ev.Type)
		}
	})

	t.Run("branch_no_op_keeps_pr", func(t *testing.T) {
		other := e.seedTicket(board, "RepointNoOp")
		sess := e.seedSession(other)
		prNum := int64(9)
		if err := e.store.UpdateSessionPR(context.Background(), sess.ID, "open", &prNum,
			"https://example.com/pull/9", "keep me"); err != nil {
			t.Fatalf("UpdateSessionPR: %v", err)
		}
		// Re-saving the existing branch must not clear the cached PR fields.
		resp := e.patch(fmt.Sprintf("/api/sessions/%d/branch", sess.ID),
			map[string]any{"branch_name": sess.BranchName})
		assertStatus(t, resp, 200)
		got, err := e.store.GetSession(context.Background(), sess.ID)
		if err != nil {
			t.Fatalf("GetSession: %v", err)
		}
		if got.PRNumber == nil || *got.PRNumber != 9 {
			t.Errorf("pr_number = %v; want 9 (no-op should not clear)", got.PRNumber)
		}
	})

	t.Run("branch_rejects_empty_and_malformed", func(t *testing.T) {
		other := e.seedTicket(board, "RepointBad")
		sess := e.seedSession(other)
		for _, bad := range []string{"", "   ", "has space", "-leading-dash", "a..b"} {
			resp := e.patch(fmt.Sprintf("/api/sessions/%d/branch", sess.ID),
				map[string]any{"branch_name": bad})
			assertStatus(t, resp, 400)
		}
	})

	t.Run("branch_unknown_session_404", func(t *testing.T) {
		resp := e.patch("/api/sessions/9999/branch",
			map[string]any{"branch_name": "kanban/whatever"})
		assertStatus(t, resp, 404)
	})

	t.Run("harness_set_and_clear", func(t *testing.T) {
		other := e.seedTicket(board, "Harness")
		sess := e.seedSession(other)

		events := e.subscribeBoardEvents(board.ID)
		defer events.close()
		events.waitReady(t)

		resp := e.put(fmt.Sprintf("/api/sessions/%d/harness", sess.ID), map[string]any{"harness": "pi"})
		assertStatus(t, resp, 200)
		if got := decodeJSON[db.Session](t, resp); got.Harness != "pi" {
			t.Errorf("response harness = %q; want pi", got.Harness)
		}
		if ev := events.next(t); ev.Type != "session_updated" {
			t.Fatalf("expected session_updated event, got %q", ev.Type)
		}

		resp = e.put(fmt.Sprintf("/api/sessions/%d/harness", sess.ID), map[string]any{"harness": ""})
		assertStatus(t, resp, 200)
		got, err := e.store.GetSession(context.Background(), sess.ID)
		if err != nil {
			t.Fatalf("GetSession: %v", err)
		}
		if got.Harness != "" {
			t.Errorf("persisted harness = %q; want cleared", got.Harness)
		}
	})

	t.Run("harness_switch_without_agent_keeps_status", func(t *testing.T) {
		// Only stopping a running agent resets the status it reported; with
		// no agent attached a switch is just a DB write.
		other := e.seedTicket(board, "HarnessNoAgent")
		sess := e.seedSession(other)
		if err := e.store.UpdateSessionStatus(context.Background(), sess.ID, db.SessionStatusWorking); err != nil {
			t.Fatal(err)
		}
		resp := e.put(fmt.Sprintf("/api/sessions/%d/harness", sess.ID), map[string]any{"harness": "pi"})
		assertStatus(t, resp, 200)
		got := decodeJSON[db.Session](t, resp)
		if got.Harness != "pi" || got.Status != db.SessionStatusWorking {
			t.Errorf("session = harness %q, status %q; want pi, working", got.Harness, got.Status)
		}
	})

	t.Run("harness_rejects_unknown", func(t *testing.T) {
		other := e.seedTicket(board, "HarnessBad")
		sess := e.seedSession(other)
		resp := e.put(fmt.Sprintf("/api/sessions/%d/harness", sess.ID), map[string]any{"harness": "nope"})
		assertStatus(t, resp, 400)
	})

	t.Run("harness_unknown_session_404", func(t *testing.T) {
		resp := e.put("/api/sessions/9999/harness", map[string]any{"harness": "pi"})
		assertStatus(t, resp, 404)
	})
}

func TestListHarnessesFlagsBoardDefault(t *testing.T) {
	e := newEnv(t)
	board := e.seedBoard("HarnessDefault")

	resp := e.get("/api/harnesses")
	assertStatus(t, resp, 200)
	for _, h := range decodeJSON[[]map[string]any](t, resp) {
		if _, ok := h["default"]; ok {
			t.Errorf("harness %v flagged default without ?board", h["id"])
		}
	}

	resp = e.get(fmt.Sprintf("/api/harnesses?board=%d", board.ID))
	assertStatus(t, resp, 200)
	var defaults []any
	for _, h := range decodeJSON[[]map[string]any](t, resp) {
		if h["default"] == true {
			defaults = append(defaults, h["id"])
		}
	}
	if len(defaults) != 1 {
		t.Errorf("default harnesses = %v; want exactly one", defaults)
	}

	assertStatus(t, e.get("/api/harnesses?board=9999"), 404)
	assertStatus(t, e.get("/api/harnesses?board=abc"), 400)
}

// ---------- Tasks ----------

func TestTasks(t *testing.T) {
	e := newEnv(t)
	board := e.seedBoard("Tasks")
	tk := e.seedTicket(board, "DoTasks")
	sess := e.seedSession(tk)

	t.Run("discover_empty_worktree_returns_empty", func(t *testing.T) {
		_ = os.MkdirAll(sess.WorktreePath, 0o755)
		resp := e.get(fmt.Sprintf("/api/sessions/%d/discover-tasks", sess.ID))
		assertStatus(t, resp, 200)
		var body struct {
			Tasks    []map[string]any `json:"tasks"`
			Warnings []string         `json:"warnings"`
		}
		if err := json.Unmarshal(readBody(t, resp), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Tasks) != 0 {
			t.Errorf("empty discover tasks = %v; want []", body.Tasks)
		}
		if len(body.Warnings) != 0 {
			t.Errorf("empty discover warnings = %v; want []", body.Warnings)
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
		var body struct {
			Tasks    []map[string]any `json:"tasks"`
			Warnings []string         `json:"warnings"`
		}
		if err := json.Unmarshal(readBody(t, resp), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Tasks) != 1 {
			t.Fatalf("found = %d tasks; want 1: %+v", len(body.Tasks), body.Tasks)
		}
		if body.Tasks[0]["label"] != "Start Frontend" {
			t.Errorf("label = %v; want 'Start Frontend'", body.Tasks[0]["label"])
		}
		if body.Tasks[0]["has_port"] != true {
			t.Errorf("has_port = %v; want true", body.Tasks[0]["has_port"])
		}
		if cp, _ := body.Tasks[0]["container_port"].(float64); int(cp) != 3000 {
			t.Errorf("container_port = %v; want 3000", body.Tasks[0]["container_port"])
		}
		if len(body.Warnings) != 0 {
			t.Errorf("warnings = %v; want []", body.Warnings)
		}
	})

	t.Run("discover_reports_invalid_json_warning", func(t *testing.T) {
		_ = os.MkdirAll(filepath.Join(sess.WorktreePath, ".vscode"), 0o755)
		if err := os.WriteFile(filepath.Join(sess.WorktreePath, ".vscode", "tasks.json"), []byte(`{ "tasks": [`), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			// Restore the valid file the next subtest expects.
			tasksJSON := `{
				"version": "2.0.0",
				"tasks": [
					{"label": "Start Frontend", "type": "shell", "command": "npm", "args": ["run", "dev"]}
				]
			}`
			_ = os.WriteFile(filepath.Join(sess.WorktreePath, ".vscode", "tasks.json"), []byte(tasksJSON), 0o644)
		})

		resp := e.get(fmt.Sprintf("/api/sessions/%d/discover-tasks", sess.ID))
		assertStatus(t, resp, 200)
		var body struct {
			Tasks    []map[string]any `json:"tasks"`
			Warnings []string         `json:"warnings"`
		}
		if err := json.Unmarshal(readBody(t, resp), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Warnings) == 0 {
			t.Fatalf("expected a warning for invalid tasks.json; got none")
		}
		if !strings.Contains(body.Warnings[0], "tasks.json") {
			t.Errorf("warning = %q; want to mention tasks.json", body.Warnings[0])
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

// ---------- Plans ----------

func TestSessionPlans(t *testing.T) {
	// Point XDG_CONFIG_HOME at a temp dir with a kanban config that uses a
	// relative plans dir, so the resolution is rooted at the session worktree.
	cfgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	if err := os.MkdirAll(filepath.Join(cfgHome, "kanban"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(cfgHome, "kanban", "config.toml"),
		[]byte("[plans]\ndir = \"./plans\"\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	e := newEnv(t)
	board := e.seedBoard("Plans")
	tk := e.seedTicket(board, "P")
	sess := e.seedSession(tk)

	t.Run("empty_when_dir_missing", func(t *testing.T) {
		resp := e.get(fmt.Sprintf("/api/sessions/%d/plans", sess.ID))
		assertStatus(t, resp, 200)
		body := strings.TrimSpace(string(readBody(t, resp)))
		if body != "[]" {
			t.Errorf("plans = %s; want []", body)
		}
	})

	t.Run("lists_md_files", func(t *testing.T) {
		dir := filepath.Join(sess.WorktreePath, "plans")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		mdContent := "# Hello\n\nbody"
		if err := os.WriteFile(filepath.Join(dir, "first.md"), []byte(mdContent), 0o644); err != nil {
			t.Fatal(err)
		}
		// A non-.md file should be filtered out.
		if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		resp := e.get(fmt.Sprintf("/api/sessions/%d/plans", sess.ID))
		assertStatus(t, resp, 200)
		var plans []map[string]any
		if err := json.Unmarshal(readBody(t, resp), &plans); err != nil {
			t.Fatal(err)
		}
		if len(plans) != 1 || plans[0]["name"] != "first.md" {
			t.Errorf("plans = %+v; want [first.md]", plans)
		}
	})

	t.Run("reads_plan_content", func(t *testing.T) {
		resp := e.get(fmt.Sprintf("/api/sessions/%d/plans/first.md", sess.ID))
		assertStatus(t, resp, 200)
		if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/markdown") {
			t.Errorf("Content-Type = %q; want text/markdown", ct)
		}
		body := string(readBody(t, resp))
		if !strings.Contains(body, "# Hello") {
			t.Errorf("body = %q; want it to contain the markdown", body)
		}
	})

	t.Run("rejects_traversal", func(t *testing.T) {
		resp := e.get(fmt.Sprintf("/api/sessions/%d/plans/..%%2Fescape.md", sess.ID))
		// Path normalization in Go's mux may turn this into a different route
		// or leave it; either way the handler must reject it.
		if resp.StatusCode == 200 {
			t.Errorf("expected non-200 for traversal attempt")
		}
	})

	t.Run("missing_plan_404", func(t *testing.T) {
		resp := e.get(fmt.Sprintf("/api/sessions/%d/plans/nope.md", sess.ID))
		assertStatus(t, resp, 404)
	})

	t.Run("unknown_session_404", func(t *testing.T) {
		resp := e.get("/api/sessions/999999/plans")
		assertStatus(t, resp, 404)
	})
}

func TestSessionFile(t *testing.T) {
	e := newEnv(t)
	board := e.seedBoard("Files")
	tk := e.seedTicket(board, "F")
	sess := e.seedSession(tk)

	if err := os.MkdirAll(filepath.Join(sess.WorktreePath, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(sess.WorktreePath, "src", "main.go"),
		[]byte("package main\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	t.Run("returns_file_contents", func(t *testing.T) {
		resp := e.get(fmt.Sprintf("/api/sessions/%d/file?path=src/main.go", sess.ID))
		assertStatus(t, resp, 200)
		var f map[string]string
		if err := json.Unmarshal(readBody(t, resp), &f); err != nil {
			t.Fatal(err)
		}
		if f["path"] != "src/main.go" || f["contents"] != "package main\n" {
			t.Errorf("file = %+v; want path=src/main.go contents=%q", f, "package main\n")
		}
	})

	t.Run("missing_path_400", func(t *testing.T) {
		resp := e.get(fmt.Sprintf("/api/sessions/%d/file", sess.ID))
		assertStatus(t, resp, 400)
	})

	t.Run("missing_file_404", func(t *testing.T) {
		resp := e.get(fmt.Sprintf("/api/sessions/%d/file?path=src/nope.go", sess.ID))
		assertStatus(t, resp, 404)
	})

	t.Run("rejects_traversal", func(t *testing.T) {
		resp := e.get(fmt.Sprintf("/api/sessions/%d/file?path=..%%2F..%%2Fsecret", sess.ID))
		if resp.StatusCode == 200 {
			t.Errorf("expected non-200 for traversal attempt")
		}
	})

	t.Run("unknown_session_404", func(t *testing.T) {
		resp := e.get("/api/sessions/999999/file?path=x")
		assertStatus(t, resp, 404)
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
