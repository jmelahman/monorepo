package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/jmelahman/kanban/internal/client"
)

func TestParseDetachKeys(t *testing.T) {
	cases := []struct {
		in      string
		want    []byte
		wantErr bool
	}{
		{"ctrl-p,ctrl-q", []byte{0x10, 0x11}, false},
		{"ctrl-]", []byte{0x1d}, false},
		{"CTRL-A", []byte{0x01}, false},
		{"ctrl-@", []byte{0x00}, false},
		{" ctrl-p , q ", []byte{0x10, 'q'}, false},
		{"a,b", []byte{'a', 'b'}, false},
		{"", nil, true},
		{"ctrl-", nil, true},
		{"ctrl-1", nil, true},
		{"ab", nil, true},
		{"ctrl-p,", nil, true},
	}
	for _, c := range cases {
		got, err := parseDetachKeys(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("%q: err = %v, wantErr %v", c.in, err, c.wantErr)
			continue
		}
		if !c.wantErr && !bytes.Equal(got, c.want) {
			t.Errorf("%q = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestDetachDetector(t *testing.T) {
	seq := []byte{0x10, 0x11}
	check := func(t *testing.T, d *detachDetector, in string, wantFwd string, wantHit bool) {
		t.Helper()
		fwd, hit := d.feed([]byte(in))
		if string(fwd) != wantFwd || hit != wantHit {
			t.Errorf("feed(%q) = (%q, %v), want (%q, %v)", in, fwd, hit, wantFwd, wantHit)
		}
	}
	t.Run("plain_bytes_pass", func(t *testing.T) {
		check(t, &detachDetector{seq: seq}, "hello", "hello", false)
	})
	t.Run("sequence_in_one_feed", func(t *testing.T) {
		check(t, &detachDetector{seq: seq}, "ab\x10\x11cd", "ab", true)
	})
	t.Run("split_across_feeds", func(t *testing.T) {
		d := &detachDetector{seq: seq}
		check(t, d, "a\x10", "a", false)
		check(t, d, "\x11", "", true)
	})
	t.Run("prefix_released_on_mismatch", func(t *testing.T) {
		d := &detachDetector{seq: seq}
		check(t, d, "\x10", "", false)
		check(t, d, "x", "\x10x", false)
	})
	t.Run("restarts_on_repeated_first_byte", func(t *testing.T) {
		check(t, &detachDetector{seq: seq}, "\x10\x10\x11", "\x10", true)
	})
	t.Run("single_key_sequence", func(t *testing.T) {
		check(t, &detachDetector{seq: []byte{0x1d}}, "ab\x1d", "ab", true)
	})
	t.Run("empty_sequence_never_hits", func(t *testing.T) {
		check(t, &detachDetector{}, "\x10\x11", "\x10\x11", false)
	})
}

func TestSessionWSURL(t *testing.T) {
	cases := []struct {
		in         string
		id         int64
		kind       string
		wantURL    string
		wantOrigin string
	}{
		{"http://localhost:7474", 5, "agent", "ws://localhost:7474/ws/sessions/5/pty", "http://localhost:7474"},
		{"https://kanban.example.com/", 7, "shell", "wss://kanban.example.com/ws/sessions/7/shell", "https://kanban.example.com"},
		{"http://host:1/prefix/?x=1#f", 2, "agent", "ws://host:1/prefix/ws/sessions/2/pty", "http://host:1"},
		{"ws://host:1", 3, "agent", "ws://host:1/ws/sessions/3/pty", "http://host:1"},
	}
	for _, c := range cases {
		gotURL, gotOrigin, err := sessionWSURL(c.in, c.id, c.kind)
		if err != nil {
			t.Errorf("%q: %v", c.in, err)
			continue
		}
		if gotURL != c.wantURL || gotOrigin != c.wantOrigin {
			t.Errorf("%q = (%q, %q), want (%q, %q)", c.in, gotURL, gotOrigin, c.wantURL, c.wantOrigin)
		}
	}
	for _, bad := range []string{"ftp://x", "http://", "::bad"} {
		if _, _, err := sessionWSURL(bad, 1, "agent"); err == nil {
			t.Errorf("%q: expected error", bad)
		}
	}
}

// syncBuffer is a bytes.Buffer safe for the attach reader goroutine.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

type attachOutcome struct {
	detached bool
	err      error
}

// wsServer serves one upgraded connection per request to handle.
func wsServer(t *testing.T, handle func(r *http.Request, conn *websocket.Conn)) *httptest.Server {
	t.Helper()
	up := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		handle(r, conn)
	}))
	t.Cleanup(srv.Close)
	return srv
}

type resizeMsg struct {
	Type string `json:"type"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

func TestAttachSessionEchoAndDetach(t *testing.T) {
	var (
		mu       sync.Mutex
		origin   string
		path     string
		resizes  []resizeMsg
		closeErr error
	)
	srv := wsServer(t, func(r *http.Request, conn *websocket.Conn) {
		mu.Lock()
		origin, path = r.Header.Get("Origin"), r.URL.Path
		mu.Unlock()
		_ = conn.WriteMessage(websocket.BinaryMessage, []byte("\x1bcwelcome"))
		for {
			mt, data, err := conn.ReadMessage()
			if err != nil {
				mu.Lock()
				closeErr = err
				mu.Unlock()
				return
			}
			switch mt {
			case websocket.TextMessage:
				var m resizeMsg
				_ = json.Unmarshal(data, &m)
				mu.Lock()
				resizes = append(resizes, m)
				mu.Unlock()
			case websocket.BinaryMessage:
				_ = conn.WriteMessage(websocket.BinaryMessage, append([]byte("echo:"), data...))
			}
		}
	})

	inR, inW := io.Pipe()
	t.Cleanup(func() { inW.Close() })
	var out syncBuffer
	resize := make(chan struct{}, 1)
	var sizeMu sync.Mutex
	cols, rows := 80, 24
	size := func() (int, int, bool) {
		sizeMu.Lock()
		defer sizeMu.Unlock()
		return cols, rows, true
	}
	done := make(chan attachOutcome, 1)
	go func() {
		d, err := attachSession(t.Context(), srv.URL, 9, "agent", []byte{0x10, 0x11}, inR, &out, size, resize)
		done <- attachOutcome{d, err}
	}()

	waitFor(t, "welcome frame", func() bool { return strings.Contains(out.String(), "\x1bcwelcome") })
	mu.Lock()
	if origin != srv.URL || path != "/ws/sessions/9/pty" {
		t.Errorf("origin=%q path=%q, want %q and /ws/sessions/9/pty", origin, path, srv.URL)
	}
	mu.Unlock()
	waitFor(t, "initial resize", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(resizes) == 1 && resizes[0] == resizeMsg{"resize", 80, 24}
	})

	if _, err := inW.Write([]byte("abc")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "echo", func() bool { return strings.Contains(out.String(), "echo:abc") })

	sizeMu.Lock()
	cols, rows = 100, 40
	sizeMu.Unlock()
	resize <- struct{}{}
	waitFor(t, "second resize", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(resizes) == 2 && resizes[1] == resizeMsg{"resize", 100, 40}
	})

	// A held-back prefix that doesn't complete is forwarded.
	if _, err := inW.Write([]byte("\x10z")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "released prefix", func() bool { return strings.Contains(out.String(), "echo:\x10z") })

	if _, err := inW.Write([]byte("\x10\x11ignored")); err != nil {
		t.Fatal(err)
	}
	select {
	case o := <-done:
		if o.err != nil || !o.detached {
			t.Fatalf("detached=%v err=%v, want detached", o.detached, o.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("attach did not return after the detach sequence")
	}
	if strings.Contains(out.String(), "ignored") {
		t.Errorf("bytes after the detach sequence were forwarded: %q", out.String())
	}
	waitFor(t, "server close", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return closeErr != nil
	})
	mu.Lock()
	var ce *websocket.CloseError
	if !errors.As(closeErr, &ce) || ce.Code != websocket.CloseNormalClosure {
		t.Errorf("server saw %v, want a normal close frame", closeErr)
	}
	mu.Unlock()
}

func TestAttachSessionRemoteEnd(t *testing.T) {
	t.Run("error_frame", func(t *testing.T) {
		srv := wsServer(t, func(_ *http.Request, conn *websocket.Conn) {
			_, _, _ = conn.ReadMessage() // resize
			_ = conn.WriteMessage(websocket.TextMessage, []byte("error: boom"))
		})
		inR, inW := io.Pipe()
		t.Cleanup(func() { inW.Close() })
		var out syncBuffer
		d, err := attachSession(t.Context(), srv.URL, 1, "agent", []byte{0x1d}, inR, &out, func() (int, int, bool) { return 80, 24, true }, nil)
		if d || err == nil || err.Error() != "attach: boom" {
			t.Errorf("detached=%v err=%v, want error boom", d, err)
		}
	})

	t.Run("session_ended_then_abrupt_close", func(t *testing.T) {
		srv := wsServer(t, func(_ *http.Request, conn *websocket.Conn) {
			_, _, _ = conn.ReadMessage()
			_ = conn.WriteMessage(websocket.TextMessage, []byte("\r\n[session ended]\r\n"))
			// No close frame: the client sees an abnormal (1006) close.
			_ = conn.NetConn().Close()
		})
		inR, inW := io.Pipe()
		t.Cleanup(func() { inW.Close() })
		var out syncBuffer
		d, err := attachSession(t.Context(), srv.URL, 1, "shell", []byte{0x1d}, inR, &out, func() (int, int, bool) { return 80, 24, true }, nil)
		if d || err != nil {
			t.Errorf("detached=%v err=%v, want clean end", d, err)
		}
		if !strings.Contains(out.String(), "[session ended]") {
			t.Errorf("out = %q", out.String())
		}
	})

	t.Run("clean_close", func(t *testing.T) {
		srv := wsServer(t, func(_ *http.Request, conn *websocket.Conn) {
			_, _, _ = conn.ReadMessage()
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"))
		})
		inR, inW := io.Pipe()
		t.Cleanup(func() { inW.Close() })
		var out syncBuffer
		d, err := attachSession(t.Context(), srv.URL, 1, "agent", []byte{0x1d}, inR, &out, func() (int, int, bool) { return 80, 24, true }, nil)
		if d || err != nil {
			t.Errorf("detached=%v err=%v, want clean end", d, err)
		}
	})

	t.Run("stdin_eof_detaches", func(t *testing.T) {
		srv := wsServer(t, func(_ *http.Request, conn *websocket.Conn) {
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		})
		var out syncBuffer
		// Unknown terminal size: no resize frame is sent, which must not
		// block the loop.
		d, err := attachSession(t.Context(), srv.URL, 1, "agent", []byte{0x1d}, strings.NewReader(""), &out, func() (int, int, bool) { return 0, 0, false }, nil)
		if !d || err != nil {
			t.Errorf("detached=%v err=%v, want detached", d, err)
		}
	})

	t.Run("context_cancel", func(t *testing.T) {
		srv := wsServer(t, func(_ *http.Request, conn *websocket.Conn) {
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		})
		ctx, cancel := context.WithCancel(t.Context())
		inR, inW := io.Pipe()
		t.Cleanup(func() { inW.Close() })
		var out syncBuffer
		done := make(chan attachOutcome, 1)
		go func() {
			d, err := attachSession(ctx, srv.URL, 1, "agent", []byte{0x1d}, inR, &out, func() (int, int, bool) { return 80, 24, true }, nil)
			done <- attachOutcome{d, err}
		}()
		cancel()
		select {
		case o := <-done:
			if !errors.Is(o.err, context.Canceled) {
				t.Errorf("err = %v, want context.Canceled", o.err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("attach did not return after cancel")
		}
	})

	t.Run("handshake_rejected", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "session not running", http.StatusBadRequest)
		}))
		t.Cleanup(srv.Close)
		var out syncBuffer
		_, err := attachSession(t.Context(), srv.URL, 1, "agent", []byte{0x1d}, strings.NewReader(""), &out, func() (int, int, bool) { return 80, 24, true }, nil)
		if err == nil || !strings.Contains(err.Error(), "400") || !strings.Contains(err.Error(), "session not running") {
			t.Errorf("err = %v, want the HTTP status and body", err)
		}
	})
}

// fakeSessionAPI answers the session endpoints ensureRunningSession uses,
// with a scriptable session row. PUT .../harness records the order of calls
// in *calls so tests can check the switch lands before the start.
func fakeSessionAPI(t *testing.T, sess *sessionInfo, onStart func()) (*httptest.Server, *int) {
	srv, starts, _ := fakeSessionAPIWithCalls(t, sess, onStart)
	return srv, starts
}

func fakeSessionAPIWithCalls(t *testing.T, sess *sessionInfo, onStart func()) (*httptest.Server, *int, *[]string) {
	t.Helper()
	starts := 0
	var calls []string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/tickets/42/session":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPut && r.URL.Path == "/api/sessions/7/harness":
			var body struct {
				Harness string `json:"harness"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			sess.Harness = body.Harness
			calls = append(calls, "harness:"+body.Harness)
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Path == "/api/sessions/7/start":
			starts++
			calls = append(calls, "start")
			if onStart != nil {
				onStart()
			}
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(sess)
	}))
	t.Cleanup(srv.Close)
	return srv, &starts, &calls
}

func TestEnsureRunningSession(t *testing.T) {
	t.Run("already_running", func(t *testing.T) {
		sess := &sessionInfo{ID: 7, TicketID: 42, Status: "idle", ContainerID: "abc"}
		srv, starts := fakeSessionAPI(t, sess, nil)
		var out bytes.Buffer
		got, err := ensureRunningSession(t.Context(), client.New(srv.URL, nil), &out, 42, "")
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != 7 || *starts != 0 || out.Len() != 0 {
			t.Errorf("got %+v starts=%d out=%q", got, *starts, out.String())
		}
	})

	t.Run("starts_stopped_session", func(t *testing.T) {
		sess := &sessionInfo{ID: 7, TicketID: 42, Status: "stopped"}
		srv, starts := fakeSessionAPI(t, sess, func() { sess.Status, sess.ContainerID = "idle", "abc" })
		var out bytes.Buffer
		got, err := ensureRunningSession(t.Context(), client.New(srv.URL, nil), &out, 42, "")
		if err != nil {
			t.Fatal(err)
		}
		if got.ContainerID != "abc" || *starts != 1 {
			t.Errorf("got %+v starts=%d", got, *starts)
		}
		if !strings.Contains(out.String(), "starting session #7") {
			t.Errorf("out = %q", out.String())
		}
	})

	t.Run("switches_harness_before_start", func(t *testing.T) {
		sess := &sessionInfo{ID: 7, TicketID: 42, Status: "stopped"}
		srv, _, calls := fakeSessionAPIWithCalls(t, sess, func() { sess.Status, sess.ContainerID = "idle", "abc" })
		var out bytes.Buffer
		got, err := ensureRunningSession(t.Context(), client.New(srv.URL, nil), &out, 42, "pi")
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{"harness:pi", "start"}; !reflect.DeepEqual(*calls, want) {
			t.Errorf("calls = %v, want %v", *calls, want)
		}
		if got.Harness != "pi" || !strings.Contains(out.String(), "switching session #7 to the pi harness") {
			t.Errorf("got %+v out=%q", got, out.String())
		}
	})

	t.Run("same_harness_is_not_resent", func(t *testing.T) {
		sess := &sessionInfo{ID: 7, TicketID: 42, Status: "idle", ContainerID: "abc", Harness: "pi"}
		srv, _, calls := fakeSessionAPIWithCalls(t, sess, nil)
		var out bytes.Buffer
		if _, err := ensureRunningSession(t.Context(), client.New(srv.URL, nil), &out, 42, "pi"); err != nil {
			t.Fatal(err)
		}
		if len(*calls) != 0 {
			t.Errorf("calls = %v, want none", *calls)
		}
	})

	t.Run("start_did_not_produce_container", func(t *testing.T) {
		sess := &sessionInfo{ID: 7, TicketID: 42, Status: "error"}
		srv, _ := fakeSessionAPI(t, sess, nil)
		var out bytes.Buffer
		_, err := ensureRunningSession(t.Context(), client.New(srv.URL, nil), &out, 42, "")
		if err == nil || !strings.Contains(err.Error(), "not running") {
			t.Errorf("err = %v", err)
		}
	})
}

func TestRunTicketAttachNeedsTTY(t *testing.T) {
	restore := stdinIsTerminal
	stdinIsTerminal = func() bool { return false }
	t.Cleanup(func() { stdinIsTerminal = restore })

	var out bytes.Buffer
	err := runTicketAttach(t.Context(), "http://127.0.0.1:1", &out, 1, "agent", defaultDetachKeys, "")
	if err == nil || !strings.Contains(err.Error(), "interactive terminal") {
		t.Errorf("err = %v", err)
	}
	err = runTicketAttach(t.Context(), "http://127.0.0.1:1", &out, 1, "agent", "ctrl-1", "")
	if err == nil || !strings.Contains(err.Error(), "detach keys") {
		t.Errorf("bad detach keys: err = %v", err)
	}
}
