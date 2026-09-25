package curator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/jmelahman/agilecbt/internal/tools"
)

// DefaultAnthropicModel is used when APP_MODEL is unset.
const DefaultAnthropicModel = "claude-opus-5"

// refusalText replaces a reply the model declined to give.
const refusalText = "I'm not able to help with that one here. If you're struggling right now, the \"Need help now?\" link at the top has people you can reach any time."

// Anthropic runs the curator against the Claude API with an API key
// ($ANTHROPIC_API_KEY). This bills the API, not a Claude subscription.
type Anthropic struct {
	Model    string
	Registry *tools.Registry
	// Options are extra client options (tests point BaseURL at a fake).
	Options []option.RequestOption
}

// Name implements Backend.
func (a *Anthropic) Name() string { return "anthropic" }

func (a *Anthropic) model() string {
	if a.Model != "" {
		return a.Model
	}
	return DefaultAnthropicModel
}

func (a *Anthropic) client() anthropic.Client {
	return anthropic.NewClient(a.Options...)
}

// Status implements Backend. It only checks that a key is configured, so the
// health check doesn't spend tokens.
func (a *Anthropic) Status(context.Context) (bool, string) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" && len(a.Options) == 0 {
		return false, "ANTHROPIC_API_KEY is not set"
	}
	return true, "Claude API (" + a.model() + ")"
}

func (a *Anthropic) tools() ([]anthropic.ToolUnionParam, error) {
	var out []anthropic.ToolUnionParam
	for _, t := range a.Registry.List() {
		var schema struct {
			Properties map[string]any `json:"properties"`
			Required   []string       `json:"required"`
		}
		if err := json.Unmarshal(t.InputSchema, &schema); err != nil {
			return nil, fmt.Errorf("tool %s schema: %w", t.Name, err)
		}
		tp := anthropic.ToolParam{
			Name:        t.Name,
			Description: anthropic.String(t.Description),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties:  schema.Properties,
				Required:    schema.Required,
				ExtraFields: map[string]any{"additionalProperties": false},
			},
		}
		out = append(out, anthropic.ToolUnionParam{OfTool: &tp})
	}
	return out, nil
}

func (a *Anthropic) params(system string, msgs []anthropic.MessageParam) anthropic.MessageNewParams {
	adaptive := anthropic.ThinkingConfigAdaptiveParam{}
	return anthropic.MessageNewParams{
		Model:     a.model(),
		MaxTokens: 16000,
		// Cache the stable prompt (and the tools before it) across turns.
		System:       []anthropic.TextBlockParam{{Text: system, CacheControl: anthropic.NewCacheControlEphemeralParam()}},
		Thinking:     anthropic.ThinkingConfigParamUnion{OfAdaptive: &adaptive},
		OutputConfig: anthropic.OutputConfigParam{Effort: anthropic.OutputConfigEffortMedium},
		Messages:     msgs,
	}
}

// Turn implements Backend with a manual, streaming tool loop.
func (a *Anthropic) Turn(ctx context.Context, req TurnRequest, onText func(string)) (TurnResult, error) {
	toolDefs, err := a.tools()
	if err != nil {
		return TurnResult{}, err
	}
	var msgs []anthropic.MessageParam
	for _, m := range req.History {
		switch m.Role {
		case "user":
			msgs = append(msgs, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Text)))
		case "assistant":
			msgs = append(msgs, anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Text)))
		}
	}
	msgs = append(msgs, anthropic.NewUserMessage(anthropic.NewTextBlock(req.User)))

	c := a.client()
	var text strings.Builder
	for range maxToolRounds {
		params := a.params(req.System, msgs)
		params.Tools = toolDefs
		stream := c.Messages.NewStreaming(ctx, params)
		msg := anthropic.Message{}
		for stream.Next() {
			ev := stream.Current()
			if err := msg.Accumulate(ev); err != nil {
				return TurnResult{Text: text.String()}, err
			}
			switch e := ev.AsAny().(type) {
			case anthropic.ContentBlockStartEvent:
				if e.ContentBlock.Type == "text" && text.Len() > 0 {
					text.WriteString("\n\n")
					onText("\n\n")
				}
			case anthropic.ContentBlockDeltaEvent:
				if d, ok := e.Delta.AsAny().(anthropic.TextDelta); ok {
					text.WriteString(d.Text)
					onText(d.Text)
				}
			}
		}
		if err := stream.Err(); err != nil {
			return TurnResult{Text: text.String()}, err
		}

		switch msg.StopReason {
		case anthropic.StopReasonRefusal:
			if text.Len() > 0 {
				text.WriteString("\n\n")
				onText("\n\n")
			}
			text.WriteString(refusalText)
			onText(refusalText)
			return TurnResult{Text: text.String()}, nil
		case anthropic.StopReasonToolUse:
		default:
			return TurnResult{Text: strings.TrimSpace(text.String())}, nil
		}

		msgs = append(msgs, msg.ToParam())
		var results []anthropic.ContentBlockParamUnion
		for _, block := range msg.Content {
			if tu, ok := block.AsAny().(anthropic.ToolUseBlock); ok {
				out, err := a.Registry.Call(ctx, tu.Name, json.RawMessage(tu.JSON.Input.Raw()))
				if err != nil {
					results = append(results, anthropic.NewToolResultBlock(tu.ID, tools.ErrorText(err), true))
				} else {
					results = append(results, anthropic.NewToolResultBlock(tu.ID, tools.ResultText(out), false))
				}
			}
		}
		msgs = append(msgs, anthropic.NewUserMessage(results...))
	}
	return TurnResult{Text: strings.TrimSpace(text.String())}, fmt.Errorf("stopped after %d tool rounds", maxToolRounds)
}

// Complete implements Backend.
func (a *Anthropic) Complete(ctx context.Context, system, user string) (string, error) {
	c := a.client()
	stream := c.Messages.NewStreaming(ctx, a.params(system, []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(user)),
	}))
	msg := anthropic.Message{}
	for stream.Next() {
		if err := msg.Accumulate(stream.Current()); err != nil {
			return "", err
		}
	}
	if err := stream.Err(); err != nil {
		return "", err
	}
	if msg.StopReason == anthropic.StopReasonRefusal {
		return "", errors.New("the model declined to draft this retro")
	}
	var b strings.Builder
	for _, block := range msg.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			b.WriteString(t.Text)
		}
	}
	return strings.TrimSpace(b.String()), nil
}
