package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jmelahman/agilecbt/internal/app"
	"github.com/jmelahman/agilecbt/internal/db"
)

func (d Deps) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", d.handleHealth)
	mux.HandleFunc("POST /api/login", d.handleLogin)
	mux.HandleFunc("POST /api/logout", d.handleLogout)

	mux.HandleFunc("GET /api/today", handle(d.getToday))
	mux.HandleFunc("GET /api/mood", handle(d.getMood))

	mux.HandleFunc("GET /api/values", handle(d.listValues))
	mux.HandleFunc("POST /api/values", handle(d.createValue))
	mux.HandleFunc("PATCH /api/values/{id}", handle(d.updateValue))
	mux.HandleFunc("DELETE /api/values/{id}", handle(d.deleteValue))

	mux.HandleFunc("GET /api/goals", handle(d.listGoals))
	mux.HandleFunc("POST /api/goals", handle(d.createGoal))
	mux.HandleFunc("GET /api/goals/{id}", handle(d.getGoal))
	mux.HandleFunc("PATCH /api/goals/{id}", handle(d.updateGoal))
	mux.HandleFunc("DELETE /api/goals/{id}", handle(d.deleteGoal))

	mux.HandleFunc("GET /api/steps", handle(d.listSteps))
	mux.HandleFunc("POST /api/steps", handle(d.createStep))
	mux.HandleFunc("GET /api/steps/{id}", handle(d.getStep))
	mux.HandleFunc("PATCH /api/steps/{id}", handle(d.updateStep))
	mux.HandleFunc("POST /api/steps/{id}/complete", handle(d.completeStep))
	mux.HandleFunc("DELETE /api/steps/{id}", handle(d.deleteStep))

	mux.HandleFunc("GET /api/weeks", handle(d.listWeeks))
	mux.HandleFunc("GET /api/weeks/current", handle(d.currentWeek))
	mux.HandleFunc("PATCH /api/weeks/{id}", handle(d.updateWeek))
	mux.HandleFunc("GET /api/weeks/{id}/review", handle(d.reviewWeek))
	mux.HandleFunc("PUT /api/weeks/{id}/retro", handle(d.putRetro))
	mux.HandleFunc("POST /api/weeks/{id}/retro/draft", handle(d.draftRetro))

	mux.HandleFunc("GET /api/checkins", handle(d.listCheckins))
	mux.HandleFunc("POST /api/checkins", handle(d.createCheckin))
	mux.HandleFunc("GET /api/checkins/{id}", handle(d.getCheckin))
	mux.HandleFunc("PATCH /api/checkins/{id}", handle(d.updateCheckin))
	mux.HandleFunc("POST /api/checkins/{id}/messages", d.handleChat)
	mux.HandleFunc("GET /api/conversations/roadmap", handle(d.roadmapConversation))

	mux.HandleFunc("GET /api/thoughts", handle(d.listThoughts))
	mux.HandleFunc("POST /api/thoughts", handle(d.createThought))
	mux.HandleFunc("GET /api/thoughts/{id}", handle(d.getThought))
	mux.HandleFunc("PATCH /api/thoughts/{id}", handle(d.updateThought))
	mux.HandleFunc("DELETE /api/thoughts/{id}", handle(d.deleteThought))

	mux.HandleFunc("GET /api/notes", handle(d.listNotes))
	mux.HandleFunc("POST /api/notes", handle(d.createNote))
	mux.HandleFunc("PATCH /api/notes/{id}", handle(d.updateNote))
	mux.HandleFunc("DELETE /api/notes/{id}", handle(d.deleteNote))

	mux.HandleFunc("GET /api/settings", handle(d.getSettings))
	mux.HandleFunc("PATCH /api/settings", handle(d.patchSettings))
	mux.HandleFunc("GET /api/support", handle(d.getSupport))
	mux.HandleFunc("GET /api/llm", handle(d.getLLM))
	mux.HandleFunc("PATCH /api/llm", handle(d.patchLLM))
	mux.HandleFunc("GET /api/llm/models", handle(d.listModels))

	mux.HandleFunc("GET /api/ai-actions", handle(d.listActions))
	mux.HandleFunc("POST /api/ai-actions/{id}/undo", handle(d.undoAction))

	mux.HandleFunc("GET /api/export", handle(d.exportData))
	mux.HandleFunc("POST /api/import", handle(d.importData))
}

func (d Deps) store() *db.Store { return d.App.Store }

func (d Deps) handleHealth(w http.ResponseWriter, r *http.Request) {
	llm := LLMStatus{Backend: "none", Detail: "The AI coach is not configured"}
	if d.Curator != nil {
		llm = d.Curator.Status(r.Context())
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "ok",
		"version":       d.Build.Version,
		"auth_required": d.Secret != "",
		"authenticated": d.authenticated(r),
		"llm":           llm,
	})
}

func (d Deps) getToday(r *http.Request) (any, error) {
	return d.App.TodaySnapshot()
}

func (d Deps) getMood(r *http.Request) (any, error) {
	days, err := queryInt(r, "days", 14)
	if err != nil {
		return nil, err
	}
	if days < 1 || days > 366 {
		return nil, badRequest("days must be between 1 and 366")
	}
	return d.App.MoodHistory(days)
}

// Values.

func (d Deps) listValues(r *http.Request) (any, error) { return d.store().ListValues() }

func (d Deps) createValue(r *http.Request) (any, error) {
	var p db.ValuePatch
	if err := decode(r, &p); err != nil {
		return nil, err
	}
	return d.store().CreateValue(p)
}

func (d Deps) updateValue(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	var p db.ValuePatch
	if err := decode(r, &p); err != nil {
		return nil, err
	}
	return d.store().UpdateValue(id, p)
}

func (d Deps) deleteValue(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	return nil, d.store().DeleteValue(id)
}

// Goals.

func (d Deps) listGoals(r *http.Request) (any, error) {
	return d.store().ListGoals(r.URL.Query().Get("status"))
}

func (d Deps) createGoal(r *http.Request) (any, error) {
	var p db.GoalPatch
	if err := decode(r, &p); err != nil {
		return nil, err
	}
	return d.store().CreateGoal(p)
}

func (d Deps) getGoal(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	return d.store().GetGoal(id)
}

func (d Deps) updateGoal(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	var p db.GoalPatch
	if err := decode(r, &p); err != nil {
		return nil, err
	}
	return d.store().UpdateGoal(id, p)
}

func (d Deps) deleteGoal(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	return nil, d.store().DeleteGoal(id)
}

// Steps.

func (d Deps) listSteps(r *http.Request) (any, error) {
	var f db.StepFilter
	if lanes := r.URL.Query().Get("lane"); lanes != "" {
		f.Lanes = strings.Split(lanes, ",")
	}
	if g := r.URL.Query().Get("goal_id"); g != "" {
		n, err := queryInt(r, "goal_id", 0)
		if err != nil {
			return nil, err
		}
		id := int64(n)
		f.GoalID = &id
	}
	return d.store().ListSteps(f)
}

func (d Deps) createStep(r *http.Request) (any, error) {
	var req struct {
		db.StepPatch
		Lane string `json:"lane"`
	}
	if err := decode(r, &req); err != nil {
		return nil, err
	}
	return d.store().CreateStep(req.Lane, req.StepPatch)
}

func (d Deps) getStep(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	return d.store().GetStep(id)
}

// updateStep edits fields and/or moves the step. A move sets lane and an
// optional index (position within the lane; omitted = end).
func (d Deps) updateStep(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	var req struct {
		db.StepPatch
		Lane  *string `json:"lane"`
		Index *int    `json:"index"`
	}
	if err := decode(r, &req); err != nil {
		return nil, err
	}
	st, err := d.store().UpdateStep(id, req.StepPatch)
	if err != nil {
		return nil, err
	}
	if req.Lane == nil && req.Index == nil {
		return st, nil
	}
	lane := st.Lane
	if req.Lane != nil {
		lane = *req.Lane
	}
	return d.store().MoveStep(id, lane, req.Index)
}

func (d Deps) completeStep(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	var req struct {
		Mastery  *int `json:"mastery"`
		Pleasure *int `json:"pleasure"`
	}
	if err := decode(r, &req); err != nil {
		return nil, err
	}
	return ok(d.store().CompleteStep(id, req.Mastery, req.Pleasure))
}

func (d Deps) deleteStep(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	return nil, d.store().DeleteStep(id)
}

// Weeks and retros.

func (d Deps) listWeeks(r *http.Request) (any, error) { return d.store().ListWeeks() }

func (d Deps) currentWeek(r *http.Request) (any, error) {
	w, err := d.App.CurrentWeek()
	if err != nil {
		return nil, err
	}
	return d.App.ReviewWeek(w.ID)
}

func (d Deps) updateWeek(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	var req struct {
		Intention string `json:"intention"`
	}
	if err := decode(r, &req); err != nil {
		return nil, err
	}
	return d.store().SetWeekIntention(id, req.Intention)
}

func (d Deps) reviewWeek(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	return d.App.ReviewWeek(id)
}

func (d Deps) putRetro(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	var p db.RetroPatch
	if err := decode(r, &p); err != nil {
		return nil, err
	}
	return d.store().UpsertRetro(id, p)
}

func (d Deps) draftRetro(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	if d.Curator == nil || !d.Curator.Status(r.Context()).Available {
		return nil, errUnavailable
	}
	return ok(d.Curator.DraftRetro(r.Context(), id))
}

// Check-ins.

func (d Deps) listCheckins(r *http.Request) (any, error) {
	q := r.URL.Query()
	return d.store().ListCheckins(q.Get("from"), q.Get("to"))
}

// newCheckin is a check-in to create. Intro, if set, is the opening of its
// conversation (the app's greeting, or its standup questions and answers), stored
// as the first messages without a coach turn.
type newCheckin struct {
	db.CheckinPatch
	Intro []introMessage `json:"intro"`
}

type introMessage struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

func (d Deps) createCheckin(r *http.Request) (any, error) {
	var p newCheckin
	if err := decode(r, &p); err != nil {
		return nil, err
	}
	if len(p.Intro) > 20 {
		return nil, badRequest("intro has at most 20 messages")
	}
	for _, m := range p.Intro {
		if m.Role != "user" && m.Role != "assistant" {
			return nil, badRequest("intro role must be user or assistant")
		}
		if strings.TrimSpace(m.Text) == "" {
			return nil, badRequest("intro messages need text")
		}
	}
	if p.Date == nil {
		today := d.App.Today()
		p.Date = &today
	}
	var c db.Checkin
	err := d.store().Tx(func(s *db.Store) error {
		var err error
		if c, err = s.CreateCheckin(p.CheckinPatch); err != nil {
			return err
		}
		for _, m := range p.Intro {
			if _, err := s.AppendMessage(c.ID, m.Role, strings.TrimSpace(m.Text), ""); err != nil {
				return err
			}
		}
		return nil
	})
	return c, err
}

// CheckinDetail is a check-in with its chat transcript and AI actions.
type CheckinDetail struct {
	db.Checkin
	Messages []db.Message `json:"messages"`
	Actions  []db.Action  `json:"actions"`
}

func (d Deps) getCheckin(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	c, err := d.store().GetCheckin(id)
	if err != nil {
		return nil, err
	}
	out := CheckinDetail{Checkin: c}
	if out.Messages, err = d.store().ListMessages(id); err != nil {
		return nil, err
	}
	if out.Actions, err = d.store().ListActions(&id, 200); err != nil {
		return nil, err
	}
	return out, nil
}

// roadmapConversation returns today's Roadmap-page chat, or null before the
// first message (the client creates it with POST /api/checkins).
func (d Deps) roadmapConversation(r *http.Request) (any, error) {
	c, err := d.store().LatestCheckin(d.App.Today(), db.KindAdhoc, db.TopicRoadmap)
	if errors.Is(err, db.ErrNotFound) {
		return okBody{nil}, nil
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (d Deps) updateCheckin(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	var p db.CheckinPatch
	if err := decode(r, &p); err != nil {
		return nil, err
	}
	return d.store().UpdateCheckin(id, p)
}

// Thought records.

func (d Deps) listThoughts(r *http.Request) (any, error) {
	limit, err := queryInt(r, "limit", 0)
	if err != nil {
		return nil, err
	}
	return d.store().ListThoughtRecords(limit)
}

func (d Deps) createThought(r *http.Request) (any, error) {
	var p db.ThoughtRecordPatch
	if err := decode(r, &p); err != nil {
		return nil, err
	}
	return d.store().CreateThoughtRecord(p)
}

func (d Deps) getThought(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	return d.store().GetThoughtRecord(id)
}

func (d Deps) updateThought(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	var p db.ThoughtRecordPatch
	if err := decode(r, &p); err != nil {
		return nil, err
	}
	return d.store().UpdateThoughtRecord(id, p)
}

func (d Deps) deleteThought(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	return nil, d.store().DeleteThoughtRecord(id)
}

// Curator notes.

type noteBody struct {
	Text string `json:"text"`
}

func (d Deps) listNotes(r *http.Request) (any, error) { return d.store().ListNotes() }

func (d Deps) createNote(r *http.Request) (any, error) {
	var req noteBody
	if err := decode(r, &req); err != nil {
		return nil, err
	}
	return d.store().CreateNote(req.Text)
}

func (d Deps) updateNote(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	var req noteBody
	if err := decode(r, &req); err != nil {
		return nil, err
	}
	return d.store().UpdateNote(id, req.Text)
}

func (d Deps) deleteNote(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	return nil, d.store().DeleteNote(id)
}

// Settings.

func (d Deps) getSettings(r *http.Request) (any, error) {
	return d.App.Settings()
}

// getSupport returns the crisis lines shown in Settings → Support. They're
// read-only here; see app.CrisisResources.
func (d Deps) getSupport(r *http.Request) (any, error) {
	return map[string]string{"crisis_resources": d.App.CrisisResources()}, nil
}

func (d Deps) patchSettings(r *http.Request) (any, error) {
	var req map[string]string
	if err := decode(r, &req); err != nil {
		return nil, err
	}
	for k, v := range req {
		if err := d.App.SetSetting(k, v); err != nil {
			return nil, err
		}
	}
	return d.App.Settings()
}

// The coach's model.

var errNoCurator = fmt.Errorf("%w: the AI coach isn't available on this server", db.ErrInvalid)

func (d Deps) getLLM(r *http.Request) (any, error) {
	if d.Curator == nil {
		return nil, errNoCurator
	}
	return d.Curator.LLMSettings()
}

func (d Deps) patchLLM(r *http.Request) (any, error) {
	if d.Curator == nil {
		return nil, errNoCurator
	}
	var req map[string]*string
	if err := decode(r, &req); err != nil {
		return nil, err
	}
	return d.Curator.UpdateLLM(req)
}

func (d Deps) listModels(r *http.Request) (any, error) {
	if d.Curator == nil {
		return nil, errNoCurator
	}
	ids, err := d.Curator.Models(r.Context())
	if err != nil {
		// The status line already explains why; offer no suggestions.
		return []string{}, nil
	}
	return ids, nil
}

// AI actions.

func (d Deps) listActions(r *http.Request) (any, error) {
	var checkinID *int64
	if r.URL.Query().Get("checkin_id") != "" {
		n, err := queryInt(r, "checkin_id", 0)
		if err != nil {
			return nil, err
		}
		id := int64(n)
		checkinID = &id
	}
	limit, err := queryInt(r, "limit", 50)
	if err != nil {
		return nil, err
	}
	return d.store().ListActions(checkinID, limit)
}

func (d Deps) undoAction(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	act, err := d.App.Undo(id)
	if err != nil {
		return nil, err
	}
	// Undo is a POST but doesn't create anything.
	return okBody{act}, nil
}

// Export / import.

func (d Deps) exportData(r *http.Request) (any, error) {
	return d.App.Export()
}

func (d Deps) importData(r *http.Request) (any, error) {
	var dump app.Dump
	if err := decode(r, &dump); err != nil {
		return nil, err
	}
	if err := d.App.Import(dump); err != nil {
		return nil, err
	}
	return okBody{map[string]string{"status": "imported"}}, nil
}
