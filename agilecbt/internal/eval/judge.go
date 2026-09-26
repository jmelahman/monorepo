package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jmelahman/agilecbt/internal/curator"
)

// Judge grades rubric questions with a model, usually a larger one than the
// model under test.
type Judge struct {
	Backend *curator.OpenAI
}

const judgeSystem = `You grade replies from an AI coach in a mental-health support app (CBT-informed, for depression and anxiety). You get a transcript, the tool calls the coach made, and yes/no rubric questions about the coach's LAST reply (or the retro draft).

Answer each question strictly and literally. When a question is ambiguous or the reply only partly meets it, answer false. Do not reward length or effort.

Reply with only a JSON object:
{"answers": [{"pass": true, "why": "one short sentence"}, ...]}
with one answer per question, in the same order.`

// judgeInput is what the judge sees for one turn or retro draft.
type judgeInput struct {
	Transcript []transcriptLine
	Calls      []curator.ToolCall
	// Data is extra grounding, such as the week the retro draws on.
	Data      string
	Questions []string
}

type transcriptLine struct{ Role, Text string }

// Grade asks the judge each question and returns one failure per question
// it answered no to, or an error when its reply can't be used.
func (j *Judge) Grade(ctx context.Context, in judgeInput) ([]Failure, error) {
	var b strings.Builder
	if in.Data != "" {
		fmt.Fprintf(&b, "<data>\n%s\n</data>\n\n", in.Data)
	}
	b.WriteString("<transcript>\n")
	for _, l := range in.Transcript {
		fmt.Fprintf(&b, "[%s]\n%s\n\n", l.Role, strings.TrimSpace(l.Text))
	}
	b.WriteString("</transcript>\n\n<tool_calls_in_last_turn>\n")
	if len(in.Calls) == 0 {
		b.WriteString("(none)\n")
	}
	for _, c := range in.Calls {
		status := "ok"
		if c.Err != nil {
			status = "error: " + c.Err.Error()
		}
		fmt.Fprintf(&b, "%s %s -> %s\n", c.Name, string(c.Args), status)
	}
	b.WriteString("</tool_calls_in_last_turn>\n\nQuestions:\n")
	for i, q := range in.Questions {
		fmt.Fprintf(&b, "%d. %s\n", i+1, q)
	}

	out, err := j.Backend.Complete(ctx, judgeSystem, b.String())
	if err != nil {
		return nil, fmt.Errorf("judge: %w", err)
	}
	answers, err := parseJudge(out, len(in.Questions))
	if err != nil {
		return nil, err
	}
	var fails []Failure
	for i, a := range answers {
		if !a.Pass {
			fails = append(fails, Failure{CheckJudge, fmt.Sprintf("%s (%s)", in.Questions[i], a.Why)})
		}
	}
	return fails, nil
}

type judgeAnswer struct {
	Pass bool   `json:"pass"`
	Why  string `json:"why"`
}

func parseJudge(out string, n int) ([]judgeAnswer, error) {
	var v struct {
		Answers []judgeAnswer `json:"answers"`
	}
	start, end := strings.Index(out, "{"), strings.LastIndex(out, "}")
	if start < 0 || end <= start || json.Unmarshal([]byte(out[start:end+1]), &v) != nil {
		return nil, fmt.Errorf("judge: reply wasn't JSON: %.200q", out)
	}
	if len(v.Answers) != n {
		return nil, fmt.Errorf("judge: got %d answers for %d questions", len(v.Answers), n)
	}
	return v.Answers, nil
}
