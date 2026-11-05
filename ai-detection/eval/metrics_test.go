package eval_test

import (
	"errors"
	"math"
	"testing"

	"github.com/jmelahman/local-ai-detection/eval"
)

// observations pairs scores with labels: scores below 0.5 are machine.
func observations(humans, machines []float64) []eval.Observation {
	all := make([]eval.Observation, 0, len(humans)+len(machines))
	for i, score := range humans {
		all = append(all, eval.Observation{ID: i, Score: score})
	}
	for i, score := range machines {
		all = append(all, eval.Observation{ID: len(humans) + i, Machine: true, Score: score})
	}
	return all
}

func TestAUC(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		humans   []float64
		machines []float64
		want     float64
	}{
		{name: "perfect separation", humans: []float64{0.9, 0.95}, machines: []float64{0.8, 0.85}, want: 1},
		{name: "reversed", humans: []float64{0.1}, machines: []float64{0.9}, want: 0},
		{name: "all tied", humans: []float64{0.5}, machines: []float64{0.5}, want: 0.5},
		{name: "mixed", humans: []float64{0.4, 0.8}, machines: []float64{0.3, 0.6}, want: 0.75},
		{
			// Machine 0.2 beats both humans (two); machine 0.5 ties one
			// (half) and loses to the other. Two and a half of four.
			name:     "some ties",
			humans:   []float64{0.4, 0.5},
			machines: []float64{0.2, 0.5},
			want:     0.625,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := eval.AUC(observations(tt.humans, tt.machines))
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(got-tt.want) > 1e-12 {
				t.Errorf("AUC() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAUCNeedsBothClasses(t *testing.T) {
	t.Parallel()

	if _, err := eval.AUC(nil); !errors.Is(err, eval.ErrNoHumans) {
		t.Errorf("AUC(empty) = %v, want ErrNoHumans", err)
	}
	if _, err := eval.AUC([]eval.Observation{{Score: 0.5}}); !errors.Is(err, eval.ErrNoMachines) {
		t.Errorf("AUC(only humans) = %v, want ErrNoMachines", err)
	}
}

func TestEvaluateAt(t *testing.T) {
	t.Parallel()

	// Two machines, both below threshold; two humans, one below. A machine
	// prediction is score < threshold.
	all := observations([]float64{0.2, 0.8}, []float64{0.1, 0.3})
	m := eval.EvaluateAt(all, 0.5)
	if m.TP != 2 || m.FP != 1 || m.TN != 1 || m.FN != 0 {
		t.Errorf("confusion = %+v, want TP 2, FP 1, TN 1, FN 0", m.Confusion)
	}
	if math.Abs(m.Recall-1) > 1e-12 {
		t.Errorf("recall = %v, want 1", m.Recall)
	}
	if math.Abs(m.FPR-0.5) > 1e-12 {
		t.Errorf("FPR = %v, want 0.5", m.FPR)
	}
	if math.Abs(m.Accuracy-0.75) > 1e-12 {
		t.Errorf("accuracy = %v, want 0.75", m.Accuracy)
	}
	if math.Abs(m.Precision-2.0/3) > 1e-12 {
		t.Errorf("precision = %v, want 2/3", m.Precision)
	}
}

func TestThresholdAtFPR(t *testing.T) {
	t.Parallel()

	humans := []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0}
	all := observations(humans, []float64{0.05})

	tests := []struct {
		target   float64
		want     float64
		wantRate float64
	}{
		{target: 0.05, want: 0.1, wantRate: 0},
		{target: 0.2, want: 0.3, wantRate: 0.2},
		{target: 0.5, want: 0.6, wantRate: 0.5},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			t.Parallel()

			threshold, err := eval.ThresholdAtFPR(all, tt.target)
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(threshold-tt.want) > 1e-12 {
				t.Errorf("ThresholdAtFPR(%v) = %v, want %v", tt.target, threshold, tt.want)
			}
			got := eval.EvaluateAt(all, threshold)
			if got.FPR > tt.target+1e-12 {
				t.Errorf("FPR at the chosen threshold = %v, want at most %v", got.FPR, tt.target)
			}
			if math.Abs(got.FPR-tt.wantRate) > 1e-12 {
				t.Errorf("FPR at the chosen threshold = %v, want %v", got.FPR, tt.wantRate)
			}
		})
	}

	if _, err := eval.ThresholdAtFPR(all, 0); !errors.Is(err, eval.ErrBadTarget) {
		t.Errorf("ThresholdAtFPR(0) = %v, want ErrBadTarget", err)
	}
	if _, err := eval.ThresholdAtFPR(observations(nil, []float64{0.1}), 0.1); !errors.Is(err, eval.ErrNoHumans) {
		t.Errorf("ThresholdAtFPR(no humans) = %v, want ErrNoHumans", err)
	}
}

func TestCrossFitSeparatesCleanly(t *testing.T) {
	t.Parallel()

	humans := make([]float64, 50)
	machines := make([]float64, 50)
	for i := range humans {
		humans[i] = 0.7 + float64(i)*0.001
		machines[i] = 0.3 + float64(i)*0.001
	}
	all := observations(humans, machines)

	predictions, err := eval.CrossFit(all, 5, 0.05, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(predictions) != len(all) {
		t.Fatalf("CrossFit returned %d predictions, want %d", len(predictions), len(all))
	}
	m := eval.EvaluatePredictions(all, predictions, 0)
	// The threshold admits about as many false positives as the target FPR,
	// but every machine should be below it.
	if m.TP != 50 || m.FN != 0 {
		t.Errorf("confusion = %+v, want all 50 machines found", m.Confusion)
	}
	if m.FPR > 0.15 {
		t.Errorf("FPR = %v, want about the 5%% target", m.FPR)
	}
	if m.Accuracy < 0.9 {
		t.Errorf("accuracy = %v, want at least 0.9", m.Accuracy)
	}
	folds := make(map[int]bool)
	for _, prediction := range predictions {
		folds[prediction.Fold] = true
	}
	if len(folds) != 5 {
		t.Errorf("predictions cover %d folds, want 5", len(folds))
	}
}

func TestCrossFitRejectsTooFewObservations(t *testing.T) {
	t.Parallel()

	if _, err := eval.CrossFit(observations(nil, nil), 5, 0.05, 1); !errors.Is(err, eval.ErrNoHumans) {
		t.Errorf("CrossFit(empty) = %v, want ErrNoHumans", err)
	}
	if _, err := eval.CrossFit(observations([]float64{0.9}, []float64{0.1}), 2, 0.05, 1); err == nil {
		t.Error("CrossFit with one human per fold = nil error, want one")
	}
}

func TestBootstrapAUC(t *testing.T) {
	t.Parallel()

	all := observations([]float64{0.9, 0.95}, []float64{0.1, 0.2})
	lo, hi, err := eval.BootstrapAUC(all, 50, 1)
	if err != nil {
		t.Fatal(err)
	}
	if lo != 1 || hi != 1 {
		t.Errorf("BootstrapAUC on perfect separation = [%v, %v], want [1, 1]", lo, hi)
	}
}

func TestWilson(t *testing.T) {
	t.Parallel()

	lo, hi := eval.Wilson(50, 100)
	if math.Abs(lo-0.4038) > 1e-3 || math.Abs(hi-0.5962) > 1e-3 {
		t.Errorf("Wilson(50, 100) = [%v, %v], want about [0.404, 0.596]", lo, hi)
	}
	if lo, hi := eval.Wilson(0, 0); !math.IsNaN(lo) || !math.IsNaN(hi) {
		t.Errorf("Wilson(0, 0) = [%v, %v], want NaNs", lo, hi)
	}
}
