package typesafe

import (
	"bytes"
	"encoding/json"
	"math"
)

// An Answer is the model's response to one [Question]. The implementations are
// [NoulAnswer], [ChoiceAnswer], [ScoreAnswer], and [UnknownAnswer]; no other
// type can implement it.
//
// Read answers with the typed getters [SystemOneResponse.Noul],
// [SystemOneResponse.Choice], and [SystemOneResponse.Score] rather than by type
// switch, unless you need to handle [UnknownAnswer] yourself.
type Answer interface {
	// Kind reports the answer variant.
	Kind() Kind
	isAnswer()
}

// Answers are the answers to a request's questions, keyed by the names the
// request used.
type Answers map[string]Answer

// NoulAnswer answers a [NoulQuestion].
type NoulAnswer struct {
	// Noul is the probability, from 0 to 1, that the answer is yes. There is no
	// separate confidence: a value near 0.5 means the model finds yes and no
	// about equally likely, not that it is moderately sure of a middling answer.
	Noul float64 `json:"noul"`
}

// ChoiceAnswer answers a [ChoiceQuestion].
type ChoiceAnswer struct {
	// Choice is the selected label, one of the question's criteria keys.
	Choice string `json:"choice"`
	// Confidence, from 0 to 1, summarizes how concentrated Probabilities is. It
	// says nothing about whether the selection is correct.
	Confidence float64 `json:"confidence"`
	// Probabilities gives each criteria label's probability. They sum to about 1.
	Probabilities map[string]float64 `json:"probabilities"`
}

// TypedChoiceAnswer is a [ChoiceAnswer] whose labels have your own string type.
// Get one with [ChoiceAs]. The fields mean what they do on [ChoiceAnswer].
type TypedChoiceAnswer[T ~string] struct {
	Choice        T
	Confidence    float64
	Probabilities map[T]float64
}

// ScoreAnswer answers a [ScoreQuestion].
type ScoreAnswer struct {
	// Score is the probability-weighted position on the rubric, so it is
	// usually between two levels rather than on one. Use [ScoreAnswer.Level]
	// for the nearest level.
	Score float64 `json:"score"`
	// Confidence, from 0 to 1, summarizes how concentrated Probabilities is.
	Confidence float64 `json:"confidence"`
	// Legend echoes the rubric, keyed by score level.
	Legend map[int]Content `json:"legend"`
	// Probabilities gives each level's probability. They sum to about 1.
	Probabilities map[int]float64 `json:"probabilities"`
}

// UnknownAnswer is an answer of a type this SDK version does not know. It is
// kept rather than discarded so that a server that adds an answer type does not
// break existing code; inspect [UnknownAnswer.Raw] to handle it.
type UnknownAnswer struct {
	// Type is the wire type, which may be empty if the answer had none.
	Type Kind
	// Raw is the answer's JSON, exactly as it was received.
	Raw json.RawMessage
}

func (NoulAnswer) Kind() Kind      { return KindNoul }
func (ChoiceAnswer) Kind() Kind    { return KindChoice }
func (ScoreAnswer) Kind() Kind     { return KindScore }
func (a UnknownAnswer) Kind() Kind { return a.Type }

func (NoulAnswer) isAnswer()    {}
func (ChoiceAnswer) isAnswer()  {}
func (ScoreAnswer) isAnswer()   {}
func (UnknownAnswer) isAnswer() {}

// Level returns the rubric level nearest to Score, which is the level to use
// when a single one is needed. Ties round up.
func (a ScoreAnswer) Level() int { return int(math.Floor(a.Score + 0.5)) }

// Description returns the rubric description of [ScoreAnswer.Level], or nil if
// the legend does not cover it.
func (a ScoreAnswer) Description() Content { return a.Legend[a.Level()] }

// UnmarshalJSON decodes each answer according to its type discriminator. An
// answer whose type is missing or unrecognized becomes an [UnknownAnswer]
// holding the original JSON.
func (as *Answers) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	out := make(Answers, len(raw))
	for name, item := range raw {
		answer, err := unmarshalAnswer(item)
		if err != nil {
			return errf("answers.%s: %v", name, jsonMessage(err))
		}
		out[name] = answer
	}
	*as = out
	return nil
}

func unmarshalAnswer(data []byte) (Answer, error) {
	// A JSON null answer decodes to a nil Answer rather than an empty unknown
	// one, so a caller sees "no answer" instead of a zero-valued one.
	if string(bytes.TrimSpace(data)) == "null" {
		return nil, nil
	}

	var probe struct {
		Type Kind `json:"type"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, err
	}

	switch probe.Type {
	case KindNoul:
		var a NoulAnswer
		if err := json.Unmarshal(data, &a); err != nil {
			return nil, err
		}
		return a, nil
	case KindChoice:
		var a ChoiceAnswer
		if err := json.Unmarshal(data, &a); err != nil {
			return nil, err
		}
		return a, nil
	case KindScore:
		var a ScoreAnswer
		if err := json.Unmarshal(data, &a); err != nil {
			return nil, err
		}
		return a, nil
	}
	return UnknownAnswer{Type: probe.Type, Raw: append(json.RawMessage(nil), data...)}, nil
}

// jsonMessage strips the "typesafe: " prefix a nested SDK error would add, so a
// wrapped message does not repeat it.
func jsonMessage(err error) string {
	if e, ok := err.(*Error); ok {
		return e.Message
	}
	return err.Error()
}
