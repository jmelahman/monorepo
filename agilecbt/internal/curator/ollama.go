package curator

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/jmelahman/agilecbt/internal/tools"
)

// maxToolRounds bounds the model→tool→model loop per user turn.
const maxToolRounds = 10

// Ollama runs the curator against a local Ollama server, so check-ins never
// leave the machine. Tools run in-process through the registry.
type Ollama struct {
	Host     string // e.g. http://localhost:11434
	Model    string // default qwen3:14b
	Registry *tools.Registry
	HTTP     *http.Client
}

// DefaultOllamaModel is solid at tool calling while fitting on one GPU.
const DefaultOllamaModel = "qwen3:14b"

// Name implements Backend.
func (o *Ollama) Name() string { return "ollama" }

func (o *Ollama) model() string {
	if o.Model != "" {
		return o.Model
	}
	return DefaultOllamaModel
}

func (o *Ollama) client() *http.Client {
	if o.HTTP != nil {
		return o.HTTP
	}
	return http.DefaultClient
}

// Status implements Backend: Ollama must answer and have the model pulled.
func (o *Ollama) Status(ctx context.Context) (bool, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.Host+"/api/tags", nil)
	if err != nil {
		return false, err.Error()
	}
	resp, err := o.client().Do(req)
	if err != nil {
		return false, "Ollama is not reachable at " + o.Host
	}
	defer resp.Body.Close()
	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return false, "unexpected response from Ollama"
	}
	want := o.model()
	for _, m := range tags.Models {
		if m.Name == want || (!strings.Contains(want, ":") && m.Name == want+":latest") {
			return true, "Ollama (" + want + ")"
		}
	}
	return false, fmt.Sprintf("model %s is not pulled (run `ollama pull %s`)", want, want)
}

type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
	ToolName  string           `json:"tool_name,omitempty"`
}

type ollamaToolCall struct {
	Function struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"function"`
}

type ollamaTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function"`
}

func (o *Ollama) tools() []ollamaTool {
	var out []ollamaTool
	for _, t := range o.Registry.List() {
		var ot ollamaTool
		ot.Type = "function"
		ot.Function.Name = t.Name
		ot.Function.Description = t.Description
		ot.Function.Parameters = t.InputSchema
		out = append(out, ot)
	}
	return out
}

// Turn implements Backend with a manual tool loop.
func (o *Ollama) Turn(ctx context.Context, req TurnRequest, onText func(string)) (TurnResult, error) {
	msgs := []ollamaMessage{{Role: "system", Content: req.System}}
	for _, m := range req.History {
		if m.Role == "user" || m.Role == "assistant" {
			msgs = append(msgs, ollamaMessage{Role: m.Role, Content: m.Text})
		}
	}
	msgs = append(msgs, ollamaMessage{Role: "user", Content: req.User})

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
			result := runTool(ctx, o.Registry, call.Function.Name, call.Function.Arguments)
			msgs = append(msgs, ollamaMessage{Role: "tool", ToolName: call.Function.Name, Content: result})
		}
	}
	return TurnResult{Text: strings.TrimSpace(text.String())}, fmt.Errorf("stopped after %d tool rounds", maxToolRounds)
}

// Complete implements Backend.
func (o *Ollama) Complete(ctx context.Context, system, user string) (string, error) {
	var text strings.Builder
	_, err := o.chat(ctx, []ollamaMessage{{Role: "system", Content: system}, {Role: "user", Content: user}}, false,
		func(s string) { text.WriteString(s) })
	return strings.TrimSpace(text.String()), err
}

// chat streams one /api/chat call, returning the assembled assistant message.
func (o *Ollama) chat(ctx context.Context, msgs []ollamaMessage, withTools bool, onText func(string)) (ollamaMessage, error) {
	// think:false keeps Qwen3 and other hybrid models from spending a
	// reasoning pass before each reply. Check-in chat wants short, tool-heavy
	// answers, not a hidden chain of thought.
	body := map[string]any{"model": o.model(), "messages": msgs, "stream": true, "think": false}
	if withTools {
		body["tools"] = o.tools()
	}
	b, err := json.Marshal(body)
	if err != nil {
		return ollamaMessage{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.Host+"/api/chat", bytes.NewReader(b))
	if err != nil {
		return ollamaMessage{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.client().Do(req)
	if err != nil {
		return ollamaMessage{}, fmt.Errorf("ollama: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return ollamaMessage{}, fmt.Errorf("ollama: %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	out := ollamaMessage{Role: "assistant"}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		var chunk struct {
			Message ollamaMessage `json:"message"`
			Done    bool          `json:"done"`
			Error   string        `json:"error"`
		}
		if err := json.Unmarshal(sc.Bytes(), &chunk); err != nil {
			continue
		}
		if chunk.Error != "" {
			return out, fmt.Errorf("ollama: %s", chunk.Error)
		}
		if chunk.Message.Content != "" {
			out.Content += chunk.Message.Content
			onText(chunk.Message.Content)
		}
		out.ToolCalls = append(out.ToolCalls, chunk.Message.ToolCalls...)
		if chunk.Done {
			break
		}
	}
	return out, sc.Err()
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
