package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/jmelahman/agilecbt/internal/app"
	"github.com/jmelahman/agilecbt/internal/db"
)

func writeTools() []Tool {
	emotionItems := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":      map[string]any{"type": "string"},
			"intensity": map[string]any{"type": "integer", "minimum": 0, "maximum": 100},
		},
		"required": []string{"name", "intensity"},
	}
	thoughtProps := []prop{
		str("situation", "What happened: where, when, who."),
		array("emotions", "Feelings and how intense they were (0-100).", emotionItems),
		str("automatic_thought", "The thought that went through their mind, in their words."),
		array("distortions", "Thinking traps spotted, e.g. all_or_nothing, catastrophizing, mind_reading, fortune_telling, should_statements, labeling, personalization, overgeneralization, mental_filter, discounting_positives, emotional_reasoning.", map[string]any{"type": "string"}),
		str("evidence_for", "Facts supporting the thought."),
		str("evidence_against", "Facts that don't fit the thought."),
		str("balanced_thought", "A more balanced alternative the user finds believable."),
		array("rerated_emotions", "The same emotions re-rated after reflection.", emotionItems),
	}

	return []Tool{
		define("create_value",
			"Add a life value (an area that matters to the user, like Health or Creativity).",
			true, []prop{str("name", "Short name.").req(), str("description", "What this value means to the user.")},
			func(ctx context.Context, a *app.App, in db.ValuePatch) (any, error) {
				v, err := a.Store.CreateValue(in)
				if err != nil {
					return nil, err
				}
				return v, record(ctx, a, "create_value", fmt.Sprintf("Added value %q", v.Name), app.EntityValue, v.ID, nil, v)
			}),

		define("create_goal",
			"Add a goal to the roadmap. Link it to a value when you can so every step traces back to why it matters.",
			true, []prop{
				str("title", "The goal, phrased kindly and concretely.").req(),
				str("why", "Why it matters to the user, in their words where possible."),
				integer("value_id", "The value this goal serves."),
				str("horizon", "Rough timeframe, e.g. 'this month' or 'this season'."),
			},
			func(ctx context.Context, a *app.App, in db.GoalPatch) (any, error) {
				g, err := a.Store.CreateGoal(in)
				if err != nil {
					return nil, err
				}
				return g, record(ctx, a, "create_goal", fmt.Sprintf("Added goal %q", g.Title), app.EntityGoal, g.ID, nil, g)
			}),

		define("update_goal",
			"Change a goal's title, why, horizon, value, or status. Use status 'resting' to pause without guilt.",
			true, []prop{
				integer("id", "Goal id.").req(),
				str("title", ""), str("why", ""), str("horizon", ""),
				integer("value_id", "New value id."),
				enum("status", "", db.GoalActive, db.GoalResting, db.GoalDone),
			},
			func(ctx context.Context, a *app.App, in struct {
				ID int64 `json:"id"`
				db.GoalPatch
			},
			) (any, error) {
				before, err := a.Store.GetGoal(in.ID)
				if err != nil {
					return nil, err
				}
				g, err := a.Store.UpdateGoal(in.ID, in.GoalPatch)
				if err != nil {
					return nil, err
				}
				return g, record(ctx, a, "update_goal", fmt.Sprintf("Updated goal %q", g.Title), app.EntityGoal, g.ID, before, g)
			}),

		define("create_step",
			"Add a small, concrete step. Prefer tiny steps (energy_cost 1) on low-energy days. Lane defaults to someday.",
			true, []prop{
				str("title", "A concrete action, e.g. '10-minute walk after lunch'.").req(),
				integer("goal_id", "The goal this step serves."),
				integer("energy_cost", "1 = tiny, 2 = moderate, 3 = big.", 1, 3),
				enum("lane", "Where to put it.", db.LaneSomeday, db.LaneWeek, db.LaneToday),
				str("notes", ""),
				integer("predicted_pleasure", "How enjoyable the user expects it to be (0-10).", 0, 10),
			},
			func(ctx context.Context, a *app.App, in struct {
				db.StepPatch
				Lane string `json:"lane"`
			},
			) (any, error) {
				if in.Lane == db.LaneDone || in.Lane == db.LaneLetGo {
					return nil, fmt.Errorf("%w: new steps go in someday, week, or today", db.ErrInvalid)
				}
				st, err := a.Store.CreateStep(in.Lane, in.StepPatch)
				if err != nil {
					return nil, err
				}
				return st, record(ctx, a, "create_step", fmt.Sprintf("Added %q to %s", st.Title, laneName(st.Lane)), app.EntityStep, st.ID, nil, st)
			}),

		define("update_step",
			"Edit a step's title, notes, energy cost, goal, or predicted pleasure. Use move_step to change lanes.",
			true, []prop{
				integer("id", "Step id.").req(),
				str("title", ""), str("notes", ""),
				integer("energy_cost", "", 1, 3),
				integer("goal_id", ""),
				integer("predicted_pleasure", "", 0, 10),
			},
			func(ctx context.Context, a *app.App, in struct {
				ID int64 `json:"id"`
				db.StepPatch
			},
			) (any, error) {
				before, err := a.Store.GetStep(in.ID)
				if err != nil {
					return nil, err
				}
				in.Mastery, in.Pleasure = nil, nil // ratings come from complete_step
				st, err := a.Store.UpdateStep(in.ID, in.StepPatch)
				if err != nil {
					return nil, err
				}
				return st, record(ctx, a, "update_step", fmt.Sprintf("Edited %q", st.Title), app.EntityStep, st.ID, before, st)
			}),

		define("move_step",
			"Move a step between someday, week (this week), and today. Use complete_step for done and let_go_step to let it go.",
			true, []prop{
				integer("id", "Step id.").req(),
				enum("lane", "", db.LaneSomeday, db.LaneWeek, db.LaneToday).req(),
				integer("index", "Position in the lane (0 = top); omit for the bottom."),
			},
			func(ctx context.Context, a *app.App, in struct {
				ID    int64  `json:"id"`
				Lane  string `json:"lane"`
				Index *int   `json:"index"`
			},
			) (any, error) {
				if in.Lane == db.LaneDone || in.Lane == db.LaneLetGo {
					return nil, fmt.Errorf("%w: use complete_step or let_go_step", db.ErrInvalid)
				}
				before, err := a.Store.GetStep(in.ID)
				if err != nil {
					return nil, err
				}
				st, err := a.Store.MoveStep(in.ID, in.Lane, in.Index)
				if err != nil {
					return nil, err
				}
				return st, record(ctx, a, "move_step", fmt.Sprintf("Moved %q to %s", st.Title, laneName(st.Lane)), app.EntityStep, st.ID, before, st)
			}),

		define("complete_step",
			"Mark a step done, with optional mastery (sense of accomplishment) and pleasure ratings, 0-10. Every completed step counts.",
			true, []prop{
				integer("id", "Step id.").req(),
				integer("mastery", "", 0, 10),
				integer("pleasure", "", 0, 10),
			},
			func(ctx context.Context, a *app.App, in struct {
				ID       int64 `json:"id"`
				Mastery  *int  `json:"mastery"`
				Pleasure *int  `json:"pleasure"`
			},
			) (any, error) {
				before, err := a.Store.GetStep(in.ID)
				if err != nil {
					return nil, err
				}
				st, err := a.Store.CompleteStep(in.ID, in.Mastery, in.Pleasure)
				if err != nil {
					return nil, err
				}
				return st, record(ctx, a, "complete_step", fmt.Sprintf("Marked %q done", st.Title), app.EntityStep, st.ID, before, st)
			}),

		define("let_go_step",
			"Let a step go: it no longer fits, and that's fine. This is how steps are removed; nothing is ever deleted.",
			true, []prop{integer("id", "Step id.").req()},
			func(ctx context.Context, a *app.App, in struct {
				ID int64 `json:"id"`
			},
			) (any, error) {
				before, err := a.Store.GetStep(in.ID)
				if err != nil {
					return nil, err
				}
				st, err := a.Store.MoveStep(in.ID, db.LaneLetGo, nil)
				if err != nil {
					return nil, err
				}
				return st, record(ctx, a, "let_go_step", fmt.Sprintf("Let go of %q", st.Title), app.EntityStep, st.ID, before, st)
			}),

		define("record_checkin",
			"Record how the user is arriving: mood, energy, anxiety (0-10), a short note in their words, and/or a one-line summary of the check-in. Updates the current check-in; outside a check-in it starts a new one for today.",
			true, []prop{
				integer("mood", "0 = very low, 10 = great.", 0, 10),
				integer("energy", "0 = empty, 10 = full.", 0, 10),
				integer("anxiety", "0 = calm, 10 = overwhelming.", 0, 10),
				str("note", ""),
				str("summary", "One line capturing the check-in, for the history view."),
			},
			func(ctx context.Context, a *app.App, in db.CheckinPatch) (any, error) {
				in.Date, in.Kind = nil, nil
				if id := app.ActorFrom(ctx).CheckinID; id != nil {
					before, err := a.Store.GetCheckin(*id)
					if err != nil {
						return nil, err
					}
					c, err := a.Store.UpdateCheckin(*id, in)
					if err != nil {
						return nil, err
					}
					return c, record(ctx, a, "record_checkin", "Updated check-in", app.EntityCheckin, c.ID, before, c)
				}
				today, kind := a.Today(), app.DefaultKind(a.Now())
				in.Date, in.Kind = &today, &kind
				c, err := a.Store.CreateCheckin(in)
				if err != nil {
					return nil, err
				}
				return c, record(ctx, a, "record_checkin", "Recorded a check-in", app.EntityCheckin, c.ID, nil, c)
			}),

		define("create_thought_record",
			"Save a CBT thought record. Fill in what the user has actually shared; leave the rest empty for them to complete.",
			true, thoughtProps,
			func(ctx context.Context, a *app.App, in db.ThoughtRecordPatch) (any, error) {
				t, err := a.Store.CreateThoughtRecord(in)
				if err != nil {
					return nil, err
				}
				return t, record(ctx, a, "create_thought_record", "Started a thought record", app.EntityThought, t.ID, nil, t)
			}),

		define("update_thought_record",
			"Fill in more of an existing thought record.",
			true, append([]prop{integer("id", "Thought record id.").req()}, thoughtProps...),
			func(ctx context.Context, a *app.App, in struct {
				ID int64 `json:"id"`
				db.ThoughtRecordPatch
			},
			) (any, error) {
				before, err := a.Store.GetThoughtRecord(in.ID)
				if err != nil {
					return nil, err
				}
				t, err := a.Store.UpdateThoughtRecord(in.ID, in.ThoughtRecordPatch)
				if err != nil {
					return nil, err
				}
				return t, record(ctx, a, "update_thought_record", "Updated a thought record", app.EntityThought, t.ID, before, t)
			}),

		define("set_week_intention",
			"Set this week's sprint intention: one gentle sentence about what the week is for.",
			true, []prop{str("intention", "").req()},
			func(ctx context.Context, a *app.App, in struct {
				Intention string `json:"intention"`
			},
			) (any, error) {
				before, err := a.CurrentWeek()
				if err != nil {
					return nil, err
				}
				w, err := a.Store.SetWeekIntention(before.ID, in.Intention)
				if err != nil {
					return nil, err
				}
				return w, record(ctx, a, "set_week_intention", fmt.Sprintf("Set this week's intention: %q", w.Intention), app.EntityWeek, w.ID, before, w)
			}),

		define("save_retro_draft",
			"Save a draft weekly retro for the user to edit: what went well, what was hard, one thing to try next, plus observed patterns in draft.",
			true, []prop{
				str("went_well", ""), str("was_hard", ""), str("try_next", "One small experiment for next week."),
				str("draft", "Your full reflection, including patterns you noticed (e.g. 'walks preceded higher-mood days').").req(),
				str("week_start", "Any date in the week; omit for this week."),
			},
			func(ctx context.Context, a *app.App, in struct {
				WentWell  *string `json:"went_well"`
				WasHard   *string `json:"was_hard"`
				TryNext   *string `json:"try_next"`
				Draft     string  `json:"draft"`
				WeekStart string  `json:"week_start"`
			},
			) (any, error) {
				w, err := weekFor(a, in.WeekStart)
				if err != nil {
					return nil, err
				}
				var before any
				if r, err := a.Store.GetRetro(w.ID); err == nil {
					before = r
				} else if !errors.Is(err, db.ErrNotFound) {
					return nil, err
				}
				p := db.RetroPatch{AIDraft: &in.Draft}
				// Don't overwrite what the user already wrote.
				if before == nil {
					p.WentWell, p.WasHard, p.TryNext = in.WentWell, in.WasHard, in.TryNext
				}
				r, err := a.Store.UpsertRetro(w.ID, p)
				if err != nil {
					return nil, err
				}
				if before == nil {
					return r, record(ctx, a, "save_retro_draft", "Drafted this week's retro", app.EntityRetro, r.ID, nil, r)
				}
				return r, record(ctx, a, "save_retro_draft", "Updated the retro draft", app.EntityRetro, r.ID, before, r)
			}),

		define("remember",
			"Remember a fact about the user for future check-ins (preferences, what helps, important context). Keep it short. The user can see and edit these.",
			true, []prop{str("text", "").req()},
			func(ctx context.Context, a *app.App, in struct {
				Text string `json:"text"`
			},
			) (any, error) {
				n, err := a.Store.CreateNote(in.Text)
				if err != nil {
					return nil, err
				}
				return n, record(ctx, a, "remember", fmt.Sprintf("Remembered: %s", n.Text), app.EntityNote, n.ID, nil, n)
			}),

		define("forget",
			"Forget a remembered note that's outdated or that the user asked you to drop.",
			true, []prop{integer("id", "Note id.").req()},
			func(ctx context.Context, a *app.App, in struct {
				ID int64 `json:"id"`
			},
			) (any, error) {
				n, err := a.Store.GetNote(in.ID)
				if err != nil {
					return nil, err
				}
				if err := a.Store.DeleteNote(in.ID); err != nil {
					return nil, err
				}
				return map[string]string{"status": "forgotten"}, record(ctx, a, "forget", fmt.Sprintf("Forgot: %s", n.Text), app.EntityNote, n.ID, n, nil)
			}),
	}
}

// record logs an AI action. The write already happened, so a logging failure
// is reported but shouldn't be mistaken for the write failing.
func record(ctx context.Context, a *app.App, tool, summary, entity string, id int64, before, after any) error {
	if _, err := a.Record(ctx, tool, summary, entity, id, before, after); err != nil {
		return fmt.Errorf("change saved, but logging it for undo failed: %w", err)
	}
	return nil
}

func laneName(lane string) string {
	switch lane {
	case db.LaneSomeday:
		return "Someday"
	case db.LaneWeek:
		return "This week"
	case db.LaneToday:
		return "Today"
	case db.LaneDone:
		return "Done"
	case db.LaneLetGo:
		return "Let go"
	}
	return lane
}
