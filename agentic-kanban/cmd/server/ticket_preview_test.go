package server

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jmelahman/kanban/internal/db"
)

// previewStub routes the preview endpoints to canned deploys (the real ones
// need the orchestrator and a manifest) and lets every lookup through to the
// real server. GET /api/previews/7 answers each of polls in turn, repeating
// the last.
func previewStub(t *testing.T, real string, created string, polls []string, logs string) (*httptest.Server, *string) {
	t.Helper()
	target, _ := url.Parse(real)
	proxy := httputil.NewSingleHostReverseProxy(target)
	var deployed string
	var n atomic.Int32
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/previews"):
			deployed = r.URL.Path
			w.WriteHeader(http.StatusAccepted)
			fmt.Fprint(w, created)
		case r.URL.Path == "/api/previews/7":
			i := int(n.Add(1)) - 1
			fmt.Fprint(w, polls[min(i, len(polls)-1)])
		case r.URL.Path == "/api/previews/7/logs":
			fmt.Fprint(w, logs)
		default:
			proxy.ServeHTTP(w, r)
		}
	}))
	t.Cleanup(stub.Close)
	return stub, &deployed
}

func TestRunTicketPreview(t *testing.T) {
	old := previewPollInterval
	previewPollInterval = time.Millisecond
	t.Cleanup(func() { previewPollInterval = old })

	srv, store, board := newKanbanCLITestServer(t)
	cols, _ := store.ListColumns(t.Context(), board.ID)
	tk := &db.Ticket{BoardID: board.ID, ColumnID: cols[0].ID, Title: "Preview me", Slug: "preview-me"}
	if err := store.CreateTicket(t.Context(), tk); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := runTicketPreview(t.Context(), srv.URL, &out, tk.ID, true, false)
	if err == nil || !strings.Contains(err.Error(), "no session") {
		t.Fatalf("preview without a session: err = %v", err)
	}

	sess := &db.Session{TicketID: tk.ID, Status: db.SessionStatusIdle, BranchName: "kanban/cli-board/preview-me"}
	if err := store.UpsertSession(t.Context(), sess); err != nil {
		t.Fatal(err)
	}

	t.Run("ready", func(t *testing.T) {
		stub, deployed := previewStub(t, srv.URL,
			`{"id":7,"short_sha":"abc1234","status":"queued"}`,
			[]string{
				`{"id":7,"short_sha":"abc1234","status":"building"}`,
				`{"id":7,"short_sha":"abc1234","status":"ready","fe_hash":"f","preview_url":"http://abc1234-cli.preview.localhost/",` +
					`"artifacts":[{"name":"cli","files":[{"name":"kanban linux","size":3}]}]}`,
			}, "")
		out.Reset()
		if err := runTicketPreview(t.Context(), stub.URL, &out, tk.ID, true, false); err != nil {
			t.Fatal(err)
		}
		if want := fmt.Sprintf("/api/sessions/%d/previews", sess.ID); *deployed != want {
			t.Errorf("deployed %q, want %q", *deployed, want)
		}
		for _, want := range []string{
			"preview #7 of kanban/cli-board/preview-me (abc1234): queued\n",
			"preview #7: building\n",
			"preview #7: ready\n",
			"http://abc1234-cli.preview.localhost/\n",
			"cli/kanban linux: " + stub.URL + "/api/previews/7/artifacts/cli/kanban%20linux\n",
		} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("output = %q, want it to contain %q", out.String(), want)
			}
		}
	})

	t.Run("failed", func(t *testing.T) {
		var log strings.Builder
		for i := range 30 {
			fmt.Fprintf(&log, "line %d\n", i)
		}
		stub, _ := previewStub(t, srv.URL,
			`{"id":7,"short_sha":"abc1234","status":"building"}`,
			[]string{`{"id":7,"short_sha":"abc1234","status":"failed","error":"build exited 1"}`},
			log.String())
		out.Reset()
		err := runTicketPreview(t.Context(), stub.URL, &out, tk.ID, true, false)
		if err == nil || !strings.Contains(err.Error(), "build exited 1") {
			t.Fatalf("err = %v, want the deploy error", err)
		}
		if !strings.Contains(out.String(), "line 29\n") || strings.Contains(out.String(), "line 9\n") {
			t.Errorf("output = %q, want only the last %d log lines", out.String(), previewLogTail)
		}
	})

	t.Run("no wait", func(t *testing.T) {
		stub, _ := previewStub(t, srv.URL,
			`{"id":7,"short_sha":"abc1234","status":"queued"}`,
			[]string{`{"id":7,"short_sha":"abc1234","status":"failed"}`}, "")
		out.Reset()
		if err := runTicketPreview(t.Context(), stub.URL, &out, tk.ID, false, true); err != nil {
			t.Fatal(err)
		}
		if got := strings.TrimSpace(out.String()); got != `{"id":7,"short_sha":"abc1234","status":"queued"}` {
			t.Errorf("output = %q, want the queued deploy JSON", got)
		}
	})
}
