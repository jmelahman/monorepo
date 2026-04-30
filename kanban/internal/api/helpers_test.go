package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmelahman/kanban/internal/api"
	"github.com/jmelahman/kanban/internal/config"
	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/docker"
	"github.com/jmelahman/kanban/internal/hooks"
	"github.com/jmelahman/kanban/internal/session"
)

// testEnv wires up the full HTTP stack against ephemeral, real dependencies:
// a temp dir, an in-place git repo, an on-disk SQLite DB, and the actual
// Docker SDK client (which constructs without contacting the daemon — calls
// that try to reach Docker fail fast and are asserted as such).
type testEnv struct {
	t        *testing.T
	dir      string
	repoPath string // initialized git repo, used as Board.SourceRepoPath
	store    *db.Store
	cfg      *config.Config
	srv      *httptest.Server
}

func newEnv(t *testing.T) *testEnv {
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

	cfg := &config.Config{DataDir: dir, PortRangeStart: 13000, PortRangeEnd: 13099}
	dockerCli, err := docker.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dockerCli.Close() })
	hookRunner := hooks.NewRunner(store)
	sessionMgr := session.NewManager(store, dockerCli, hookRunner)

	handler := api.NewMux(api.Deps{
		Store: store, Docker: dockerCli, Sessions: sessionMgr, Hooks: hookRunner, Config: cfg,
	})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	return &testEnv{t: t, dir: dir, repoPath: repoPath, store: store, cfg: cfg, srv: srv}
}

func (e *testEnv) seedBoard(name string) *db.Board {
	e.t.Helper()
	b := &db.Board{
		Name:           name,
		Slug:           strings.ToLower(strings.ReplaceAll(name, " ", "-")),
		SourceRepoPath: e.repoPath,
		WorktreeRoot:   filepath.Join(e.dir, "worktrees", strings.ToLower(name)),
		BaseBranch:     "main",
	}
	if err := e.store.CreateBoard(context.Background(), b); err != nil {
		e.t.Fatal(err)
	}
	return b
}

func (e *testEnv) seedTicket(board *db.Board, title string) *db.Ticket {
	e.t.Helper()
	cols, err := e.store.ListColumns(context.Background(), board.ID)
	if err != nil || len(cols) == 0 {
		e.t.Fatalf("seedTicket: no columns: %v", err)
	}
	tk := &db.Ticket{
		BoardID:  board.ID,
		ColumnID: cols[0].ID,
		Title:    title,
		Slug:     strings.ToLower(strings.ReplaceAll(title, " ", "-")),
	}
	if err := e.store.CreateTicket(context.Background(), tk); err != nil {
		e.t.Fatal(err)
	}
	return tk
}

func (e *testEnv) seedSession(ticket *db.Ticket) *db.Session {
	e.t.Helper()
	s := &db.Session{
		TicketID:     ticket.ID,
		WorktreePath: filepath.Join(e.dir, "wt", fmt.Sprintf("ticket-%d", ticket.ID)),
		BranchName:   fmt.Sprintf("kanban/test/%d", ticket.ID),
		Status:       db.SessionStatusStopped,
	}
	if err := e.store.UpsertSession(context.Background(), s); err != nil {
		e.t.Fatal(err)
	}
	return s
}

func (e *testEnv) seedPort(session *db.Session, label string, container, host int) *db.PortAllocation {
	e.t.Helper()
	p := &db.PortAllocation{
		SessionID:     session.ID,
		Label:         label,
		ContainerPort: container,
		HostPort:      host,
	}
	if err := e.store.CreatePort(context.Background(), p); err != nil {
		e.t.Fatal(err)
	}
	return p
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
}

// HTTP helpers

func (e *testEnv) get(path string) *http.Response {
	e.t.Helper()
	resp, err := http.Get(e.srv.URL + path)
	if err != nil {
		e.t.Fatal(err)
	}
	return resp
}

func (e *testEnv) post(path string, body any) *http.Response {
	e.t.Helper()
	return e.send("POST", path, body)
}

func (e *testEnv) patch(path string, body any) *http.Response {
	e.t.Helper()
	return e.send("PATCH", path, body)
}

func (e *testEnv) delete(path string) *http.Response {
	e.t.Helper()
	return e.send("DELETE", path, nil)
}

func (e *testEnv) send(method, path string, body any) *http.Response {
	e.t.Helper()
	var buf io.Reader
	if body != nil {
		switch v := body.(type) {
		case string:
			buf = strings.NewReader(v)
		case []byte:
			buf = bytes.NewReader(v)
		default:
			b, err := json.Marshal(body)
			if err != nil {
				e.t.Fatal(err)
			}
			buf = bytes.NewReader(b)
		}
	}
	req, err := http.NewRequest(method, e.srv.URL+path, buf)
	if err != nil {
		e.t.Fatal(err)
	}
	if buf != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	return resp
}

// readBody reads and closes resp.Body, returning the bytes.
func readBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func decodeJSON[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	var v T
	body := readBody(t, resp)
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("decode: %v\nbody: %s", err, body)
	}
	return v
}

func assertStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("status = %d; want %d. body: %s", resp.StatusCode, want, body)
	}
}
