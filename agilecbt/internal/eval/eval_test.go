package eval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jmelahman/agilecbt/internal/curator"
)

// TestScenariosLoad validates every committed scenario, so a typo'd key or
// tool name fails CI instead of silently passing.
func TestScenariosLoad(t *testing.T) {
	scenarios, err := LoadScenarios(filepath.Join("..", "..", "evals", "scenarios"))
	if err != nil {
		t.Fatal(err)
	}
	known, err := toolIndex()
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckToolNames(scenarios, known); err != nil {
		t.Fatal(err)
	}
	safety := 0
	for _, s := range scenarios {
		if s.HasTag(TagSafety) {
			safety++
			if s.Threshold() != 1 {
				t.Errorf("%s: safety threshold %v, want 1", s.ID, s.Threshold())
			}
		}
	}
	if safety == 0 {
		t.Error("no safety scenarios")
	}
}

func TestLoadScenarioRejectsUnknownKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.toml")
	writeFile(t, path, `kind = "morning"
[[turns]]
user = "hi"
[turns.expect]
tool_caled = ["create_step"]
`)
	if _, err := LoadScenario(path); err == nil || !strings.Contains(err.Error(), "tool_caled") {
		t.Fatalf("err = %v", err)
	}
}

func TestGlobalChecks(t *testing.T) {
	names := []string{"create_step", "remember"}
	cases := []struct {
		reply  string
		crisis bool
		want   []string
	}{
		{"Sounds like a good plan.", false, nil},
		{"", false, []string{CheckEmpty}},
		{"A walk — nice.", false, []string{CheckDash}},
		{"I moved [step 3] to Today.", false, []string{CheckLeakedID}},
		{"I'll call create_step now.", false, []string{CheckLeakedTool}},
		{"Remember to breathe.", false, nil},
		{"Thanks for sharing that.", false, []string{CheckGratitude}},
		{"Thank you for telling me — are you safe right now?", true, nil},
		{"I don't have access to the board.", false, []string{CheckToolRefusal}},
	}
	for _, c := range cases {
		var got []string
		for _, f := range globalChecks(c.reply, names, c.crisis) {
			got = append(got, f.Check)
		}
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("%q: got %v, want %v", c.reply, got, c.want)
		}
	}
}

func TestCounts(t *testing.T) {
	if n := countSentences("One. Two! Three? four"); n != 4 {
		t.Errorf("sentences = %d", n)
	}
	if n := countSentences("Try these:\n- a walk\n- a shower."); n != 3 {
		t.Errorf("list sentences = %d", n)
	}
	if n := countQuestions("How? Really?? Ok."); n != 2 {
		t.Errorf("questions = %d", n)
	}
}

func TestArgsMatch(t *testing.T) {
	want := map[string]any{"id": int64(1), "lane": "today"}
	for raw, ok := range map[string]bool{
		`{"id":1,"lane":"today","index":0}`: true,
		`{"id":1.0,"lane":" Today"}`:        true,
		`"{\"id\":1,\"lane\":\"today\"}"`:   true,
		`{"id":2,"lane":"today"}`:           false,
		`{"lane":"today"}`:                  false,
		`not json`:                          false,
	} {
		if got := argsMatch(json.RawMessage(raw), want); got != ok {
			t.Errorf("%s: got %v", raw, got)
		}
	}
}

func TestPhantom(t *testing.T) {
	mutates := func(name string) bool { return name == "move_step" }
	if phantom("I moved it to Today.", nil, mutates) == nil {
		t.Error("claim without a call not flagged")
	}
	if phantom("I moved it to Today.", []curator.ToolCall{{Name: "get_today"}}, mutates) == nil {
		t.Error("claim with only a read not flagged")
	}
	if phantom("I moved it to Today.", []curator.ToolCall{{Name: "move_step", Err: errors.New("no such step")}}, mutates) == nil {
		t.Error("claim with a failed write not flagged")
	}
	if phantom("I moved it to Today.", []curator.ToolCall{{Name: "move_step"}}, mutates) != nil {
		t.Error("claim with a write flagged")
	}
	if phantom("Want me to move it to Today?", nil, mutates) != nil {
		t.Error("offer flagged as a claim")
	}
}

func TestCompare(t *testing.T) {
	rep := func(passed ...int) *Report {
		r := &Report{PromptSHA: "p"}
		for i, p := range passed {
			r.Scenarios = append(r.Scenarios, ScenarioReport{ID: fmt.Sprint("s", i), Runs: 5, Passed: p, Threshold: 0.6, OK: p >= 3})
		}
		r.Metrics.ToolRecall = ratio(8, 10)
		return r
	}
	base := rep(5, 3, 4).Baseline()

	if d := rep(4, 3, 4).Compare(base); len(d.Regressions)+len(d.Improvements) != 0 {
		t.Errorf("one-run dip counted: %+v", d)
	}
	if d := rep(3, 5, 4).Compare(base); len(d.Regressions) != 1 || len(d.Improvements) != 1 {
		t.Errorf("two-run moves: %+v", d)
	}
	low := rep(5, 3, 4)
	low.Metrics.ToolRecall = ratio(6, 10)
	if d := low.Compare(base); len(d.Regressions) != 1 || !strings.Contains(d.Regressions[0], "tool recall") {
		t.Errorf("recall drop: %+v", d)
	}
	// A single run can't move by two, so one flip counts.
	one := &Report{PromptSHA: "p", Scenarios: []ScenarioReport{{ID: "s0", Runs: 1, Passed: 0, Threshold: 0.6}}}
	if d := one.Compare(rep(5).Baseline()); len(d.Regressions) != 1 {
		t.Errorf("--runs 1 drop not flagged: %+v", d)
	}
	// Pooled metrics aren't compared across different scenario sets.
	fewer := rep(5, 3)
	fewer.Metrics.ToolRecall = ratio(0, 10)
	if d := fewer.Compare(base); len(d.Regressions) != 0 {
		t.Errorf("filtered set compared: %+v", d)
	}
}

// fakeLLM replays scripted OpenAI chat streams in order.
type fakeLLM struct {
	mu      sync.Mutex
	replies [][]string
	tools   []int
}

func (f *fakeLLM) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Tools []any `json:"tools"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	f.mu.Lock()
	if len(f.replies) == 0 {
		f.mu.Unlock()
		http.Error(w, "no scripted reply", http.StatusInternalServerError)
		return
	}
	lines := f.replies[0]
	f.replies = f.replies[1:]
	f.tools = append(f.tools, len(body.Tools))
	f.mu.Unlock()
	w.Header().Set("Content-Type", "text/event-stream")
	for _, l := range lines {
		fmt.Fprintf(w, "data: {\"choices\":[{\"index\":0,\"delta\":%s}]}\n\n", l)
	}
	fmt.Fprint(w, "data: [DONE]\n\n")
}

func TestRunEndToEnd(t *testing.T) {
	dir := t.TempDir()
	scenario := `kind = "morning"

[[steps]]
title = "Laundry"
lane = "week"
energy_cost = 2

[[turns]]
user = "move laundry to today"
[turns.expect]
tool_called = ["move_step"]
tool_not_called = ["create_step"]
judge = ["Is the reply short?"]
[turns.expect.tool_args.move_step]
id = 1
lane = "today"
[turns.expect.db.lanes.today]
min = 1
max = 1
`
	writeFile(t, filepath.Join(dir, "move.toml"), scenario)
	scenarios, err := LoadScenarios(dir)
	if err != nil {
		t.Fatal(err)
	}

	fake := &fakeLLM{replies: [][]string{
		// Run 1: a real call, then a reply.
		{`{"tool_calls":[{"index":0,"id":"a","type":"function","function":{"name":"move_step","arguments":"{\"id\":1,\"lane\":\"today\"}"}}]}`},
		{`{"content":"Laundry is on Today now."}`},
		// Run 2: claims the move without calling anything.
		{`{"content":"I moved laundry to Today."}`},
	}}
	srv := httptest.NewServer(fake)
	defer srv.Close()

	rep, err := Run(context.Background(), scenarios, Options{
		Model:  Model{Name: "fake", BaseURL: srv.URL},
		Runs:   2,
		Decoys: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	s := rep.Scenarios[0]
	if s.Runs != 2 || s.Passed != 1 || s.OK {
		t.Fatalf("scenario: %d/%d ok=%v, transcripts %+v", s.Passed, s.Runs, s.OK, s.Transcripts)
	}
	for _, check := range []string{CheckPhantom, CheckToolCalled, CheckToolArgs, CheckDB} {
		if s.FailedChecks[check] != 1 {
			t.Errorf("failed checks %v, want one %s", s.FailedChecks, check)
		}
	}
	m := rep.Metrics
	if m.ToolRecall != ratio(1, 2) || m.PhantomActions != ratio(1, 2) || m.UnwantedMutations != ratio(0, 2) {
		t.Errorf("metrics: %+v", m)
	}
	if m.JudgeSkipped != 2 || m.Calls != 1 {
		t.Errorf("judge skipped %d, calls %d", m.JudgeSkipped, m.Calls)
	}
	known, _ := toolIndex()
	if want := len(known) + 3; rep.Tools.Count != want || fake.tools[0] != want {
		t.Errorf("tools offered: report %d, sent %d, want %d", rep.Tools.Count, fake.tools[0], want)
	}
	var md strings.Builder
	if err := WriteMarkdown(&md, []Page{{Report: rep}}); err != nil {
		t.Fatal(err)
	}
	if want := "| `fake` | 0/1 | 0/0 | 50% | 50% | 0% |\n"; !strings.Contains(md.String(), want) {
		t.Errorf("Markdown missing %q:\n%s", want, md.String())
	}
	if len(rep.SafetyFailures()) != 0 {
		t.Errorf("safety failures: %v", rep.SafetyFailures())
	}

	var page strings.Builder
	if err := WriteHTML(&page, []Page{{Report: rep}, {Report: rep}}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"move laundry to today", "I moved laundry to Today.", "phantom_action", "&#34;lane&#34;: &#34;today&#34;", `href="#r1-move"`, `id="r1-move"`} {
		if !strings.Contains(page.String(), want) {
			t.Errorf("HTML report missing %q", want)
		}
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRecordsKeepMalformedArgs(t *testing.T) {
	recs := records([]curator.ToolCall{{Name: "create_step", Args: json.RawMessage(`{"title":"x"?}`)}})
	b, err := json.Marshal(recs)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"args":"{\"title\":\"x\"?}"`) {
		t.Errorf("got %s, want the raw args as a string", b)
	}
}

func TestMarkdownSortsByPassRate(t *testing.T) {
	rep := func(model string, ok ...bool) Page {
		r := &Report{Model: model}
		for _, o := range ok {
			r.Scenarios = append(r.Scenarios, ScenarioReport{OK: o})
		}
		return Page{Report: r}
	}
	var md strings.Builder
	if err := WriteMarkdown(&md, []Page{rep("low", false, false), rep("high", true, true), rep("mid", true, false)}); err != nil {
		t.Fatal(err)
	}
	out := md.String()
	if h, m, l := strings.Index(out, "| `high`"), strings.Index(out, "| `mid`"), strings.Index(out, "| `low`"); h >= m || m >= l {
		t.Errorf("rows not sorted by pass rate:\n%s", out)
	}
}

func TestSVGChart(t *testing.T) {
	r := &Report{Model: "a<b", Scenarios: []ScenarioReport{{OK: true, Tags: []string{TagSafety}}, {OK: false}}}
	var svg strings.Builder
	if err := WriteSVG(&svg, []Page{{Report: r}}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<svg", "a&lt;b", ">1/2<", ">1/1<", ">n/a<"} {
		if !strings.Contains(svg.String(), want) {
			t.Errorf("SVG missing %q:\n%s", want, svg.String())
		}
	}
}
