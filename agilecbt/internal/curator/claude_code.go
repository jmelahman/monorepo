package curator

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// mcpServerName is what the coach's tools are registered as in Claude Code,
// which names them mcp__<server>__<tool>.
const mcpServerName = "agilecbt"

// ClaudeModels are the model aliases Claude Code accepts, offered in
// Settings → Coach.
var ClaudeModels = []string{"opus", "sonnet", "haiku", "fable"}

// ClaudeCode runs the curator through the Claude Code CLI (`claude -p`), so
// replies count against the signed-in Claude subscription rather than an
// API key. Each turn is one non-interactive run with Claude Code's own tools
// off; the coach's tools reach the app over its MCP endpoint.
type ClaudeCode struct {
	// Command is the claude binary (default "claude" on $PATH).
	Command string
	// Model is a Claude model alias or id; anything else, such as an Ollama
	// model left over from the openai backend, uses Claude Code's default.
	Model string
	// ReasoningEffort maps onto --effort.
	ReasoningEffort string
	// MCPURL is the app's own /mcp endpoint, e.g. http://127.0.0.1:8080/mcp.
	// Empty runs every turn without tools.
	MCPURL string
	// Secret is sent to MCPURL as a bearer token when set.
	Secret string
	// OnToolCall, when set, observes every tool call the model makes.
	OnToolCall func(ToolCall)
}

// Name implements Backend.
func (c *ClaudeCode) Name() string { return "claude-code" }

func (c *ClaudeCode) command() string { return cmp.Or(c.Command, "claude") }

// model returns the --model to pass, or "" for Claude Code's default.
func (c *ClaudeCode) model() string {
	m := strings.TrimSpace(c.Model)
	for _, alias := range ClaudeModels {
		if m == alias {
			return m
		}
	}
	if strings.HasPrefix(m, "claude-") {
		return m
	}
	return ""
}

// effort maps the shared reasoning_effort setting onto --effort, which has
// no "off": none and minimal become low.
func (c *ClaudeCode) effort() string {
	switch c.ReasoningEffort {
	case "none", "minimal":
		return "low"
	case "low", "medium", "high":
		return c.ReasoningEffort
	}
	return ""
}

// Status implements Backend: claude must be installed and signed in.
func (c *ClaudeCode) Status(ctx context.Context) (bool, string) {
	path, err := exec.LookPath(c.command())
	if err != nil {
		return false, "Claude Code isn't installed (no `" + c.command() + "` on PATH)"
	}
	out, err := exec.CommandContext(ctx, path, "auth", "status").Output()
	var st struct {
		LoggedIn         bool   `json:"loggedIn"`
		AuthMethod       string `json:"authMethod"`
		SubscriptionType string `json:"subscriptionType"`
	}
	if jerr := json.Unmarshal(out, &st); jerr != nil {
		if err == nil {
			err = jerr
		}
		return false, "couldn't check Claude Code's login: " + err.Error()
	}
	if !st.LoggedIn {
		return false, "Claude Code isn't signed in (run `claude` and log in)"
	}
	via := cmp.Or(st.SubscriptionType, st.AuthMethod)
	return true, fmt.Sprintf("%s via Claude Code (%s)", cmp.Or(c.model(), "default model"), via)
}

// Turn implements Backend. Claude Code can't be handed earlier assistant
// turns, so the history goes into the prompt as a transcript.
func (c *ClaudeCode) Turn(ctx context.Context, req TurnRequest, onText func(string)) (TurnResult, error) {
	var p strings.Builder
	if len(req.History) > 0 {
		p.WriteString("The conversation so far:\n\n<transcript>\n")
		for _, m := range req.History {
			if m.Role == "user" || m.Role == "assistant" {
				fmt.Fprintf(&p, "<%s>\n%s\n</%s>\n", m.Role, m.Text, m.Role)
			}
		}
		p.WriteString("</transcript>\n\nReply to the user's new message:\n\n")
	}
	p.WriteString(req.User)
	mcpURL := ""
	if !req.NoTools && c.MCPURL != "" {
		mcpURL = c.MCPURL
		if req.CheckinID != 0 {
			// Attribute tool calls to this check-in, as undoable chips.
			mcpURL += "?checkin=" + strconv.FormatInt(req.CheckinID, 10)
		}
	}
	text, err := c.run(ctx, req.System, p.String(), mcpURL, onText)
	return TurnResult{Text: text}, err
}

// Complete implements Backend.
func (c *ClaudeCode) Complete(ctx context.Context, system, user string) (string, error) {
	return c.run(ctx, system, user, "", func(string) {})
}

// run is one `claude -p` call: the prompt goes in on stdin and the reply
// streams back as stream-json events.
func (c *ClaudeCode) run(ctx context.Context, system, prompt, mcpURL string, onText func(string)) (string, error) {
	// A scratch working directory keeps Claude Code from picking up a
	// CLAUDE.md, and holds the system prompt and the MCP config, which has
	// the secret, out of the process list.
	dir, err := os.MkdirTemp("", "agilecbt-claude-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	sysFile := filepath.Join(dir, "system.md")
	if err := os.WriteFile(sysFile, []byte(system), 0o600); err != nil {
		return "", err
	}
	args := []string{
		"-p", "--output-format", "stream-json", "--verbose", "--include-partial-messages",
		"--system-prompt-file", sysFile,
		// No built-in tools, user/project settings, or saved sessions.
		"--tools", "", "--setting-sources", "", "--no-session-persistence",
		"--strict-mcp-config", "--permission-mode", "dontAsk",
	}
	if m := c.model(); m != "" {
		args = append(args, "--model", m)
	}
	if e := c.effort(); e != "" {
		args = append(args, "--effort", e)
	}
	if mcpURL != "" {
		server := map[string]any{"type": "http", "url": mcpURL}
		if c.Secret != "" {
			server["headers"] = map[string]string{"Authorization": "Bearer " + c.Secret}
		}
		b, err := json.Marshal(map[string]any{"mcpServers": map[string]any{mcpServerName: server}})
		if err != nil {
			return "", err
		}
		mcpFile := filepath.Join(dir, "mcp.json")
		if err := os.WriteFile(mcpFile, b, 0o600); err != nil {
			return "", err
		}
		args = append(args, "--mcp-config", mcpFile, "--allowedTools", "mcp__"+mcpServerName)
	}

	cmd := exec.CommandContext(ctx, c.command(), args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(prompt)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start Claude Code: %w", err)
	}
	s := newClaudeStream(onText, c.OnToolCall)
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		s.handle(sc.Bytes())
	}
	scanErr := sc.Err()
	waitErr := cmd.Wait()
	text := strings.TrimSpace(s.text.String())
	switch {
	case ctx.Err() != nil:
		return text, ctx.Err()
	case s.resultErr != "":
		return text, errors.New("Claude Code: " + s.resultErr)
	case !s.gotResult:
		detail := strings.TrimSpace(stderr.String())
		if err := cmp.Or(waitErr, scanErr); detail == "" && err != nil {
			detail = err.Error()
		}
		detail = cmp.Or(detail, "exited without a reply")
		return text, errors.New("Claude Code: " + lastLine(detail))
	}
	if text == "" {
		text = strings.TrimSpace(s.result)
	}
	return text, nil
}

func lastLine(s string) string {
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		return s[i+1:]
	}
	return s
}

// claudeStream assembles a reply from Claude Code's stream-json events.
type claudeStream struct {
	onText     func(string)
	onToolCall func(ToolCall)
	text       strings.Builder
	// sep is set after a tool call, so the next text starts a new paragraph.
	sep bool
	// calls holds tool_use blocks awaiting their result, by id.
	calls     map[string]ToolCall
	gotResult bool
	result    string
	resultErr string
}

func newClaudeStream(onText func(string), onToolCall func(ToolCall)) *claudeStream {
	return &claudeStream{onText: onText, onToolCall: onToolCall, calls: map[string]ToolCall{}}
}

type claudeBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	// Content is a tool_result's: a string or a list of text blocks.
	Content json.RawMessage `json:"content"`
	IsError bool            `json:"is_error"`
}

type claudeEvent struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	Event   struct {
		Type  string `json:"type"`
		Delta struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"delta"`
	} `json:"event"`
	Message struct {
		Content []claudeBlock `json:"content"`
	} `json:"message"`
	Result  string   `json:"result"`
	IsError bool     `json:"is_error"`
	Errors  []string `json:"errors"`
}

func (s *claudeStream) handle(line []byte) {
	var ev claudeEvent
	if json.Unmarshal(line, &ev) != nil {
		return
	}
	switch ev.Type {
	case "stream_event":
		if ev.Event.Type == "content_block_delta" && ev.Event.Delta.Type == "text_delta" && ev.Event.Delta.Text != "" {
			if s.sep && s.text.Len() > 0 {
				s.text.WriteString("\n\n")
				s.onText("\n\n")
			}
			s.sep = false
			s.text.WriteString(ev.Event.Delta.Text)
			s.onText(ev.Event.Delta.Text)
		}
	case "assistant":
		for _, b := range ev.Message.Content {
			if b.Type == "tool_use" {
				s.sep = true
				s.calls[b.ID] = ToolCall{Name: strings.TrimPrefix(b.Name, "mcp__"+mcpServerName+"__"), Args: b.Input}
			}
		}
	case "user":
		for _, b := range ev.Message.Content {
			call, ok := s.calls[b.ToolUseID]
			if b.Type != "tool_result" || !ok {
				continue
			}
			delete(s.calls, b.ToolUseID)
			call.Result = toolResultText(b.Content)
			if b.IsError {
				call.Err = errors.New(call.Result)
			}
			if s.onToolCall != nil {
				s.onToolCall(call)
			}
		}
	case "result":
		s.gotResult = true
		s.result = ev.Result
		if ev.IsError || ev.Subtype != "success" {
			s.resultErr = cmp.Or(strings.Join(ev.Errors, "; "), ev.Result, ev.Subtype)
		}
	}
}

func toolResultText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []claudeBlock
	if json.Unmarshal(raw, &blocks) != nil {
		return string(raw)
	}
	var b strings.Builder
	for _, bl := range blocks {
		b.WriteString(bl.Text)
	}
	return b.String()
}

// SelfMCPURL is the /mcp URL a server listening on addr can reach itself at.
func SelfMCPURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		host, port = addr, "80"
	}
	if host == "" || net.ParseIP(host).IsUnspecified() {
		host = "127.0.0.1"
	}
	return (&url.URL{Scheme: "http", Host: net.JoinHostPort(host, port), Path: "/mcp"}).String()
}
