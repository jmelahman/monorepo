package eval_test

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jmelahman/local-ai-detection/eval"
)

func TestLoad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		csv     string
		want    []eval.Example
		wantErr bool
	}{
		{
			name: "MAGE labels and BOM",
			csv: "\ufefftext,label,src\n" +
				"first,1,wp_human\n" +
				"second,0,wp_machine_continuation_gpt-3.5-trubo\n",
			want: []eval.Example{
				{Text: "first", Source: "wp_human", ID: 0, Machine: false},
				{Text: "second", Source: "wp_machine_continuation_gpt-3.5-trubo", ID: 1, Machine: true},
			},
		},
		{
			name: "columns in another order and word labels",
			csv:  "src,label,text\na,machine,x\nb,human,y\n",
			want: []eval.Example{
				{Text: "x", Source: "a", ID: 0, Machine: true},
				{Text: "y", Source: "b", ID: 1, Machine: false},
			},
		},
		{name: "unknown label", csv: "text,label,src\na,sep,nope\n", wantErr: true},
		{name: "short row", csv: "text,label,src\na,1\n", wantErr: true},
		{name: "no text column", csv: "label,src\n1,a\n", wantErr: true},
		{name: "no source column", csv: "text,label\na,1\n", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := eval.Load(strings.NewReader(tt.csv))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Load() = %+v, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("Load() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSample(t *testing.T) {
	t.Parallel()

	examples := make([]eval.Example, 100)
	for i := range examples {
		examples[i] = eval.Example{ID: i}
	}

	first := eval.Sample(examples, 20, 7)
	second := eval.Sample(examples, 20, 7)
	if len(first) != 20 {
		t.Fatalf("Sample(20) returned %d examples", len(first))
	}
	if !slices.Equal(first, second) {
		t.Error("the same seed returned different samples")
	}
	for i := 1; i < len(first); i++ {
		if first[i-1].ID >= first[i].ID {
			t.Fatalf("sample is not in ID order: %d then %d", first[i-1].ID, first[i].ID)
		}
	}
	if other := eval.Sample(examples, 20, 8); slices.Equal(first, other) {
		t.Error("different seeds returned the same sample")
	}
	if all := eval.Sample(examples, 0, 1); len(all) != len(examples) {
		t.Errorf("Sample(0) returned %d examples, want all %d", len(all), len(examples))
	}
}

func TestShard(t *testing.T) {
	t.Parallel()

	examples := make([]eval.Example, 10)
	for i := range examples {
		examples[i] = eval.Example{ID: i}
	}
	seen := make(map[int]bool)
	for shard := range 3 {
		for _, example := range eval.Shard(examples, shard, 3) {
			if seen[example.ID] {
				t.Errorf("example %d is in more than one shard", example.ID)
			}
			seen[example.ID] = true
		}
	}
	if len(seen) != len(examples) {
		t.Errorf("shards cover %d of %d examples", len(seen), len(examples))
	}
	if got := eval.Shard(examples, 0, 1); !slices.Equal(got, examples) {
		t.Error("a single shard did not keep every example")
	}
}

func TestTruncateWords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		n    int
		want string
	}{
		{name: "short enough", text: "one two", n: 3, want: "one two"},
		{name: "truncates", text: "one two three", n: 2, want: "one two"},
		{name: "no limit", text: "one two three", n: 0, want: "one two three"},
		{name: "negative", text: "one two three", n: -1, want: "one two three"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := eval.TruncateWords(tt.text, tt.n); got != tt.want {
				t.Errorf("TruncateWords(%q, %d) = %q, want %q", tt.text, tt.n, got, tt.want)
			}
		})
	}
}

func TestObservationsRoundTrip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "scores.csv")
	writer, err := eval.CreateObservations(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []eval.Observation{
		{Text: "a", Source: "wp_human", ID: 3, Score: 0.91, LogPPL: 3.2, LogXPPL: 3.5, Tokens: 10, Words: 8},
		{Text: "b", Source: "wp_machine", ID: 4, Machine: true, Score: 0.42, LogPPL: 1.1, LogXPPL: 2.6, Tokens: 12, Words: 9},
	}
	for _, observation := range want {
		if err := writer.Write(observation); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := eval.ReadObservations(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("read %d observations, want %d", len(got), len(want))
	}
	for i := range want {
		want[i].Text = "" // Text is not part of the score file.
		if got[i].Example != want[i].Example || math.Abs(got[i].Score-want[i].Score) > 1e-9 {
			t.Errorf("observation %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestAppendObservationsCreatesHeader(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "scores.csv")
	writer, err := eval.AppendObservations(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Write(eval.Observation{ID: 1, Score: 0.5}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(contents), "id,label,source,score") {
		t.Errorf("new file starts %q, want the header", string(contents))
	}
}

func TestParseObservationsRejectsBadScores(t *testing.T) {
	t.Parallel()

	_, err := eval.ParseObservations(strings.NewReader("id,label,source,score,log_ppl,log_x_ppl,tokens,words\n1,human,a,not-a-number,1,2,3,4\n"))
	if err == nil {
		t.Error("ParseObservations() = nil error, want one")
	}
}

func TestLoadObservationsMissingFile(t *testing.T) {
	t.Parallel()

	if _, err := eval.ReadObservations(filepath.Join(t.TempDir(), "missing.csv")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("ReadObservations(missing) = %v, want fs.ErrNotExist", err)
	}
}
