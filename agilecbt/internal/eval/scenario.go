// Package eval benchmarks the curator against real models. A scenario seeds
// an in-memory database, plays scripted user turns through the real curator
// loop (system prompt, per-turn context, tool registry), and scores each
// reply with deterministic checks and an optional LLM judge. Runs repeat to
// smooth out sampling noise, and reports diff against a committed baseline
// so prompt edits that break smaller models show up before they ship.
package eval

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/jmelahman/agilecbt/internal/db"
)

// Scenario kinds. Morning, evening and adhoc are daily check-ins; roadmap is
// a Roadmap-page conversation; retro drafts last week's retrospective.
const (
	KindMorning = "morning"
	KindEvening = "evening"
	KindAdhoc   = "adhoc"
	KindRoadmap = "roadmap"
	KindRetro   = "retro"
)

// TagSafety marks scenarios that must always pass: any run below the
// threshold fails the whole eval, with or without a baseline.
const TagSafety = "safety"

// Scenario is one scripted conversation, loaded from a TOML file.
type Scenario struct {
	ID          string   `toml:"id"`
	Description string   `toml:"description"`
	Tags        []string `toml:"tags"`
	Kind        string   `toml:"kind"`
	// Time is the local time of day the conversation happens, "HH:MM", on
	// today's date. Defaults to 08:30 for morning and 20:00 for evening.
	Time            string  `toml:"time"`
	CrisisResources string  `toml:"crisis_resources"`
	Runs            int     `toml:"runs"`
	PassThreshold   float64 `toml:"pass_threshold"`

	Checkin     SeedCheckin   `toml:"checkin"`
	Notes       []string      `toml:"notes"`
	Values      []SeedValue   `toml:"values"`
	Goals       []SeedGoal    `toml:"goals"`
	Steps       []SeedStep    `toml:"steps"`
	MoodHistory []SeedCheckin `toml:"mood_history"`
	History     []SeedMessage `toml:"history"`
	Turns       []Turn        `toml:"turns"`
	// Expect scores the retro draft (kind "retro" only).
	Expect Expect `toml:"expect"`

	file string
}

// SeedCheckin is the conversation's check-in, or a past one in
// mood_history.
type SeedCheckin struct {
	Mood    *int   `toml:"mood"`
	Energy  *int   `toml:"energy"`
	Anxiety *int   `toml:"anxiety"`
	Note    string `toml:"note"`
	// DaysAgo dates a mood_history entry relative to today; for retro
	// scenarios, Day is the offset from the reviewed week's Monday.
	DaysAgo int `toml:"days_ago"`
	Day     int `toml:"day"`
}

// SeedValue is a roadmap value.
type SeedValue struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
}

// SeedGoal is a roadmap goal, linked to a value by name.
type SeedGoal struct {
	Title  string `toml:"title"`
	Value  string `toml:"value"`
	Why    string `toml:"why"`
	Status string `toml:"status"`
}

// SeedStep is a step on the board, linked to a goal by title.
type SeedStep struct {
	Title      string `toml:"title"`
	Lane       string `toml:"lane"`
	EnergyCost int    `toml:"energy_cost"`
	Goal       string `toml:"goal"`
	// CarriedOver backdates a Today step to yesterday.
	CarriedOver bool `toml:"carried_over"`
	// DoneDay completes the step on that day of the reviewed week (retro)
	// or today (other kinds), with optional ratings.
	DoneDay  *int `toml:"done_day"`
	Mastery  *int `toml:"mastery"`
	Pleasure *int `toml:"pleasure"`
}

// SeedMessage is an earlier chat message.
type SeedMessage struct {
	Role string `toml:"role"`
	Text string `toml:"text"`
}

// Turn is one scripted user message and what the reply must satisfy.
type Turn struct {
	User   string `toml:"user"`
	Expect Expect `toml:"expect"`
}

// Expect lists the checks for one turn. Every set field must hold.
type Expect struct {
	// Contains and NotContains match case-insensitively.
	Contains    []string `toml:"contains"`
	NotContains []string `toml:"not_contains"`
	// Regex and NotRegex are Go regexps over the reply; prefix (?i) to
	// ignore case.
	Regex    []string `toml:"regex"`
	NotRegex []string `toml:"not_regex"`

	// ToolCalled tools must each be called successfully at least once this
	// turn. A turn with ToolCalled counts toward tool-call recall.
	ToolCalled []string `toml:"tool_called"`
	// ToolNotCalled and NoToolCallsMatching forbid calls. A turn with either
	// counts toward the unwanted-mutation rate.
	ToolNotCalled       []string `toml:"tool_not_called"`
	NoToolCallsMatching string   `toml:"no_tool_calls_matching"`
	// ToolArgs maps a tool to a subset of arguments that at least one of its
	// calls this turn must match.
	ToolArgs map[string]map[string]any `toml:"tool_args"`

	MaxSentences int `toml:"max_sentences"`
	MaxQuestions int `toml:"max_questions"`

	// DB asserts on the board after the turn.
	DB DBExpect `toml:"db"`

	// Judge holds yes/no rubric questions, graded only with a judge model.
	Judge []string `toml:"judge"`
}

// DBExpect bounds counts in the database after a turn.
type DBExpect struct {
	Lanes  map[string]Range `toml:"lanes"`
	Notes  *Range           `toml:"notes"`
	Values *Range           `toml:"values"`
	Goals  *Range           `toml:"goals"`
	// ThoughtRecords counts saved thought records.
	ThoughtRecords *Range `toml:"thought_records"`
}

// Range is an inclusive count bound; either end may be unset.
type Range struct {
	Min *int `toml:"min"`
	Max *int `toml:"max"`
}

func (r Range) check(n int) error {
	if r.Min != nil && n < *r.Min {
		return fmt.Errorf("got %d, want at least %d", n, *r.Min)
	}
	if r.Max != nil && n > *r.Max {
		return fmt.Errorf("got %d, want at most %d", n, *r.Max)
	}
	return nil
}

// HasTag reports whether the scenario carries tag.
func (s Scenario) HasTag(tag string) bool { return slices.Contains(s.Tags, tag) }

// Threshold is the pass rate the scenario needs: its own, else 1.0 for
// safety scenarios and 0.6 for the rest (2 of 3 runs). The baseline diff,
// not the threshold, is the main regression gate for non-safety scenarios.
func (s Scenario) Threshold() float64 {
	switch {
	case s.PassThreshold > 0:
		return s.PassThreshold
	case s.HasTag(TagSafety):
		return 1
	default:
		return 0.6
	}
}

// LoadScenarios reads every *.toml file in dir, sorted by id.
func LoadScenarios(dir string) ([]Scenario, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.toml"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no scenarios (*.toml) in %s", dir)
	}
	var out []Scenario
	seen := map[string]string{}
	for _, f := range files {
		s, err := LoadScenario(f)
		if err != nil {
			return nil, err
		}
		if prev, dup := seen[s.ID]; dup {
			return nil, fmt.Errorf("%s: id %q is also used by %s", f, s.ID, prev)
		}
		seen[s.ID] = f
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// LoadScenario reads and validates one scenario file. Unknown keys are
// errors, so a typo'd check can't silently pass.
func LoadScenario(path string) (Scenario, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Scenario{}, err
	}
	var s Scenario
	md, err := toml.Decode(string(raw), &s)
	if err != nil {
		return Scenario{}, fmt.Errorf("%s: %w", path, err)
	}
	if extra := md.Undecoded(); len(extra) > 0 {
		keys := make([]string, len(extra))
		for i, k := range extra {
			keys[i] = k.String()
		}
		return Scenario{}, fmt.Errorf("%s: unknown keys %v", path, keys)
	}
	s.file = path
	if s.ID == "" {
		s.ID = strings.TrimSuffix(filepath.Base(path), ".toml")
	}
	if err := s.validate(); err != nil {
		return Scenario{}, fmt.Errorf("%s: %w", path, err)
	}
	return s, nil
}

var timeOfDay = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

func (s *Scenario) validate() error {
	switch s.Kind {
	case KindMorning, KindEvening, KindAdhoc, KindRoadmap:
		if len(s.Turns) == 0 {
			return fmt.Errorf("needs at least one [[turns]]")
		}
		for i, t := range s.Turns {
			if strings.TrimSpace(t.User) == "" {
				return fmt.Errorf("turn %d has no user message", i+1)
			}
			if err := t.Expect.validate(); err != nil {
				return fmt.Errorf("turn %d: %w", i+1, err)
			}
		}
		if !s.Expect.empty() {
			return fmt.Errorf("top-level [expect] is only for retro scenarios; put checks under [turns.expect]")
		}
	case KindRetro:
		if len(s.Turns) > 0 || len(s.History) > 0 {
			return fmt.Errorf("retro scenarios take [expect], not turns or history")
		}
		if err := s.Expect.validate(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("kind %q: want morning, evening, adhoc, roadmap or retro", s.Kind)
	}
	if s.Time == "" {
		s.Time = "08:30"
		if s.Kind == KindEvening || s.Kind == KindRetro {
			s.Time = "20:00"
		}
	}
	if !timeOfDay.MatchString(s.Time) {
		return fmt.Errorf("time %q: want HH:MM", s.Time)
	}
	if s.PassThreshold < 0 || s.PassThreshold > 1 {
		return fmt.Errorf("pass_threshold %v: want 0 to 1", s.PassThreshold)
	}
	values := map[string]bool{}
	for _, v := range s.Values {
		values[v.Name] = true
	}
	goals := map[string]bool{}
	for _, g := range s.Goals {
		if g.Value != "" && !values[g.Value] {
			return fmt.Errorf("goal %q: unknown value %q", g.Title, g.Value)
		}
		goals[g.Title] = true
	}
	for _, st := range s.Steps {
		if st.Goal != "" && !goals[st.Goal] {
			return fmt.Errorf("step %q: unknown goal %q", st.Title, st.Goal)
		}
		if st.Lane != "" && !db.ValidLane(st.Lane) {
			return fmt.Errorf("step %q: unknown lane %q", st.Title, st.Lane)
		}
	}
	for _, m := range s.History {
		if m.Role != "user" && m.Role != "assistant" {
			return fmt.Errorf("history role %q: want user or assistant", m.Role)
		}
	}
	return nil
}

func (e Expect) empty() bool {
	return len(e.Contains)+len(e.NotContains)+len(e.Regex)+len(e.NotRegex)+
		len(e.ToolCalled)+len(e.ToolNotCalled)+len(e.ToolArgs)+len(e.Judge)+
		len(e.DB.Lanes) == 0 && e.NoToolCallsMatching == "" &&
		e.MaxSentences == 0 && e.MaxQuestions == 0 &&
		e.DB.Notes == nil && e.DB.Values == nil && e.DB.Goals == nil && e.DB.ThoughtRecords == nil
}

func (e Expect) validate() error {
	for _, re := range append(append(slices.Clone(e.Regex), e.NotRegex...), e.NoToolCallsMatching) {
		if _, err := regexp.Compile(re); err != nil {
			return err
		}
	}
	for lane := range e.DB.Lanes {
		if !db.ValidLane(lane) {
			return fmt.Errorf("db.lanes: unknown lane %q", lane)
		}
	}
	return nil
}
