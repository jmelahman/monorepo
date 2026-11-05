package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/jmelahman/local-ai-detection/binoculars"
	"github.com/jmelahman/local-ai-detection/eval"
)

func runReport(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		scores    = fs.String("scores", "", "comma-separated score CSVs")
		name      = fs.String("name", "", "benchmark name for the report")
		folds     = fs.Int("folds", 5, "cross-fitting folds for the threshold")
		targetFPR = fs.Float64("fpr", 0.05, "false-positive rate the threshold is calibrated to")
		paper     = fs.Float64("threshold", binoculars.Threshold, "a fixed threshold to also report")
		reps      = fs.Int("bootstrap", 1000, "bootstrap replicates for the AUC interval")
		jsonPath  = fs.String("json", "", "write the metrics as JSON here")
		minGroup  = fs.Int("min-group", 10, "hide text rows for source groups smaller than this")
	)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *scores == "" {
		return errors.New("report: --scores is required")
	}

	observations, err := loadScores(*scores)
	if err != nil {
		return err
	}
	if len(observations) == 0 {
		return errors.New("report: no observations")
	}
	auc, err := eval.AUC(observations)
	if err != nil {
		return err
	}
	aucLo, aucHi, err := eval.BootstrapAUC(observations, *reps, 1)
	if err != nil {
		return err
	}
	fixed := eval.EvaluateAt(observations, *paper)

	predictions, err := eval.CrossFit(observations, *folds, *targetFPR, 1)
	if err != nil {
		return err
	}
	thresholds := make([]float64, *folds)
	for _, prediction := range predictions {
		thresholds[prediction.Fold] = prediction.Threshold
	}
	meanThreshold := 0.0
	for _, threshold := range thresholds {
		meanThreshold += threshold
	}
	meanThreshold /= float64(len(thresholds))
	crossFitted := eval.EvaluatePredictions(observations, predictions, meanThreshold)

	domains := groupBy(observations, predictions, func(o eval.Observation) string {
		return parseMAGESource(o.Source).domain
	})
	models := groupBy(observations, predictions, func(o eval.Observation) string {
		if !o.Machine {
			return ""
		}
		return parseMAGESource(o.Source).model
	})
	strategies := groupBy(observations, predictions, func(o eval.Observation) string {
		if !o.Machine {
			return ""
		}
		return parseMAGESource(o.Source).strategy
	})
	humanSources := groupBy(observations, predictions, func(o eval.Observation) string {
		if o.Machine {
			return ""
		}
		return parseMAGESource(o.Source).domain
	})

	var humans, machines, words, tokens int
	for _, o := range observations {
		if o.Machine {
			machines++
		} else {
			humans++
		}
		words += o.Words
		tokens += o.Tokens
	}

	report := reportJSON{
		Name:              *name,
		N:                 len(observations),
		Humans:            humans,
		Machines:          machines,
		Words:             words,
		Tokens:            tokens,
		AUC:               auc,
		AUCCI:             [2]float64{aucLo, aucHi},
		FixedThreshold:    thresholdJSONFor(fixed),
		CrossFit:          crossFitJSON{thresholdJSON: thresholdJSONFor(crossFitted), Folds: *folds, TargetFPR: *targetFPR, Thresholds: thresholds},
		Domains:           toGroupJSON(domains),
		MachineModels:     toGroupJSON(models),
		MachineStrategies: toGroupJSON(strategies),
		HumanSources:      toGroupJSON(humanSources),
	}
	writeText(stdout, report, *scores, *minGroup)
	if *jsonPath != "" {
		encoded, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(*jsonPath, append(encoded, '\n'), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func loadScores(paths string) ([]eval.Observation, error) {
	var observations []eval.Observation
	seen := make(map[int]bool)
	for path := range strings.SplitSeq(paths, ",") {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		scored, err := eval.ReadObservations(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		for _, o := range scored {
			if seen[o.ID] {
				continue
			}
			seen[o.ID] = true
			observations = append(observations, o)
		}
	}
	slices.SortFunc(observations, func(a, b eval.Observation) int { return a.ID - b.ID })
	return observations, nil
}

// sourceParts splits MAGE's src column:
//
//	wp_human
//	wpxsum_machine_continuation_gpt-3.5-trubo
//
// into a domain, a generation strategy and a model. Unknown shapes come back
// with only Domain set, to the raw source, so they still group together.
type sourceParts struct {
	domain   string
	strategy string
	model    string
}

func parseMAGESource(source string) sourceParts {
	if domain, ok := strings.CutSuffix(source, "_human"); ok {
		return sourceParts{domain: domain}
	}
	domain, rest, ok := strings.Cut(source, "_machine_")
	if !ok {
		return sourceParts{domain: source}
	}
	strategy, model, ok := strings.Cut(rest, "_")
	if !ok {
		return sourceParts{domain: domain, strategy: rest}
	}
	return sourceParts{domain: domain, strategy: strategy, model: model}
}

// group is the confusion count of a source group under cross-fitted
// predictions.
type group struct {
	name    string
	human   int
	machine int
	fp      int
	tp      int
}

func (g group) tpr() (float64, bool) {
	if g.machine == 0 {
		return 0, false
	}
	return float64(g.tp) / float64(g.machine), true
}

func (g group) fpr() (float64, bool) {
	if g.human == 0 {
		return 0, false
	}
	return float64(g.fp) / float64(g.human), true
}

func groupBy(observations []eval.Observation, predictions []eval.Prediction, key func(eval.Observation) string) []group {
	byName := make(map[string]*group)
	for i, o := range observations {
		name := key(o)
		if name == "" {
			continue
		}
		g, ok := byName[name]
		if !ok {
			g = &group{name: name}
			byName[name] = g
		}
		predicted := i < len(predictions) && predictions[i].Machine
		if o.Machine {
			g.machine++
			if predicted {
				g.tp++
			}
		} else {
			g.human++
			if predicted {
				g.fp++
			}
		}
	}
	groups := make([]group, 0, len(byName))
	for _, g := range byName {
		groups = append(groups, *g)
	}
	slices.SortFunc(groups, func(a, b group) int { return strings.Compare(a.name, b.name) })
	return groups
}

type reportJSON struct {
	Name              string        `json:"name,omitempty"`
	N                 int           `json:"n"`
	Humans            int           `json:"humans"`
	Machines          int           `json:"machines"`
	Words             int           `json:"words"`
	Tokens            int           `json:"tokens"`
	AUC               float64       `json:"auc"`
	AUCCI             [2]float64    `json:"auc_95"`
	FixedThreshold    thresholdJSON `json:"fixed_threshold"`
	CrossFit          crossFitJSON  `json:"cross_fit"`
	Domains           []groupJSON   `json:"domains"`
	MachineModels     []groupJSON   `json:"machine_models"`
	MachineStrategies []groupJSON   `json:"machine_strategies"`
	HumanSources      []groupJSON   `json:"human_sources"`
}

type thresholdJSON struct {
	eval.ThresholdMetrics
	RecallCI [2]float64 `json:"recall_95"`
	FPRCI    [2]float64 `json:"fpr_95"`
}

func thresholdJSONFor(m eval.ThresholdMetrics) thresholdJSON {
	recallLo, recallHi := eval.Wilson(m.TP, m.TP+m.FN)
	fprLo, fprHi := eval.Wilson(m.FP, m.FP+m.TN)
	return thresholdJSON{ThresholdMetrics: m, RecallCI: [2]float64{recallLo, recallHi}, FPRCI: [2]float64{fprLo, fprHi}}
}

type crossFitJSON struct {
	thresholdJSON
	Folds      int       `json:"folds"`
	TargetFPR  float64   `json:"target_fpr"`
	Thresholds []float64 `json:"fold_thresholds"`
}

type groupJSON struct {
	Name    string   `json:"name"`
	Human   int      `json:"human"`
	Machine int      `json:"machine"`
	TP      int      `json:"tp"`
	FP      int      `json:"fp"`
	TPR     *float64 `json:"tpr,omitempty"`
	FPR     *float64 `json:"fpr,omitempty"`
}

func toGroupJSON(groups []group) []groupJSON {
	out := make([]groupJSON, 0, len(groups))
	for _, g := range groups {
		row := groupJSON{Name: g.name, Human: g.human, Machine: g.machine, TP: g.tp, FP: g.fp}
		if rate, ok := g.tpr(); ok {
			row.TPR = &rate
		}
		if rate, ok := g.fpr(); ok {
			row.FPR = &rate
		}
		out = append(out, row)
	}
	return out
}

func writeText(w io.Writer, r reportJSON, scores string, minGroup int) {
	if r.Name != "" {
		fmt.Fprintf(w, "Benchmark: %s\n", r.Name)
	}
	fmt.Fprintf(w, "Scores:    %s\n", scores)
	fmt.Fprintf(w, "Examples:  %d (%d human, %d machine), %d words, %d tokens\n\n",
		r.N, r.Humans, r.Machines, r.Words, r.Tokens)

	fmt.Fprintln(w, "Threshold-free")
	fmt.Fprintf(w, "  %-24s %6.3f  [95%% CI %.3f, %.3f]\n\n", "ROC-AUC", r.AUC, r.AUCCI[0], r.AUCCI[1])

	writeThreshold(w, fmt.Sprintf("At the fixed threshold score < %.4f", r.FixedThreshold.Threshold), r.FixedThreshold)
	fmt.Fprintln(w)
	writeThreshold(w, fmt.Sprintf("At the cross-fitted threshold score < %.4f (%d folds, target FPR %.1f%%)",
		r.CrossFit.Threshold, r.CrossFit.Folds, 100*r.CrossFit.TargetFPR), r.CrossFit.thresholdJSON)
	if len(r.CrossFit.Thresholds) > 1 {
		fmt.Fprintf(w, "  %-24s %s\n", "fold thresholds", formatFloats(r.CrossFit.Thresholds))
	}
	fmt.Fprintln(w)

	writeGroups(w, "By domain (cross-fitted predictions)", r.Domains, minGroup)
	fmt.Fprintln(w)
	writeGroups(w, "By machine model (cross-fitted predictions)", r.MachineModels, minGroup)
	fmt.Fprintln(w)
	writeGroups(w, "By machine generation strategy (cross-fitted predictions)", r.MachineStrategies, minGroup)
	fmt.Fprintln(w)
	writeGroups(w, "By human source (cross-fitted predictions)", r.HumanSources, minGroup)
}

func writeThreshold(w io.Writer, title string, m thresholdJSON) {
	fmt.Fprintln(w, title)
	fmt.Fprintf(w, "  %-24s %6.1f%%  [95%% CI %.1f%%, %.1f%%]\n", "recall (TPR)", 100*m.Recall, 100*m.RecallCI[0], 100*m.RecallCI[1])
	fmt.Fprintf(w, "  %-24s %6.1f%%  [95%% CI %.1f%%, %.1f%%]\n", "false positive rate", 100*m.FPR, 100*m.FPRCI[0], 100*m.FPRCI[1])
	fmt.Fprintf(w, "  %-24s %6.1f%%\n", "accuracy", 100*m.Accuracy)
	fmt.Fprintf(w, "  %-24s %6.1f%%\n", "precision", 100*m.Precision)
	fmt.Fprintf(w, "  %-24s %6.3f\n", "F1", m.F1)
	fmt.Fprintf(w, "  %-24s TP %d  FP %d  TN %d  FN %d\n", "confusion", m.TP, m.FP, m.TN, m.FN)
}

func writeGroups(w io.Writer, title string, groups []groupJSON, minSize int) {
	fmt.Fprintln(w, title)
	if len(groups) == 0 {
		fmt.Fprintln(w, "  (none)")
		return
	}
	hidden := 0
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(tw, "group\tN\thuman\tmachine\tTPR\tFPR\t")
	for _, g := range groups {
		if g.Human+g.Machine < minSize {
			hidden++
			continue
		}
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%s\t%s\t\n",
			g.Name, g.Human+g.Machine, g.Human, g.Machine, formatRate(g.TPR), formatRate(g.FPR))
	}
	_ = tw.Flush()
	if hidden > 0 {
		fmt.Fprintf(w, "  (%d groups smaller than %d hidden; see --json)\n", hidden, minSize)
	}
}

func formatRate(rate *float64) string {
	if rate == nil {
		return "-"
	}
	return fmt.Sprintf("%.1f%%", 100*(*rate))
}

func formatFloats(values []float64) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, fmt.Sprintf("%.4f", value))
	}
	return strings.Join(parts, ", ")
}
