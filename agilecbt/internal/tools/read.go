package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/jmelahman/agilecbt/internal/app"
	"github.com/jmelahman/agilecbt/internal/db"
)

var lanes = []string{db.LaneSomeday, db.LaneWeek, db.LaneToday, db.LaneDone, db.LaneLetGo}

func readTools() []Tool {
	return []Tool{
		define("get_today",
			"Get today's picture: morning/evening check-ins, steps in the Today lane (carried_over marks ones put there on an earlier day), steps finished today, this week's intention and This-week lane, and the last 7 days of mood.",
			false, nil,
			func(ctx context.Context, a *app.App, _ struct{}) (any, error) {
				snap, err := a.TodaySnapshot()
				if err != nil {
					return nil, err
				}
				mood, err := a.MoodHistory(7)
				if err != nil {
					return nil, err
				}
				return map[string]any{"today": snap, "mood_last_7_days": mood}, nil
			}),

		define("get_mood_history",
			"Daily mood, energy, and anxiety (0-10, null when no check-in) for the last N days, oldest first.",
			false, []prop{integer("days", "How many days back, including today (default 14).", 1, 366)},
			func(ctx context.Context, a *app.App, in struct {
				Days int `json:"days"`
			},
			) (any, error) {
				return a.MoodHistory(in.Days)
			}),

		define("list_values",
			"List the user's life values (areas like Health or Connection) that goals serve.",
			false, nil,
			func(ctx context.Context, a *app.App, _ struct{}) (any, error) {
				return a.Store.ListValues()
			}),

		define("list_goals",
			"List roadmap goals. Status 'resting' is a guilt-free pause.",
			false, []prop{enum("status", "Filter by status; omit for all.", db.GoalActive, db.GoalResting, db.GoalDone)},
			func(ctx context.Context, a *app.App, in struct {
				Status string `json:"status"`
			},
			) (any, error) {
				return a.Store.ListGoals(in.Status)
			}),

		define("list_steps",
			"List steps on the board, optionally filtered by lanes and/or goal. energy_cost is 1 (tiny) to 3 (big).",
			false, []prop{
				array("lanes", "Lanes to include; omit for all.", map[string]any{"type": "string", "enum": lanes}),
				integer("goal_id", "Only steps for this goal."),
			},
			func(ctx context.Context, a *app.App, in struct {
				Lanes  []string `json:"lanes"`
				GoalID *int64   `json:"goal_id"`
			},
			) (any, error) {
				return a.Store.ListSteps(db.StepFilter{Lanes: in.Lanes, GoalID: in.GoalID})
			}),

		define("list_thought_records",
			"List recent CBT thought records, newest first.",
			false, []prop{integer("limit", "Maximum records (default 10).", 1, 100)},
			func(ctx context.Context, a *app.App, in struct {
				Limit int `json:"limit"`
			},
			) (any, error) {
				if in.Limit == 0 {
					in.Limit = 10
				}
				return a.Store.ListThoughtRecords(in.Limit)
			}),

		define("get_week",
			"Review a week (sprint): its intention, check-ins, steps completed with mastery/pleasure, thought records, and retro if any. Defaults to the current week.",
			false, []prop{str("week_start", "Any date (YYYY-MM-DD) within the week; omit for this week.")},
			func(ctx context.Context, a *app.App, in struct {
				WeekStart string `json:"week_start"`
			},
			) (any, error) {
				w, err := weekFor(a, in.WeekStart)
				if err != nil {
					return nil, err
				}
				return a.ReviewWeek(w.ID)
			}),

		define("list_curator_notes",
			"List what you (the curator) have remembered about the user. The user can see and edit these.",
			false, nil,
			func(ctx context.Context, a *app.App, _ struct{}) (any, error) {
				return a.Store.ListNotes()
			}),
	}
}

// weekFor returns the week containing date (YYYY-MM-DD), or the current week
// when date is empty.
func weekFor(a *app.App, date string) (db.Week, error) {
	if date == "" {
		return a.CurrentWeek()
	}
	t, err := time.ParseInLocation(time.DateOnly, date, a.Now().Location())
	if err != nil {
		return db.Week{}, fmt.Errorf("%w: week_start must be YYYY-MM-DD", db.ErrInvalid)
	}
	return a.Store.EnsureWeek(app.WeekStart(t))
}
