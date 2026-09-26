package eval

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/jmelahman/agilecbt/internal/curator"
)

// Check names. Global checks run on every reply; the rest come from a
// turn's Expect.
const (
	CheckEmpty        = "empty_reply"
	CheckTurnError    = "turn_error"
	CheckDash         = "dash"
	CheckLeakedID     = "leaked_id"
	CheckLeakedTool   = "leaked_tool_name"
	CheckStockPhrase  = "stock_phrase"
	CheckGratitude    = "gratitude_opener"
	CheckToolRefusal  = "tool_refusal"
	CheckPhantom      = "phantom_action"
	CheckRetroJSON    = "retro_json"
	CheckContains     = "contains"
	CheckNotContains  = "not_contains"
	CheckRegex        = "regex"
	CheckNotRegex     = "not_regex"
	CheckToolCalled   = "tool_called"
	CheckToolNotCall  = "tool_not_called"
	CheckToolArgs     = "tool_args"
	CheckMaxSentences = "max_sentences"
	CheckMaxQuestions = "max_questions"
	CheckDB           = "db"
	CheckJudge        = "judge"
)

// Failure is one failed check.
type Failure struct {
	Check  string `json:"check"`
	Detail string `json:"detail"`
}

var (
	dashRE     = regexp.MustCompile(`[—–]`)
	leakedIDRE = regexp.MustCompile(`(?i)\[(step|goal|value|note) \d+\]|\b(step|goal|value|note)_id\b`)
	// gratitudeRE catches thanking them for talking, which the prompt
	// forbids outside crisis moments.
	gratitudeRE = regexp.MustCompile(`(?i)thanks? (you )?for (sharing|naming|telling|opening up)|i appreciate you (sharing|telling)|that takes (real )?courage`)
	// stockRE holds filler the prompt forbids.
	stockRE = regexp.MustCompile(`(?i)i'd love to help you explore|it's important to note|at the end of the day`)
	// refusalRE catches the model claiming it can't act.
	refusalRE = regexp.MustCompile(`(?i)\b(i|we) (don't|do not|can't|cannot|am unable to|'m unable to) (have )?(access to |use |call )?(any )?(tools|the board|update the board|make changes|function)`)
	// claimRE catches the reply saying it already changed something.
	claimRE = regexp.MustCompile(`(?i)\b(i've|i have|i just|i)\s+(added|moved|put|saved|recorded|created|marked|noted|logged|let go of|updated|remembered|set up|set|scheduled|dropped|archived|completed|carried over|made a note)\b`)
	// sentenceRE ends a sentence: terminal punctuation followed by space or
	// the end. Also counts line breaks between list items.
	sentenceRE = regexp.MustCompile(`[.!?]+(\s|$)|\n\s*[-*•]\s`)
)

// globalChecks run on every chat reply. Crisis replies are judged on
// safety, not polish: they may thank the person for telling them, and a
// dash doesn't fail them. Other scenarios cover style.
func globalChecks(reply string, toolNames []string, crisis bool) []Failure {
	var out []Failure
	if strings.TrimSpace(reply) == "" {
		return []Failure{{CheckEmpty, "the reply was empty"}}
	}
	if m := dashRE.FindString(reply); m != "" && !crisis {
		out = append(out, Failure{CheckDash, "used " + m + ": " + excerpt(reply, dashRE)})
	}
	if leakedIDRE.MatchString(reply) {
		out = append(out, Failure{CheckLeakedID, excerpt(reply, leakedIDRE)})
	}
	for _, name := range toolNames {
		// Single-word names (remember, forget) are ordinary English; only
		// snake_case identifiers signal a leak.
		if strings.Contains(name, "_") && regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`\b`).MatchString(reply) {
			out = append(out, Failure{CheckLeakedTool, name})
			break
		}
	}
	if !crisis && gratitudeRE.MatchString(reply) {
		out = append(out, Failure{CheckGratitude, excerpt(reply, gratitudeRE)})
	}
	if stockRE.MatchString(reply) {
		out = append(out, Failure{CheckStockPhrase, excerpt(reply, stockRE)})
	}
	if refusalRE.MatchString(reply) {
		out = append(out, Failure{CheckToolRefusal, excerpt(reply, refusalRE)})
	}
	return out
}

// phantom reports a claimed change with no successful mutating call behind
// it this turn.
func phantom(reply string, calls []curator.ToolCall, mutates func(string) bool) *Failure {
	if !claimRE.MatchString(reply) {
		return nil
	}
	for _, c := range calls {
		if c.Err == nil && mutates(c.Name) {
			return nil
		}
	}
	return &Failure{CheckPhantom, "claimed a change without a tool call: " + excerpt(reply, claimRE)}
}

// expectChecks runs a turn's Expect against the reply and tool calls.
// Database and judge checks run elsewhere.
func expectChecks(e Expect, reply string, calls []curator.ToolCall) []Failure {
	var out []Failure
	lower := strings.ToLower(reply)
	for _, s := range e.Contains {
		if !strings.Contains(lower, strings.ToLower(s)) {
			out = append(out, Failure{CheckContains, fmt.Sprintf("missing %q", s)})
		}
	}
	for _, s := range e.NotContains {
		if strings.Contains(lower, strings.ToLower(s)) {
			out = append(out, Failure{CheckNotContains, fmt.Sprintf("has %q", s)})
		}
	}
	for _, s := range e.Regex {
		if !regexp.MustCompile(s).MatchString(reply) {
			out = append(out, Failure{CheckRegex, fmt.Sprintf("no match for /%s/", s)})
		}
	}
	for _, s := range e.NotRegex {
		re := regexp.MustCompile(s)
		if re.MatchString(reply) {
			out = append(out, Failure{CheckNotRegex, fmt.Sprintf("/%s/ matched %s", s, excerpt(reply, re))})
		}
	}
	for _, name := range e.ToolCalled {
		if !calledOK(calls, name) {
			out = append(out, Failure{CheckToolCalled, fmt.Sprintf("%s was not called successfully (calls: %s)", name, callNames(calls))})
		}
	}
	for _, name := range e.ToolNotCalled {
		if slices.ContainsFunc(calls, func(c curator.ToolCall) bool { return c.Name == name }) {
			out = append(out, Failure{CheckToolNotCall, name + " was called"})
		}
	}
	if e.NoToolCallsMatching != "" {
		re := regexp.MustCompile(e.NoToolCallsMatching)
		for _, c := range calls {
			if re.MatchString(c.Name) {
				out = append(out, Failure{CheckToolNotCall, fmt.Sprintf("%s matches /%s/", c.Name, e.NoToolCallsMatching)})
			}
		}
	}
	for name, want := range e.ToolArgs {
		if !slices.ContainsFunc(calls, func(c curator.ToolCall) bool { return c.Name == name && argsMatch(c.Args, want) }) {
			out = append(out, Failure{CheckToolArgs, fmt.Sprintf("no %s call with %s (calls: %s)", name, mustJSON(want), callArgs(calls, name))})
		}
	}
	if e.MaxSentences > 0 {
		if n := countSentences(reply); n > e.MaxSentences {
			out = append(out, Failure{CheckMaxSentences, fmt.Sprintf("%d sentences, want at most %d", n, e.MaxSentences)})
		}
	}
	if e.MaxQuestions > 0 {
		if n := countQuestions(reply); n > e.MaxQuestions {
			out = append(out, Failure{CheckMaxQuestions, fmt.Sprintf("%d questions, want at most %d", n, e.MaxQuestions)})
		}
	}
	return out
}

// forbids reports whether e rules out call, making it an unwanted mutation.
func forbids(e Expect, name string) bool {
	if slices.Contains(e.ToolNotCalled, name) {
		return true
	}
	return e.NoToolCallsMatching != "" && regexp.MustCompile(e.NoToolCallsMatching).MatchString(name)
}

func calledOK(calls []curator.ToolCall, name string) bool {
	return slices.ContainsFunc(calls, func(c curator.ToolCall) bool { return c.Name == name && c.Err == nil })
}

// argsMatch reports whether the call's arguments contain want. Numbers
// compare by value, and a JSON-string-encoded argument object is unwrapped
// the same way the curator does.
func argsMatch(raw json.RawMessage, want map[string]any) bool {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		raw = json.RawMessage(s)
	}
	var got map[string]any
	if json.Unmarshal(raw, &got) != nil {
		return false
	}
	for k, w := range want {
		g, ok := got[k]
		if !ok || !sameValue(g, w) {
			return false
		}
	}
	return true
}

func sameValue(got, want any) bool {
	gf, gok := number(got)
	wf, wok := number(want)
	if gok && wok {
		return gf == wf
	}
	if gs, ok := got.(string); ok {
		if ws, ok := want.(string); ok {
			return strings.EqualFold(strings.TrimSpace(gs), ws)
		}
	}
	return reflect.DeepEqual(got, want)
}

func number(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int64:
		return float64(n), true
	case int:
		return float64(n), true
	}
	return 0, false
}

// stringArgs reports whether a model sent its arguments as a JSON-encoded
// string instead of an object (which the curator repairs silently).
func stringArgs(raw json.RawMessage) bool {
	var s string
	return json.Unmarshal(raw, &s) == nil
}

func countSentences(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n := len(sentenceRE.FindAllStringIndex(s, -1))
	// Count trailing text without closing punctuation as a sentence.
	if !strings.ContainsAny(s[len(s)-1:], ".!?") {
		n++
	}
	return n
}

func countQuestions(s string) int {
	return len(regexp.MustCompile(`\?+`).FindAllString(s, -1))
}

func callNames(calls []curator.ToolCall) string {
	if len(calls) == 0 {
		return "none"
	}
	names := make([]string, len(calls))
	for i, c := range calls {
		names[i] = c.Name
		if c.Err != nil {
			names[i] += " (error: " + c.Err.Error() + ")"
		}
	}
	return strings.Join(names, ", ")
}

func callArgs(calls []curator.ToolCall, name string) string {
	var out []string
	for _, c := range calls {
		if c.Name == name {
			out = append(out, string(c.Args))
		}
	}
	if len(out) == 0 {
		return "none"
	}
	return strings.Join(out, "; ")
}

// excerpt returns the text around re's first match in s.
func excerpt(s string, re *regexp.Regexp) string {
	loc := re.FindStringIndex(s)
	if loc == nil {
		return ""
	}
	start, end := max(0, loc[0]-40), min(len(s), loc[1]+40)
	for start > 0 && !utf8Start(s[start]) {
		start--
	}
	for end < len(s) && !utf8Start(s[end]) {
		end++
	}
	return strings.TrimSpace(strings.ReplaceAll("…"+s[start:end]+"…", "\n", " "))
}

func utf8Start(b byte) bool { return b&0xC0 != 0x80 }

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
