package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

type event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

type eventBus struct {
	mu   sync.Mutex
	subs map[int64]map[chan event]struct{}
}

func newEventBus() *eventBus {
	return &eventBus{subs: map[int64]map[chan event]struct{}{}}
}

func (b *eventBus) publish(boardID int64, typ string, data any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs[boardID] {
		select {
		case ch <- event{Type: typ, Data: data}:
		default:
		}
	}
}

func (b *eventBus) subscribe(boardID int64) (chan event, func()) {
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
		httpError(w, fmt.Errorf("streaming unsupported"), 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

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
