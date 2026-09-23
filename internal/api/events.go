package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

type event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

type EventBus struct {
	mu   sync.Mutex
	subs map[int64]map[chan event]struct{}
}

func NewEventBus() *EventBus {
	return &EventBus{subs: map[int64]map[chan event]struct{}{}}
}

func (b *EventBus) Publish(boardID int64, typ string, data any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs[boardID] {
		select {
		case ch <- event{Type: typ, Data: data}:
		default:
		}
	}
}

func (b *EventBus) subscribe(boardID int64) (chan event, func()) {
	ch := make(chan event, 16)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subs[boardID] == nil {
		b.subs[boardID] = map[chan event]struct{}{}
	}
	b.subs[boardID][ch] = struct{}{}
	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.subs[boardID], ch)
		close(ch)
	}
}

func (h *handlers) boardEvents(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.httpError(w, fmt.Errorf("streaming unsupported"), 500)
		return
	}
	writeSSEHeaders(w)

	ch, cancel := h.bus.subscribe(id)
	defer cancel()

	// Send a hello so the client knows the stream opened.
	fmt.Fprintf(w, "event: ready\ndata: {}\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			payload, _ := json.Marshal(ev)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, payload)
			flusher.Flush()
		}
	}
}

// maxMultiplexBoards caps how many boards one stream can fan in. The frontend
// only ever opens one multiplexed stream per page, so this is a guard against
// accidental amplification, not a real product limit.
const maxMultiplexBoards = 200

// boardsEvents handles GET /api/events?boards=1,2,3 — a multiplexed SSE stream
// that fans in events from N boards over a single HTTP/1.1 connection.
// Browsers cap per-origin HTTP/1.1 connections at 6 and route WebSocket
// upgrades through the same pool; running one EventSource per board saturates
// it and queues new PTY WebSocket handshakes indefinitely (terminal stuck on a
// blank cursor in Overview). This endpoint replaces the per-board stream for
// callers that subscribe to many boards.
//
// Wire format mirrors boardEvents but adds board_id so the client can
// demultiplex:
//
//	event: ticket_updated
//	data: {"type":"ticket_updated","data":{...},"board_id":42}
func (h *handlers) boardsEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.httpError(w, fmt.Errorf("streaming unsupported"), 500)
		return
	}
	raw := r.URL.Query().Get("boards")
	if raw == "" {
		h.httpError(w, fmt.Errorf("boards query parameter required"), 400)
		return
	}
	parts := strings.Split(raw, ",")
	if len(parts) > maxMultiplexBoards {
		h.httpError(w, fmt.Errorf("too many boards (max %d)", maxMultiplexBoards), 400)
		return
	}
	seen := make(map[int64]struct{}, len(parts))
	ids := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		id, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			h.httpError(w, fmt.Errorf("invalid board id %q", p), 400)
			return
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		h.httpError(w, fmt.Errorf("boards query parameter required"), 400)
		return
	}

	writeSSEHeaders(w)

	type tagged struct {
		boardID int64
		ev      event
	}
	merged := make(chan tagged, 32)
	cancels := make([]func(), 0, len(ids))
	ctx := r.Context()
	var wg sync.WaitGroup

	for _, id := range ids {
		ch, cancel := h.bus.subscribe(id)
		cancels = append(cancels, cancel)
		wg.Add(1)
		go func(boardID int64, ch chan event) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case ev, ok := <-ch:
					if !ok {
						return
					}
					select {
					case <-ctx.Done():
						return
					case merged <- tagged{boardID: boardID, ev: ev}:
					}
				}
			}
		}(id, ch)
	}
	defer func() {
		for _, cancel := range cancels {
			cancel()
		}
		wg.Wait()
	}()

	fmt.Fprintf(w, "event: ready\ndata: {}\n\n")
	flusher.Flush()

	for {
		select {
		case <-ctx.Done():
			return
		case m := <-merged:
			payload, _ := json.Marshal(struct {
				Type    string `json:"type"`
				Data    any    `json:"data"`
				BoardID int64  `json:"board_id"`
			}{m.ev.Type, m.ev.Data, m.boardID})
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", m.ev.Type, payload)
			flusher.Flush()
		}
	}
}
