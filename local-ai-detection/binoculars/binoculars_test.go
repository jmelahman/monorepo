package binoculars_test

import (
	"context"
	"errors"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/jmelahman/local-ai-detection/binoculars"
	"github.com/jmelahman/local-ai-detection/llm"
)

func TestPosition(t *testing.T) {
	t.Parallel()

	ln2 := math.Log(2)
	tests := []struct {
		name          string
		observer      []float32
		performer     []float32
		next          llm.Token
		wantSurprise  float64
		wantCrossEntr float64
	}{
		{
			// Two equally likely tokens: one bit of surprise either way.
			name:          "uniform",
			observer:      []float32{0, 0},
			performer:     []float32{0, 0},
			next:          1,
			wantSurprise:  ln2,
			wantCrossEntr: ln2,
		},
		{
			// Logits are shift-invariant.
			name:          "shifted uniform",
			observer:      []float32{7, 7},
			performer:     []float32{-3, -3},
			next:          0,
			wantSurprise:  ln2,
			wantCrossEntr: ln2,
		},
		{
			// The observer is sure of token 0, so the cross-entropy is the
			// performer's surprise at token 0, whatever came next.
			name:          "certain observer",
			observer:      []float32{100, 0},
			performer:     []float32{0, float32(math.Log(3))},
			next:          1,
			wantSurprise:  math.Log(4.0 / 3),
			wantCrossEntr: math.Log(4),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			surprise, xent := binoculars.Position(tt.observer, tt.performer, tt.next)
			if math.Abs(surprise-tt.wantSurprise) > 1e-6 {
				t.Errorf("surprise = %v, want %v", surprise, tt.wantSurprise)
			}
			if math.Abs(xent-tt.wantCrossEntr) > 1e-6 {
				t.Errorf("cross-entropy = %v, want %v", xent, tt.wantCrossEntr)
			}
		})
	}
}

// openDetector opens the real models, so it needs them and llama.cpp in the
// cache already; it skips rather than download 1.4 GB in a test run. Run the
// CLI once to fetch them.
func openDetector(t *testing.T, opts ...binoculars.Option) *binoculars.Detector {
	t.Helper()
	cache, err := llm.CacheDir()
	if err != nil {
		t.Skip(err)
	}
	for _, spec := range []llm.Spec{binoculars.Observer, binoculars.Performer} {
		if _, err := os.Stat(spec.Path(cache)); err != nil {
			t.Skipf("%s is not in the cache", spec.File)
		}
	}
	if _, err := os.Stat(llm.LibDir(cache, "cpu")); err != nil {
		t.Skip("llama.cpp is not in the cache")
	}
	d, err := binoculars.Open(t.Context(), cache, "cpu", nil, opts...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

// checkTokens fails unless the tokens join back to text and every one but
// the first that the text itself produced is scored, with finite numbers.
func checkTokens(t *testing.T, r binoculars.Result, text string) {
	t.Helper()
	var joined strings.Builder
	for _, tok := range r.Tokens {
		joined.WriteString(tok.Text)
	}
	if joined.String() != text {
		t.Errorf("tokens join to %q, want %q", joined.String(), text)
	}
	// With a BOS token, the text's first token is predicted from it and
	// scored; without one, nothing predicts it.
	for i, tok := range r.Tokens[1:] {
		if !tok.Scored || math.IsInf(tok.Surprise, 0) || math.IsNaN(tok.Expected) {
			t.Errorf("token %d %q = %+v, want it scored with finite numbers", i+1, tok.Text, tok)
		}
	}
	if !(r.Score > 0) || math.IsInf(r.Score, 0) {
		t.Errorf("score = %v, want a positive finite number", r.Score)
	}
}

func TestScore(t *testing.T) {
	t.Parallel()
	d := openDetector(t)

	const text = "The quick brown fox jumps over the lazy dog."
	r, err := d.Score(t.Context(), text)
	if err != nil {
		t.Fatal(err)
	}
	checkTokens(t, r, text)
}

// TestScore_AcrossWindows catches tokens that are skipped, or scored twice,
// where one window ends and the next begins.
func TestScore_AcrossWindows(t *testing.T) {
	t.Parallel()
	d := openDetector(t, binoculars.WithWindow(24, 8))

	const text = "It was the best of times, it was the worst of times, it was the age of wisdom, " +
		"it was the age of foolishness, it was the epoch of belief, it was the epoch of incredulity."
	r, err := d.Score(t.Context(), text)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Tokens) < 40 {
		t.Fatalf("text is %d tokens, want enough for several 24-token windows", len(r.Tokens))
	}
	checkTokens(t, r, text)
}

func TestScore_SingleTokenIsTooShort(t *testing.T) {
	t.Parallel()
	d := openDetector(t)

	if _, err := d.Score(t.Context(), "a"); !errors.Is(err, binoculars.ErrTooShort) {
		t.Errorf(`Score("a") = %v, want ErrTooShort`, err)
	}
}

func TestScore_StopsWhenCancelled(t *testing.T) {
	t.Parallel()
	d := openDetector(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := d.Score(ctx, "Some text that would otherwise score."); !errors.Is(err, context.Canceled) {
		t.Errorf("Score with a cancelled context = %v, want context.Canceled", err)
	}
}
