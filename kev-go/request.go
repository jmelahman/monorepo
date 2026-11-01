package kev

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

const maxOptions = 255

// object is a JSON object that keeps its keys' order: the model reads a choice's
// options, and an object state's fields, in the order the request gives them.
type object struct {
	keys   []string
	values map[string]any
}

// decodeValue reads the next JSON value: string, json.Number, bool, nil, []any
// or *object.
func decodeValue(d *json.Decoder) (any, error) {
	t, err := d.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return t, nil
	}
	if delim == '[' {
		list := []any{}
		for d.More() {
			v, err := decodeValue(d)
			if err != nil {
				return nil, err
			}
			list = append(list, v)
		}
		_, err := d.Token()
		return list, err
	}
	o := &object{values: map[string]any{}}
	for d.More() {
		k, err := d.Token()
		if err != nil {
			return nil, err
		}
		key, ok := k.(string)
		if !ok {
			return nil, fmt.Errorf("object key %v isn't a string", k)
		}
		v, err := decodeValue(d)
		if err != nil {
			return nil, err
		}
		if _, dup := o.values[key]; !dup {
			o.keys = append(o.keys, key)
		}
		o.values[key] = v
	}
	_, err = d.Token()
	return o, err
}

// render flattens a value into the text the model sees, keeping field names as
// labels. It matches kev/api.py's, including Python's spelling of scalars.
func render(v any, indent int) string {
	pad := strings.Repeat("  ", indent)
	switch v := v.(type) {
	case nil:
		return ""
	case string:
		return v
	case bool:
		if v {
			return "True"
		}
		return "False"
	case json.Number:
		return pythonNumber(v)
	case []any:
		lines := make([]string, len(v))
		for i, x := range v {
			lines[i] = pad + "- " + strings.TrimLeftFunc(render(x, indent+1), pythonSpace)
		}
		return strings.Join(lines, "\n")
	case *object:
		lines := make([]string, len(v.keys))
		for i, k := range v.keys {
			x := v.values[k]
			switch x.(type) {
			case []any, *object:
				lines[i] = pad + k + ":\n" + render(x, indent+1)
			default:
				lines[i] = pad + k + ": " + render(x, 0)
			}
		}
		return strings.Join(lines, "\n")
	}
	panic(fmt.Sprintf("kev: unexpected JSON value %T", v))
}

func pythonSpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}

// pythonNumber spells a JSON number as Python's str does.
func pythonNumber(n json.Number) string {
	s := n.String()
	if !strings.ContainsAny(s, ".eE") {
		if s == "-0" {
			return "0"
		}
		return s
	}
	f, err := n.Float64()
	if err != nil {
		if math.IsInf(f, 1) {
			return "inf"
		}
		if math.IsInf(f, -1) {
			return "-inf"
		}
		return s
	}
	if f == 0 {
		return strconv.FormatFloat(f, 'f', 1, 64)
	}
	if a := math.Abs(f); a >= 1e16 || a < 1e-4 {
		return strconv.FormatFloat(f, 'e', -1, 64)
	}
	out := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.Contains(out, ".") {
		out += ".0"
	}
	return out
}

func optionText(name string, desc any) string {
	if desc == nil || desc == "" {
		return name
	}
	return name + ": " + render(desc, 0)
}

type questionKind string

const (
	kindNoul   questionKind = "noul"
	kindChoice questionKind = "choice"
	kindScore  questionKind = "score"
)

// parsed is a question as the request gave it.
type parsed struct {
	id   string
	kind questionKind
	q    Question
	// keys names the options in answers: choice labels, or noul's false and true.
	keys []string
}

type request struct {
	model     string
	state     string
	questions []parsed
}

// A RequestError means a request is malformed.
type RequestError struct{ Message string }

func (e *RequestError) Error() string { return e.Message }

func parseRequest(body []byte) (*request, error) {
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	top, err := decodeValue(d)
	if err != nil {
		return nil, &RequestError{"body isn't JSON: " + err.Error()}
	}
	if _, err := d.Token(); !errors.Is(err, io.EOF) {
		return nil, &RequestError{"body has more than one JSON value"}
	}
	root, ok := top.(*object)
	if !ok {
		return nil, &RequestError{"body must be an object"}
	}
	if _, ok := root.values["state"]; !ok {
		return nil, &RequestError{"state is required"}
	}
	req := &request{state: render(root.values["state"], 0), model: "kev-latest"}
	if m, ok := root.values["model"].(string); ok {
		req.model = m
	}
	qs, ok := root.values["questions"].(*object)
	if !ok || len(qs.keys) == 0 {
		return nil, &RequestError{"questions must be an object with at least one question"}
	}
	for _, id := range qs.keys {
		q, ok := qs.values[id].(*object)
		if !ok {
			return nil, &RequestError{"question " + id + " must be an object"}
		}
		p, err := parseQuestion(id, q)
		if err != nil {
			return nil, err
		}
		req.questions = append(req.questions, p)
	}
	return req, nil
}

func parseQuestion(id string, q *object) (parsed, error) {
	kind, _ := q.values["type"].(string)
	p := parsed{id: id, kind: questionKind(kind)}
	p.q.Instructions = render(q.values["instructions"], 0)
	bad := func(msg string) (parsed, error) { return p, &RequestError{"question " + id + ": " + msg} }
	switch p.kind {
	case kindNoul:
		var no, yes any
		switch c := q.values["criteria"].(type) {
		case nil:
		case *object:
			no, yes = c.values["false"], c.values["true"]
		default:
			return bad("criteria must be an object")
		}
		p.q.Options = []string{optionText("no", no), optionText("yes", yes)}
		p.keys = []string{"false", "true"}
	case kindChoice:
		c, ok := q.values["criteria"].(*object)
		if !ok {
			return bad("criteria must be an object")
		}
		if len(c.keys) < 1 || len(c.keys) > maxOptions {
			return bad(fmt.Sprintf("criteria must have 1..%d options", maxOptions))
		}
		for _, k := range c.keys {
			p.q.Options = append(p.q.Options, optionText(k, c.values[k]))
			p.keys = append(p.keys, k)
		}
	case kindScore:
		c, ok := q.values["criteria"].([]any)
		if !ok || len(c) < 1 || len(c) > maxOptions {
			return bad(fmt.Sprintf("criteria must be an array of 1..%d levels", maxOptions))
		}
		for i, level := range c {
			p.q.Options = append(p.q.Options, render(level, 0))
			p.keys = append(p.keys, strconv.Itoa(i))
		}
	default:
		return bad(fmt.Sprintf("unknown type %q", kind))
	}
	return p, nil
}

// roundProb gives probabilities the four decimals Kev's server reports, which
// keep a rounded distribution's sum within TypeSafe's tolerance at 255 options.
func roundProb(x float64) float64 {
	out, err := strconv.ParseFloat(strconv.FormatFloat(x, 'f', 4, 64), 64)
	if err != nil {
		return x
	}
	return out
}

func normalized(p []float64) []float64 {
	var total float64
	for _, x := range p {
		total += x
	}
	out := make([]float64, len(p))
	for i, x := range p {
		if total == 0 {
			out[i] = 1 / float64(len(p))
		} else {
			out[i] = x / total
		}
	}
	return out
}

// choiceConfidence is 0 at a uniform distribution and 1 at certainty.
func choiceConfidence(p []float64) float64 {
	k := float64(len(p))
	if len(p) == 1 {
		return 1
	}
	return (slices.Max(normalized(p)) - 1/k) / (1 - 1/k)
}

// scoreConfidence is 1 when all mass is on one level, and 0 when it's as spread
// around the likeliest level as a uniform distribution is around its mean.
func scoreConfidence(p []float64) float64 {
	levels := len(p)
	if levels == 1 {
		return 1
	}
	p = normalized(p)
	mode := firstMax(p)
	var d, spread float64
	for i, x := range p {
		d += math.Abs(float64(i) - float64(levels-1)/2)
		spread += x * math.Abs(float64(i-mode))
	}
	return max(0, 1-spread/(d/float64(levels)))
}

func firstMax(p []float64) int {
	best := 0
	for i, x := range p {
		if x > p[best] {
			best = i
		}
	}
	return best
}

// answer is the JSON form of one question's result.
func (q parsed) answer(p []float64) map[string]any {
	probs := map[string]float64{}
	for i, k := range q.keys {
		probs[k] = roundProb(p[i])
	}
	switch q.kind {
	case kindNoul:
		return map[string]any{"type": "noul", "noul": roundProb(p[1])}
	case kindChoice:
		return map[string]any{"type": "choice", "choice": q.keys[firstMax(p)], "confidence": roundProb(choiceConfidence(p)), "probabilities": probs}
	case kindScore:
	}
	var score float64
	legend := map[string]string{}
	for i, x := range p {
		score += float64(i) * x
		legend[q.keys[i]] = q.q.Options[i]
	}
	return map[string]any{"type": "score", "score": roundProb(score), "legend": legend, "probabilities": probs, "confidence": roundProb(scoreConfidence(p))}
}
