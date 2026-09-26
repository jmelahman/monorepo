// Package app holds domain logic shared by the REST API, the AI tool
// registry, and the curator: the clock, "today"/"this week" views, mood
// history, and the AI action log with undo.
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jmelahman/agilecbt/internal/db"
)

// App bundles the store with a clock and the action broker.
type App struct {
	Store *db.Store
	// Now returns the current time; its Location defines what "today" means.
	Now    func() time.Time
	Broker *Broker
	// ConfigCrisisResources is crisis_resources from config.toml (or
	// $APP_CRISIS_RESOURCES); see CrisisResources.
	ConfigCrisisResources string
}

// New returns an App using the local wall clock.
func New(store *db.Store) *App {
	return &App{Store: store, Now: time.Now, Broker: NewBroker()}
}

// Today returns the local date as YYYY-MM-DD.
func (a *App) Today() string {
	return a.Now().Format(time.DateOnly)
}

// WeekStart returns the Monday on or before t as YYYY-MM-DD.
func WeekStart(t time.Time) string {
	offset := (int(t.Weekday()) + 6) % 7 // Monday = 0
	return t.AddDate(0, 0, -offset).Format(time.DateOnly)
}

// CurrentWeek returns this week's sprint, creating it if needed.
func (a *App) CurrentWeek() (db.Week, error) {
	return a.Store.EnsureWeek(WeekStart(a.Now()))
}

// startOfDayUTC returns local midnight of t's day formatted like the DB's
// UTC timestamps, for comparing against created_at/completed_at columns.
func startOfDayUTC(t time.Time) string {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location()).UTC().Format("2006-01-02T15:04:05Z")
}

// localDate converts a DB UTC timestamp to a local YYYY-MM-DD in loc.
func localDate(ts string, loc *time.Location) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ""
	}
	return t.In(loc).Format(time.DateOnly)
}

// TodayStep is a step in the Today lane, flagged when it was put there on an
// earlier day so the UI can offer "keep for today" or "let go" without
// treating it as overdue.
type TodayStep struct {
	db.Step
	CarriedOver bool `json:"carried_over"`
}

// Snapshot is everything the Today screen (and the curator) needs.
type Snapshot struct {
	Date      string      `json:"date"`
	Morning   *db.Checkin `json:"morning"`
	Evening   *db.Checkin `json:"evening"`
	Today     []TodayStep `json:"today"`
	DoneToday []db.Step   `json:"done_today"`
	Week      db.Week     `json:"week"`
	WeekSteps []db.Step   `json:"week_steps"`
}

// TodaySnapshot assembles the Today view.
func (a *App) TodaySnapshot() (Snapshot, error) {
	now := a.Now()
	snap := Snapshot{Date: now.Format(time.DateOnly), Today: []TodayStep{}}
	for kind, dst := range map[string]**db.Checkin{db.KindMorning: &snap.Morning, db.KindEvening: &snap.Evening} {
		c, err := a.Store.LatestCheckin(snap.Date, kind, "")
		switch {
		case err == nil:
			*dst = &c
		case err != db.ErrNotFound:
			return Snapshot{}, err
		}
	}
	today, err := a.Store.ListSteps(db.StepFilter{Lanes: []string{db.LaneToday}})
	if err != nil {
		return Snapshot{}, err
	}
	for _, st := range today {
		snap.Today = append(snap.Today, TodayStep{Step: st, CarriedOver: localDate(st.LaneChangedAt, now.Location()) < snap.Date})
	}
	if snap.DoneToday, err = a.Store.ListSteps(db.StepFilter{CompletedSince: startOfDayUTC(now)}); err != nil {
		return Snapshot{}, err
	}
	if snap.Week, err = a.CurrentWeek(); err != nil {
		return Snapshot{}, err
	}
	if snap.WeekSteps, err = a.Store.ListSteps(db.StepFilter{Lanes: []string{db.LaneWeek}}); err != nil {
		return Snapshot{}, err
	}
	return snap, nil
}

// DayMood is the latest mood/energy/anxiety reading for one day.
type DayMood struct {
	Date    string `json:"date"`
	Mood    *int   `json:"mood"`
	Energy  *int   `json:"energy"`
	Anxiety *int   `json:"anxiety"`
}

// MoodHistory returns one entry per day for the last days days (including
// today), oldest first. Days without a check-in have nil readings. When a day
// has several check-ins, the latest non-nil value of each metric wins.
func (a *App) MoodHistory(days int) ([]DayMood, error) {
	if days <= 0 {
		days = 7
	}
	now := a.Now()
	from := now.AddDate(0, 0, -(days - 1)).Format(time.DateOnly)
	checkins, err := a.Store.ListCheckins(from, now.Format(time.DateOnly))
	if err != nil {
		return nil, err
	}
	byDate := map[string]*DayMood{}
	out := make([]DayMood, days)
	for i := range days {
		d := now.AddDate(0, 0, -(days - 1 - i)).Format(time.DateOnly)
		out[i] = DayMood{Date: d}
		byDate[d] = &out[i]
	}
	for _, c := range checkins {
		dm := byDate[c.Date]
		if dm == nil {
			continue
		}
		if c.Mood != nil {
			dm.Mood = c.Mood
		}
		if c.Energy != nil {
			dm.Energy = c.Energy
		}
		if c.Anxiety != nil {
			dm.Anxiety = c.Anxiety
		}
	}
	return out, nil
}

// WeekReview gathers a week's data for the retro.
type WeekReview struct {
	Week           db.Week            `json:"week"`
	EndDate        string             `json:"end_date"`
	Checkins       []db.Checkin       `json:"checkins"`
	Completed      []db.Step          `json:"completed"`
	ThoughtRecords []db.ThoughtRecord `json:"thought_records"`
	Retro          *db.Retro          `json:"retro"`
}

// ReviewWeek collects check-ins, completed steps, and thought records for the
// week with weekID.
func (a *App) ReviewWeek(weekID int64) (WeekReview, error) {
	w, err := a.Store.GetWeek(weekID)
	if err != nil {
		return WeekReview{}, err
	}
	start, err := time.ParseInLocation(time.DateOnly, w.StartDate, a.Now().Location())
	if err != nil {
		return WeekReview{}, err
	}
	end := start.AddDate(0, 0, 7)
	r := WeekReview{Week: w, EndDate: end.AddDate(0, 0, -1).Format(time.DateOnly)}
	if r.Checkins, err = a.Store.ListCheckins(w.StartDate, r.EndDate); err != nil {
		return WeekReview{}, err
	}
	startUTC, endUTC := startOfDayUTC(start), startOfDayUTC(end)
	done, err := a.Store.ListSteps(db.StepFilter{CompletedSince: startUTC})
	if err != nil {
		return WeekReview{}, err
	}
	r.Completed = []db.Step{}
	for _, st := range done {
		if *st.CompletedAt < endUTC {
			r.Completed = append(r.Completed, st)
		}
	}
	thoughts, err := a.Store.ListThoughtRecordsSince(startUTC)
	if err != nil {
		return WeekReview{}, err
	}
	r.ThoughtRecords = []db.ThoughtRecord{}
	for _, t := range thoughts {
		if t.CreatedAt < endUTC {
			r.ThoughtRecords = append(r.ThoughtRecords, t)
		}
	}
	if retro, err := a.Store.GetRetro(weekID); err == nil {
		r.Retro = &retro
	} else if err != db.ErrNotFound {
		return WeekReview{}, err
	}
	return r, nil
}

// Actor identifies who is making AI writes, carried on the context.
type Actor struct {
	Source    string // "curator" or "mcp"
	CheckinID *int64
}

type actorKey struct{}

// WithActor attaches an Actor to ctx.
func WithActor(ctx context.Context, a Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, a)
}

// ActorFrom returns the Actor on ctx, defaulting to source "mcp".
func ActorFrom(ctx context.Context) Actor {
	if a, ok := ctx.Value(actorKey{}).(Actor); ok {
		return a
	}
	return Actor{Source: "mcp"}
}

// Record logs an AI write and publishes it to subscribers. before is nil for
// creations, after is nil for deletions.
func (a *App) Record(ctx context.Context, tool, summary, entity string, id int64, before, after any) (db.Action, error) {
	actor := ActorFrom(ctx)
	act := db.Action{
		CheckinID: actor.CheckinID,
		Source:    actor.Source,
		Tool:      tool,
		Summary:   summary,
		Entity:    entity,
		EntityID:  id,
	}
	var err error
	if act.BeforeJSON, err = optJSON(before); err != nil {
		return db.Action{}, err
	}
	if act.AfterJSON, err = optJSON(after); err != nil {
		return db.Action{}, err
	}
	act, err = a.Store.CreateAction(act)
	if err != nil {
		return db.Action{}, err
	}
	a.Broker.Publish(act)
	return act, nil
}

func optJSON(v any) (*string, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	s := string(b)
	return &s, nil
}

// Undo reverts an AI action: deletes what it created, or restores the
// before-state of what it changed or deleted.
func (a *App) Undo(actionID int64) (db.Action, error) {
	act, err := a.Store.GetAction(actionID)
	if err != nil {
		return db.Action{}, err
	}
	if act.UndoneAt != nil {
		return db.Action{}, fmt.Errorf("%w: action already undone", db.ErrInvalid)
	}
	if act.BeforeJSON == nil {
		err = a.deleteEntity(act.Entity, act.EntityID)
		if err == db.ErrNotFound {
			err = nil // already gone
		}
	} else {
		err = a.restoreEntity(act.Entity, []byte(*act.BeforeJSON))
	}
	if err != nil {
		return db.Action{}, err
	}
	if err := a.Store.MarkActionUndone(act.ID); err != nil {
		return db.Action{}, err
	}
	return a.Store.GetAction(act.ID)
}

// Entity names used in the action log.
const (
	EntityValue   = "value"
	EntityGoal    = "goal"
	EntityStep    = "step"
	EntityCheckin = "checkin"
	EntityThought = "thought_record"
	EntityWeek    = "week"
	EntityRetro   = "retro"
	EntityNote    = "note"
)

func (a *App) deleteEntity(entity string, id int64) error {
	s := a.Store
	switch entity {
	case EntityValue:
		return s.DeleteValue(id)
	case EntityGoal:
		return s.DeleteGoal(id)
	case EntityStep:
		return s.DeleteStep(id)
	case EntityCheckin:
		return s.DeleteCheckin(id)
	case EntityThought:
		return s.DeleteThoughtRecord(id)
	case EntityRetro:
		return s.DeleteRetro(id)
	case EntityNote:
		return s.DeleteNote(id)
	}
	return fmt.Errorf("undo: cannot delete entity %q", entity)
}

func (a *App) restoreEntity(entity string, raw []byte) error {
	s := a.Store
	switch entity {
	case EntityValue:
		return restore(raw, s.PutValue)
	case EntityGoal:
		return restore(raw, s.PutGoal)
	case EntityStep:
		return restore(raw, s.PutStep)
	case EntityCheckin:
		return restore(raw, s.PutCheckin)
	case EntityThought:
		return restore(raw, s.PutThoughtRecord)
	case EntityWeek:
		return restore(raw, s.PutWeek)
	case EntityRetro:
		return restore(raw, s.PutRetro)
	case EntityNote:
		return restore(raw, s.PutNote)
	}
	return fmt.Errorf("undo: cannot restore entity %q", entity)
}

func restore[T any](raw []byte, put func(T) error) error {
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}
	return put(v)
}

// Broker fans AI actions out to listeners (the curator chat stream) keyed by
// check-in id. Delivery is synchronous: Publish returns only after every
// listener has run, so a chat stream shows an action before any text the
// model writes after the tool call.
type Broker struct {
	mu     sync.Mutex
	nextID int
	subs   map[int64]map[int]func(db.Action)
}

// NewBroker returns an empty broker.
func NewBroker() *Broker {
	return &Broker{subs: map[int64]map[int]func(db.Action){}}
}

// Subscribe calls fn for each action on checkinID until cancel is called.
// fn must not call back into the broker.
func (b *Broker) Subscribe(checkinID int64, fn func(db.Action)) (cancel func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	id := b.nextID
	if b.subs[checkinID] == nil {
		b.subs[checkinID] = map[int]func(db.Action){}
	}
	b.subs[checkinID][id] = fn
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.subs[checkinID], id)
		if len(b.subs[checkinID]) == 0 {
			delete(b.subs, checkinID)
		}
	}
}

// Publish delivers act to the listeners of its check-in.
func (b *Broker) Publish(act db.Action) {
	if act.CheckinID == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, fn := range b.subs[*act.CheckinID] {
		fn(act)
	}
}

// DefaultKind picks morning or evening for a new check-in at t.
func DefaultKind(t time.Time) string {
	if t.Hour() >= 15 {
		return db.KindEvening
	}
	return db.KindMorning
}
