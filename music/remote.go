package main

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
)

// The remote: a page playing in a browser is a player the phone can see
// and drive. The server keeps no queue of its own; it passes each
// player's state out to whoever watches, and commands in to the player.
// Both sides keep one event stream open to api/remote/events.

// maxRemoteBody bounds a state or a command: a queue of a few thousand
// tracks fits with room to spare.
const maxRemoteBody = 4 << 20

// remotePing keeps an idle stream from being cut by a proxy, and lets a
// watcher notice a dead one.
var remotePing = 20 * time.Second

// remotePlayer is one page playing somewhere: what it calls itself, its
// last state as it put it, and its open streams.
type remotePlayer struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	State json.RawMessage `json:"state"`
	conns int
}

// remoteEvent is one message out on a stream: its name and JSON.
type remoteEvent struct {
	name string
	data []byte
}

type remoteSub struct {
	player string // the player it is, or "" for a watcher
	ch     chan remoteEvent
}

type remoteHub struct {
	mu      sync.Mutex
	players map[string]*remotePlayer
	subs    map[*remoteSub]struct{}
}

func newRemoteHub() *remoteHub {
	return &remoteHub{players: map[string]*remotePlayer{}, subs: map[*remoteSub]struct{}{}}
}

// list is the players with a stream open, by name; mu held.
func (h *remoteHub) list() []*remotePlayer {
	out := []*remotePlayer{}
	for _, p := range h.players {
		if p.conns > 0 {
			out = append(out, p)
		}
	}
	slices.SortFunc(out, func(a, b *remotePlayer) int { return strings.Compare(a.Name+a.ID, b.Name+b.ID) })
	return out
}

// send hands an event to a stream without waiting on it: a stream too
// far behind to take it is dropped, and reconnects; mu held.
func (h *remoteHub) send(s *remoteSub, e remoteEvent) {
	select {
	case s.ch <- e:
	default:
		delete(h.subs, s)
		close(s.ch)
	}
}

// announce tells every stream the players as they now stand; mu held.
func (h *remoteHub) announce() {
	data, _ := json.Marshal(h.list())
	for s := range h.subs {
		h.send(s, remoteEvent{"players", data})
	}
}

func (h *remoteHub) subscribe(id, name string) *remoteSub {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := &remoteSub{player: id, ch: make(chan remoteEvent, 64)}
	h.subs[s] = struct{}{}
	if id != "" {
		p := h.players[id]
		if p == nil {
			p = &remotePlayer{ID: id, State: json.RawMessage("null")}
			h.players[id] = p
		}
		p.Name = name
		p.conns++
		h.announce()
	} else {
		data, _ := json.Marshal(h.list())
		h.send(s, remoteEvent{"players", data})
	}
	return s
}

func (h *remoteHub) unsubscribe(s *remoteSub) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.subs[s]; ok {
		delete(h.subs, s)
		close(s.ch)
	}
	if s.player == "" {
		return
	}
	if p := h.players[s.player]; p != nil {
		p.conns--
		if p.conns <= 0 {
			delete(h.players, s.player)
		}
		h.announce()
	}
}

// setState keeps a player's state and passes it on. It reports false
// for a player with no stream open.
func (h *remoteHub) setState(id string, state json.RawMessage) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	p := h.players[id]
	if p == nil {
		return false
	}
	p.State = state
	h.announce()
	return true
}

// command hands a command to a player's streams. It reports false for a
// player with none open.
func (h *remoteHub) command(id string, cmd json.RawMessage) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	sent := false
	for s := range h.subs {
		if s.player == id {
			h.send(s, remoteEvent{"command", cmd})
			sent = true
		}
	}
	return sent
}

func readJSON(w http.ResponseWriter, r *http.Request) (json.RawMessage, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRemoteBody))
	if err != nil || !json.Valid(body) {
		http.Error(w, "want JSON", http.StatusBadRequest)
		return nil, false
	}
	return body, true
}

func routeRemote(ctx context.Context, mux *http.ServeMux, hub *remoteHub) {
	// A player opens it with its id and name, to be listed and take
	// commands; a watcher opens it bare. Both are sent "players" with
	// every change.
	mux.HandleFunc("GET /api/remote/events", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		id := r.URL.Query().Get("id")
		name := cmp.Or(r.URL.Query().Get("name"), "A browser")
		h := w.Header()
		h.Set("Content-Type", "text/event-stream")
		h.Set("Cache-Control", "no-cache")
		h.Set("X-Accel-Buffering", "no") // nginx would otherwise hold it
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		s := hub.subscribe(id, name)
		defer hub.unsubscribe(s)
		ping := time.NewTicker(remotePing)
		defer ping.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-r.Context().Done():
				return
			case <-ping.C:
				if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
					return
				}
			case e, ok := <-s.ch:
				if !ok {
					return
				}
				if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.name, e.data); err != nil {
					return
				}
			}
			flusher.Flush()
		}
	})
	mux.HandleFunc("GET /api/remote/players", func(w http.ResponseWriter, _ *http.Request) {
		hub.mu.Lock()
		defer hub.mu.Unlock()
		writeJSON(w, hub.list())
	})
	mux.HandleFunc("PUT /api/remote/players/{id}", func(w http.ResponseWriter, r *http.Request) {
		state, ok := readJSON(w, r)
		if !ok {
			return
		}
		if !hub.setState(r.PathValue("id"), state) {
			http.Error(w, "no such player", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/remote/players/{id}/command", func(w http.ResponseWriter, r *http.Request) {
		cmd, ok := readJSON(w, r)
		if !ok {
			return
		}
		if !hub.command(r.PathValue("id"), cmd) {
			http.Error(w, "that player has closed", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
