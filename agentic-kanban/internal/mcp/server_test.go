package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmelahman/kanban/internal/api"
	"github.com/jmelahman/kanban/internal/client"
	"github.com/jmelahman/kanban/internal/config"
	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/docker"
	"github.com/jmelahman/kanban/internal/hooks"
	"github.com/jmelahman/kanban/internal/secrets"
	"github.com/jmelahman/kanban/internal/session"
)

// TestRun_CreateTicketFlow drives the MCP server end-to-end: real kanban
// HTTP stack via httptest.Server, MCP server speaking JSON-RPC over an
// in-memory pipe, and asserts the ticket lands in the SQLite store.
func TestRun_CreateTicketFlow(t *testing.T) {
	srv, store, board := newKanbanTestServer(t)

	in, stdin := io.Pipe()
	stdout, out := io.Pipe()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- run(ctx, srv.URL, in, out, srv.Client()) }()

	send := func(payload string) {
		if _, err := stdin.Write([]byte(payload + "\n")); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	reader := bufio.NewReader(stdout)
	read := func() map[string]json.RawMessage {
		t.Helper()
		line, err := reader.ReadBytes('\n')
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		var resp map[string]json.RawMessage
		if err := json.Unmarshal(line, &resp); err != nil {
			t.Fatalf("unmarshal %q: %v", line, err)
		}
		if e, ok := resp["error"]; ok {
			t.Fatalf("rpc error: %s", string(e))
		}
		return resp
	}

	// 1. initialize
	send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`)
	initResp := read()
	if !strings.Contains(string(initResp["result"]), `"serverInfo"`) {
		t.Fatalf("initialize missing serverInfo: %s", initResp["result"])
	}
	send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)

	// 2. tools/list — assert every tool we expose is present.
	send(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	listResp := read()
	expectTools := []string{
		"list_boards", "create_board", "get_board", "update_board", "delete_board", "board_state",
		"list_archived", "delete_archived",
		"create_ticket", "update_ticket", "move_ticket", "archive_ticket", "unarchive_ticket",
		"delete_ticket", "done_ticket", "sync_ticket", "merge_ticket",
		"archive_column_tickets",
		"ensure_session", "start_session", "stop_session", "restart_session",
		"list_config", "get_config", "set_config", "unset_config",
		"list_board_env", "set_board_env", "unset_board_env",
	}
	for _, name := range expectTools {
		if !strings.Contains(string(listResp["result"]), `"`+name+`"`) {
			t.Fatalf("tools/list missing %q: %s", name, listResp["result"])
		}
	}

	// 3. tools/call create_ticket by slug, no column
	callJSON, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 3, "method": "tools/call",
		"params": map[string]any{
			"name": "create_ticket",
			"arguments": map[string]any{
				"board": board.Slug,
				"title": "From MCP",
				"body":  "via mcp test",
			},
		},
	})
	send(string(callJSON))
	callResp := read()
	if strings.Contains(string(callResp["result"]), `"isError":true`) {
		t.Fatalf("tools/call returned error: %s", callResp["result"])
	}

	// Verify the ticket landed in the store.
	tickets, err := store.ListTickets(ctx, board.ID)
	if err != nil {
		t.Fatalf("ListTickets: %v", err)
	}
	var found *db.Ticket
	for i := range tickets {
		if tickets[i].Title == "From MCP" {
			found = &tickets[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("ticket not created; got %+v", tickets)
	}
	cols, _ := store.ListColumns(ctx, board.ID)
	if found.ColumnID != cols[0].ID {
		t.Errorf("ticket landed in column %d, want leftmost %d", found.ColumnID, cols[0].ID)
	}

	// 4. unknown method -> JSON-RPC -32601
	send(`{"jsonrpc":"2.0","id":4,"method":"nope"}`)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(line), `"code":-32601`) {
		t.Errorf("expected method-not-found, got %s", line)
	}

	// Closing stdin makes Run return.
	stdin.Close()
	out.Close()
	if err := <-done; err != nil {
		t.Errorf("run returned error: %v", err)
	}
}

// TestCallTool_LifecycleCoverage exercises a representative slice of the new
// tools by invoking callTool directly, instead of full JSON-RPC framing, so
// each dispatch case is checked without ceremony.
func TestCallTool_LifecycleCoverage(t *testing.T) {
	srv, store, board := newKanbanTestServer(t)
	s := &server{client: client.New(srv.URL, srv.Client())}
	ctx := t.Context()

	call := func(name string, args map[string]any) map[string]any {
		t.Helper()
		argBuf, _ := json.Marshal(args)
		paramBuf, _ := json.Marshal(map[string]any{"name": name, "arguments": json.RawMessage(argBuf)})
		out, err := s.callTool(ctx, paramBuf)
		if err != nil {
			t.Fatalf("callTool %s: %v", name, err)
		}
		if isErr, _ := out["isError"].(bool); isErr {
			text := ""
			if c, ok := out["content"].([]map[string]any); ok && len(c) > 0 {
				text, _ = c[0]["text"].(string)
			}
			t.Fatalf("tool %s returned error: %s", name, text)
		}
		return out
	}

	call("create_ticket", map[string]any{"board": board.Slug, "title": "first", "body": "b"})
	tickets, _ := store.ListTickets(ctx, board.ID)
	if len(tickets) != 1 || tickets[0].Title != "first" {
		t.Fatalf("create_ticket did not land: %+v", tickets)
	}
	tid := tickets[0].ID

	call("update_ticket", map[string]any{"ticket": tid, "title": "renamed"})
	updated, _ := store.GetTicket(ctx, tid)
	if updated.Title != "renamed" {
		t.Errorf("update_ticket: got %q", updated.Title)
	}

	call("archive_ticket", map[string]any{"ticket": tid})
	updated, _ = store.GetTicket(ctx, tid)
	if updated.ArchivedAt == nil {
		t.Error("archive_ticket: not archived")
	}

	call("unarchive_ticket", map[string]any{"ticket": tid})
	updated, _ = store.GetTicket(ctx, tid)
	if updated.ArchivedAt != nil {
		t.Error("unarchive_ticket: still archived")
	}

	cols, _ := store.ListColumns(ctx, board.ID)
	call("move_ticket", map[string]any{"ticket": tid, "column_id": cols[1].ID, "position": 0})
	updated, _ = store.GetTicket(ctx, tid)
	if updated.ColumnID != cols[1].ID {
		t.Errorf("move_ticket: column %d, want %d", updated.ColumnID, cols[1].ID)
	}

	call("archive_column_tickets", map[string]any{"column_id": cols[1].ID})
	leftInCol, _ := store.ListTicketsInColumn(ctx, cols[1].ID)
	if len(leftInCol) != 0 {
		t.Errorf("archive_column_tickets: %d tickets left", len(leftInCol))
	}

	call("delete_archived", map[string]any{"board": board.Slug})
	archived, _ := store.ListArchivedTickets(ctx, board.ID)
	if len(archived) != 0 {
		t.Errorf("delete_archived: %d archived left", len(archived))
	}

	// Board env vars: set → list (keys only, never the value) → unset.
	out := call("set_board_env", map[string]any{"board": board.Slug, "vars": map[string]string{"MY_API_KEY": "s3cret-value"}})
	if text := toolText(t, out); strings.Contains(text, "s3cret-value") {
		t.Errorf("set_board_env leaked the value: %s", text)
	}
	out = call("list_board_env", map[string]any{"board": board.Slug})
	text := toolText(t, out)
	if !strings.Contains(text, "MY_API_KEY") {
		t.Errorf("list_board_env missing key: %s", text)
	}
	if strings.Contains(text, "s3cret-value") {
		t.Errorf("list_board_env leaked the value: %s", text)
	}
	vars, err := store.GetBoardEnvVars(ctx, board.ID)
	if err != nil || vars["MY_API_KEY"] != "s3cret-value" {
		t.Errorf("set_board_env did not persist: vars=%v err=%v", vars, err)
	}

	out = call("unset_board_env", map[string]any{"board": board.Slug, "keys": []string{"MY_API_KEY"}})
	if text := toolText(t, out); strings.Contains(text, "MY_API_KEY") {
		t.Errorf("unset_board_env still lists key: %s", text)
	}
}

// toolText extracts the text payload from a callTool result.
func toolText(t *testing.T, out map[string]any) string {
	t.Helper()
	content, ok := out["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatalf("tool result has no content: %v", out)
	}
	text, _ := content[0]["text"].(string)
	return text
}

// TestCallTool_Config exercises the config tools through callTool, asserting
// set -> get round-trips a value and unset clears it. KANBAN_CONFIG is
// redirected so the global write hits a throwaway file.
func TestCallTool_Config(t *testing.T) {
	t.Setenv("KANBAN_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	srv, _, _ := newKanbanTestServer(t)
	s := &server{client: client.New(srv.URL, srv.Client())}
	ctx := t.Context()

	call := func(name string, args map[string]any) map[string]any {
		t.Helper()
		argBuf, _ := json.Marshal(args)
		paramBuf, _ := json.Marshal(map[string]any{"name": name, "arguments": json.RawMessage(argBuf)})
		out, err := s.callTool(ctx, paramBuf)
		if err != nil {
			t.Fatalf("callTool %s: %v", name, err)
		}
		if isErr, _ := out["isError"].(bool); isErr {
			t.Fatalf("tool %s returned error: %s", name, textOf(out))
		}
		return out
	}

	call("set_config", map[string]any{"key": "sync.allow_rebase", "value": true, "scope": "global"})

	out := call("get_config", map[string]any{"key": "sync.allow_rebase"})
	if !strings.Contains(textOf(out), `"value":true`) {
		t.Fatalf("get_config after set: %s", textOf(out))
	}

	call("unset_config", map[string]any{"key": "sync.allow_rebase", "scope": "global"})

	out = call("get_config", map[string]any{"key": "sync.allow_rebase"})
	if !strings.Contains(textOf(out), `"value":null`) {
		t.Fatalf("get_config after unset: %s", textOf(out))
	}
}

func textOf(out map[string]any) string {
	if c, ok := out["content"].([]map[string]any); ok && len(c) > 0 {
		text, _ := c[0]["text"].(string)
		return text
	}
	return ""
}

func newKanbanTestServer(t *testing.T) (*httptest.Server, *db.Store, *db.Board) {
	t.Helper()
	dir := t.TempDir()

	repoPath := filepath.Join(dir, "repo")
	mustGit(t, "", "init", "-q", "-b", "main", repoPath)
	mustGit(t, repoPath, "config", "user.email", "test@example.com")
	mustGit(t, repoPath, "config", "user.name", "Test")
	mustGit(t, repoPath, "commit", "--allow-empty", "-q", "-m", "init")

	store, err := db.Open(filepath.Join(dir, "kanban.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	envKey, err := secrets.NewRandomKey()
	if err != nil {
		t.Fatal(err)
	}
	box, err := secrets.NewBox(envKey)
	if err != nil {
		t.Fatal(err)
	}
	store.SetEnvCipher(box)

	dockerCli, err := docker.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dockerCli.Close() })

	cfg := &config.Config{DataDir: dir, PortRangeStart: 13000, PortRangeEnd: 13099}
	hookRunner := hooks.NewRunner(store)
	sessionMgr := session.NewManager(store, dockerCli, hookRunner)
	handler := api.NewMux(api.Deps{
		Store: store, Docker: dockerCli, Sessions: sessionMgr, Hooks: hookRunner, Config: cfg,
	})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	board := &db.Board{
		Name:         "MCP Board",
		Slug:         "mcp-board",
		RepoPath:     repoPath,
		WorktreeRoot: filepath.Join(dir, "worktrees", "mcp"),
		BaseBranch:   "main",
	}
	if err := store.CreateBoard(t.Context(), board); err != nil {
		t.Fatal(err)
	}
	return srv, store, board
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}
