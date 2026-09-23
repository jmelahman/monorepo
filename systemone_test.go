package typesafe

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func questions() Questions {
	return Questions{
		"spam": Noul("Is this spam?"),
		"tone": Choice("What is the tone?", ChoiceCriteria{"angry": nil, "calm": nil}),
		"heat": Score("How urgent?", ScoreCriteria{"low", "high"}),
	}
}

func answers() Answers {
	return Answers{
		"spam": NoulAnswer{Noul: 0.9},
		"tone": ChoiceAnswer{Choice: "angry", Probabilities: map[string]float64{"angry": 0.9, "calm": 0.1}},
		"heat": ScoreAnswer{Score: 0.8, Legend: map[int]Content{0: "low", 1: "high"},
			Probabilities: map[int]float64{0: 0.2, 1: 0.8}},
	}
}

func TestVerifyAnswers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		mutil func(Questions, Answers)
		want  string
	}{{
		name: "matching",
	}, {
		name:  "missing answer",
		mutil: func(_ Questions, a Answers) { delete(a, "spam") },
		want:  `question "spam" has no answer`,
	}, {
		name:  "null answer",
		mutil: func(_ Questions, a Answers) { a["spam"] = nil },
		want:  `question "spam" has no answer`,
	}, {
		name:  "extra answer",
		mutil: func(_ Questions, a Answers) { a["bonus"] = NoulAnswer{} },
		want:  `answer "bonus" was not asked for`,
	}, {
		name:  "wrong kind",
		mutil: func(_ Questions, a Answers) { a["spam"] = ScoreAnswer{} },
		want:  `question "spam" is a noul question but its answer is score`,
	}, {
		name: "choice outside the criteria",
		mutil: func(_ Questions, a Answers) {
			c := a["tone"].(ChoiceAnswer)
			c.Choice = "bored"
			a["tone"] = c
		},
		want: `answer "tone" selected "bored"`,
	}, {
		name: "choice probabilities diverge",
		mutil: func(_ Questions, a Answers) {
			c := a["tone"].(ChoiceAnswer)
			c.Probabilities = map[string]float64{"angry": 1}
			a["tone"] = c
		},
		want: `answer "tone" has probabilities for "angry"`,
	}, {
		name: "score probabilities diverge",
		mutil: func(_ Questions, a Answers) {
			s := a["heat"].(ScoreAnswer)
			s.Probabilities = map[int]float64{0: 0.2, 1: 0.7, 2: 0.1}
			a["heat"] = s
		},
		want: `answer "heat" has probabilities for levels [0 1 2]`,
	}, {
		name: "unknown answer is not a mismatch",
		mutil: func(_ Questions, a Answers) {
			a["spam"] = UnknownAnswer{Type: "rank", Raw: json.RawMessage(`{}`)}
		},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			qs, as := questions(), answers()
			if tc.mutil != nil {
				tc.mutil(qs, as)
			}
			err := verifyAnswers(qs, as)
			switch {
			case tc.want == "":
				if err != nil {
					t.Fatalf("verifyAnswers() = %v, want nil", err)
				}
			case err == nil:
				t.Fatalf("verifyAnswers() = nil, want an error containing %q", tc.want)
			case !strings.Contains(err.Error(), tc.want):
				t.Errorf("verifyAnswers() = %q, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestTypedGetters(t *testing.T) {
	t.Parallel()

	res := &SystemOneResponse{Answers: answers()}

	spam, err := res.Noul("spam")
	if err != nil || spam.Noul != 0.9 {
		t.Errorf("Noul(spam) = (%v, %v), want (0.9, nil)", spam.Noul, err)
	}
	if _, err := res.Choice("tone"); err != nil {
		t.Errorf("Choice(tone) = %v, want nil", err)
	}
	if _, err := res.Score("heat"); err != nil {
		t.Errorf("Score(heat) = %v, want nil", err)
	}

	// A typo names the answers that do exist.
	_, err = res.Noul("spma")
	var missing *NoAnswerError
	if !errors.As(err, &missing) {
		t.Fatalf("Noul(spma) = %T, want *NoAnswerError", err)
	}
	if !strings.Contains(err.Error(), `"spam"`) {
		t.Errorf("Noul(spma) = %q, want it to list the available names", err)
	}

	// Reading an answer as the wrong kind is distinct from a missing one.
	_, err = res.Noul("tone")
	var kind *AnswerKindError
	if !errors.As(err, &kind) {
		t.Fatalf("Noul(tone) = %T, want *AnswerKindError", err)
	}
	if kind.Want != KindNoul || kind.Got != KindChoice {
		t.Errorf("AnswerKindError = %+v, want noul/choice", kind)
	}

	// An UnknownAnswer is reported as its wire kind rather than as missing.
	res.Answers["future"] = UnknownAnswer{Type: "rank"}
	if _, err := res.Noul("future"); !errors.As(err, &kind) || kind.Got != "rank" {
		t.Errorf("Noul(future) = %v, want an *AnswerKindError reporting rank", err)
	}

	// No getter ever returns a usable zero value with a nil error.
	if _, err := (&SystemOneResponse{}).Noul("spam"); err == nil {
		t.Error("Noul on an empty response = nil error, want one")
	}
}

func TestVerifyRejectsOutOfRangeScore(t *testing.T) {
	t.Parallel()

	// Callers index their rubric with Level(), so a score beyond it must be
	// caught here rather than panicking in caller code.
	for _, score := range []float64{-0.6, 2.0, 7.5} {
		qs, as := questions(), answers()
		s := as["heat"].(ScoreAnswer)
		s.Score = score
		as["heat"] = s

		err := verifyAnswers(qs, as)
		if err == nil {
			t.Errorf("verifyAnswers(score %v) = nil, want an out-of-range error", score)
			continue
		}
		if !strings.Contains(err.Error(), "outside its 2-level rubric") {
			t.Errorf("verifyAnswers(score %v) = %q, want an out-of-range error", score, err)
		}
	}

	// A score between two levels is normal and must stay valid.
	if err := verifyAnswers(questions(), answers()); err != nil {
		t.Errorf("verifyAnswers() = %v, want nil for an in-range score", err)
	}
}

type tone string

const (
	toneAngry tone = "angry"
	toneCalm  tone = "calm"
)

func TestChoiceOf(t *testing.T) {
	t.Parallel()

	q := ChoiceOf("What is the tone?", map[tone]Content{toneAngry: "Hostile", toneCalm: nil})
	got, err := json.Marshal(q)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := `{"type":"choice","instructions":"What is the tone?","criteria":{"angry":"Hostile","calm":null}}`
	if string(got) != want {
		t.Errorf("Marshal = %s, want %s", got, want)
	}
}

func TestChoiceAs(t *testing.T) {
	t.Parallel()

	res := &SystemOneResponse{Answers: answers()}
	got, err := ChoiceAs[tone](res, "tone")
	if err != nil {
		t.Fatalf("ChoiceAs: %v", err)
	}
	if got.Choice != toneAngry || got.Probabilities[toneCalm] != 0.1 || len(got.Probabilities) != 2 {
		t.Errorf("ChoiceAs = %+v, want angry with both probabilities", got)
	}

	var missing *NoAnswerError
	if _, err := ChoiceAs[tone](res, "tones"); !errors.As(err, &missing) {
		t.Errorf("ChoiceAs(missing) = %v, want *NoAnswerError", err)
	}
	var wrongKind *AnswerKindError
	if _, err := ChoiceAs[tone](res, "spam"); !errors.As(err, &wrongKind) {
		t.Errorf("ChoiceAs(noul) = %v, want *AnswerKindError", err)
	}
}
