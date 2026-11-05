// Package eval scores a labelled corpus with the Binoculars detector and
// reports how well it separates human from machine-written text.
//
// The package is deliberately free of model loading: [Example] and
// [Observation] describe the data, and the metrics in metrics.go describe the
// scores. The command in cmd/eval loads the models and does the scoring.
package eval

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"slices"
	"strings"
)

// ErrNoHumans and ErrNoMachines are returned by metrics that need both
// classes.
var (
	ErrNoHumans   = errors.New("no human examples")
	ErrNoMachines = errors.New("no machine examples")
)

// Example is one labelled text. Label is true for machine-generated text.
type Example struct {
	Text    string
	Source  string
	ID      int
	Machine bool
}

// Observation is an [Example] that the detector has scored.
type Observation struct {
	Example
	Score   float64
	LogPPL  float64
	LogXPPL float64
	Tokens  int
	Words   int
}

// Load reads examples from a CSV with a header naming at least text, label and
// src columns, in any order. It is the format of MAGE's test.csv: label 1 is
// human and 0 is machine. The words "human" and "machine" are accepted too.
// IDs are the row's position in the file, starting at zero.
func Load(r io.Reader) ([]Example, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1

	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	col := make(map[string]int, len(header))
	for i, name := range header {
		col[strings.ToLower(strings.TrimPrefix(name, "\ufeff"))] = i
	}
	textCol, ok := col["text"]
	if !ok {
		return nil, errors.New(`CSV has no "text" column`)
	}
	labelCol, ok := col["label"]
	if !ok {
		return nil, errors.New(`CSV has no "label" column`)
	}
	srcCol, ok := col["src"]
	if !ok {
		srcCol, ok = col["source"]
	}
	if !ok {
		return nil, errors.New(`CSV has no "src" or "source" column`)
	}

	var examples []Example
	for line := 2; ; line++ {
		record, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		recordID := len(examples)
		if len(record) <= max(textCol, max(labelCol, srcCol)) {
			return nil, fmt.Errorf("line %d: %d columns, want at least %d", line, len(record), max(textCol, max(labelCol, srcCol))+1)
		}
		machine, err := parseLabel(record[labelCol])
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		examples = append(examples, Example{
			Text:    record[textCol],
			Source:  record[srcCol],
			ID:      recordID,
			Machine: machine,
		})
	}
	return examples, nil
}

// parseLabel maps a CSV label to true for machine text. MAGE numbers human 1
// and machine 0, the reverse of the usual convention.
func parseLabel(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "human":
		return false, nil
	case "0", "machine":
		return true, nil
	default:
		return false, fmt.Errorf("unknown label %q, want 1/human or 0/machine", s)
	}
}

// Sample returns n examples chosen uniformly without replacement, in ID order.
// It returns a copy of every example when n is zero or at least len(examples).
// The choice depends only on seed, so a run can be repeated or split into
// shards.
func Sample(examples []Example, n int, seed uint64) []Example {
	if n <= 0 || n >= len(examples) {
		return slices.Clone(examples)
	}
	order := make([]int, len(examples))
	for i := range order {
		order[i] = i
	}
	r := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	r.Shuffle(len(order), func(i, j int) {
		order[i], order[j] = order[j], order[i]
	})
	picked := make([]Example, 0, n)
	for _, i := range order[:n] {
		picked = append(picked, examples[i])
	}
	slices.SortFunc(picked, func(a, b Example) int { return a.ID - b.ID })
	return picked
}

// Shard keeps the examples whose ID falls in shard of count, so several
// processes can score one sample in parallel. Count below one keeps all.
func Shard(examples []Example, shard, count int) []Example {
	if count <= 1 {
		return examples
	}
	kept := make([]Example, 0, len(examples)/count+1)
	for _, e := range examples {
		if e.ID%count == shard {
			kept = append(kept, e)
		}
	}
	return kept
}

// TruncateWords keeps the first n whitespace-separated words of text. It
// returns text unchanged when n is zero or negative.
func TruncateWords(text string, n int) string {
	if n <= 0 {
		return text
	}
	fields := strings.Fields(text)
	if len(fields) <= n {
		return text
	}
	return strings.Join(fields[:n], " ")
}
