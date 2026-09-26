package eval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jmelahman/agilecbt/internal/app"
	"github.com/jmelahman/agilecbt/internal/curator"
	"github.com/jmelahman/agilecbt/internal/db"
	"github.com/jmelahman/agilecbt/internal/tools"
)

// Model is the model under test, at an OpenAI-compatible endpoint.
type Model struct {
	Name    string `toml:"name" json:"name"`
	BaseURL string `toml:"base_url" json:"base_url"`
	APIKey  string `toml:"api_key" json:"-"`
	// APIKeyEnv names an environment variable holding the key, so a
	// committed matrix can reference hosted models without the secret.
	APIKeyEnv       string `toml:"api_key_env" json:"-"`
	ReasoningEffort string `toml:"reasoning_effort" json:"reasoning_effort,omitempty"`
}

// Options configure a benchmark run.
type Options struct {
	Model Model
	// Judge grades rubric questions; nil skips them.
	Judge *Judge
	// Prompt and RetroPrompt replace the built-in prompts when set.
	Prompt, RetroPrompt string
	// Tools restricts the tools sent to the model; empty sends them all.
	Tools []string
	// Decoys adds that many plausible, never-correct tools.
	Decoys int
	// Runs overrides each scenario's run count when positive.
	Runs int
	// Parallel is how many runs go at once (default 1).
	Parallel    int
	TurnTimeout time.Duration
	HTTP        *http.Client
	// Progress receives one line per finished run; nil discards them.
	Progress io.Writer
}

// DefaultRuns is how often a scenario repeats unless it or Options says
// otherwise. A 27B turn takes a minute or two, so keep it small; safety
// scenarios ask for more.
const DefaultRuns = 3

// Run benchmarks the model on every scenario.
func Run(ctx context.Context, scenarios []Scenario, opts Options) (*Report, error) {
	if opts.Parallel < 1 {
		opts.Parallel = 1
	}
	if opts.TurnTimeout <= 0 {
		opts.TurnTimeout = 5 * time.Minute
	}
	if opts.Progress == nil {
		opts.Progress = io.Discard
	}
	known, err := toolIndex()
	if err != nil {
		return nil, err
	}
	for _, name := range opts.Tools {
		if _, ok := known[name]; !ok {
			return nil, fmt.Errorf("--tools: unknown tool %q", name)
		}
	}
	if err := CheckToolNames(scenarios, known); err != nil {
		return nil, err
	}

	type job struct{ si, run int }
	var jobs []job
	results := make([][]runResult, len(scenarios))
	for si, s := range scenarios {
		n := s.Runs
		if opts.Runs > 0 {
			n = opts.Runs
		}
		if n < 1 {
			n = DefaultRuns
		}
		results[si] = make([]runResult, n)
		for r := range n {
			jobs = append(jobs, job{si, r})
		}
	}

	start := time.Now()
	var mu sync.Mutex
	done := 0
	sem := make(chan struct{}, opts.Parallel)
	var wg sync.WaitGroup
	for _, j := range jobs {
		if ctx.Err() != nil {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer func() { <-sem; wg.Done() }()
			s := scenarios[j.si]
			res := runOnce(ctx, s, opts)
			mu.Lock()
			defer mu.Unlock()
			results[j.si][j.run] = res
			done++
			mark := "✓"
			if !res.passed() {
				mark = "✗ " + strings.Join(res.failedChecks(), ", ")
			}
			fmt.Fprintf(opts.Progress, "[%d/%d] %s run %d: %s (%s)\n", done, len(jobs), s.ID, j.run+1, mark, res.Latency.Round(time.Second))
		}()
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	rep := newReport(opts, scenarios, results, time.Since(start))
	return rep, nil
}

// CheckToolNames reports scenarios that name a tool the registry doesn't
// have.
func CheckToolNames(scenarios []Scenario, known map[string]bool) error {
	for _, s := range scenarios {
		for i, t := range append(slices.Clone(s.Turns), Turn{Expect: s.Expect}) {
			names := slices.Concat(t.Expect.ToolCalled, t.Expect.ToolNotCalled)
			for name := range t.Expect.ToolArgs {
				names = append(names, name)
			}
			for _, name := range names {
				if _, ok := known[name]; !ok {
					return fmt.Errorf("%s: turn %d: unknown tool %q", s.ID, i+1, name)
				}
			}
		}
	}
	return nil
}

// toolIndex maps every registry tool name to whether it mutates.
func toolIndex() (map[string]bool, error) {
	store, err := db.Open(":memory:")
	if err != nil {
		return nil, err
	}
	defer store.Close()
	out := map[string]bool{}
	for _, t := range tools.New(app.New(store)).List() {
		out[t.Name] = t.Mutates
	}
	return out, nil
}

// runResult is the outcome of one scenario run.
type runResult struct {
	Turns   []TurnResult
	Latency time.Duration
	// Err is a setup failure, before any turn ran.
	Err string

	ToolPayload ToolPayload
	counts      metricCounts
}

// TurnResult is one turn of a run.
type TurnResult struct {
	User     string        `json:"user,omitempty"`
	Reply    string        `json:"reply"`
	Calls    []CallRecord  `json:"calls,omitempty"`
	Failures []Failure     `json:"failures,omitempty"`
	Latency  time.Duration `json:"latency_ns"`
}

// CallRecord is a tool call, as saved in the report.
type CallRecord struct {
	Name  string          `json:"name"`
	Args  json.RawMessage `json:"args"`
	Error string          `json:"error,omitempty"`
}

// metricCounts are one run's contributions to the tool-use metrics.
type metricCounts struct {
	NeedCall, GotCall         int // turns with tool_called; of those, all called
	Constrained, Unwanted     int // turns forbidding calls; of those, violated
	Replies, Phantom          int // chat replies; of those, phantom actions
	Calls, UnknownTool        int
	ToolErrors, StringArgs    int
	RoundLimit, DecoyCalls    int
	JudgeErrors, JudgeSkipped int
}

func (r runResult) passed() bool {
	if r.Err != "" {
		return false
	}
	for _, t := range r.Turns {
		if len(t.Failures) > 0 {
			return false
		}
	}
	return true
}

func (r runResult) failedChecks() []string {
	if r.Err != "" {
		return []string{"setup"}
	}
	var out []string
	for _, t := range r.Turns {
		for _, f := range t.Failures {
			if !slices.Contains(out, f.Check) {
				out = append(out, f.Check)
			}
		}
	}
	return out
}

// runOnce plays a scenario once on a fresh in-memory database.
func runOnce(ctx context.Context, s Scenario, opts Options) runResult {
	start := time.Now()
	res, err := play(ctx, s, opts)
	if err != nil {
		res.Err = err.Error()
	}
	res.Latency = time.Since(start)
	return res
}

func play(ctx context.Context, s Scenario, opts Options) (runResult, error) {
	var res runResult
	store, err := db.Open(":memory:")
	if err != nil {
		return res, err
	}
	defer store.Close()
	a := app.New(store)
	now, err := scenarioNow(s)
	if err != nil {
		return res, err
	}
	a.Now = func() time.Time { return now }
	a.ConfigCrisisResources = s.CrisisResources
	reg := tools.New(a)

	offered, decoys := toolset(reg, opts)
	res.ToolPayload = payload(offered, len(decoys))
	mutates := map[string]bool{}
	var allNames []string
	for _, t := range reg.List() {
		mutates[t.Name] = t.Mutates
		allNames = append(allNames, t.Name)
	}
	allNames = append(allNames, decoys...)

	var calls []curator.ToolCall
	var callsMu sync.Mutex
	backend := &curator.OpenAI{
		BaseURL:         opts.Model.BaseURL,
		APIKey:          opts.Model.APIKey,
		Model:           opts.Model.Name,
		ReasoningEffort: opts.Model.ReasoningEffort,
		Registry:        reg,
		HTTP:            opts.HTTP,
		Tools:           offered,
		OnToolCall: func(c curator.ToolCall) {
			callsMu.Lock()
			calls = append(calls, c)
			callsMu.Unlock()
		},
	}
	cur := curator.New(reg, backend)
	cur.Prompt, cur.RetroPrompt = opts.Prompt, opts.RetroPrompt

	if s.Kind == KindRetro {
		return playRetro(ctx, s, opts, a, cur, now, res)
	}

	checkinID, err := seed(a, s, now)
	if err != nil {
		return res, fmt.Errorf("seed: %w", err)
	}
	for _, t := range s.Turns {
		callsMu.Lock()
		calls = nil
		callsMu.Unlock()
		before, err := store.ListMessages(checkinID)
		if err != nil {
			return res, err
		}
		turnStart := time.Now()
		tctx, cancel := context.WithTimeout(ctx, opts.TurnTimeout)
		chatErr := cur.Chat(tctx, checkinID, t.User, func(string, any) {})
		cancel()
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		after, err := store.ListMessages(checkinID)
		if err != nil {
			return res, err
		}
		reply := ""
		if len(after) > len(before)+1 && after[len(after)-1].Role == "assistant" {
			reply = after[len(after)-1].Text
		}
		callsMu.Lock()
		turnCalls := slices.Clone(calls)
		callsMu.Unlock()

		tr := TurnResult{User: t.User, Reply: reply, Latency: time.Since(turnStart), Calls: records(turnCalls)}
		if chatErr != nil {
			tr.Failures = append(tr.Failures, Failure{CheckTurnError, chatErr.Error()})
			if strings.Contains(chatErr.Error(), "tool rounds") {
				res.counts.RoundLimit++
			}
		}
		tr.Failures = append(tr.Failures, globalChecks(reply, allNames, s.HasTag(TagSafety))...)
		isMutating := func(name string) bool { return mutates[name] }
		if f := phantom(reply, turnCalls, isMutating); f != nil {
			tr.Failures = append(tr.Failures, *f)
			res.counts.Phantom++
		}
		res.counts.Replies++
		tr.Failures = append(tr.Failures, expectChecks(t.Expect, reply, turnCalls)...)
		tr.Failures = append(tr.Failures, dbChecks(a, t.Expect.DB)...)
		countCalls(&res.counts, t.Expect, turnCalls, mutates, decoys)

		if len(t.Expect.Judge) > 0 {
			if opts.Judge == nil {
				res.counts.JudgeSkipped += len(t.Expect.Judge)
			} else {
				tr.Failures = append(tr.Failures, judgeTurn(ctx, opts.Judge, after, turnCalls, "", t.Expect.Judge, &res.counts)...)
			}
		}
		res.Turns = append(res.Turns, tr)
		if errors.Is(chatErr, context.DeadlineExceeded) {
			break // later turns build on a reply we never got
		}
	}
	return res, nil
}

func judgeTurn(ctx context.Context, j *Judge, msgs []db.Message, calls []curator.ToolCall, data string, qs []string, c *metricCounts) []Failure {
	in := judgeInput{Calls: calls, Data: data, Questions: qs}
	for _, m := range msgs {
		in.Transcript = append(in.Transcript, transcriptLine{m.Role, m.Text})
	}
	fails, err := j.Grade(ctx, in)
	if err != nil {
		c.JudgeErrors++
		return []Failure{{CheckJudge, err.Error()}}
	}
	return fails
}

func playRetro(ctx context.Context, s Scenario, opts Options, a *app.App, cur *curator.Curator, now time.Time, res runResult) (runResult, error) {
	weekID, err := seedRetro(a, s, now)
	if err != nil {
		return res, fmt.Errorf("seed: %w", err)
	}
	review, err := a.ReviewWeek(weekID)
	if err != nil {
		return res, err
	}
	data, err := json.MarshalIndent(review, "", " ")
	if err != nil {
		return res, err
	}
	turnStart := time.Now()
	tctx, cancel := context.WithTimeout(ctx, opts.TurnTimeout)
	retro, draftErr := cur.DraftRetro(tctx, weekID)
	cancel()
	if ctx.Err() != nil {
		return res, ctx.Err()
	}
	tr := TurnResult{Reply: retro.AIDraft, Latency: time.Since(turnStart)}
	if draftErr != nil {
		tr.Failures = append(tr.Failures, Failure{CheckTurnError, draftErr.Error()})
	}
	if draftErr == nil && (retro.WentWell == "" || retro.WasHard == "" || retro.TryNext == "") {
		tr.Failures = append(tr.Failures, Failure{CheckRetroJSON, "the draft wasn't the expected JSON with went_well, was_hard and try_next"})
	}
	tr.Failures = append(tr.Failures, globalChecks(retro.AIDraft, nil, false)...)
	tr.Failures = append(tr.Failures, expectChecks(s.Expect, retro.AIDraft, nil)...)
	if len(s.Expect.Judge) > 0 {
		if opts.Judge == nil {
			res.counts.JudgeSkipped += len(s.Expect.Judge)
		} else {
			draft := []db.Message{{Role: "retro draft", Text: retro.AIDraft}}
			tr.Failures = append(tr.Failures, judgeTurn(ctx, opts.Judge, draft, nil, "The week's data the draft was written from:\n"+string(data), s.Expect.Judge, &res.counts)...)
		}
	}
	res.Turns = append(res.Turns, tr)
	return res, nil
}

func countCalls(c *metricCounts, e Expect, calls []curator.ToolCall, mutates map[string]bool, decoys []string) {
	if len(e.ToolCalled) > 0 {
		c.NeedCall++
		if !slices.ContainsFunc(e.ToolCalled, func(name string) bool { return !calledOK(calls, name) }) {
			c.GotCall++
		}
	}
	if len(e.ToolNotCalled) > 0 || e.NoToolCallsMatching != "" {
		c.Constrained++
		if slices.ContainsFunc(calls, func(tc curator.ToolCall) bool {
			return forbids(e, tc.Name) && (mutates[tc.Name] || slices.Contains(decoys, tc.Name))
		}) {
			c.Unwanted++
		}
	}
	for _, tc := range calls {
		c.Calls++
		switch {
		case slices.Contains(decoys, tc.Name):
			c.DecoyCalls++
		case errors.Is(tc.Err, tools.ErrUnknownTool):
			c.UnknownTool++
		case tc.Err != nil:
			c.ToolErrors++
		}
		if stringArgs(tc.Args) {
			c.StringArgs++
		}
	}
}

func records(calls []curator.ToolCall) []CallRecord {
	out := make([]CallRecord, len(calls))
	for i, c := range calls {
		out[i] = CallRecord{Name: c.Name, Args: c.Args}
		if !json.Valid(c.Args) {
			// Keep malformed arguments readable without breaking the report.
			out[i].Args, _ = json.Marshal(string(c.Args))
		}
		if c.Err != nil {
			out[i].Error = c.Err.Error()
		}
	}
	return out
}

func dbChecks(a *app.App, e DBExpect) []Failure {
	var out []Failure
	fail := func(what string, err error) {
		out = append(out, Failure{CheckDB, what + ": " + err.Error()})
	}
	for lane, r := range e.Lanes {
		steps, err := a.Store.ListSteps(db.StepFilter{Lanes: []string{lane}})
		if err == nil {
			err = r.check(len(steps))
		}
		if err != nil {
			fail(lane+" lane", err)
		}
	}
	count := func(what string, r *Range, n func() (int, error)) {
		if r == nil {
			return
		}
		got, err := n()
		if err == nil {
			err = r.check(got)
		}
		if err != nil {
			fail(what, err)
		}
	}
	count("notes", e.Notes, func() (int, error) { n, err := a.Store.ListNotes(); return len(n), err })
	count("values", e.Values, func() (int, error) { v, err := a.Store.ListValues(); return len(v), err })
	count("goals", e.Goals, func() (int, error) { g, err := a.Store.ListGoals(""); return len(g), err })
	count("thought records", e.ThoughtRecords, func() (int, error) { t, err := a.Store.ListThoughtRecords(1000); return len(t), err })
	return out
}

// scenarioNow is today's date at the scenario's time of day.
func scenarioNow(s Scenario) (time.Time, error) {
	tod, err := time.Parse("15:04", s.Time)
	if err != nil {
		return time.Time{}, err
	}
	y, m, d := time.Now().Date()
	return time.Date(y, m, d, tod.Hour(), tod.Minute(), 0, 0, time.Local), nil
}

func utc(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05Z") }

// seedBoard adds values, goals, notes and steps. doneAt places completed
// steps in time.
func seedBoard(a *app.App, s Scenario, now time.Time, doneAt func(day int) time.Time) error {
	st := a.Store
	valueIDs := map[string]int64{}
	for _, v := range s.Values {
		created, err := st.CreateValue(db.ValuePatch{Name: &v.Name, Description: &v.Description})
		if err != nil {
			return err
		}
		valueIDs[v.Name] = created.ID
	}
	goalIDs := map[string]int64{}
	for _, g := range s.Goals {
		p := db.GoalPatch{Title: &g.Title, Why: &g.Why}
		if g.Value != "" {
			id := valueIDs[g.Value]
			p.ValueID = &id
		}
		if g.Status != "" {
			p.Status = &g.Status
		}
		created, err := st.CreateGoal(p)
		if err != nil {
			return err
		}
		goalIDs[g.Title] = created.ID
	}
	for _, n := range s.Notes {
		if _, err := st.CreateNote(n); err != nil {
			return err
		}
	}
	for _, sd := range s.Steps {
		p := db.StepPatch{Title: &sd.Title}
		if sd.EnergyCost > 0 {
			p.EnergyCost = &sd.EnergyCost
		}
		if sd.Goal != "" {
			id := goalIDs[sd.Goal]
			p.GoalID = &id
		}
		lane := sd.Lane
		if sd.DoneDay != nil {
			lane = db.LaneToday
		}
		step, err := st.CreateStep(lane, p)
		if err != nil {
			return err
		}
		switch {
		case sd.DoneDay != nil:
			if step, err = st.CompleteStep(step.ID, sd.Mastery, sd.Pleasure); err != nil {
				return err
			}
			if at := doneAt(*sd.DoneDay); !at.IsZero() {
				ts := utc(at)
				step.CompletedAt, step.LaneChangedAt, step.UpdatedAt = &ts, ts, ts
				if err := st.PutStep(step); err != nil {
					return err
				}
			}
		case sd.CarriedOver:
			step.LaneChangedAt = utc(now.AddDate(0, 0, -1))
			if err := st.PutStep(step); err != nil {
				return err
			}
		}
	}
	return nil
}

// seed prepares a check-in conversation and returns its check-in id.
func seed(a *app.App, s Scenario, now time.Time) (int64, error) {
	st := a.Store
	if err := seedBoard(a, s, now, func(int) time.Time { return time.Time{} }); err != nil {
		return 0, err
	}
	for _, h := range s.MoodHistory {
		date := now.AddDate(0, 0, -h.DaysAgo).Format(time.DateOnly)
		kind := db.KindMorning
		if _, err := st.CreateCheckin(db.CheckinPatch{Date: &date, Kind: &kind, Mood: h.Mood, Energy: h.Energy, Anxiety: h.Anxiety, Note: &h.Note}); err != nil {
			return 0, err
		}
	}
	today := now.Format(time.DateOnly)
	kind, topic := s.Kind, ""
	if s.Kind == KindRoadmap {
		kind, topic = db.KindAdhoc, db.TopicRoadmap
	}
	c := s.Checkin
	checkin, err := st.CreateCheckin(db.CheckinPatch{Date: &today, Kind: &kind, Topic: &topic, Mood: c.Mood, Energy: c.Energy, Anxiety: c.Anxiety, Note: &c.Note})
	if err != nil {
		return 0, err
	}
	for _, m := range s.History {
		if _, err := st.AppendMessage(checkin.ID, m.Role, m.Text, ""); err != nil {
			return 0, err
		}
	}
	return checkin.ID, nil
}

// seedRetro fills last week and returns its week id.
func seedRetro(a *app.App, s Scenario, now time.Time) (int64, error) {
	thisWeek, err := time.ParseInLocation(time.DateOnly, app.WeekStart(now), now.Location())
	if err != nil {
		return 0, err
	}
	monday := thisWeek.AddDate(0, 0, -7)
	week, err := a.Store.EnsureWeek(monday.Format(time.DateOnly))
	if err != nil {
		return 0, err
	}
	noon := func(day int) time.Time { return monday.AddDate(0, 0, day).Add(12 * time.Hour) }
	if err := seedBoard(a, s, now, noon); err != nil {
		return 0, err
	}
	for _, h := range s.MoodHistory {
		date := monday.AddDate(0, 0, h.Day).Format(time.DateOnly)
		kind := db.KindMorning
		if _, err := a.Store.CreateCheckin(db.CheckinPatch{Date: &date, Kind: &kind, Mood: h.Mood, Energy: h.Energy, Anxiety: h.Anxiety, Note: &h.Note}); err != nil {
			return 0, err
		}
	}
	return week.ID, nil
}
