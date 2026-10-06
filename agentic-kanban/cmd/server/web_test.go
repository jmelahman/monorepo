package server

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/jmelahman/kanban/internal/session"
)

// newProxiedBackend starts a stand-in kanban backend and a `kanban web`
// handler in front of it, returning the front's base URL.
func newProxiedBackend(t *testing.T, backend http.Handler) *url.URL {
	t.Helper()
	back := httptest.NewServer(backend)
	t.Cleanup(back.Close)
	target, err := parseBackendURL(back.URL)
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(newWebHandler(target))
	t.Cleanup(front.Close)
	u, err := url.Parse(front.URL)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestWebProxiesAPI(t *testing.T) {
	var gotHost, gotBody, gotForwardedFor string
	front := newProxiedBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/boards" || r.URL.RawQuery != "x=1" {
			http.NotFound(w, r)
			return
		}
		b, _ := io.ReadAll(r.Body)
		gotHost, gotBody, gotForwardedFor = r.Host, string(b), r.Header.Get("X-Forwarded-For")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":7}`)
	}))

	req, _ := http.NewRequest(http.MethodPost, front.String()+"/api/boards?x=1", strings.NewReader(`{"name":"b"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "10.9.9.9")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated || string(body) != `{"id":7}` {
		t.Fatalf("got %d %q, want 201 and the backend's body", resp.StatusCode, body)
	}
	if gotBody != `{"name":"b"}` {
		t.Fatalf("backend saw body %q", gotBody)
	}
	if gotForwardedFor != "127.0.0.1" {
		t.Fatalf("backend saw X-Forwarded-For %q, want only the address the proxy saw", gotForwardedFor)
	}
	if gotHost != front.Host {
		t.Fatalf("backend saw Host %q, want the browser-facing %q", gotHost, front.Host)
	}
}

func TestWebDoesNotProxyOtherPaths(t *testing.T) {
	front := newProxiedBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("backend received %s, which should be served locally", r.URL.Path)
	}))
	for _, path := range []string{"/", "/metrics", "/prometheus/api/v1/query", "/apiary"} {
		resp, err := http.Get(front.String() + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}
}

func TestWebReportsUnreachableBackend(t *testing.T) {
	back := httptest.NewServer(http.NotFoundHandler())
	target, _ := parseBackendURL(back.URL)
	back.Close()
	front := httptest.NewServer(newWebHandler(target))
	defer front.Close()

	resp, err := http.Get(front.URL + "/api/boards")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("got %d, want 502", resp.StatusCode)
	}
}

// A streamed chunk must reach the client while the response is still open; a
// buffering proxy would only deliver it once the backend hangs up.
func TestWebStreamsWithoutBuffering(t *testing.T) {
	for _, contentType := range []string{"text/event-stream", "text/plain"} {
		t.Run(contentType, func(t *testing.T) {
			release := make(chan struct{})
			t.Cleanup(func() { close(release) })
			front := newProxiedBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", contentType)
				_, _ = io.WriteString(w, "data: first\n\n")
				w.(http.Flusher).Flush()
				select {
				case <-release:
				case <-r.Context().Done():
				}
			}))

			resp, err := http.Get(front.String() + "/api/events")
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			line := make(chan string, 1)
			go func() {
				s, _ := bufio.NewReader(resp.Body).ReadString('\n')
				line <- s
			}()
			select {
			case got := <-line:
				if got != "data: first\n" {
					t.Fatalf("got %q", got)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("chunk was not delivered while the stream was open")
			}
		})
	}
}

// The backend guards its PTY WebSockets with session.CheckSameOrigin. Through
// the proxy the browser's Origin names the front server, so the upgrade only
// passes if the proxy forwards the browser's Host.
// See REGRESSIONS.md: "Proxying `/ws` must preserve `Host`".
func TestWebProxiesPTYWebSocket(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: session.CheckSameOrigin}
	front := newProxiedBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if err := conn.WriteMessage(mt, append([]byte("echo:"), msg...)); err != nil {
				return
			}
		}
	}))
	wsURL := fmt.Sprintf("ws://%s/ws/sessions/1/pty", front.Host)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Origin": {front.String()}})
	if err != nil {
		t.Fatalf("same-origin upgrade through the proxy failed: %v", err)
	}
	defer conn.Close()
	if err := conn.WriteMessage(websocket.TextMessage, []byte("ls")); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil || string(msg) != "echo:ls" {
		t.Fatalf("got %q, %v", msg, err)
	}

	// The proxy must not turn a cross-site page into a same-origin one.
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Origin": {"http://evil.example"}})
	if err == nil {
		t.Fatal("cross-origin upgrade was accepted")
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin upgrade: got %v, want 403", resp)
	}
}

func TestParseBackendURL(t *testing.T) {
	for _, bad := range []string{"", "devbox:7474", "localhost", "ftp://devbox", "http://"} {
		if _, err := parseBackendURL(bad); err == nil {
			t.Errorf("parseBackendURL(%q) accepted", bad)
		}
	}
	if _, err := parseBackendURL("https://devbox.tailnet.ts.net"); err != nil {
		t.Errorf("https URL rejected: %v", err)
	}
}
