package typesafe

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
)

// Kind identifies a question or answer variant.
type Kind string

const (
	// KindNoul is a yes/no question, answered with the probability of yes.
	KindNoul Kind = "noul"
	// KindChoice is a question that selects one of a set of named choices.
	KindChoice Kind = "choice"
	// KindScore is a question that rates the state against an ordered rubric.
	KindScore Kind = "score"
)

// A Question is one question in a [SystemOneRequest]. The implementations are
// [NoulQuestion], [ChoiceQuestion], and [ScoreQuestion]; no other type can
// implement it.
//
// Build questions with [Noul], [Choice], and [Score].
type Question interface {
	// Kind reports the question variant.
	Kind() Kind
	isQuestion()
}

// Questions are the questions in a request, keyed by the names used to identify
// their answers. The names are yours to choose and are not sent to the model.
type Questions map[string]Question

// NoulCriteria describes what counts as a yes or a no answer. Either field may
// be left nil, which omits it from the request.
type NoulCriteria struct {
	// True describes what counts as a yes answer.
	True Content `json:"true,omitzero"`
	// False describes what counts as a no answer.
	False Content `json:"false,omitzero"`
}

// NoulQuestion is a yes/no question or statement, answered with the probability
// that the answer is yes or the statement is true. Build one with [Noul].
type NoulQuestion struct {
	// Instructions is the yes/no question or statement to evaluate. It is
	// omitted from the request when nil; use [Null] to send an explicit null.
	Instructions Content `json:"instructions,omitzero"`

	// Criteria optionally clarifies what counts as a yes or a no.
	Criteria *NoulCriteria `json:"criteria,omitzero"`
}

// ChoiceCriteria maps each choice name to a description of when it applies. A
// nil description is sent as null, which asks the model to interpret the choice
// by its name alone.
type ChoiceCriteria map[string]Content

// ChoiceQuestion is a question that selects one of the named choices. Build one
// with [Choice].
type ChoiceQuestion struct {
	// Instructions is what the model should decide. It is omitted from the
	// request when nil; use [Null] to send an explicit null.
	Instructions Content `json:"instructions,omitzero"`

	// Criteria holds the available choices. It is required.
	Criteria ChoiceCriteria `json:"criteria"`
}

// ScoreCriteria is an ordered rubric. Each description's index is its score,
// starting at zero.
type ScoreCriteria []Content

// ScoreQuestion is a question that rates the state against an ordered rubric.
// Build one with [Score].
type ScoreQuestion struct {
	// Instructions is what the model should rate. It is omitted from the
	// request when nil; use [Null] to send an explicit null.
	Instructions Content `json:"instructions,omitzero"`

	// Criteria describes the score levels in order, lowest first. At least two
	// levels are required.
	Criteria ScoreCriteria `json:"criteria"`
}

func (NoulQuestion) Kind() Kind   { return KindNoul }
func (ChoiceQuestion) Kind() Kind { return KindChoice }
func (ScoreQuestion) Kind() Kind  { return KindScore }

func (NoulQuestion) isQuestion()   {}
func (ChoiceQuestion) isQuestion() {}
func (ScoreQuestion) isQuestion()  {}

// The type discriminator is emitted here rather than exposed as a field, so a
// question can never carry a kind that disagrees with its Go type. Embedding an
// unexported struct type promotes its fields, tags included.

func (q NoulQuestion) MarshalJSON() ([]byte, error) {
	type wire NoulQuestion
	return json.Marshal(struct {
		Type Kind `json:"type"`
		wire
	}{KindNoul, wire(q)})
}

func (q ChoiceQuestion) MarshalJSON() ([]byte, error) {
	type wire ChoiceQuestion
	return json.Marshal(struct {
		Type Kind `json:"type"`
		wire
	}{KindChoice, wire(q)})
}

func (q ScoreQuestion) MarshalJSON() ([]byte, error) {
	type wire ScoreQuestion
	return json.Marshal(struct {
		Type Kind `json:"type"`
		wire
	}{KindScore, wire(q)})
}

// Noul returns a yes/no question or statement. The answer is the probability
// that the answer is yes.
//
// Add descriptions of the two outcomes with [NoulQuestion.WithCriteria].
func Noul(instructions Content) NoulQuestion {
	return NoulQuestion{Instructions: instructions}
}

// WithCriteria returns a copy of q describing what counts as a yes and a no.
func (q NoulQuestion) WithCriteria(c NoulCriteria) NoulQuestion {
	q.Criteria = &c
	return q
}

// Choice returns a question that selects one of the choices in criteria. The
// answer names the selected choice and gives a probability for each one.
func Choice(instructions Content, criteria ChoiceCriteria) ChoiceQuestion {
	return ChoiceQuestion{Instructions: instructions, Criteria: criteria}
}

// ChoiceOf is [Choice] with labels of your own string type, such as an enum of
// constants. Read its answer with [ChoiceAs] to get the selection back as T.
//
//	type Tone string
//
//	const (
//		Calm  Tone = "calm"
//		Angry Tone = "angry"
//	)
//
//	typesafe.ChoiceOf("What is the tone?", map[Tone]typesafe.Content{Calm: nil, Angry: nil})
func ChoiceOf[T ~string](instructions Content, criteria map[T]Content) ChoiceQuestion {
	c := make(ChoiceCriteria, len(criteria))
	for label, description := range criteria {
		c[string(label)] = description
	}
	return Choice(instructions, c)
}

// Score returns a question that rates the state against an ordered rubric. Each
// description's position is its score, starting at zero, so criteria must list
// at least two levels, lowest first.
func Score(instructions Content, criteria ScoreCriteria) ScoreQuestion {
	return ScoreQuestion{Instructions: instructions, Criteria: criteria}
}

// minScoreLevels is the smallest usable rubric. The API's schema accepts one
// level, but a one-level rubric has no range to score against and the HTTP
// documentation requires two, so this is caught before a request is sent.
const minScoreLevels = 2

// validate checks the questions the API and its documentation require, so that a
// malformed request fails here rather than as a 422.
func (qs Questions) validate() error {
	if len(qs) == 0 {
		return errf("at least one question is required")
	}
	for _, name := range slices.Sorted(maps.Keys(qs)) {
		if err := validateQuestion(name, qs[name]); err != nil {
			return err
		}
	}
	return nil
}

func validateQuestion(name string, q Question) error {
	if q == nil {
		return errf("question %q is nil", name)
	}
	switch q := q.(type) {
	case NoulQuestion:
		if err := validateContent(fmt.Sprintf("question %q instructions", name), q.Instructions); err != nil {
			return err
		}
		if q.Criteria != nil {
			if err := validateContent(fmt.Sprintf("question %q criteria true", name), q.Criteria.True); err != nil {
				return err
			}
			if err := validateContent(fmt.Sprintf("question %q criteria false", name), q.Criteria.False); err != nil {
				return err
			}
		}
	case ChoiceQuestion:
		if err := validateContent(fmt.Sprintf("question %q instructions", name), q.Instructions); err != nil {
			return err
		}
		if len(q.Criteria) == 0 {
			return errf("choice question %q requires criteria", name)
		}
		for _, label := range slices.Sorted(maps.Keys(q.Criteria)) {
			if err := validateContent(fmt.Sprintf("question %q criteria %q", name, label), q.Criteria[label]); err != nil {
				return err
			}
		}
	case ScoreQuestion:
		if err := validateContent(fmt.Sprintf("question %q instructions", name), q.Instructions); err != nil {
			return err
		}
		if len(q.Criteria) < minScoreLevels {
			return errf("score question %q has %d criteria; at least %d score levels are required",
				name, len(q.Criteria), minScoreLevels)
		}
		for i, level := range q.Criteria {
			if level == nil || level == Null {
				return errf("question %q criteria %d must describe a score level, got null", name, i)
			}
			if err := validateContent(fmt.Sprintf("question %q criteria %d", name, i), level); err != nil {
				return err
			}
		}
	default:
		return errf("question %q has unsupported type %T", name, q)
	}
	return nil
}
