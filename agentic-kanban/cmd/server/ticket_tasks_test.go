package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/jmelahman/kanban/internal/client"
)

// fakeTasksAPI serves just enough of the kanban API for `ticket tasks`:
// ticket 42 on board 1 with session 7, two tasks (one with a port), and a
// run history. onOutput, when set, replaces the default output stream.
type fakeTasksAPI struct {
	mu          sync.Mutex
	noSession   bool
	runs        []client.TaskRun
	ports       []client.Port
	started     []string
	stopped     []int64
	portOpens   int
	outputCalls int
	exitCode    int
	onOutput    func(w http.ResponseWriter, r *http.Request)
}

func (f *fakeTasksAPI) handler(t *testing.T) http.Handler {
	mux := http.NewServeMux()
	writeJSON := func(w http.ResponseWriter, code int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(v)
	}
	session := map[string]any{"id": 7, "ticket_id": 42, "status": "idle", "container_id": "c1"}
	mux.HandleFunc("GET /api/tickets/42", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"id": 42, "board_id": 1, "column_id": 2, "title": "T", "slug": "t"})
	})
	mux.HandleFunc("GET /api/boards/1/state", func(w http.ResponseWriter, r *http.Request) {
		sessions := []any{session}
		if f.noSession {
			sessions = []any{}
		}
		writeJSON(w, 200, map[string]any{
			"board":    map[string]any{"id": 1, "name": "B", "slug": "b"},
			"columns":  []any{map[string]any{"id": 2, "name": "Todo"}},
			"sessions": sessions,
		})
	})
	mux.HandleFunc("POST /api/tickets/42/session", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 201, session)
	})
	mux.HandleFunc("GET /api/sessions/7/discover-tasks", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{
			"tasks": []client.Task{
				{Label: "Web", Command: "npm run dev", ContainerPort: 5173, HasPort: true},
				{Label: "Tests", Command: "go test ./..."},
			},
			"warnings": []string{},
		})
	})
	mux.HandleFunc("GET /api/sessions/7/ports", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		writeJSON(w, 200, nonNilPorts(f.ports))
	})
	mux.HandleFunc("POST /api/sessions/7/ports", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.portOpens++
		if len(f.ports) == 0 {
			f.ports = []client.Port{{ID: 1, SessionID: 7, Label: "Web", ContainerPort: 5173, HostPort: 13001, ProxyActive: true}}
		}
		writeJSON(w, 201, f.ports)
	})
	mux.HandleFunc("GET /api/sessions/7/task-runs", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		writeJSON(w, 200, f.runs)
	})
	mux.HandleFunc("POST /api/sessions/7/task-runs", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Label string }
		_ = json.NewDecoder(r.Body).Decode(&req)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.started = append(f.started, req.Label)
		tr := client.TaskRun{ID: 100 + int64(len(f.started)), SessionID: 7, TaskLabel: req.Label, Status: "running"}
		f.runs = append([]client.TaskRun{tr}, f.runs...)
		writeJSON(w, 201, tr)
	})
	mux.HandleFunc("DELETE /api/task-runs/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
		f.mu.Lock()
		f.stopped = append(f.stopped, id)
		f.mu.Unlock()
		w.WriteHeader(204)
	})
	mux.HandleFunc("GET /api/task-runs/{id}/output", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.outputCalls++
		f.mu.Unlock()
		if f.onOutput != nil {
			f.onOutput(w, r)
			return
		}
		id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
		f.finish(id, "exited", f.exitCode)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: hello\n\ndata:  indented\n\nevent: end\ndata: {}\n\n")
	})
	return mux
}

// finish marks run id as ended, as the server's runner does before it
// closes the output stream.
func (f *fakeTasksAPI) finish(id int64, status string, code int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.runs {
		if f.runs[i].ID == id {
			f.runs[i].Status = status
			f.runs[i].ExitCode = &code
		}
	}
}

func nonNilPorts(p []client.Port) []client.Port {
	if p == nil {
		return []client.Port{}
	}
	return p
}

func runTasksCmd(t *testing.T, srvURL string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	// These exercise the non-interactive paths; never open the task view.
	restore := stdinIsTerminal
	stdinIsTerminal = func() bool { return false }
	defer func() { stdinIsTerminal = restore }()
	cmd := ticketCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(append(append([]string{"tasks"}, args...), "--server", srvURL))
	err = cmd.ExecuteContext(t.Context())
	return out.String(), errOut.String(), err
}

func newFakeTasksServer(t *testing.T, f *fakeTasksAPI) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(f.handler(t))
	t.Cleanup(srv.Close)
	return srv
}

func TestTicketTasksList(t *testing.T) {
	zero := 0
	f := &fakeTasksAPI{
		runs: []client.TaskRun{
			{ID: 5, SessionID: 7, TaskLabel: "Web", Status: "running"},
			{ID: 4, SessionID: 7, TaskLabel: "Tests", Status: "exited", ExitCode: &zero},
			{ID: 3, SessionID: 7, TaskLabel: "Web", Status: "exited", ExitCode: &zero},
		},
		ports: []client.Port{{ID: 1, SessionID: 7, Label: "Web", ContainerPort: 5173, HostPort: 13001, ProxyActive: true}},
	}
	srv := newFakeTasksServer(t, f)

	out, _, err := runTasksCmd(t, srv.URL, "42")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"TASK", "Web", "5173", "running (run #5)", "http://127.0.0.1:13001", "Tests", "exited 0"} {
		if !strings.Contains(out, want) {
			t.Errorf("list missing %q:\n%s", want, out)
		}
	}

	out, _, err = runTasksCmd(t, srv.URL, "42", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Tasks []taskRow `json:"tasks"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decode %q: %v", out, err)
	}
	if len(got.Tasks) != 2 || got.Tasks[0].URL != "http://127.0.0.1:13001" || got.Tasks[0].LastRun.ID != 5 || got.Tasks[1].URL != "" {
		t.Errorf("json rows = %+v", got.Tasks)
	}
}

func TestTicketTasksNoSession(t *testing.T) {
	srv := newFakeTasksServer(t, &fakeTasksAPI{noSession: true})
	_, _, err := runTasksCmd(t, srv.URL, "42")
	if err == nil || !strings.Contains(err.Error(), "no session yet") {
		t.Fatalf("err = %v, want no-session error", err)
	}
}

func TestTicketTasksRunFollows(t *testing.T) {
	f := &fakeTasksAPI{}
	srv := newFakeTasksServer(t, f)

	out, errOut, err := runTasksCmd(t, srv.URL, "42", "--run", "Web")
	if err != nil {
		t.Fatalf("%v\n%s", err, errOut)
	}
	if out != "hello\n indented\n" {
		t.Errorf("stdout = %q, want only the task's output", out)
	}
	if !strings.Contains(errOut, "Web → http://127.0.0.1:13001 (container port 5173)") {
		t.Errorf("stderr missing proxied URL:\n%s", errOut)
	}
	if !strings.Contains(errOut, "exited 0") {
		t.Errorf("stderr missing exit status:\n%s", errOut)
	}
	if len(f.started) != 1 || f.started[0] != "Web" || f.portOpens != 1 {
		t.Errorf("started=%v portOpens=%d", f.started, f.portOpens)
	}

	f.exitCode = 3
	_, _, err = runTasksCmd(t, srv.URL, "42", "--run", "Tests")
	if err == nil || !strings.Contains(err.Error(), "exited with code 3") {
		t.Errorf("err = %v, want exit code 3", err)
	}
	if f.portOpens != 1 {
		t.Errorf("a task without a port opened a proxy (opens=%d)", f.portOpens)
	}
}

func TestTicketTasksRunDetach(t *testing.T) {
	f := &fakeTasksAPI{}
	srv := newFakeTasksServer(t, f)

	out, errOut, err := runTasksCmd(t, srv.URL, "42", "--run", "Web", "-d")
	if err != nil {
		t.Fatal(err)
	}
	if out != "" || f.outputCalls != 0 {
		t.Errorf("detached run streamed output: stdout=%q calls=%d", out, f.outputCalls)
	}
	if !strings.Contains(errOut, "http://127.0.0.1:13001") || !strings.Contains(errOut, `--stop "Web"`) {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestTicketTasksRunUnknownLabel(t *testing.T) {
	f := &fakeTasksAPI{}
	srv := newFakeTasksServer(t, f)
	_, _, err := runTasksCmd(t, srv.URL, "42", "--run", "Nope")
	if err == nil || !strings.Contains(err.Error(), `available: "Web", "Tests"`) {
		t.Fatalf("err = %v", err)
	}
	if len(f.started) != 0 {
		t.Errorf("started %v for an unknown label", f.started)
	}
}

func TestTicketTasksStop(t *testing.T) {
	zero := 0
	f := &fakeTasksAPI{runs: []client.TaskRun{
		{ID: 9, SessionID: 7, TaskLabel: "Web", Status: "running"},
		{ID: 8, SessionID: 7, TaskLabel: "Web", Status: "exited", ExitCode: &zero},
		{ID: 6, SessionID: 7, TaskLabel: "Tests", Status: "running"},
	}}
	srv := newFakeTasksServer(t, f)

	out, _, err := runTasksCmd(t, srv.URL, "42", "--stop", "Web")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.stopped) != 1 || f.stopped[0] != 9 || !strings.Contains(out, "run #9") {
		t.Errorf("stopped=%v out=%q", f.stopped, out)
	}

	f.stopped = nil
	if _, _, err := runTasksCmd(t, srv.URL, "42", "--stop", "Missing"); err == nil {
		t.Error("stopping a task with no running run should fail")
	}
}

func TestTicketTasksFlagValidation(t *testing.T) {
	srv := newFakeTasksServer(t, &fakeTasksAPI{})
	for _, args := range [][]string{
		{"42", "--run", "Web", "--stop", "Web"},
		{"42", "--detach"},
		{"42", "--run", "Web", "--json"},
	} {
		if _, _, err := runTasksCmd(t, srv.URL, args...); err == nil {
			t.Errorf("%v: expected an error", args)
		}
	}
}

func TestProxyURL(t *testing.T) {
	for in, want := range map[string]string{
		"http://localhost:7474":   "http://localhost:13001",
		"https://kanban.lan:7474": "http://kanban.lan:13001",
		"http://[::1]:7474":       "http://[::1]:13001",
		"::bad":                   "http://localhost:13001",
	} {
		if got := proxyURL(in, 13001); got != want {
			t.Errorf("proxyURL(%q) = %q, want %q", in, got, want)
		}
	}
}
