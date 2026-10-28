// Package binoculars detects machine-generated text without training, by the
// method of Hans et al., "Spotting LLMs With Binoculars" (2024,
// https://arxiv.org/abs/2401.12070).
//
// A language model finds machine-generated text unsurprising, but so it does
// any text on a predictable subject. Binoculars divides the first effect by
// the second: the performer's log-perplexity of the text, over how surprised
// the performer is, on average, by what the observer would have written in
// its place. A score well below 1 means the text is more predictable than
// even its own context would explain.
//
// The two models must share a tokenizer: [Observer] and [Performer] are a
// base model and its instruction-tuned sibling, as in the paper.
package binoculars

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/jmelahman/local-ai-detection/llm"
)

// ErrTooShort is returned by [Detector.Score] for text with fewer than two
// tokens, since only a token with one before it can be scored.
var ErrTooShort = errors.New("text is too short to score")

// ErrVocabMismatch is returned by [New] for models whose vocabularies differ.
var ErrVocabMismatch = errors.New("observer and performer vocabularies differ")

// ErrBadWindow is returned by [New] for a [WithWindow] whose overlap leaves
// no room to make progress.
var ErrBadWindow = errors.New("window overlap must be at least 0 and less than the window")

// Observer and Performer are Qwen3-0.6B, base and instruction-tuned, at Q8_0.
var (
	Observer = llm.Spec{
		Repo:     "ggml-org/Qwen3-0.6B-Base-GGUF",
		Revision: "6ca7d568ed24e5f392fd4162d0093d21e975fad4",
		File:     "Qwen3-0.6B-Base-Q8_0.gguf",
		SHA256:   "ebb25a17e79b1f43834410fb711ac3dc985364eb875b45914181f55b9993f2d0",
		Size:     639446720,
	}
	Performer = llm.Spec{
		Repo:     "ggml-org/Qwen3-0.6B-GGUF",
		Revision: "13ac06624948e4d8589cb4b6f90100837342fcb8",
		File:     "Qwen3-0.6B-Q8_0.gguf",
		SHA256:   "361cc68159042c36ebff7715dc5a2e4612153e88f3e9c9c234820849d6dc9e1d",
		Size:     804753632,
	}
)

// Threshold separates the scores of human (above) and machine (below) text.
// It is the paper's low-false-positive threshold for Falcon-7B, a stand-in
// until the eval harness calibrates one for [Observer] and [Performer].
const Threshold = 0.9015

const batchSize = 512

// Option configures [New] and [Open].
type Option func(*options)

type options struct {
	window  int
	overlap int
}

// WithWindow sets how many tokens the models see at once, 2048 by default,
// and by how many consecutive windows of a longer text overlap, 512 by
// default. Each window rescores none of the overlap: it only gives the
// tokens after it context.
func WithWindow(size, overlap int) Option {
	return func(o *options) { o.window, o.overlap = size, overlap }
}

// Token is one token of scored text.
type Token struct {
	// Text is the token's piece of the input; see [llm.Model.Piece].
	Text string
	// Surprise is the performer's negative log-probability of the token.
	Surprise float64
	// Expected is the performer's cross-entropy against the observer at the
	// token's position: the surprise the observer's own choice would cause.
	Expected float64
	// Scored is false for the first token, which nothing predicts.
	Scored bool
}

// Result is the score of a text.
type Result struct {
	Tokens []Token
	// LogPPL is the mean [Token.Surprise] of the scored tokens.
	LogPPL float64
	// LogXPPL is the mean [Token.Expected] of the scored tokens.
	LogXPPL float64
	// Score is LogPPL / LogXPPL. Lower is more machine-like.
	Score float64
}

// Machine reports whether the score is below threshold.
func (r Result) Machine(threshold float64) bool {
	return r.Score < threshold
}

// Detector scores text with an observer and a performer model. It is not
// safe for concurrent use.
type Detector struct {
	observer  *llm.Model
	obsCtx    *llm.Context
	perfCtx   *llm.Context
	closeFunc func() error
	options
}

// Open installs llama.cpp for processor and fetches [Observer] and
// [Performer] into cache, unless they are there already, and loads them.
// Download progress goes to progress.
func Open(ctx context.Context, cache, processor string, progress io.Writer, opts ...Option) (*Detector, error) {
	libDir := llm.LibDir(cache, processor)
	if err := llm.Install(ctx, libDir, processor, progress); err != nil {
		return nil, err
	}
	if err := llm.Init(libDir); err != nil {
		return nil, err
	}
	models := make([]*llm.Model, 0, 2)
	closeAll := func() error {
		errs := make([]error, 0, len(models))
		for _, m := range models {
			errs = append(errs, m.Close())
		}
		return errors.Join(errs...)
	}
	for _, spec := range []llm.Spec{Observer, Performer} {
		path, err := llm.Fetch(ctx, spec, cache, progress)
		if err != nil {
			return nil, errors.Join(err, closeAll())
		}
		if err := ctx.Err(); err != nil {
			return nil, errors.Join(err, closeAll())
		}
		m, err := llm.Open(path)
		if err != nil {
			return nil, errors.Join(err, closeAll())
		}
		models = append(models, m)
	}
	d, err := New(models[0], models[1], opts...)
	if err != nil {
		return nil, errors.Join(err, closeAll())
	}
	d.closeFunc = closeAll
	return d, nil
}

// New makes a detector from loaded models, which must outlive it.
func New(observer, performer *llm.Model, opts ...Option) (*Detector, error) {
	o := options{window: 2048, overlap: 512}
	for _, opt := range opts {
		opt(&o)
	}
	if o.overlap < 0 || o.overlap >= o.window {
		return nil, fmt.Errorf("%w: window %d, overlap %d", ErrBadWindow, o.window, o.overlap)
	}
	if observer.VocabSize() != performer.VocabSize() {
		return nil, fmt.Errorf("%w: %d and %d", ErrVocabMismatch, observer.VocabSize(), performer.VocabSize())
	}
	batch := min(batchSize, o.window)
	obsCtx, err := observer.NewContext(o.window, batch)
	if err != nil {
		return nil, err
	}
	perfCtx, err := performer.NewContext(o.window, batch)
	if err != nil {
		return nil, errors.Join(err, obsCtx.Close())
	}
	return &Detector{observer: observer, obsCtx: obsCtx, perfCtx: perfCtx, options: o}, nil
}

// Close frees the detector, and the models too if [Open] loaded them.
func (d *Detector) Close() error {
	err := errors.Join(d.obsCtx.Close(), d.perfCtx.Close())
	if d.closeFunc != nil {
		err = errors.Join(err, d.closeFunc())
	}
	return err
}

// Score scores text. Text longer than the window is scored in windows that
// overlap (see [WithWindow]), so every token after the first is scored once,
// with at least the overlap as context. It stops early, with ctx's error, if
// ctx is cancelled.
func (d *Detector) Score(ctx context.Context, text string) (Result, error) {
	tokens, bos := d.observer.Tokenize(text)
	if len(tokens) < 2 {
		return Result{}, ErrTooShort
	}

	scored := make([]Token, len(tokens))
	for i, tok := range tokens {
		scored[i].Text = d.observer.Piece(tok)
	}
	// Each window scores the tokens from "from" up to its end; the ones
	// between its start and "from" are the overlap, there for context.
	for start, from := 0, 1; from < len(tokens); {
		end := min(start+d.window, len(tokens))
		if err := d.scoreWindow(ctx, tokens, scored, start, from, end); err != nil {
			return Result{}, err
		}
		from, start = end, end-d.overlap
	}
	if bos {
		scored = scored[1:]
	}

	var r Result
	var n float64
	for _, t := range scored {
		if t.Scored {
			r.LogPPL += t.Surprise
			r.LogXPPL += t.Expected
			n++
		}
	}
	if n == 0 {
		return Result{}, ErrTooShort
	}
	r.Tokens = scored
	r.LogPPL /= n
	r.LogXPPL /= n
	r.Score = r.LogPPL / r.LogXPPL
	return r, nil
}

// scoreWindow evaluates tokens[start:end] from an empty context and scores
// tokens[from:end], each from the logits of the token before it.
func (d *Detector) scoreWindow(ctx context.Context, tokens []llm.Token, scored []Token, start, from, end int) error {
	if err := errors.Join(d.obsCtx.Reset(), d.perfCtx.Reset()); err != nil {
		return err
	}
	batch := min(batchSize, d.window)
	for b := start; b < end; b += batch {
		if err := ctx.Err(); err != nil {
			return err
		}
		chunk := tokens[b:min(b+batch, end)]
		if err := errors.Join(d.obsCtx.Decode(chunk, b-start), d.perfCtx.Decode(chunk, b-start)); err != nil {
			return err
		}
		for k := range chunk {
			next := b + k + 1
			if next < from {
				continue
			}
			if next >= end {
				break
			}
			obs, err := d.obsCtx.Logits(k)
			if err != nil {
				return err
			}
			perf, err := d.perfCtx.Logits(k)
			if err != nil {
				return err
			}
			scored[next].Surprise, scored[next].Expected = Position(obs, perf, tokens[next])
			scored[next].Scored = true
		}
	}
	return nil
}

// Position scores one position from the observer's and performer's logits
// there and the token that actually came next. Surprise is the performer's
// negative log-probability of next; crossEntropy is the performer's expected
// negative log-probability under the observer's distribution,
// -Σ p_observer(v) · log p_performer(v).
//
// The roles follow the paper's reference implementation
// (https://github.com/ahans30/Binoculars), in which perplexity comes from the
// performer and the observer supplies the distribution it is crossed with.
func Position(observer, performer []float32, next llm.Token) (surprise, crossEntropy float64) {
	lsePerf := logSumExp(performer)
	// Σ p_observer(v)·performer(v), with the observer's softmax normalized
	// once at the end so each of its logits is exponentiated only once.
	m := maxOf(observer)
	var sum, dot float64
	for v, o := range observer {
		e := math.Exp(float64(o) - m)
		sum += e
		dot += e * float64(performer[v])
	}
	return lsePerf - float64(performer[next]), lsePerf - dot/sum
}

func logSumExp(logits []float32) float64 {
	m := maxOf(logits)
	var sum float64
	for _, x := range logits {
		sum += math.Exp(float64(x) - m)
	}
	return m + math.Log(sum)
}

func maxOf(logits []float32) float64 {
	m := math.Inf(-1)
	for _, x := range logits {
		m = max(m, float64(x))
	}
	return m
}
