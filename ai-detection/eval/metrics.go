package eval

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
)

// ErrBadTarget is returned for a target false-positive rate outside (0, 1].
var ErrBadTarget = errors.New("target false-positive rate must be in (0, 1]")

// Confusion counts predictions at one threshold. A machine prediction is a
// score below the threshold.
type Confusion struct {
	TP int `json:"tp"`
	FP int `json:"fp"`
	TN int `json:"tn"`
	FN int `json:"fn"`
}

// Total is the number of predictions.
func (c Confusion) Total() int {
	return c.TP + c.FP + c.TN + c.FN
}

// ThresholdMetrics are the rates implied by a [Confusion].
type ThresholdMetrics struct {
	Threshold   float64 `json:"threshold"`
	Accuracy    float64 `json:"accuracy"`
	Precision   float64 `json:"precision"`
	Recall      float64 `json:"recall"`
	Specificity float64 `json:"specificity"`
	FPR         float64 `json:"fpr"`
	F1          float64 `json:"f1"`
	Confusion
}

// EvaluateAt predicts machine for every score below threshold.
func EvaluateAt(observations []Observation, threshold float64) ThresholdMetrics {
	var c Confusion
	for _, o := range observations {
		predicted := o.Score < threshold
		switch {
		case predicted && o.Machine:
			c.TP++
		case predicted && !o.Machine:
			c.FP++
		case !predicted && o.Machine:
			c.FN++
		default:
			c.TN++
		}
	}
	return metricsFor(c, threshold)
}

func metricsFor(c Confusion, threshold float64) ThresholdMetrics {
	m := ThresholdMetrics{Threshold: threshold, Confusion: c}
	n := c.Total()
	if n == 0 {
		return m
	}
	m.Accuracy = float64(c.TP+c.TN) / float64(n)
	if c.TP+c.FP > 0 {
		m.Precision = float64(c.TP) / float64(c.TP+c.FP)
	}
	if c.TP+c.FN > 0 {
		m.Recall = float64(c.TP) / float64(c.TP+c.FN)
	}
	if c.TN+c.FP > 0 {
		m.Specificity = float64(c.TN) / float64(c.TN+c.FP)
		m.FPR = float64(c.FP) / float64(c.TN+c.FP)
	}
	if m.Precision+m.Recall > 0 {
		m.F1 = 2 * m.Precision * m.Recall / (m.Precision + m.Recall)
	}
	return m
}

// AUC is the probability that a random machine example scores below a random
// human one, with ties counted as half. It does not depend on a threshold.
func AUC(observations []Observation) (float64, error) {
	var humans, machines int
	for _, o := range observations {
		if o.Machine {
			machines++
		} else {
			humans++
		}
	}
	if humans == 0 {
		return 0, ErrNoHumans
	}
	if machines == 0 {
		return 0, ErrNoMachines
	}

	type pair struct {
		score   float64
		machine bool
	}
	pairs := make([]pair, len(observations))
	for i, o := range observations {
		pairs[i] = pair{o.Score, o.Machine}
	}
	slices.SortFunc(pairs, func(a, b pair) int { return cmp.Compare(a.score, b.score) })

	// Rank sum of the machine scores, averaging ranks within ties. With lower
	// scores machine-like, U counts pairs where a machine outranks a human, so
	// the complement is the AUC: P(machine score < human score).
	var rankSum float64
	for i := 0; i < len(pairs); {
		j := i + 1
		for j < len(pairs) && pairs[j].score == pairs[i].score {
			j++
		}
		meanRank := (float64(i+1) + float64(j)) / 2
		for _, p := range pairs[i:j] {
			if p.machine {
				rankSum += meanRank
			}
		}
		i = j
	}
	greater := (rankSum - float64(machines*(machines+1))/2) / float64(machines*humans)
	return 1 - greater, nil
}

// ThresholdAtFPR is the highest threshold whose false-positive rate is at most
// target: the score of the human example at that quantile. Scores equal to the
// threshold are called human, so ties make the rate lower, never higher.
func ThresholdAtFPR(observations []Observation, target float64) (float64, error) {
	if !(target > 0 && target <= 1) {
		return 0, fmt.Errorf("%w: %v", ErrBadTarget, target)
	}
	var humans []float64
	for _, o := range observations {
		if !o.Machine {
			humans = append(humans, o.Score)
		}
	}
	if len(humans) == 0 {
		return 0, ErrNoHumans
	}
	slices.Sort(humans)
	k := int(target * float64(len(humans)))
	if k >= len(humans) {
		return math.Inf(1), nil
	}
	return humans[k], nil
}

// Prediction is a cross-fitted prediction for one observation: the threshold
// came from a fold that did not contain it.
type Prediction struct {
	Threshold float64
	Fold      int
	Machine   bool
}

// CrossFit predicts every observation with a threshold calibrated on the other
// folds, so no example helps choose the threshold that judges it. Folds are
// stratified by label. The returned predictions are aligned with observations.
func CrossFit(observations []Observation, folds int, targetFPR float64, seed uint64) ([]Prediction, error) {
	if folds < 2 {
		return nil, fmt.Errorf("folds = %d, want at least 2", folds)
	}
	humanIdx, machineIdx := make([]int, 0, len(observations)), make([]int, 0, len(observations))
	for i, o := range observations {
		if o.Machine {
			machineIdx = append(machineIdx, i)
		} else {
			humanIdx = append(humanIdx, i)
		}
	}
	if len(humanIdx) < folds {
		return nil, fmt.Errorf("%w: %d humans for %d folds", ErrNoHumans, len(humanIdx), folds)
	}
	if len(machineIdx) < folds {
		return nil, fmt.Errorf("%w: %d machines for %d folds", ErrNoMachines, len(machineIdx), folds)
	}

	r := rand.New(rand.NewPCG(seed, seed^0xd1b54a32d192ed03))
	r.Shuffle(len(humanIdx), func(i, j int) { humanIdx[i], humanIdx[j] = humanIdx[j], humanIdx[i] })
	r.Shuffle(len(machineIdx), func(i, j int) { machineIdx[i], machineIdx[j] = machineIdx[j], machineIdx[i] })

	foldOf := make([]int, len(observations))
	for i, idx := range humanIdx {
		foldOf[idx] = i % folds
	}
	for i, idx := range machineIdx {
		foldOf[idx] = i % folds
	}

	predictions := make([]Prediction, len(observations))
	for fold := range folds {
		training := make([]Observation, 0, len(observations))
		for i, o := range observations {
			if foldOf[i] != fold {
				training = append(training, o)
			}
		}
		threshold, err := ThresholdAtFPR(training, targetFPR)
		if err != nil {
			return nil, err
		}
		for i, o := range observations {
			if foldOf[i] == fold {
				predictions[i] = Prediction{
					Threshold: threshold,
					Fold:      fold,
					Machine:   o.Score < threshold,
				}
			}
		}
	}
	return predictions, nil
}

// EvaluatePredictions turns explicit predictions into metrics. threshold is
// reported as-is; for cross-fitted predictions it is usually the mean of the
// folds' thresholds.
func EvaluatePredictions(observations []Observation, predictions []Prediction, threshold float64) ThresholdMetrics {
	var c Confusion
	for i, o := range observations {
		predicted := i < len(predictions) && predictions[i].Machine
		switch {
		case predicted && o.Machine:
			c.TP++
		case predicted && !o.Machine:
			c.FP++
		case !predicted && o.Machine:
			c.FN++
		default:
			c.TN++
		}
	}
	return metricsFor(c, threshold)
}

// BootstrapAUC returns a percentile confidence interval for [AUC], resampling
// humans and machines separately so every replicate keeps both classes.
func BootstrapAUC(observations []Observation, replicates int, seed uint64) (lo, hi float64, err error) {
	var humans, machines []Observation
	for _, o := range observations {
		if o.Machine {
			machines = append(machines, o)
		} else {
			humans = append(humans, o)
		}
	}
	if len(humans) == 0 {
		return 0, 0, ErrNoHumans
	}
	if len(machines) == 0 {
		return 0, 0, ErrNoMachines
	}
	if replicates < 1 {
		return 0, 0, errors.New("replicates must be at least 1")
	}

	r := rand.New(rand.NewPCG(seed, seed^0x5851f42d4c957f2d))
	estimate := make([]float64, replicates)
	sample := make([]Observation, 0, len(observations))
	for i := range estimate {
		sample = sample[:0]
		for range humans {
			sample = append(sample, humans[r.IntN(len(humans))])
		}
		for range machines {
			sample = append(sample, machines[r.IntN(len(machines))])
		}
		estimate[i], err = AUC(sample)
		if err != nil {
			return 0, 0, err
		}
	}
	slices.Sort(estimate)
	return percentile(estimate, 0.025), percentile(estimate, 0.975), nil
}

// percentile interpolates linearly between order statistics.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return math.NaN()
	}
	pos := p * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := min(lo+1, len(sorted)-1)
	return sorted[lo] + (pos-float64(lo))*(sorted[hi]-sorted[lo])
}

// Wilson is the 95% Wilson score interval for k successes out of n. It is
// well behaved for the small groups and extreme rates a benchmark produces.
func Wilson(k, n int) (lo, hi float64) {
	if n <= 0 {
		return math.NaN(), math.NaN()
	}
	const z = 1.959963984540054
	p := float64(k) / float64(n)
	denom := 1 + z*z/float64(n)
	centre := p + z*z/(2*float64(n))
	margin := z * math.Sqrt(p*(1-p)/float64(n)+z*z/(4*float64(n)*float64(n)))
	return (centre - margin) / denom, (centre + margin) / denom
}
