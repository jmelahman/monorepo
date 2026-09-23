package typesafe

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestAnswersUnmarshal(t *testing.T) {
	t.Parallel()

	const body = `{
		"spam":    {"type": "noul", "noul": 0.92},
		"tone":    {"type": "choice", "choice": "angry", "confidence": 0.8,
		            "probabilities": {"angry": 0.8, "calm": 0.2}},
		"urgency": {"type": "score", "score": 1.4, "confidence": 0.6,
		            "legend": {"0": "low", "1": "medium", "2": "high"},
		            "probabilities": {"0": 0.1, "1": 0.4, "2": 0.5}},
		"future":  {"type": "rank", "ranking": ["a", "b"]},
		"absent":  null
	}`

	var answers Answers
	if err := json.Unmarshal([]byte(body), &answers); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got, want := answers["spam"], (NoulAnswer{Noul: 0.92}); got != want {
		t.Errorf("spam = %#v, want %#v", got, want)
	}

	wantTone := ChoiceAnswer{
		Choice: "angry", Confidence: 0.8,
		Probabilities: map[string]float64{"angry": 0.8, "calm": 0.2},
	}
	if got := answers["tone"]; !reflect.DeepEqual(got, wantTone) {
		t.Errorf("tone = %#v, want %#v", got, wantTone)
	}

	urgency, ok := answers["urgency"].(ScoreAnswer)
	if !ok {
		t.Fatalf("urgency = %#v, want a ScoreAnswer", answers["urgency"])
	}
	if want := (map[int]float64{0: 0.1, 1: 0.4, 2: 0.5}); !reflect.DeepEqual(urgency.Probabilities, want) {
		t.Errorf("urgency.Probabilities = %v, want %v", urgency.Probabilities, want)
	}
	if got, want := urgency.Level(), 1; got != want {
		t.Errorf("urgency.Level() = %d, want %d", got, want)
	}
	if got, want := urgency.Description(), Content("medium"); got != want {
		t.Errorf("urgency.Description() = %v, want %v", got, want)
	}

	// An unrecognized type is preserved rather than dropped, so a server that
	// adds an answer type does not break existing code.
	future, ok := answers["future"].(UnknownAnswer)
	if !ok {
		t.Fatalf("future = %#v, want an UnknownAnswer", answers["future"])
	}
	if got, want := future.Kind(), Kind("rank"); got != want {
		t.Errorf("future.Kind() = %q, want %q", got, want)
	}
	if !strings.Contains(string(future.Raw), `"ranking"`) {
		t.Errorf("future.Raw = %s, want the original JSON", future.Raw)
	}

	if got, ok := answers["absent"]; !ok || got != nil {
		t.Errorf("absent = %#v (present %t), want a present nil answer", got, ok)
	}
}

func TestAnswersUnmarshalError(t *testing.T) {
	t.Parallel()

	var answers Answers
	err := json.Unmarshal([]byte(`{"urgency": {"type":"score","probabilities":{"high":0.5}}}`), &answers)
	if err == nil {
		t.Fatal("Unmarshal = nil, want an error naming the answer")
	}
	if !strings.Contains(err.Error(), "answers.urgency") {
		t.Errorf("Unmarshal = %q, want it to name answers.urgency", err)
	}
}

func TestScoreAnswerLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		score float64
		want  int
	}{{0, 0}, {0.49, 0}, {0.5, 1}, {1.4, 1}, {1.5, 2}, {2, 2}}
	for _, tc := range tests {
		if got := (ScoreAnswer{Score: tc.score}).Level(); got != tc.want {
			t.Errorf("ScoreAnswer{Score: %v}.Level() = %d, want %d", tc.score, got, tc.want)
		}
	}
}
