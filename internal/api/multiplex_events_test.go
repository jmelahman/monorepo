package api_test

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"testing"
)

// TestBoardsEvents_Multiplexes verifies that GET /api/events?boards=… fans in
// events from multiple boards over a single SSE stream and tags each event
// with its board_id so the client can demultiplex. This is the endpoint that
// replaces N per-board EventSources to keep the browser's HTTP/1.1 pool from
// starving session PTY WebSocket upgrades.
func TestBoardsEvents_Multiplexes(t *testing.T) {
	e := newEnv(t)
	b1 := e.seedBoard("Multi One")
	b2 := e.seedBoard("Multi Two")
	tk1 := e.seedTicket(b1, "TicketOne")
	tk2 := e.seedTicket(b2, "TicketTwo")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/api/events?boards=%d,%d", e.srv.URL, b1.ID, b2.ID), nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d; want 200", resp.StatusCode)
	}
	conn := &sseConn{resp: resp, cancel: cancel, r: bufio.NewReader(resp.Body)}
	conn.waitReady(t)

	// Trigger an event on each board.
	r1 := e.patch(fmt.Sprintf("/api/tickets/%d", tk1.ID), map[string]any{"title": "renamed-1"})
	assertStatus(t, r1, 200)
	r2 := e.patch(fmt.Sprintf("/api/tickets/%d", tk2.ID), map[string]any{"title": "renamed-2"})
	assertStatus(t, r2, 200)

	got := map[int64]string{}
	for len(got) < 2 {
		ev := conn.next(t)
		if ev.Type != "ticket_updated" {
			continue
		}
		bidRaw, ok := ev.Data["board_id"]
		if !ok {
			t.Fatalf("event missing board_id: %#v", ev.Data)
		}
		bid := int64(bidRaw.(float64))
		payload, _ := ev.Data["data"].(map[string]any)
		if payload == nil {
			t.Fatalf("event has no data payload: %#v", ev.Data)
		}
		got[bid] = payload["title"].(string)
	}
	if got[b1.ID] != "renamed-1" {
		t.Errorf("board %d title = %q; want renamed-1", b1.ID, got[b1.ID])
	}
	if got[b2.ID] != "renamed-2" {
		t.Errorf("board %d title = %q; want renamed-2", b2.ID, got[b2.ID])
	}
}

func TestBoardsEvents_RejectsBadInput(t *testing.T) {
	e := newEnv(t)
	for _, tc := range []struct {
		name string
		q    string
	}{
		{"missing", ""},
		{"empty", "boards="},
		{"non_integer", "boards=abc"},
		{"trailing_bad", "boards=1,abc"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Get(e.srv.URL + "/api/events?" + tc.q)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != 400 {
				t.Fatalf("status = %d; want 400", resp.StatusCode)
			}
		})
	}
}
