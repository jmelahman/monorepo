package main

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// stream opens api/remote/events and hands back its events, name and
// data, as they come.
func stream(t *testing.T, ctx context.Context, url string) <-chan [2]string {
	t.Helper()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type %q", ct)
	}
	out := make(chan [2]string, 16)
	go func() {
		defer resp.Body.Close()
		defer close(out)
		sc := bufio.NewScanner(resp.Body)
		var name string
		for sc.Scan() {
			line := sc.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				name = line[len("event: "):]
			case strings.HasPrefix(line, "data: "):
				out <- [2]string{name, line[len("data: "):]}
			}
		}
	}()
	return out
}

func next(t *testing.T, ch <-chan [2]string, name string) string {
	t.Helper()
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				t.Fatalf("stream ended waiting for %s", name)
			}
			if e[0] == name {
				return e[1]
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("no %s event", name)
		}
	}
}

func send(t *testing.T, method, url, body string) int {
	t.Helper()
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp.StatusCode
}

func TestRemote(t *testing.T) {
	defer func(g time.Duration) { remoteGrace = g }(remoteGrace)
	remoteGrace = 300 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	mux := http.NewServeMux()
	routeRemote(ctx, mux, newRemoteHub())
	srv := httptest.NewServer(mux)
	// The streams end before the server waits on them.
	defer srv.Close()
	defer cancel()

	// No player yet: a command and a state have nowhere to go.
	if code := send(t, "POST", srv.URL+"/api/remote/players/p1/command", `{"do":"next"}`); code != 404 {
		t.Fatalf("command to no player: %d", code)
	}

	watcher := stream(t, ctx, srv.URL+"/api/remote/events")
	if got := next(t, watcher, "players"); got != "[]" {
		t.Fatalf("players before any: %s", got)
	}

	pctx, closePlayer := context.WithCancel(ctx)
	player := stream(t, pctx, srv.URL+"/api/remote/events?id=p1&name=Desk")
	if got := next(t, watcher, "players"); !strings.Contains(got, `"id":"p1","name":"Desk","state":null`) {
		t.Fatalf("watcher saw %s", got)
	}

	if code := send(t, "PUT", srv.URL+"/api/remote/players/p1", `{"title":"Song"}`); code != 204 {
		t.Fatalf("state: %d", code)
	}
	if got := next(t, watcher, "players"); !strings.Contains(got, `"state":{"title":"Song"}`) {
		t.Fatalf("watcher saw %s", got)
	}
	if code := send(t, "PUT", srv.URL+"/api/remote/players/p1", `not json`); code != 400 {
		t.Fatalf("bad state: %d", code)
	}

	if code := send(t, "POST", srv.URL+"/api/remote/players/p1/command", `{"do":"next"}`); code != 204 {
		t.Fatalf("command: %d", code)
	}
	if got := next(t, player, "command"); got != `{"do":"next"}` {
		t.Fatalf("player got %s", got)
	}

	// A reload drops the page's stream for a moment: it stays listed,
	// with its state, and is the same player when it comes back.
	closePlayer()
	// The server learns of the close a moment after the page does.
	for deadline := time.Now().Add(2 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		if send(t, "POST", srv.URL+"/api/remote/players/p1/command", `{"do":"stale"}`) == 404 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("a player between streams still took commands")
		}
	}
	pctx, closePlayer = context.WithCancel(ctx)
	player = stream(t, pctx, srv.URL+"/api/remote/events?id=p1&name=Desk")
	if got := next(t, watcher, "players"); !strings.Contains(got, `"state":{"title":"Song"}`) {
		t.Fatalf("watcher saw %s after the reload", got)
	}
	if code := send(t, "POST", srv.URL+"/api/remote/players/p1/command", `{"do":"prev"}`); code != 204 {
		t.Fatalf("command after reload: %d", code)
	}
	if got := next(t, player, "command"); got != `{"do":"prev"}` {
		t.Fatalf("player got %s", got)
	}

	// Closed for good, it leaves the list once the grace is up.
	closePlayer()
	start := time.Now()
	if got := next(t, watcher, "players"); got != "[]" {
		t.Fatalf("players after close: %s", got)
	}
	if time.Since(start) < remoteGrace/2 {
		t.Fatal("the player left before its grace was up")
	}
}
