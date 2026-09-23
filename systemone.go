package typesafe

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
)

const systemOnePath = "/v1/systemone"

// SystemOneRequest asks a System One model a set of questions about one state.
//
// The questions are answered independently and in parallel, so none of them can
// see another's answer. Ask a second request when an answer is needed to build
// the next state.
type SystemOneRequest struct {
	// State is the material the questions are asked about: text, or a JSON
	// object or array when the context has several named parts.
	State Content `json:"state"`

	// Model is the model to use. When empty, the client's default model is
	// sent instead.
	Model string `json:"model,omitzero"`

	// Questions are the questions to answer, keyed by names you choose. The
	// names identify the answers and are not shown to the model, so each
	// question must carry its full meaning.
	Questions Questions `json:"questions"`
}

// Usage reports the tokens a request consumed.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// SystemOneResponse holds the answers to a [SystemOneRequest].
type SystemOneResponse struct {
	// Model is the model that answered, resolved from any alias.
	Model string `json:"model"`
	// Answers holds one answer per question, keyed by the request's names.
	// Prefer the typed getters to reading this directly.
	Answers Answers `json:"answers"`
	// Usage reports the tokens consumed.
	Usage Usage `json:"usage"`

	// Meta describes the HTTP response the answers were decoded from.
	Meta *ResponseMeta `json:"-"`
}

// SystemOne asks a System One model the request's questions.
//
// The response is verified against the request before it is returned: a missing,
// extra, or mismatched answer is reported as a [*ResponseMismatchError] rather
// than surfacing later as a confusing zero value.
func (c *Client) SystemOne(ctx context.Context, req SystemOneRequest, opts ...RequestOption) (*SystemOneResponse, error) {
	if err := req.Questions.validate(); err != nil {
		return nil, err
	}
	if req.State == nil {
		return nil, errf("state is required")
	}
	if err := validateContent("state", req.State); err != nil {
		return nil, err
	}
	if req.Model == "" {
		req.Model = c.defaultModel
	}

	var res SystemOneResponse
	meta, err := c.do(ctx, "POST", systemOnePath, req, &res, opts...)
	if err != nil {
		return nil, err
	}
	res.Meta = meta

	if err := verifyAnswers(req.Questions, res.Answers); err != nil {
		err.RequestID = meta.RequestID
		return nil, err
	}
	return &res, nil
}

// NoAnswerError reports that a response has no answer under the requested name.
type NoAnswerError struct {
	// Name is the name that was asked for.
	Name string
	// Names lists the names the response does have, in order.
	Names []string
}

func (e *NoAnswerError) Error() string {
	if len(e.Names) == 0 {
		return fmt.Sprintf("typesafe: no answer named %q; the response has no answers", e.Name)
	}
	return fmt.Sprintf("typesafe: no answer named %q; the response has %s",
		e.Name, strings.Join(quoteAll(e.Names), ", "))
}

// AnswerKindError reports that an answer exists but is of another kind, which
// means the getter does not match the question that was asked.
type AnswerKindError struct {
	// Name is the answer's name.
	Name string
	// Want is the kind the getter asked for; Got is the kind the answer is.
	Want, Got Kind
}

func (e *AnswerKindError) Error() string {
	return fmt.Sprintf("typesafe: answer %q is a %s answer, not %s", e.Name, e.Got, e.Want)
}

// Noul returns the [NoulAnswer] named name.
//
// It returns a [*NoAnswerError] if no answer has that name and an
// [*AnswerKindError] if the answer is of another kind. The zero value is never
// returned with a nil error: in a probability API, a silent zero reads as a
// confident "no".
func (r *SystemOneResponse) Noul(name string) (NoulAnswer, error) {
	return answerAs[NoulAnswer](r.Answers, name, KindNoul)
}

// Choice returns the [ChoiceAnswer] named name. See [SystemOneResponse.Noul]
// for the errors it can return.
func (r *SystemOneResponse) Choice(name string) (ChoiceAnswer, error) {
	return answerAs[ChoiceAnswer](r.Answers, name, KindChoice)
}

// Score returns the [ScoreAnswer] named name. See [SystemOneResponse.Noul] for
// the errors it can return.
func (r *SystemOneResponse) Score(name string) (ScoreAnswer, error) {
	return answerAs[ScoreAnswer](r.Answers, name, KindScore)
}

// ChoiceAs returns the [ChoiceAnswer] named name with its labels converted to
// T, for a question built with [ChoiceOf]. It returns the same errors as
// [SystemOneResponse.Choice].
//
// The conversion is sound because [Client.SystemOne] has already checked that
// the selected label and every probability key are criteria keys, which for a
// [ChoiceOf] question are values of T. A response assembled by hand is not
// checked.
//
// Nothing ties T to the type the question was built with: ChoiceAs[Priority]
// on a question built with ChoiceOf[Team] compiles and converts Team labels to
// Priority. Use the same T for both.
func ChoiceAs[T ~string](r *SystemOneResponse, name string) (TypedChoiceAnswer[T], error) {
	answer, err := r.Choice(name)
	if err != nil {
		return TypedChoiceAnswer[T]{}, err
	}
	probabilities := make(map[T]float64, len(answer.Probabilities))
	for label, p := range answer.Probabilities {
		probabilities[T(label)] = p
	}
	return TypedChoiceAnswer[T]{
		Choice:        T(answer.Choice),
		Confidence:    answer.Confidence,
		Probabilities: probabilities,
	}, nil
}

// answerAs looks up one answer and asserts it to the wanted type. The wanted
// kind is passed as data because a zero T has no usable Kind of its own.
func answerAs[T Answer](answers Answers, name string, want Kind) (T, error) {
	var zero T
	answer, ok := answers[name]
	if !ok || answer == nil {
		return zero, &NoAnswerError{Name: name, Names: slices.Sorted(maps.Keys(answers))}
	}
	typed, ok := answer.(T)
	if !ok {
		return zero, &AnswerKindError{Name: name, Want: want, Got: answer.Kind()}
	}
	return typed, nil
}

// ResponseMismatchError reports that the answers did not correspond to the
// questions that were asked.
type ResponseMismatchError struct {
	// Problems describes each mismatch found.
	Problems []string
	// RequestID is the x-typesafe-request-id header, when present.
	RequestID string
}

func (e *ResponseMismatchError) Error() string {
	var b strings.Builder
	b.WriteString("typesafe: response does not match the request: ")
	b.WriteString(strings.Join(e.Problems, "; "))
	if e.RequestID != "" {
		b.WriteString(" (request_id=")
		b.WriteString(e.RequestID)
		b.WriteString(")")
	}
	return b.String()
}

// verifyAnswers checks the answers against the questions that produced them.
//
// An [UnknownAnswer] is deliberately accepted for any question: it is the
// forward-compatibility path, and rejecting it would break every client the day
// the API gains a new answer type.
func verifyAnswers(questions Questions, answers Answers) *ResponseMismatchError {
	var problems []string

	for _, name := range slices.Sorted(maps.Keys(questions)) {
		answer, ok := answers[name]
		if !ok || answer == nil {
			problems = append(problems, fmt.Sprintf("question %q has no answer", name))
			continue
		}
		if _, unknown := answer.(UnknownAnswer); unknown {
			continue
		}
		question := questions[name]
		if answer.Kind() != question.Kind() {
			problems = append(problems, fmt.Sprintf(
				"question %q is a %s question but its answer is %s", name, question.Kind(), answer.Kind()))
			continue
		}
		problems = append(problems, verifyAnswerShape(name, question, answer)...)
	}

	for _, name := range slices.Sorted(maps.Keys(answers)) {
		if _, ok := questions[name]; !ok {
			problems = append(problems, fmt.Sprintf("answer %q was not asked for", name))
		}
	}

	if len(problems) == 0 {
		return nil
	}
	return &ResponseMismatchError{Problems: problems}
}

// verifyAnswerShape checks that an answer's labels or levels are the ones the
// question offered.
func verifyAnswerShape(name string, question Question, answer Answer) []string {
	var problems []string
	switch q := question.(type) {
	case ChoiceQuestion:
		a := answer.(ChoiceAnswer)
		if _, ok := q.Criteria[a.Choice]; !ok {
			problems = append(problems, fmt.Sprintf(
				"answer %q selected %q, which is not one of its choices", name, a.Choice))
		}
		want := slices.Sorted(maps.Keys(q.Criteria))
		got := slices.Sorted(maps.Keys(a.Probabilities))
		if !slices.Equal(want, got) {
			problems = append(problems, fmt.Sprintf(
				"answer %q has probabilities for %s, but its choices are %s",
				name, strings.Join(quoteAll(got), ", "), strings.Join(quoteAll(want), ", ")))
		}
	case ScoreQuestion:
		a := answer.(ScoreAnswer)
		want := make([]int, len(q.Criteria))
		for i := range q.Criteria {
			want[i] = i
		}
		// Callers index their rubric by Level(), so a score outside it must be
		// an error here rather than a panic there.
		if a.Score < 0 || a.Level() >= len(q.Criteria) {
			problems = append(problems, fmt.Sprintf(
				"answer %q scored %v, which is outside its %d-level rubric",
				name, a.Score, len(q.Criteria)))
		}
		if got := slices.Sorted(maps.Keys(a.Probabilities)); !slices.Equal(want, got) {
			problems = append(problems, fmt.Sprintf(
				"answer %q has probabilities for levels %v, but its rubric has %d levels",
				name, got, len(q.Criteria)))
		}
		if got := slices.Sorted(maps.Keys(a.Legend)); len(a.Legend) > 0 && !slices.Equal(want, got) {
			problems = append(problems, fmt.Sprintf(
				"answer %q has a legend for levels %v, but its rubric has %d levels",
				name, got, len(q.Criteria)))
		}
	}
	return problems
}

func quoteAll(values []string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = fmt.Sprintf("%q", v)
	}
	return out
}
