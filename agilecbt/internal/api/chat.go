package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/jmelahman/agilecbt/internal/db"
)

// handleChat runs one curator turn and streams it as server-sent events:
//
//	event: text    data: {"text": "..."}        (assistant text delta)
//	event: action  data: <db.Action>            (an AI write, undoable)
//	event: error   data: {"error": "..."}
//	event: done    data: {}
//
// Actions are delivered by the app's broker, so tool calls made during the
// turn appear as chips.
func (d Deps) handleChat(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var req struct {
		Text string `json:"text"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, r, err)
		return
	}
	req.Text = strings.TrimSpace(req.Text)
	if req.Text == "" {
		httpError(w, http.StatusBadRequest, "text is required")
		return
	}
	if _, err := d.store().GetCheckin(id); err != nil {
		fail(w, r, err)
		return
	}
	if d.Curator == nil || !d.Curator.Status(r.Context()).Available {
		fail(w, r, errUnavailable)
		return
	}
	rc := http.NewResponseController(w)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	_ = rc.Flush()

	var mu sync.Mutex
	emit := func(event string, data any) {
		b, err := json.Marshal(data)
		if err != nil {
			log.Printf("chat: encode %s event: %v", event, err)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
		_ = rc.Flush()
	}

	unsubscribe := d.App.Broker.Subscribe(id, func(act db.Action) { emit("action", act) })
	err = d.Curator.Chat(r.Context(), id, req.Text, emit)
	unsubscribe()
	if err != nil {
		log.Printf("chat on check-in %d: %v", id, err)
		emit("error", map[string]string{"error": userFacing(err)})
	}
	emit("done", struct{}{})
}

// userFacing keeps backend errors short; details go to the server log.
func userFacing(err error) string {
	msg := err.Error()
	if len(msg) > 300 {
		msg = msg[:300] + "…"
	}
	return msg
}
