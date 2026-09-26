package eval

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/jmelahman/agilecbt/internal/curator"
	"github.com/jmelahman/agilecbt/internal/prompt"
)

// Report is the outcome of one benchmark run of one model.
type Report struct {
	Model     string        `json:"model"`
	BaseURL   string        `json:"base_url"`
	StartedAt time.Time     `json:"started_at"`
	Duration  time.Duration `json:"duration_ns"`
	// PromptSHA and RetroPromptSHA identify the prompts benchmarked.
	PromptSHA      string   `json:"prompt_sha256"`
	RetroPromptSHA string   `json:"retro_prompt_sha256"`
	Judge          string   `json:"judge,omitempty"`
	ToolsFilter    []string `json:"tools_filter,omitempty"`
	// Tools is the tool payload offered on every turn.
	Tools     ToolPayload      `json:"tools"`
	Metrics   Metrics          `json:"metrics"`
	Scenarios []ScenarioReport `json:"scenarios"`
}

// ScenarioReport is one scenario's results across runs.
type ScenarioReport struct {
	ID          string   `json:"id"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags"`
	Threshold   float64  `json:"threshold"`
	Runs        int      `json:"runs"`
	Passed      int      `json:"passed"`
	// OK means the pass rate met the threshold.
	OK bool `json:"ok"`
	// FailedChecks counts, per check, the runs that failed it.
	FailedChecks map[string]int `json:"failed_checks,omitempty"`
	AvgLatency   time.Duration  `json:"avg_latency_ns"`
	Transcripts  []RunReport    `json:"transcripts"`
}

// RunReport is one run's turns, for reading failures after the fact.
type RunReport struct {
	Passed bool         `json:"passed"`
	Error  string       `json:"error,omitempty"`
	Turns  []TurnResult `json:"turns"`
}

// PassRate is Passed / Runs.
func (s ScenarioReport) PassRate() float64 {
	if s.Runs == 0 {
		return 0
	}
	return float64(s.Passed) / float64(s.Runs)
}

// Ratio is N out of Of.
type Ratio struct {
	N    int     `json:"n"`
	Of   int     `json:"of"`
	Rate float64 `json:"rate"`
}

func ratio(n, of int) Ratio {
	r := Ratio{N: n, Of: of}
	if of > 0 {
		r.Rate = float64(n) / float64(of)
	}
	return r
}

func (r Ratio) String() string {
	if r.Of == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.0f%% (%d/%d)", r.Rate*100, r.N, r.Of)
}

// Metrics pool tool use across every run.
type Metrics struct {
	// ToolRecall is the share of turns that needed tool calls and got all of
	// them, successfully.
	ToolRecall Ratio `json:"tool_recall"`
	// UnwantedMutations is the share of turns that forbid some calls where
	// the model made a forbidden write anyway.
	UnwantedMutations Ratio `json:"unwanted_mutations"`
	// PhantomActions is the share of replies that claimed a change without
	// a successful write behind it.
	PhantomActions Ratio `json:"phantom_actions"`
	Calls          int   `json:"calls"`
	UnknownTool    int   `json:"unknown_tool_calls"`
	ToolErrors     int   `json:"tool_errors"`
	StringArgs     int   `json:"string_encoded_args"`
	RoundLimit     int   `json:"round_limit_hits"`
	DecoyCalls     int   `json:"decoy_calls"`
	JudgeErrors    int   `json:"judge_errors"`
	JudgeSkipped   int   `json:"judge_skipped"`
}

func newReport(opts Options, scenarios []Scenario, results [][]runResult, took time.Duration) *Report {
	p := opts.Prompt
	if p == "" {
		p = prompt.Template()
	}
	rp := opts.RetroPrompt
	if rp == "" {
		rp = curator.RetroSystem
	}
	rep := &Report{
		Model:          opts.Model.Name,
		BaseURL:        opts.Model.BaseURL,
		StartedAt:      time.Now().Add(-took).UTC(),
		Duration:       took,
		PromptSHA:      sha(p),
		RetroPromptSHA: sha(rp),
		ToolsFilter:    opts.Tools,
	}
	if opts.Judge != nil {
		rep.Judge = opts.Judge.Backend.Model
	}
	var c metricCounts
	for si, s := range scenarios {
		sr := ScenarioReport{ID: s.ID, Description: s.Description, Tags: s.Tags, Threshold: s.Threshold(), Runs: len(results[si]), FailedChecks: map[string]int{}}
		var total time.Duration
		for _, r := range results[si] {
			if r.passed() {
				sr.Passed++
			}
			for _, name := range r.failedChecks() {
				sr.FailedChecks[name]++
			}
			total += r.Latency
			sr.Transcripts = append(sr.Transcripts, RunReport{Passed: r.passed(), Error: r.Err, Turns: r.Turns})
			if r.ToolPayload.Count > 0 {
				rep.Tools = r.ToolPayload
			}
			addCounts(&c, r.counts)
		}
		if sr.Runs > 0 {
			sr.AvgLatency = total / time.Duration(sr.Runs)
		}
		sr.OK = sr.PassRate() >= sr.Threshold-1e-9
		rep.Scenarios = append(rep.Scenarios, sr)
	}
	rep.Metrics = Metrics{
		ToolRecall:        ratio(c.GotCall, c.NeedCall),
		UnwantedMutations: ratio(c.Unwanted, c.Constrained),
		PhantomActions:    ratio(c.Phantom, c.Replies),
		Calls:             c.Calls,
		UnknownTool:       c.UnknownTool,
		ToolErrors:        c.ToolErrors,
		StringArgs:        c.StringArgs,
		RoundLimit:        c.RoundLimit,
		DecoyCalls:        c.DecoyCalls,
		JudgeErrors:       c.JudgeErrors,
		JudgeSkipped:      c.JudgeSkipped,
	}
	return rep
}

func addCounts(dst *metricCounts, c metricCounts) {
	dst.NeedCall += c.NeedCall
	dst.GotCall += c.GotCall
	dst.Constrained += c.Constrained
	dst.Unwanted += c.Unwanted
	dst.Replies += c.Replies
	dst.Phantom += c.Phantom
	dst.Calls += c.Calls
	dst.UnknownTool += c.UnknownTool
	dst.ToolErrors += c.ToolErrors
	dst.StringArgs += c.StringArgs
	dst.RoundLimit += c.RoundLimit
	dst.DecoyCalls += c.DecoyCalls
	dst.JudgeErrors += c.JudgeErrors
	dst.JudgeSkipped += c.JudgeSkipped
}

func sha(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// SafetyFailures lists safety scenarios below their threshold.
func (r *Report) SafetyFailures() []string {
	var out []string
	for _, s := range r.Scenarios {
		if !s.OK && slices.Contains(s.Tags, TagSafety) {
			out = append(out, s.ID)
		}
	}
	return out
}

// Print writes a summary table.
func (r *Report) Print(w io.Writer) {
	judge := "off"
	if r.Judge != "" {
		judge = r.Judge
	}
	fmt.Fprintf(w, "\n%s at %s: %d tools (%d decoys, ~%d tokens), judge %s, prompt %s, %s\n",
		r.Model, r.BaseURL, r.Tools.Count, r.Tools.Decoys, r.Tools.Tokens, judge, r.PromptSHA[:12], r.Duration.Round(time.Second))
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SCENARIO\tPASS\tNEED\t\tFAILED CHECKS")
	passed := 0
	for _, s := range r.Scenarios {
		mark := "ok"
		if !s.OK {
			mark = "FAIL"
		} else {
			passed++
		}
		fmt.Fprintf(tw, "%s\t%d/%d\t%.0f%%\t%s\t%s\n", s.ID, s.Passed, s.Runs, s.Threshold*100, mark, formatChecks(s.FailedChecks))
	}
	tw.Flush()
	m := r.Metrics
	fmt.Fprintf(w, "\n%d/%d scenarios met their threshold.\n", passed, len(r.Scenarios))
	fmt.Fprintf(w, "Tool recall %s · unwanted writes %s · phantom actions %s\n", m.ToolRecall, m.UnwantedMutations, m.PhantomActions)
	fmt.Fprintf(w, "Calls %d: %d unknown tool, %d tool errors, %d string-encoded args, %d decoy, %d round-limit hits\n",
		m.Calls, m.UnknownTool, m.ToolErrors, m.StringArgs, m.DecoyCalls, m.RoundLimit)
	if m.JudgeSkipped > 0 {
		fmt.Fprintf(w, "%d judge questions skipped (no --judge-model).\n", m.JudgeSkipped)
	}
	if m.JudgeErrors > 0 {
		fmt.Fprintf(w, "%d judge calls failed; those turns count as failed.\n", m.JudgeErrors)
	}
}

func formatChecks(m map[string]int) string {
	keys := slices.Sorted(maps.Keys(m))
	sort.SliceStable(keys, func(i, j int) bool { return m[keys[i]] > m[keys[j]] })
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s×%d", k, m[k])
	}
	return strings.Join(parts, " ")
}

// PrintSweep writes one row per report, for comparing tool-set sizes.
func PrintSweep(w io.Writer, reps []*Report) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "MODEL\tTOOLS\tDECOYS\t~TOKENS\tRECALL\tUNWANTED\tPHANTOM\tDECOY CALLS\tSCENARIOS OK")
	for _, r := range reps {
		ok := 0
		for _, s := range r.Scenarios {
			if s.OK {
				ok++
			}
		}
		m := r.Metrics
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%s\t%s\t%s\t%d\t%d/%d\n", r.Model, r.Tools.Count, r.Tools.Decoys, r.Tools.Tokens,
			m.ToolRecall, m.UnwantedMutations, m.PhantomActions, m.DecoyCalls, ok, len(r.Scenarios))
	}
	tw.Flush()
}

// Save writes the full report as JSON under dir/<model>/.
func (r *Report) Save(dir string) (string, error) {
	sub := filepath.Join(dir, Slug(r.Model))
	if err := os.MkdirAll(sub, 0o755); err != nil {
		return "", err
	}
	name := r.StartedAt.Format("20060102-150405")
	if r.Tools.Decoys > 0 {
		name += fmt.Sprintf("-decoys%d", r.Tools.Decoys)
	}
	path := filepath.Join(sub, name+".json")
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, append(b, '\n'), 0o644)
}

var slugRE = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// Slug makes a model id safe for a file name.
func Slug(model string) string {
	return strings.Trim(slugRE.ReplaceAllString(model, "-"), "-")
}

// Baseline is the committed summary of a report that later runs diff
// against.
type Baseline struct {
	Model          string                      `json:"model"`
	PromptSHA      string                      `json:"prompt_sha256"`
	RetroPromptSHA string                      `json:"retro_prompt_sha256"`
	Judge          string                      `json:"judge,omitempty"`
	Tools          ToolPayload                 `json:"tools"`
	Metrics        Metrics                     `json:"metrics"`
	Scenarios      map[string]BaselineScenario `json:"scenarios"`
}

// BaselineScenario is one scenario's pass count.
type BaselineScenario struct {
	Runs   int `json:"runs"`
	Passed int `json:"passed"`
}

// Baseline summarizes the report.
func (r *Report) Baseline() Baseline {
	b := Baseline{Model: r.Model, PromptSHA: r.PromptSHA, RetroPromptSHA: r.RetroPromptSHA, Judge: r.Judge,
		Tools: r.Tools, Metrics: r.Metrics, Scenarios: map[string]BaselineScenario{}}
	for _, s := range r.Scenarios {
		b.Scenarios[s.ID] = BaselineScenario{Runs: s.Runs, Passed: s.Passed}
	}
	return b
}

// BaselinePath is where a model's baseline lives in dir.
func BaselinePath(dir, model string) string {
	return filepath.Join(dir, Slug(model)+".json")
}

// LoadBaseline reads a baseline file.
func LoadBaseline(path string) (Baseline, error) {
	var b Baseline
	raw, err := os.ReadFile(path)
	if err != nil {
		return b, err
	}
	return b, json.Unmarshal(raw, &b)
}

// WriteBaseline saves the report's baseline to path.
func (r *Report) WriteBaseline(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(r.Baseline(), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// Diff is how a report compares to a baseline.
type Diff struct {
	Regressions  []string
	Improvements []string
	Notes        []string
}

// noise is how many runs a rate must move by to count: a one-run flip is
// normal sampling noise at small N.
const noise = 2

// Compare diffs the report against a baseline. A scenario regresses when
// its pass rate drops by at least two runs' worth; pooled tool metrics
// regress when they move the wrong way by at least two turns' worth.
func (r *Report) Compare(b Baseline) Diff {
	var d Diff
	if b.PromptSHA != r.PromptSHA {
		d.Notes = append(d.Notes, "the system prompt changed since the baseline")
	}
	if b.RetroPromptSHA != r.RetroPromptSHA {
		d.Notes = append(d.Notes, "the retro prompt changed since the baseline")
	}
	if b.Tools.Count != r.Tools.Count || b.Tools.Bytes != r.Tools.Bytes {
		d.Notes = append(d.Notes, fmt.Sprintf("tool payload: %d tools, ~%d tokens (baseline %d tools, ~%d tokens)",
			r.Tools.Count, r.Tools.Tokens, b.Tools.Count, b.Tools.Tokens))
	}
	if (b.Judge == "") != (r.Judge == "") {
		d.Notes = append(d.Notes, fmt.Sprintf("judge %q vs baseline %q: rubric scenarios aren't comparable", r.Judge, b.Judge))
	}

	same := len(b.Scenarios) == len(r.Scenarios)
	for _, s := range r.Scenarios {
		base, ok := b.Scenarios[s.ID]
		if !ok {
			d.Notes = append(d.Notes, s.ID+": new, not in the baseline")
			same = false
			continue
		}
		baseRate := float64(base.Passed) / float64(max(base.Runs, 1))
		drop := (baseRate - s.PassRate()) * float64(s.Runs)
		// With fewer runs than the noise margin, one flip is all a run can
		// show, so it counts.
		margin := float64(min(noise, max(s.Runs, 1)))
		switch {
		case drop >= margin-1e-9:
			d.Regressions = append(d.Regressions, fmt.Sprintf("%s: %d/%d, baseline %d/%d", s.ID, s.Passed, s.Runs, base.Passed, base.Runs))
		case drop <= -margin+1e-9:
			d.Improvements = append(d.Improvements, fmt.Sprintf("%s: %d/%d, baseline %d/%d", s.ID, s.Passed, s.Runs, base.Passed, base.Runs))
		case drop > 0 && !s.OK && baseRate >= s.Threshold:
			d.Notes = append(d.Notes, fmt.Sprintf("%s: dipped below its threshold (%d/%d, baseline %d/%d); likely noise, rerun with more --runs", s.ID, s.Passed, s.Runs, base.Passed, base.Runs))
		}
	}
	if !same {
		d.Notes = append(d.Notes, "the scenario set differs from the baseline, so pooled tool metrics weren't compared")
		return d
	}
	metric := func(name string, cur, base Ratio, higherIsBetter bool) {
		if cur.Of == 0 || base.Of == 0 {
			return
		}
		worse := (base.Rate - cur.Rate) * float64(cur.Of)
		if !higherIsBetter {
			worse = -worse
		}
		line := fmt.Sprintf("%s: %s, baseline %s", name, cur, base)
		switch {
		case worse >= noise-1e-9:
			d.Regressions = append(d.Regressions, line)
		case worse <= -noise+1e-9:
			d.Improvements = append(d.Improvements, line)
		}
	}
	metric("tool recall", r.Metrics.ToolRecall, b.Metrics.ToolRecall, true)
	metric("unwanted writes", r.Metrics.UnwantedMutations, b.Metrics.UnwantedMutations, false)
	metric("phantom actions", r.Metrics.PhantomActions, b.Metrics.PhantomActions, false)
	return d
}

// Print writes the diff.
func (d Diff) Print(w io.Writer, baselinePath string) {
	fmt.Fprintf(w, "\nCompared with %s:\n", baselinePath)
	if len(d.Regressions)+len(d.Improvements)+len(d.Notes) == 0 {
		fmt.Fprintln(w, "  no changes beyond noise")
	}
	for _, s := range d.Regressions {
		fmt.Fprintln(w, "  REGRESSION  "+s)
	}
	for _, s := range d.Improvements {
		fmt.Fprintln(w, "  improved    "+s)
	}
	for _, s := range d.Notes {
		fmt.Fprintln(w, "  note        "+s)
	}
}
