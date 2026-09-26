package curator

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/jmelahman/agilecbt/internal/tools"
)

// maxToolRounds bounds the model→tool→model loop per user turn.
const maxToolRounds = 10

// OpenAI runs the curator against any OpenAI-compatible chat completions
// API: a local Ollama (/v1), OpenRouter, llama.cpp, vLLM, or OpenAI. Tools
// run in-process through the registry.
type OpenAI struct {
	BaseURL string // e.g. http://localhost:11434/v1
	APIKey  string // sent as a bearer token when set
	Model   string
	// ReasoningEffort is sent as reasoning_effort when set. "none" keeps
	// hybrid models like Qwen3 from spending a thinking pass before each
	// reply; check-in chat wants short, tool-heavy answers.
	ReasoningEffort string
	Registry        *tools.Registry
	HTTP            *http.Client
}

// Name implements Backend.
func (o *OpenAI) Name() string { return "openai" }

func (o *OpenAI) client() *http.Client {
	if o.HTTP != nil {
		return o.HTTP
	}
	return http.DefaultClient
}

// host is BaseURL's host, for status messages.
func (o *OpenAI) host() string {
	if u, err := url.Parse(o.BaseURL); err == nil && u.Host != "" {
		return u.Host
	}
	return o.BaseURL
}

func (o *OpenAI) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(o.BaseURL, "/")+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if o.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.APIKey)
	}
	// Attribution for OpenRouter's app rankings; other servers ignore it.
	req.Header.Set("X-Title", "AgileCBT")
	return req, nil
}

// errNoList means the server doesn't list its models.
var errNoList = errors.New("the server doesn't list its models")

// Models lists the model ids the server offers, sorted.
func (o *OpenAI) Models(ctx context.Context) ([]string, error) {
	ids, _, err := o.models(ctx)
	if errors.Is(err, errNoList) {
		return []string{}, nil
	}
	return ids, err
}

// models fetches GET /models, returning the HTTP status too.
func (o *OpenAI) models(ctx context.Context) ([]string, int, error) {
	req, err := o.newRequest(ctx, http.MethodGet, "/models", nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := o.client().Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("not reachable at %s", o.BaseURL)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("%s: %s", o.host(), resp.Status)
	}
	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil || len(list.Data) == 0 {
		return nil, resp.StatusCode, errNoList
	}
	ids := make([]string, 0, len(list.Data))
	for _, m := range list.Data {
		ids = append(ids, m.ID)
	}
	sort.Strings(ids)
	return ids, resp.StatusCode, nil
}

// Status implements Backend: the server must answer and, when it lists its
// models, include the configured one.
func (o *OpenAI) Status(ctx context.Context) (bool, string) {
	if o.Model == "" {
		return false, "no model is set"
	}
	ready := o.Model + " at " + o.host()
	ids, code, err := o.models(ctx)
	switch {
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		if o.APIKey == "" {
			return false, o.host() + " needs an API key"
		}
		return false, o.host() + " rejected the API key"
	case errors.Is(err, errNoList) || code == http.StatusNotFound:
		// Not every server lists models; assume the model is there.
		return true, ready
	case err != nil:
		// Unreachable, or answering with an error such as 503 or 429.
		return false, err.Error()
	}
	for _, id := range ids {
		if id == o.Model || (!strings.Contains(o.Model, ":") && id == o.Model+":latest") {
			return true, ready
		}
	}
	return false, fmt.Sprintf("%s doesn't have model %s (for Ollama, run `ollama pull %s`)", o.host(), o.Model, o.Model)
}

type oaiMessage struct {
	Role       string        `json:"role"`
	Content    *string       `json:"content"`
	ToolCalls  []oaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

func textMessage(role, text string) oaiMessage {
	return oaiMessage{Role: role, Content: &text}
}

type oaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type oaiTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function"`
}

func (o *OpenAI) tools() []oaiTool {
	var out []oaiTool
	for _, t := range o.Registry.List() {
		var ot oaiTool
		ot.Type = "function"
		ot.Function.Name = t.Name
		ot.Function.Description = t.Description
		ot.Function.Parameters = t.InputSchema
		out = append(out, ot)
	}
	return out
}

// Turn implements Backend with a manual tool loop.
func (o *OpenAI) Turn(ctx context.Context, req TurnRequest, onText func(string)) (TurnResult, error) {
	msgs := []oaiMessage{textMessage("system", req.System)}
	for _, m := range req.History {
		if m.Role == "user" || m.Role == "assistant" {
			msgs = append(msgs, textMessage(m.Role, m.Text))
		}
	}
	msgs = append(msgs, textMessage("user", req.User))

	var text strings.Builder
	for range maxToolRounds {
		if text.Len() > 0 {
			// Separate text from before and after a tool round.
			sep := "\n\n"
			text.WriteString(sep)
			onText(sep)
		}
		before := text.Len()
		reply, err := o.chat(ctx, msgs, true, func(delta string) {
			text.WriteString(delta)
			onText(delta)
		})
		if err != nil {
			return TurnResult{Text: text.String()}, err
		}
		if text.Len() == before && before > 0 {
			// Nothing new was said; drop the separator from the saved text.
			s := strings.TrimSuffix(text.String(), "\n\n")
			text.Reset()
			text.WriteString(s)
		}
		if len(reply.ToolCalls) == 0 {
			return TurnResult{Text: strings.TrimSpace(text.String())}, nil
		}
		msgs = append(msgs, reply)
		for _, call := range reply.ToolCalls {
			args := json.RawMessage(call.Function.Arguments)
			if strings.TrimSpace(call.Function.Arguments) == "" {
				args = json.RawMessage("{}")
			}
			result := runTool(ctx, o.Registry, call.Function.Name, args)
			msgs = append(msgs, oaiMessage{Role: "tool", ToolCallID: call.ID, Content: &result})
		}
	}
	return TurnResult{Text: strings.TrimSpace(text.String())}, fmt.Errorf("stopped after %d tool rounds", maxToolRounds)
}

// Complete implements Backend.
func (o *OpenAI) Complete(ctx context.Context, system, user string) (string, error) {
	var text strings.Builder
	_, err := o.chat(ctx, []oaiMessage{textMessage("system", system), textMessage("user", user)}, false,
		func(s string) { text.WriteString(s) })
	return strings.TrimSpace(text.String()), err
}

// chat streams one /chat/completions call, returning the assembled assistant
// message.
func (o *OpenAI) chat(ctx context.Context, msgs []oaiMessage, withTools bool, onText func(string)) (oaiMessage, error) {
	body := map[string]any{"model": o.Model, "messages": msgs, "stream": true}
	if o.ReasoningEffort != "" {
		body["reasoning_effort"] = o.ReasoningEffort
	}
	if withTools {
		body["tools"] = o.tools()
	}
	b, err := json.Marshal(body)
	if err != nil {
		return oaiMessage{}, err
	}
	req, err := o.newRequest(ctx, http.MethodPost, "/chat/completions", bytes.NewReader(b))
	if err != nil {
		return oaiMessage{}, err
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := o.client().Do(req)
	if err != nil {
		return oaiMessage{}, fmt.Errorf("%s: %w", o.host(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return oaiMessage{}, fmt.Errorf("%s: %s: %s", o.host(), resp.Status, strings.TrimSpace(string(raw)))
	}

	var content strings.Builder
	var calls []oaiToolCall
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		// Server-sent events: "data: {...}" lines; skip comments and blanks.
		data, ok := strings.CutPrefix(sc.Text(), "data:")
		if !ok {
			continue
		}
		data = strings.TrimSpace(data)
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content   string `json:"content"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Error != nil {
			return oaiMessage{}, fmt.Errorf("%s: %s", o.host(), chunk.Error.Message)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		d := chunk.Choices[0].Delta
		if d.Content != "" {
			content.WriteString(d.Content)
			onText(d.Content)
		}
		// Tool calls arrive in fragments keyed by index: the first carries
		// the id and name, later ones append to the arguments.
		for _, tc := range d.ToolCalls {
			for len(calls) <= tc.Index {
				calls = append(calls, oaiToolCall{Type: "function"})
			}
			c := &calls[tc.Index]
			if tc.ID != "" {
				c.ID = tc.ID
			}
			c.Function.Name += tc.Function.Name
			c.Function.Arguments += tc.Function.Arguments
		}
	}
	if err := sc.Err(); err != nil {
		return oaiMessage{}, err
	}
	for i := range calls {
		if calls[i].ID == "" {
			calls[i].ID = fmt.Sprintf("call_%d", i)
		}
	}
	text := content.String()
	return oaiMessage{Role: "assistant", Content: &text, ToolCalls: calls}, nil
}

// runTool calls a registry tool and renders the result or error as text for
// the model.
func runTool(ctx context.Context, reg *tools.Registry, name string, args json.RawMessage) string {
	// Some models send arguments as a JSON-encoded string.
	var s string
	if json.Unmarshal(args, &s) == nil {
		args = json.RawMessage(s)
	}
	out, err := reg.Call(ctx, name, args)
	if err != nil {
		return tools.ErrorText(err)
	}
	return tools.ResultText(out)
}
