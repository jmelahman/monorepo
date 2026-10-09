package api_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmelahman/local-preview/orchestrator"

	"github.com/jmelahman/kanban/internal/config"
	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/kanbantest"
	"github.com/jmelahman/kanban/internal/session"
)

// testEnv wires up the full HTTP stack against ephemeral, real dependencies:
// a temp dir, an in-place git repo, an on-disk SQLite DB, and the actual
// Docker SDK client (which constructs without contacting the daemon — calls
// that try to reach Docker fail fast and are asserted as such).
type testEnv struct {
	t        *testing.T
	dir      string
	repoPath string // initialized git repo, used as Board.RepoPath
	store    *db.Store
	cfg      *config.Config
	srv      *httptest.Server
	sessions *session.Manager
	previews *orchestrator.Orchestrator
	// manifestDir holds out-of-repo manifests (<repo-name>.toml) for repos
	// that can't carry one; see writeLocalManifest.
	manifestDir string
}

// writeLocalManifest installs a server-side manifest for an orchestrator
// repo name (the board slug), onboarding a repo that carries no manifest.
func (e *testEnv) writeLocalManifest(repoName, content string) {
	e.t.Helper()
	p := filepath.Join(e.manifestDir, repoName+".toml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

func newEnv(t *testing.T) *testEnv {
	t.Helper()
	env := kanbantest.New(t)
	return &testEnv{
		t: t, dir: env.Dir, repoPath: env.RepoPath, store: env.Store,
		cfg: env.Config, srv: env.Server, sessions: env.Sessions,
		previews: env.Previews, manifestDir: env.ManifestDir,
	}
}

func (e *testEnv) seedBoard(name string) *db.Board {
	e.t.Helper()
	b := &db.Board{
		Name:         name,
		Slug:         strings.ToLower(strings.ReplaceAll(name, " ", "-")),
		RepoPath:     e.repoPath,
		WorktreeRoot: filepath.Join(e.dir, "worktrees", strings.ToLower(name)),
		BaseBranch:   "main",
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
	kanbantest.MustGit(t, dir, args...)
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

func (e *testEnv) put(path string, body any) *http.Response {
	e.t.Helper()
	return e.send("PUT", path, body)
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

// SSE testing

type sseEvent struct {
	Type string
	Data map[string]any
}

type sseConn struct {
	resp   *http.Response
	cancel context.CancelFunc
	r      *bufio.Reader
}

// subscribeBoardEvents opens an SSE subscription to a board's event stream.
// The returned conn buffers events; callers should defer close() and use
// waitReady to drain the initial "ready" hello before triggering work that
// is expected to publish.
func (e *testEnv) subscribeBoardEvents(boardID int64) *sseConn {
	e.t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/api/boards/%d/events", e.srv.URL, boardID), nil)
	if err != nil {
		cancel()
		e.t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		e.t.Fatal(err)
	}
	return &sseConn{resp: resp, cancel: cancel, r: bufio.NewReader(resp.Body)}
}

func (s *sseConn) close() {
	s.cancel()
	s.resp.Body.Close()
}

// waitReady reads events until the initial "ready" hello arrives, so tests
// know the subscription is registered with the bus.
func (s *sseConn) waitReady(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ev := s.next(t)
		if ev.Type == "ready" {
			return
		}
	}
	t.Fatal("never received ready event from SSE stream")
}

// next reads the next SSE event from the stream. Fails the test on read
// error or malformed framing. Blocks; rely on the test's overall timeout.
func (s *sseConn) next(t *testing.T) sseEvent {
	t.Helper()
	var typ string
	for {
		line, err := s.r.ReadString('\n')
		if err != nil {
			t.Fatalf("read SSE: %v", err)
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case strings.HasPrefix(line, "event: "):
			typ = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			payload := strings.TrimPrefix(line, "data: ")
			var data map[string]any
			if err := json.Unmarshal([]byte(payload), &data); err != nil {
				t.Fatalf("decode SSE data %q: %v", payload, err)
			}
			return sseEvent{Type: typ, Data: data}
		}
	}
}
