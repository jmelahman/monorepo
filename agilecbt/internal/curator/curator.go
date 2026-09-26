// Package curator is the in-app AI check-in coach. It builds the prompt and
// per-turn context, stores the transcript, and delegates generation to a
// Backend (an OpenAI-compatible API such as Ollama or OpenRouter).
package curator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jmelahman/agilecbt/internal/api"
	"github.com/jmelahman/agilecbt/internal/app"
	"github.com/jmelahman/agilecbt/internal/db"
	"github.com/jmelahman/agilecbt/internal/prompt"
	"github.com/jmelahman/agilecbt/internal/tools"
)

// TurnRequest is one user turn in a check-in conversation.
type TurnRequest struct {
	CheckinID int64
	System    string
	// History holds earlier user/assistant messages. It may start with the
	// assistant: a check-in opens with the app's greeting or standup
	// questions (see POST /api/checkins intro).
	History []db.Message
	// User is the new user message, with the fresh context block prepended.
	User string
}

// TurnResult is what a backend produced for a turn.
type TurnResult struct {
	Text string
}

// Backend generates curator replies. Implementations run tools through the
// registry with the ctx they're given, which carries the acting check-in.
type Backend interface {
	Name() string
	Status(ctx context.Context) (available bool, detail string)
	// Turn streams text through onText and returns the full reply.
	Turn(ctx context.Context, req TurnRequest, onText func(string)) (TurnResult, error)
	// Complete is a single tool-free call, used for drafting retros.
	Complete(ctx context.Context, system, user string) (string, error)
}

// Curator implements api.Curator on top of a Backend.
type Curator struct {
	app *app.App
	reg *tools.Registry
	// defaults is nil when the backend is fixed (see New); otherwise the
	// settings in the app layer over it (see NewConfigured).
	defaults *Config

	mu sync.Mutex
	// backend is nil when the coach is turned off.
	backend Backend
	cfg     Config
	busy    map[int64]bool
	cache   statusCache
}

var _ api.Curator = (*Curator)(nil)

// New returns a curator using a fixed backend.
func New(reg *tools.Registry, backend Backend) *Curator {
	return &Curator{app: reg.App(), reg: reg, backend: backend, busy: map[int64]bool{}}
}

func (c *Curator) current() Backend {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.backend
}

// Status reports backend availability, cached briefly because /api/health
// is polled.
func (c *Curator) Status(ctx context.Context) api.LLMStatus {
	b := c.current()
	if b == nil {
		return api.LLMStatus{Backend: llmNone, Detail: "The AI coach is turned off"}
	}
	ok, detail := c.cache.get(ctx, b.Status)
	return api.LLMStatus{Backend: b.Name(), Available: ok, Detail: detail}
}

// ErrBusy means a reply is already streaming for the check-in.
var ErrBusy = errors.New("the curator is still replying to your last message")

// ErrOff means the coach is turned off.
var ErrOff = errors.New("the AI coach is turned off")

// Chat runs one turn on a check-in and stores both messages.
func (c *Curator) Chat(ctx context.Context, checkinID int64, text string, emit func(string, any)) error {
	backend := c.current()
	if backend == nil {
		return ErrOff
	}
	c.mu.Lock()
	if c.busy[checkinID] {
		c.mu.Unlock()
		return ErrBusy
	}
	c.busy[checkinID] = true
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.busy, checkinID)
		c.mu.Unlock()
	}()

	store := c.app.Store
	checkin, err := store.GetCheckin(checkinID)
	if err != nil {
		return err
	}
	history, err := store.ListMessages(checkinID)
	if err != nil {
		return err
	}
	contextBlock, err := c.buildContext(checkin)
	if err != nil {
		return err
	}
	if _, err := store.AppendMessage(checkinID, "user", text, ""); err != nil {
		return err
	}

	ctx = app.WithActor(ctx, app.Actor{Source: "curator", CheckinID: &checkinID})
	req := TurnRequest{
		CheckinID: checkinID,
		System:    prompt.Curator(c.app.CrisisResources()),
		History:   history,
		User:      contextBlock + "\n\n" + text,
	}
	var streamed strings.Builder
	res, turnErr := backend.Turn(ctx, req, func(delta string) {
		streamed.WriteString(delta)
		emit("text", map[string]string{"text": delta})
	})
	reply := res.Text
	if reply == "" {
		reply = streamed.String()
	}
	// Keep whatever was said even if the turn failed partway, so the
	// transcript matches what the user saw.
	if strings.TrimSpace(reply) != "" {
		if _, err := store.AppendMessage(checkinID, "assistant", reply, ""); err != nil && turnErr == nil {
			turnErr = err
		}
	}
	return turnErr
}

// retroSystem is the prompt for the one-shot retro draft.
const retroSystem = `You help one person write their weekly retrospective in AgileCBT, an app that blends agile planning with CBT for depression and anxiety. Be warm, specific, and brief. Ground every point in the data you're given, and never invent events. Celebrate small wins. Frame hard things without judgment. Look for patterns between activities, mastery/pleasure ratings, and mood, energy, and anxiety (for example, "walks preceded higher-mood days"). Suggest exactly one small, concrete experiment for next week.

Sound like a careful note to a friend, not an essay. Use contractions. Prefer short sentences and concrete words from their week. Never use em dashes or en dashes; use a period, a comma, or "and"/"but" instead. Skip stock phrases and "it's not X, it's Y" reframes.

Reply with only a JSON object with these string fields:
{"went_well": "...", "was_hard": "...", "try_next": "...", "patterns": "..."}
Write each field in second person ("you"), with 1-4 short sentences or a short bulleted list.`

// DraftRetro asks the backend for a retro draft of a week and saves it.
func (c *Curator) DraftRetro(ctx context.Context, weekID int64) (db.Retro, error) {
	backend := c.current()
	if backend == nil {
		return db.Retro{}, ErrOff
	}
	review, err := c.app.ReviewWeek(weekID)
	if err != nil {
		return db.Retro{}, err
	}
	data, err := json.MarshalIndent(review, "", " ")
	if err != nil {
		return db.Retro{}, err
	}
	out, err := backend.Complete(ctx, retroSystem, "Here is the week's data:\n\n"+string(data))
	if err != nil {
		return db.Retro{}, err
	}
	draft := parseRetroDraft(out)
	p := db.RetroPatch{AIDraft: &draft.text}
	// Prefill only the fields the user hasn't written yet.
	var existing db.Retro
	if review.Retro != nil {
		existing = *review.Retro
	}
	if existing.WentWell == "" && draft.wentWell != "" {
		p.WentWell = &draft.wentWell
	}
	if existing.WasHard == "" && draft.wasHard != "" {
		p.WasHard = &draft.wasHard
	}
	if existing.TryNext == "" && draft.tryNext != "" {
		p.TryNext = &draft.tryNext
	}
	return c.app.Store.UpsertRetro(weekID, p)
}

type retroDraft struct {
	wentWell, wasHard, tryNext, text string
}

// parseRetroDraft pulls the JSON object out of a model reply. If the model
// didn't return JSON, the whole reply becomes the draft text.
func parseRetroDraft(out string) retroDraft {
	var v struct {
		WentWell string `json:"went_well"`
		WasHard  string `json:"was_hard"`
		TryNext  string `json:"try_next"`
		Patterns string `json:"patterns"`
	}
	start, end := strings.Index(out, "{"), strings.LastIndex(out, "}")
	if start < 0 || end <= start || json.Unmarshal([]byte(out[start:end+1]), &v) != nil {
		return retroDraft{text: strings.TrimSpace(out)}
	}
	var b strings.Builder
	for _, s := range []struct{ h, body string }{
		{"What went well", v.WentWell}, {"What was hard", v.WasHard},
		{"One thing to try", v.TryNext}, {"Patterns I noticed", v.Patterns},
	} {
		if strings.TrimSpace(s.body) != "" {
			fmt.Fprintf(&b, "**%s**\n%s\n\n", s.h, strings.TrimSpace(s.body))
		}
	}
	return retroDraft{wentWell: v.WentWell, wasHard: v.WasHard, tryNext: v.TryNext, text: strings.TrimSpace(b.String())}
}

// statusCache memoizes a backend's status for a short time.
type statusCache struct {
	mu     sync.Mutex
	at     time.Time
	ok     bool
	detail string
}

func (s *statusCache) get(ctx context.Context, probe func(context.Context) (bool, string)) (bool, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ttl := 60 * time.Second
	if !s.ok {
		ttl = 10 * time.Second // recover quickly once the user fixes it
	}
	if time.Since(s.at) < ttl {
		return s.ok, s.detail
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	s.ok, s.detail = probe(ctx)
	s.at = time.Now()
	return s.ok, s.detail
}

// reset forgets the cached status, e.g. after the settings change.
func (s *statusCache) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.at = time.Time{}
}
