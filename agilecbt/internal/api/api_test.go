package api_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jmelahman/agilecbt/internal/api"
	"github.com/jmelahman/agilecbt/internal/app"
	"github.com/jmelahman/agilecbt/internal/db"
	"github.com/jmelahman/agilecbt/internal/tools"
)

// fakeCurator replies with fixed text and creates one step through the tool
// registry, like a real backend would.
type fakeCurator struct {
	reg *tools.Registry
	err error
}

func (f *fakeCurator) Status(context.Context) api.LLMStatus {
	return api.LLMStatus{Backend: "fake", Available: true}
}

func (f *fakeCurator) Chat(ctx context.Context, checkinID int64, text string, emit func(string, any)) error {
	emit("text", map[string]string{"text": "Hi! "})
	ctx = app.WithActor(ctx, app.Actor{Source: "curator", CheckinID: &checkinID})
	if _, err := f.reg.Call(ctx, "create_step", json.RawMessage(`{"title":"Take a short walk","lane":"today","energy_cost":1}`)); err != nil {
		return err
	}
	emit("text", map[string]string{"text": "I added a walk."})
	return f.err
}

func (f *fakeCurator) DraftRetro(ctx context.Context, weekID int64) (db.Retro, error) {
	draft := "**What went well**\nYou showed up."
	return f.reg.App().Store.UpsertRetro(weekID, db.RetroPatch{AIDraft: &draft})
}

type harness struct {
	t   *testing.T
	h   http.Handler
	app *app.App
	// auth is added to every request (e.g. a bearer token).
	auth string
}

func newHarness(t *testing.T, secret string, withCurator bool) *harness {
	t.Helper()
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	a := app.New(store)
	d := api.Deps{App: a, Build: api.BuildInfo{Version: "test"}, Secret: secret}
	if withCurator {
		d.Curator = &fakeCurator{reg: tools.New(a)}
	}
	return &harness{t: t, h: api.NewMux(d), app: a}
}

func (h *harness) do(method, path, body string) *httptest.ResponseRecorder {
	h.t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if h.auth != "" {
		req.Header.Set("Authorization", h.auth)
	}
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	return rec
}

// ok performs a request, requires status want, and decodes the body into out.
func (h *harness) ok(want int, method, path, body string, out any) {
	h.t.Helper()
	rec := h.do(method, path, body)
	if rec.Code != want {
		h.t.Fatalf("%s %s = %d, want %d: %s", method, path, rec.Code, want, rec.Body)
	}
	if out != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			h.t.Fatalf("%s %s: decode: %v: %s", method, path, err, rec.Body)
		}
	}
}

func TestHealth(t *testing.T) {
	h := newHarness(t, "", false)
	var resp struct {
		Status       string        `json:"status"`
		Version      string        `json:"version"`
		AuthRequired bool          `json:"auth_required"`
		LLM          api.LLMStatus `json:"llm"`
	}
	h.ok(200, "GET", "/api/health", "", &resp)
	if resp.Status != "ok" || resp.Version != "test" || resp.AuthRequired || resp.LLM.Backend != "none" || resp.LLM.Available {
		t.Fatalf("unexpected health: %+v", resp)
	}
}

func TestAuth(t *testing.T) {
	h := newHarness(t, "s3cret", false)

	if rec := h.do("GET", "/api/today", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no auth: %d, want 401", rec.Code)
	}
	if rec := h.do("GET", "/api/health", ""); rec.Code != http.StatusOK {
		t.Fatalf("health must stay open: %d", rec.Code)
	}
	if rec := h.do("POST", "/api/login", `{"secret":"nope"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong secret: %d, want 401", rec.Code)
	}

	rec := h.do("POST", "/api/login", `{"secret":"s3cret"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("login: %d: %s", rec.Code, rec.Body)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || strings.Contains(cookies[0].Value, "s3cret") {
		t.Fatalf("bad session cookie: %+v", cookies)
	}
	req := httptest.NewRequest("GET", "/api/today", nil)
	req.AddCookie(cookies[0])
	rec = httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("cookie auth: %d", rec.Code)
	}

	h.auth = "Bearer s3cret"
	h.ok(200, "GET", "/api/today", "", nil)
	h.auth = "Bearer wrong"
	if rec := h.do("GET", "/api/today", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong bearer: %d", rec.Code)
	}
	if rec := h.do("POST", "/mcp", `{}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("/mcp must require auth: %d", rec.Code)
	}
}

func TestRoadmapAndBoard(t *testing.T) {
	h := newHarness(t, "", false)

	var v db.Value
	h.ok(201, "POST", "/api/values", `{"name":"Health"}`, &v)
	var g db.Goal
	h.ok(201, "POST", "/api/goals", fmt.Sprintf(`{"title":"Move more","value_id":%d,"why":"feel steadier"}`, v.ID), &g)
	if g.Status != db.GoalActive || g.ValueID == nil || *g.ValueID != v.ID {
		t.Fatalf("unexpected goal: %+v", g)
	}

	var s1, s2 db.Step
	h.ok(201, "POST", "/api/steps", fmt.Sprintf(`{"title":"Walk","goal_id":%d,"lane":"week"}`, g.ID), &s1)
	h.ok(201, "POST", "/api/steps", `{"title":"Stretch","lane":"week","energy_cost":2}`, &s2)
	if s1.EnergyCost != 1 || s1.Lane != "week" {
		t.Fatalf("defaults not applied: %+v", s1)
	}

	// Move Stretch to Today, then Walk above it.
	h.ok(200, "PATCH", fmt.Sprintf("/api/steps/%d", s2.ID), `{"lane":"today"}`, nil)
	h.ok(200, "PATCH", fmt.Sprintf("/api/steps/%d", s1.ID), `{"lane":"today","index":0}`, nil)
	var today []db.Step
	h.ok(200, "GET", "/api/steps?lane=today", "", &today)
	if len(today) != 2 || today[0].ID != s1.ID || today[1].ID != s2.ID {
		t.Fatalf("today order: %+v", today)
	}

	var done db.Step
	h.ok(200, "POST", fmt.Sprintf("/api/steps/%d/complete", s1.ID), `{"mastery":6,"pleasure":7}`, &done)
	if done.Lane != "done" || done.CompletedAt == nil || *done.Mastery != 6 {
		t.Fatalf("complete: %+v", done)
	}

	var snap app.Snapshot
	h.ok(200, "GET", "/api/today", "", &snap)
	if len(snap.Today) != 1 || len(snap.DoneToday) != 1 || snap.Week.ID == 0 {
		t.Fatalf("snapshot: %+v", snap)
	}

	// Validation and not-found.
	for _, c := range []struct{ method, path, body string }{
		{"POST", "/api/steps", `{"title":"  "}`},
		{"POST", "/api/steps", `{"title":"x","energy_cost":9}`},
		{"POST", "/api/steps", `{"title":"x","lane":"sideways"}`},
		{"POST", "/api/steps", `{`},
		{"POST", "/api/steps", `{"title":"x","bogus":1}`},
		{"POST", "/api/goals", `{"title":"x","status":"urgent"}`},
	} {
		if rec := h.do(c.method, c.path, c.body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s %s %s = %d, want 400: %s", c.method, c.path, c.body, rec.Code, rec.Body)
		}
	}
	if rec := h.do("GET", "/api/steps/999", ""); rec.Code != http.StatusNotFound {
		t.Errorf("missing step = %d, want 404", rec.Code)
	}
	if rec := h.do("GET", "/api/nope", ""); rec.Code != http.StatusNotFound {
		t.Errorf("unknown api path = %d, want 404", rec.Code)
	}
}

func TestCheckinAndThoughts(t *testing.T) {
	h := newHarness(t, "", false)
	var c db.Checkin
	h.ok(201, "POST", "/api/checkins", `{"kind":"morning","mood":4,"energy":3,"anxiety":6,"note":"tired"}`, &c)
	if c.Date == "" || *c.Mood != 4 {
		t.Fatalf("checkin: %+v", c)
	}
	h.ok(200, "PATCH", fmt.Sprintf("/api/checkins/%d", c.ID), `{"mood":5}`, &c)
	if *c.Mood != 5 || *c.Energy != 3 {
		t.Fatalf("patch checkin: %+v", c)
	}
	if rec := h.do("POST", "/api/checkins", `{"mood":11}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("mood 11 = %d, want 400", rec.Code)
	}

	var mood []app.DayMood
	h.ok(200, "GET", "/api/mood?days=3", "", &mood)
	if len(mood) != 3 || mood[2].Mood == nil || *mood[2].Mood != 5 {
		t.Fatalf("mood: %+v", mood)
	}

	var tr db.ThoughtRecord
	h.ok(201, "POST", "/api/thoughts", `{"situation":"Email from boss","automatic_thought":"I'm going to be fired","distortions":["catastrophizing"],"emotions":[{"name":"anxious","intensity":80}]}`, &tr)
	if len(tr.Distortions) != 1 || len(tr.Emotions) != 1 {
		t.Fatalf("thought: %+v", tr)
	}

	// Without a curator, chat is unavailable but the manual flow works.
	if rec := h.do("POST", fmt.Sprintf("/api/checkins/%d/messages", c.ID), `{"text":"hi"}`); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("chat without curator = %d, want 503", rec.Code)
	}
}

type sseEvent struct {
	name string
	data string
}

func readSSE(t *testing.T, body string) []sseEvent {
	t.Helper()
	var out []sseEvent
	var cur sseEvent
	sc := bufio.NewScanner(strings.NewReader(body))
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			cur.name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			cur.data = strings.TrimPrefix(line, "data: ")
		case line == "":
			if cur.name != "" {
				out = append(out, cur)
			}
			cur = sseEvent{}
		}
	}
	return out
}

func TestChatActionsAndUndo(t *testing.T) {
	h := newHarness(t, "", true)
	var c db.Checkin
	h.ok(201, "POST", "/api/checkins", `{"kind":"morning"}`, &c)

	rec := h.do("POST", fmt.Sprintf("/api/checkins/%d/messages", c.ID), `{"text":"morning!"}`)
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("chat: %d %s", rec.Code, rec.Header())
	}
	events := readSSE(t, rec.Body.String())
	var names []string
	var action db.Action
	for _, e := range events {
		names = append(names, e.name)
		if e.name == "action" {
			if err := json.Unmarshal([]byte(e.data), &action); err != nil {
				t.Fatal(err)
			}
		}
	}
	if got := strings.Join(names, ","); got != "text,action,text,done" {
		t.Fatalf("events = %s", got)
	}
	if action.Tool != "create_step" || action.CheckinID == nil || *action.CheckinID != c.ID || action.Summary == "" {
		t.Fatalf("action: %+v", action)
	}

	// The fake doesn't store messages (the real curator does, see its
	// tests); actions are recorded by the tool layer either way.
	var detail struct {
		Actions []db.Action `json:"actions"`
	}
	h.ok(200, "GET", fmt.Sprintf("/api/checkins/%d", c.ID), "", &detail)
	if len(detail.Actions) != 1 {
		t.Fatalf("detail: %+v", detail)
	}

	var today []db.Step
	h.ok(200, "GET", "/api/steps?lane=today", "", &today)
	if len(today) != 1 {
		t.Fatalf("step not created: %+v", today)
	}
	h.ok(200, "POST", fmt.Sprintf("/api/ai-actions/%d/undo", action.ID), "", nil)
	h.ok(200, "GET", "/api/steps?lane=today", "", &today)
	if len(today) != 0 {
		t.Fatalf("undo left the step: %+v", today)
	}
	if rec := h.do("POST", fmt.Sprintf("/api/ai-actions/%d/undo", action.ID), ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("double undo = %d, want 400", rec.Code)
	}

	// A failing turn still ends the stream cleanly with an error event.
	h.h = api.NewMux(api.Deps{App: h.app, Build: api.BuildInfo{Version: "test"},
		Curator: &fakeCurator{reg: tools.New(h.app), err: errors.New("backend fell over")}})
	events = readSSE(t, h.do("POST", fmt.Sprintf("/api/checkins/%d/messages", c.ID), `{"text":"again"}`).Body.String())
	if last := events[len(events)-1]; last.name != "done" || events[len(events)-2].name != "error" {
		t.Fatalf("error events: %+v", events)
	}
}

func TestRetroDraft(t *testing.T) {
	h := newHarness(t, "", true)
	var review app.WeekReview
	h.ok(200, "GET", "/api/weeks/current", "", &review)
	var retro db.Retro
	h.ok(200, "POST", fmt.Sprintf("/api/weeks/%d/retro/draft", review.Week.ID), "", &retro)
	if !strings.Contains(retro.AIDraft, "went well") {
		t.Fatalf("draft: %+v", retro)
	}
	h.ok(200, "PUT", fmt.Sprintf("/api/weeks/%d/retro", review.Week.ID), `{"try_next":"walk after lunch"}`, &retro)
	if retro.TryNext != "walk after lunch" || retro.AIDraft == "" {
		t.Fatalf("put retro: %+v", retro)
	}
}

func TestExportImport(t *testing.T) {
	src := newHarness(t, "", false)
	src.ok(201, "POST", "/api/values", `{"name":"Connection"}`, nil)
	src.ok(201, "POST", "/api/goals", `{"title":"Call a friend weekly","value_id":1}`, nil)
	src.ok(201, "POST", "/api/steps", `{"title":"Text Sam","goal_id":1,"lane":"today"}`, nil)
	src.ok(201, "POST", "/api/notes", `{"text":"Mornings are hardest"}`, nil)
	src.ok(200, "PATCH", "/api/settings", `{"crisis_resources":"Call my sister"}`, nil)
	dump := src.do("GET", "/api/export", "").Body.String()

	dst := newHarness(t, "", false)
	dst.ok(200, "POST", "/api/import", dump, nil)
	var steps []db.Step
	dst.ok(200, "GET", "/api/steps", "", &steps)
	if len(steps) != 1 || steps[0].GoalID == nil || *steps[0].GoalID != 1 {
		t.Fatalf("imported steps: %+v", steps)
	}
	var settings map[string]string
	dst.ok(200, "GET", "/api/settings", "", &settings)
	if settings["crisis_resources"] != "Call my sister" {
		t.Fatalf("settings: %+v", settings)
	}
	if rec := dst.do("POST", "/api/import", dump); rec.Code != http.StatusBadRequest {
		t.Fatalf("import into non-empty = %d, want 400", rec.Code)
	}
}
