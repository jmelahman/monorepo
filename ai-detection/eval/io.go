package eval

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
)

// Observations is an append-only CSV of scored examples. Writing a row flushes
// it, so an interrupted run keeps everything scored before the interruption
// and a rerun can resume.
type Observations struct {
	file   *os.File
	writer *csv.Writer
}

var observationHeader = []string{"id", "label", "source", "score", "log_ppl", "log_x_ppl", "tokens", "words"}

// CreateObservations truncates path and writes the header.
func CreateObservations(path string) (*Observations, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	w := csv.NewWriter(f)
	if err := w.Write(observationHeader); err != nil {
		return nil, errors.Join(err, f.Close())
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, errors.Join(err, f.Close())
	}
	return &Observations{file: f, writer: w}, nil
}

// AppendObservations opens path for appending, creating it with a header if it
// is new or empty.
func AppendObservations(path string) (*Observations, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		return nil, errors.Join(err, f.Close())
	}
	w := csv.NewWriter(f)
	if fi.Size() == 0 {
		if err := w.Write(observationHeader); err != nil {
			return nil, errors.Join(err, f.Close())
		}
		w.Flush()
		if err := w.Error(); err != nil {
			return nil, errors.Join(err, f.Close())
		}
	}
	return &Observations{file: f, writer: w}, nil
}

// Write appends one observation and flushes it.
func (o *Observations) Write(observation Observation) error {
	label := "human"
	if observation.Machine {
		label = "machine"
	}
	record := []string{
		strconv.Itoa(observation.ID),
		label,
		observation.Source,
		strconv.FormatFloat(observation.Score, 'g', -1, 64),
		strconv.FormatFloat(observation.LogPPL, 'g', -1, 64),
		strconv.FormatFloat(observation.LogXPPL, 'g', -1, 64),
		strconv.Itoa(observation.Tokens),
		strconv.Itoa(observation.Words),
	}
	if err := o.writer.Write(record); err != nil {
		return err
	}
	o.writer.Flush()
	return o.writer.Error()
}

// Close closes the file.
func (o *Observations) Close() error {
	o.writer.Flush()
	return errors.Join(o.writer.Error(), o.file.Close())
}

// ReadObservations reads a file written by [Observations]. The header may be
// absent, as it is in hand-made score files.
func ReadObservations(path string) ([]Observation, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return ParseObservations(f)
}

// ParseObservations reads observations from r.
func ParseObservations(r io.Reader) ([]Observation, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	records, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return []Observation{}, nil
	}
	start := 0
	if len(records[0]) > 0 && records[0][0] == "id" {
		start = 1
	}

	observations := make([]Observation, 0, len(records)-start)
	for line := start; line < len(records); line++ {
		o, err := parseObservation(records[line])
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line+1, err)
		}
		observations = append(observations, o)
	}
	return observations, nil
}

func parseObservation(record []string) (Observation, error) {
	if len(record) < 8 {
		return Observation{}, fmt.Errorf("%d columns, want 8", len(record))
	}
	var o Observation
	var err error
	if o.ID, err = strconv.Atoi(record[0]); err != nil {
		return Observation{}, fmt.Errorf("id: %w", err)
	}
	switch record[1] {
	case "machine":
		o.Machine = true
	case "human":
		o.Machine = false
	default:
		return Observation{}, fmt.Errorf("label %q, want human or machine", record[1])
	}
	o.Source = record[2]
	floats := []struct {
		name  string
		value *float64
	}{
		{"score", &o.Score},
		{"log_ppl", &o.LogPPL},
		{"log_x_ppl", &o.LogXPPL},
	}
	for i, field := range floats {
		*field.value, err = strconv.ParseFloat(record[3+i], 64)
		if err != nil {
			return Observation{}, fmt.Errorf("%s: %w", field.name, err)
		}
		if math.IsNaN(*field.value) || math.IsInf(*field.value, 0) {
			return Observation{}, fmt.Errorf("%s is %v", field.name, *field.value)
		}
	}
	if o.Tokens, err = strconv.Atoi(record[6]); err != nil {
		return Observation{}, fmt.Errorf("tokens: %w", err)
	}
	if o.Words, err = strconv.Atoi(record[7]); err != nil {
		return Observation{}, fmt.Errorf("words: %w", err)
	}
	return o, nil
}
