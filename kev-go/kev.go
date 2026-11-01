// Package kev runs Kev decision models in pure Go, on the CPU: given a state and
// typed questions it returns calibrated probabilities, the same ones Kev's
// Python code computes. It generates no text.
//
// A checkpoint is a directory holding the adapter (or a full backbone), the
// tokenizer and head.safetensors, which tools/convert_head.py writes from
// head.pt. Everything about the model comes from these files, so a fine-tune
// loads like the original.
package kev

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/jmelahman/kev-go/internal/qwen35"
	"github.com/jmelahman/kev-go/internal/safetensors"
	"github.com/jmelahman/kev-go/internal/tokenizer"
)

// The limits Kev's server applies, in tokens.
const (
	maxState  = 65536
	maxBranch = maxState + 8192
)

// The delimiters around a record's parts: state, question, option start and
// end, and the token whose state the model decides from.
const (
	tokState  = "<|fim_prefix|>"
	tokQ      = "<|fim_middle|>"
	tokOpt    = "<|box_start|>"
	tokEndOpt = "<|box_end|>"
	tokDecide = "<|fim_suffix|>"
)

// Options configures [Open].
type Options struct {
	// Base is the directory of the backbone an adapter was trained on, named by
	// the checkpoint's head.safetensors. A full-weight checkpoint needs none.
	Base string
	// Workers bounds the CPUs used. Zero uses all of them.
	Workers int
	// PrefixCache is how many tokens of recent states to keep ready, so more
	// questions about the same state don't read it again. Each token costs about
	// 25 KB. Zero means 16384, and negative turns it off.
	PrefixCache int
}

// A Model is a loaded Kev checkpoint. It's safe for concurrent use; requests run
// one at a time, each on every worker.
type Model struct {
	backbone    *qwen35.Model
	tok         *tokenizer.Tokenizer
	ids         [5]int32
	qW, kW      []float32
	qB, kB      []float32
	headDim     int
	temperature float64
	turn        chan struct{}

	// cached is guarded by turn.
	cached  []prefixEntry
	maxHeld int
}

type prefixEntry struct {
	key    [sha256.Size]byte
	tokens int
	cache  *qwen35.Cache
}

type headMeta struct {
	HeadDim         int     `json:"head_dim"`
	OptionIsolation bool    `json:"option_isolation"`
	Temperature     float64 `json:"temperature"`
	Weights         *string `json:"weights"`
}

// Open loads the checkpoint in dir.
func Open(dir string, opts Options) (*Model, error) {
	head, err := safetensors.Open(filepath.Join(dir, "head.safetensors"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%s has no head.safetensors: run tools/convert_head.py on it: %w", dir, err)
	}
	if err != nil {
		return nil, err
	}
	defer head.Close()
	var meta headMeta
	for field, dst := range map[string]any{"head_dim": &meta.HeadDim, "option_isolation": &meta.OptionIsolation, "temperature": &meta.Temperature, "weights": &meta.Weights} {
		if raw, ok := head.Metadata()[field]; ok {
			if err := json.Unmarshal([]byte(raw), dst); err != nil {
				return nil, fmt.Errorf("head.safetensors: %s: %w", field, err)
			}
		}
	}
	if meta.OptionIsolation {
		return nil, errors.New("option_isolation checkpoints can't run on a hybrid backbone")
	}
	if meta.HeadDim <= 0 || meta.Temperature <= 0 {
		return nil, fmt.Errorf("head.safetensors: head_dim %d, temperature %g", meta.HeadDim, meta.Temperature)
	}

	base := opts.Base
	var adapter *qwen35.Adapter
	if meta.Weights != nil {
		base = dir
	} else {
		if base == "" {
			return nil, errors.New("this checkpoint is an adapter: Options.Base must name its backbone")
		}
		if adapter, err = qwen35.OpenAdapter(dir); err != nil {
			return nil, err
		}
		defer adapter.Close()
	}
	tok, err := tokenizer.Open(filepath.Join(dir, "tokenizer.json"))
	if err != nil {
		return nil, err
	}
	backbone, err := qwen35.Open(base, adapter, opts.Workers)
	if err != nil {
		return nil, err
	}
	m := &Model{backbone: backbone, tok: tok, headDim: meta.HeadDim, temperature: meta.Temperature, turn: make(chan struct{}, 1), maxHeld: opts.PrefixCache}
	if m.maxHeld == 0 {
		m.maxHeld = 16384
	}
	d := backbone.HiddenSize()
	for name, dst := range map[string]*[]float32{"q.weight": &m.qW, "k.weight": &m.kW, "q.bias": &m.qB, "k.bias": &m.kB} {
		v, err := head.Float32s(name)
		want := meta.HeadDim
		if name == "q.weight" || name == "k.weight" {
			want *= d
		}
		if err == nil && len(v) != want {
			err = fmt.Errorf("head tensor %s has %d values, want %d", name, len(v), want)
		}
		if err != nil {
			backbone.Close()
			return nil, err
		}
		*dst = v
	}
	for i, name := range []string{tokState, tokQ, tokOpt, tokEndOpt, tokDecide} {
		id, ok := tok.ID(name)
		if !ok {
			backbone.Close()
			return nil, fmt.Errorf("tokenizer has no %s", name)
		}
		m.ids[i] = id
	}
	return m, nil
}

// Close releases the model's files.
func (m *Model) Close() { m.backbone.Close() }

var delimiterRE = regexp.MustCompile(`<\|([A-Za-z0-9_]+)\|>`)

// userTokens tokenizes caller-supplied text so it can't forge a delimiter:
// <|name|> is rewritten before the tokenizer can recognize it.
func (m *Model) userTokens(text string) []int32 {
	return m.tok.Encode(delimiterRE.ReplaceAllString(text, "<¦${1}¦>"))
}

// A Question is one decision to make about a state.
type Question struct {
	Instructions string
	Options      []string
}

// An OverflowError means a state or question is more than a model can read.
type OverflowError struct{ Message string }

func (e *OverflowError) Error() string { return e.Message }

type branch struct {
	ids    []int32
	decide int
	opts   []int
}

func (m *Model) encode(state string, questions []Question) ([]int32, []branch, error) {
	s := append([]int32{m.ids[0]}, m.userTokens(state)...)
	if len(s) > maxState {
		return nil, nil, &OverflowError{fmt.Sprintf("state exceeds %d tokens: %d", maxState, len(s))}
	}
	branches := make([]branch, len(questions))
	for i, q := range questions {
		if len(q.Options) == 0 {
			return nil, nil, fmt.Errorf("question %d has no options", i)
		}
		b := &branches[i]
		b.ids = append([]int32{m.ids[1]}, m.userTokens(q.Instructions)...)
		for _, o := range q.Options {
			b.ids = append(b.ids, m.ids[2])
			b.ids = append(b.ids, m.userTokens(o)...)
			b.ids = append(b.ids, m.ids[3])
			b.opts = append(b.opts, len(b.ids)-1)
		}
		b.ids = append(b.ids, m.ids[4])
		b.decide = len(b.ids) - 1
		if len(b.ids) > maxBranch-len(s) {
			return nil, nil, &OverflowError{fmt.Sprintf("branch too long: %d tokens with a %d-token state (row limit %d)", len(b.ids), len(s), maxBranch)}
		}
	}
	return s, branches, nil
}

// Decide returns each question's probabilities over its options, and the number
// of tokens read.
func (m *Model) Decide(ctx context.Context, state string, questions []Question) ([][]float64, int, error) {
	s, branches, err := m.encode(state, questions)
	if err != nil {
		return nil, 0, err
	}
	select {
	case m.turn <- struct{}{}:
		defer func() { <-m.turn }()
	case <-ctx.Done():
		return nil, 0, ctx.Err()
	}
	prefix, err := m.prefix(ctx, s)
	if err != nil {
		return nil, 0, err
	}
	out := make([][]float64, len(branches))
	tokens := len(s)
	for i, b := range branches {
		tokens += len(b.ids)
		hidden, err := m.backbone.Forward(ctx, b.ids, prefix.Clone())
		if err != nil {
			return nil, 0, err
		}
		out[i] = m.readout(hidden, b)
	}
	return out, tokens, nil
}

// prefix returns the cache after state s, which callers must only clone. The
// state's result doesn't depend on what follows it, so one pass serves every
// question.
func (m *Model) prefix(ctx context.Context, s []int32) (*qwen35.Cache, error) {
	raw := make([]byte, 4*len(s))
	for i, id := range s {
		binary.LittleEndian.PutUint32(raw[i*4:], uint32(id))
	}
	key := sha256.Sum256(raw)
	for i, e := range m.cached {
		if e.key == key {
			m.cached = append(slices.Delete(m.cached, i, i+1), e)
			return e.cache, nil
		}
	}

	c := m.backbone.NewCache()
	if _, err := m.backbone.Forward(ctx, s, c); err != nil {
		return nil, err
	}
	if len(s) <= m.maxHeld {
		m.cached = append(m.cached, prefixEntry{key, len(s), c})
		held := 0
		for _, e := range m.cached {
			held += e.tokens
		}
		for held > m.maxHeld {
			held -= m.cached[0].tokens
			m.cached = m.cached[1:]
		}
	}
	return c, nil
}

func (m *Model) readout(hidden []float32, b branch) []float64 {
	d := m.backbone.HiddenSize()
	q := project(m.qW, m.qB, hidden[b.decide*d:(b.decide+1)*d], d)
	logits := make([]float64, len(b.opts))
	scale := 1 / math.Sqrt(float64(m.headDim))
	best := math.Inf(-1)
	for i, at := range b.opts {
		k := project(m.kW, m.kB, hidden[at*d:(at+1)*d], d)
		var z float64
		for j := range q {
			z += float64(q[j]) * float64(k[j])
		}
		logits[i] = z * scale / m.temperature
		best = max(best, logits[i])
	}
	var sum float64
	for i, z := range logits {
		logits[i] = math.Exp(z - best)
		sum += logits[i]
	}
	for i := range logits {
		logits[i] /= sum
	}
	return logits
}

func project(w, bias, x []float32, d int) []float32 {
	out := make([]float32, len(bias))
	for i := range out {
		var s float32
		row := w[i*d : (i+1)*d]
		for j, v := range x {
			s += v * row[j]
		}
		out[i] = s + bias[i]
	}
	return out
}
