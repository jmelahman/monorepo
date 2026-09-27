package eval

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/jmelahman/agilecbt/internal/db"
	"github.com/jmelahman/agilecbt/internal/safety"
)

// ClassifierCase is one labeled message for the crisis classifier benchmark.
type ClassifierCase struct {
	Text    string          `toml:"text"`
	Want    safety.Category `toml:"want"`
	History []SeedMessage   `toml:"history"`
	// Note says why the case is there, such as "idiom" or "passive".
	Note string `toml:"note"`
}

// LoadClassifierCases reads a [[cases]] file. Unknown keys and categories
// are errors.
func LoadClassifierCases(path string) ([]ClassifierCase, error) {
	var file struct {
		Cases []ClassifierCase `toml:"cases"`
	}
	md, err := toml.DecodeFile(path, &file)
	if err != nil {
		return nil, err
	}
	if extra := md.Undecoded(); len(extra) > 0 {
		return nil, fmt.Errorf("%s: unknown keys %v", path, extra)
	}
	for i, c := range file.Cases {
		if strings.TrimSpace(c.Text) == "" {
			return nil, fmt.Errorf("%s: case %d has no text", path, i+1)
		}
		if c.Want != safety.None && !slices.Contains(safety.Categories, c.Want) {
			return nil, fmt.Errorf("%s: case %d: unknown category %q", path, i+1, c.Want)
		}
	}
	if len(file.Cases) == 0 {
		return nil, fmt.Errorf("%s: no [[cases]]", path)
	}
	return file.Cases, nil
}

// ClassifierVerdict is one case's outcome.
type ClassifierVerdict struct {
	Case    ClassifierCase
	Lexicon safety.Result
	// Model is the model tier's verdict; zero when there's no model or it
	// failed (see Err).
	Model   safety.Result
	Err     error
	Latency time.Duration
}

// Combined is what the app does: the lexicon, else the model.
func (v ClassifierVerdict) Combined() safety.Result {
	if v.Lexicon.Flagged() {
		return v.Lexicon
	}
	if v.Err != nil || v.Model.Category == "" {
		return safety.Result{Category: safety.None}
	}
	return v.Model
}

// ClassifierReport is a classifier benchmark for one model.
type ClassifierReport struct {
	// Model is empty for the lexicon alone.
	Model    string
	Verdicts []ClassifierVerdict
}

// RunClassifier classifies every case with the lexicon and, when llm is
// non-nil, the model tier on its own, so each tier's contribution shows.
func RunClassifier(ctx context.Context, cases []ClassifierCase, model string, llm safety.Completer, parallel int) (*ClassifierReport, error) {
	if parallel < 1 {
		parallel = 1
	}
	rep := &ClassifierReport{Model: model, Verdicts: make([]ClassifierVerdict, len(cases))}
	sem := make(chan struct{}, parallel)
	var wg sync.WaitGroup
	for i, c := range cases {
		v := ClassifierVerdict{Case: c, Lexicon: safety.Match(c.Text)}
		rep.Verdicts[i] = v
		if llm == nil {
			continue
		}
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer func() { <-sem; wg.Done() }()
			var history []db.Message
			for _, h := range c.History {
				history = append(history, db.Message{Role: h.Role, Text: h.Text})
			}
			start := time.Now()
			// Call the model directly: Classify would skip it on a lexicon
			// hit, hiding how the model does alone.
			cctx, cancel := context.WithTimeout(ctx, safety.DefaultTimeout)
			out, err := llm.Complete(cctx, safety.SystemPrompt, safety.Input(history, c.Text))
			cancel()
			v.Latency = time.Since(start)
			if err == nil {
				v.Model, err = safety.Parse(out)
			}
			v.Err = err
			rep.Verdicts[i] = v
		}()
	}
	wg.Wait()
	return rep, ctx.Err()
}

// classifierScore tallies one tier.
type classifierScore struct {
	caught, positives map[safety.Category]int
	falsePos, negs    int
	wrongCategory     int
	missed            []ClassifierVerdict
	flagged           []ClassifierVerdict
}

func score(vs []ClassifierVerdict, pick func(ClassifierVerdict) safety.Result) classifierScore {
	s := classifierScore{caught: map[safety.Category]int{}, positives: map[safety.Category]int{}}
	for _, v := range vs {
		got := pick(v)
		if v.Case.Want == safety.None {
			s.negs++
			if got.Flagged() {
				s.falsePos++
				s.flagged = append(s.flagged, v)
			}
			continue
		}
		s.positives[v.Case.Want]++
		switch {
		case !got.Flagged():
			s.missed = append(s.missed, v)
		case got.Category != v.Case.Want:
			s.wrongCategory++
			s.caught[v.Case.Want]++
		default:
			s.caught[v.Case.Want]++
		}
	}
	return s
}

// Missed returns the crisis cases the app (lexicon plus model) let through.
func (r *ClassifierReport) Missed() []ClassifierVerdict {
	return score(r.Verdicts, ClassifierVerdict.Combined).missed
}

// Print writes one row per tier, then the app's misses and false alarms.
func (r *ClassifierReport) Print(w io.Writer) {
	name := r.Model
	if name == "" {
		name = "(lexicon only)"
	}
	fmt.Fprintf(w, "\nCrisis classifier: %s, %d cases\n", name, len(r.Verdicts))
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprint(tw, "TIER\tRECALL")
	for _, c := range safety.Categories {
		fmt.Fprintf(tw, "\t%s", strings.ToUpper(string(c)))
	}
	fmt.Fprintln(tw, "\tFALSE ALARMS\tWRONG CATEGORY\tERRORS\tP50\tP95")
	row := func(tier string, s classifierScore, errs int, lat string) {
		caught, total := 0, 0
		for _, c := range safety.Categories {
			caught += s.caught[c]
			total += s.positives[c]
		}
		fmt.Fprintf(tw, "%s\t%s", tier, frac(caught, total))
		for _, c := range safety.Categories {
			fmt.Fprintf(tw, "\t%s", frac(s.caught[c], s.positives[c]))
		}
		fmt.Fprintf(tw, "\t%s\t%d\t%d\t%s\n", frac(s.falsePos, s.negs), s.wrongCategory, errs, lat)
	}
	row("lexicon", score(r.Verdicts, func(v ClassifierVerdict) safety.Result { return v.Lexicon }), 0, "-\t-")
	combined := score(r.Verdicts, ClassifierVerdict.Combined)
	if r.Model != "" {
		errs := 0
		var lat []time.Duration
		for _, v := range r.Verdicts {
			if v.Err != nil {
				errs++
			}
			lat = append(lat, v.Latency)
		}
		slices.Sort(lat)
		p := func(q float64) string { return lat[int(q*float64(len(lat)-1))].Round(10 * time.Millisecond).String() }
		row("model", score(r.Verdicts, func(v ClassifierVerdict) safety.Result {
			if v.Err != nil {
				return safety.Result{Category: safety.None}
			}
			return v.Model
		}), errs, p(0.5)+"\t"+p(0.95))
		row("lexicon+model", combined, errs, "-\t-")
	}
	tw.Flush()

	for _, v := range combined.missed {
		fmt.Fprintf(w, "  missed %s: %q%s\n", v.Case.Want, v.Case.Text, errNote(v))
	}
	for _, v := range combined.flagged {
		fmt.Fprintf(w, "  false alarm (%s): %q\n", v.Combined().Category, v.Case.Text)
	}
}

func errNote(v ClassifierVerdict) string {
	if v.Err != nil {
		return fmt.Sprintf(" (model error: %.80s)", v.Err.Error())
	}
	return ""
}

func frac(n, d int) string {
	if d == 0 {
		return "-"
	}
	return fmt.Sprintf("%d/%d", n, d)
}

// DefaultClassifierCases is where the committed cases live.
const DefaultClassifierCases = "evals/safety/cases.toml"
