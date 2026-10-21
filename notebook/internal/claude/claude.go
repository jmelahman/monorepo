// Package claude runs prompts through the Claude Code CLI (`claude -p`), so
// they count against the signed-in Claude subscription rather than an API
// key. It is trimmed from agilecbt's curator backend.
package claude

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// Models are the model aliases Claude Code accepts.
var Models = []string{"opus", "sonnet", "haiku", "fable"}

// Runner runs `claude -p`.
type Runner struct {
	// Command is the claude binary (default "claude" on $PATH).
	Command string
	// Model is a Claude model alias or id; empty uses Claude Code's default.
	Model string
	// Effort maps onto --effort: low, medium, high, xhigh or max.
	Effort string
	// WorkDir is where kept sessions run, and so where Claude Code files
	// them (under ~/.claude/projects). Empty turns sessions off.
	WorkDir string
}

func (r *Runner) command() string { return cmp.Or(r.Command, "claude") }

// model returns the --model to pass, or "" for Claude Code's default.
func (r *Runner) model() string {
	m := strings.TrimSpace(r.Model)
	if slices.Contains(Models, m) || strings.HasPrefix(m, "claude-") {
		return m
	}
	return ""
}

// Status reports whether claude is installed and signed in.
func (r *Runner) Status(ctx context.Context) (ok bool, detail string) {
	path, err := exec.LookPath(r.command())
	if err != nil {
		return false, "Claude Code isn't installed (no `" + r.command() + "` on PATH)"
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
	return true, fmt.Sprintf("%s via Claude Code (%s)", cmp.Or(r.model(), "default model"), cmp.Or(st.SubscriptionType, st.AuthMethod))
}

// Request is one `claude -p` call.
type Request struct {
	System, Prompt string
	// Tools are the built-in tools allowed, such as WebSearch; none by default.
	Tools []string
	// Schema, when set, asks for structured output that matches it, returned
	// in Result.Structured.
	Schema json.RawMessage
	// Keep saves the session in the Runner's WorkDir so that a later request
	// can Resume it; Resume continues a kept session.
	Keep   bool
	Resume string
	// OnText receives the reply's text as it streams.
	OnText func(string)
	// OnTool observes each tool call as the model makes it.
	OnTool func(name string, input json.RawMessage)
}

// Result is what a call returned.
type Result struct {
	Text       string
	Structured json.RawMessage
	// SessionID is set when the session was kept.
	SessionID string
}

// ErrNoSession is returned when Resume names a session Claude Code no
// longer has.
var ErrNoSession = errors.New("the Claude Code session was lost")

// Run is one `claude -p` call: the prompt goes in on stdin and the reply
// streams back as stream-json events.
func (r *Runner) Run(ctx context.Context, req Request) (Result, error) {
	// A scratch directory holds the system prompt and the schema, out of the
	// process list. Claude Code runs there too unless the session is kept,
	// which keeps it from picking up a CLAUDE.md.
	dir, err := os.MkdirTemp("", "notebook-claude-")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(dir)
	sysFile := filepath.Join(dir, "system.md")
	if err := os.WriteFile(sysFile, []byte(req.System), 0o600); err != nil {
		return Result{}, err
	}
	args := []string{
		"-p", "--output-format", "stream-json", "--verbose", "--include-partial-messages",
		"--system-prompt-file", sysFile,
		// Only the tools asked for, and no user/project settings or MCP servers.
		"--tools", strings.Join(req.Tools, ","), "--setting-sources", "",
		"--strict-mcp-config", "--permission-mode", "dontAsk",
	}
	if len(req.Tools) > 0 {
		args = append(args, "--allowedTools", strings.Join(req.Tools, ","))
	}
	if len(req.Schema) > 0 {
		args = append(args, "--json-schema", string(req.Schema))
	}
	cwd := dir
	if req.Keep || req.Resume != "" {
		if r.WorkDir == "" {
			return Result{}, errors.New("claude: sessions need a WorkDir")
		}
		if err := os.MkdirAll(r.WorkDir, 0o700); err != nil {
			return Result{}, err
		}
		cwd = r.WorkDir
		if req.Resume != "" {
			args = append(args, "--resume", req.Resume)
		}
	} else {
		args = append(args, "--no-session-persistence")
	}
	if m := r.model(); m != "" {
		args = append(args, "--model", m)
	}
	if r.Effort != "" {
		args = append(args, "--effort", r.Effort)
	}

	cmd := exec.CommandContext(ctx, r.command(), args...)
	cmd.Dir = cwd
	cmd.Stdin = strings.NewReader(req.Prompt)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, err
	}
	if err := cmd.Start(); err != nil {
		return Result{}, fmt.Errorf("start Claude Code: %w", err)
	}
	s := &stream{onText: req.OnText, onTool: req.OnTool}
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
	for sc.Scan() {
		s.handle(sc.Bytes())
	}
	scanErr := sc.Err()
	waitErr := cmd.Wait()
	res := Result{Text: strings.TrimSpace(s.text.String()), Structured: s.structured}
	switch {
	case ctx.Err() != nil:
		return res, ctx.Err()
	case s.resultErr != "" || !s.gotResult:
		detail := strings.TrimSpace(stderr.String())
		if err := cmp.Or(waitErr, scanErr); detail == "" && err != nil {
			detail = err.Error()
		}
		if s.gotResult {
			detail = cmp.Or(lastLine(detail), s.resultErr)
			if !strings.Contains(detail, s.resultErr) {
				detail = s.resultErr + ": " + detail
			}
		}
		if req.Resume != "" && strings.Contains(detail, "No conversation found") {
			return res, ErrNoSession
		}
		return res, errors.New("Claude Code: " + lastLine(cmp.Or(detail, "exited without a reply")))
	}
	if res.Text == "" {
		res.Text = strings.TrimSpace(s.result)
	}
	if len(req.Schema) > 0 && len(res.Structured) == 0 {
		return res, errors.New("the Claude Code reply had no structured output")
	}
	if req.Keep || req.Resume != "" {
		res.SessionID = s.sessionID
	}
	return res, nil
}

func lastLine(s string) string {
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		return s[i+1:]
	}
	return s
}

// stream assembles a reply from Claude Code's stream-json events.
type stream struct {
	onText func(string)
	onTool func(string, json.RawMessage)
	text   strings.Builder
	// sep is set after a tool call, so the next text starts a new paragraph.
	sep        bool
	gotResult  bool
	result     string
	resultErr  string
	structured json.RawMessage
	sessionID  string
}

type event struct {
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
		Content []struct {
			Type  string          `json:"type"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
	} `json:"message"`
	SessionID        string          `json:"session_id"`
	Result           string          `json:"result"`
	StructuredOutput json.RawMessage `json:"structured_output"`
	IsError          bool            `json:"is_error"`
	Errors           []string        `json:"errors"`
}

func (s *stream) handle(line []byte) {
	var ev event
	if json.Unmarshal(line, &ev) != nil {
		return
	}
	switch ev.Type {
	case "stream_event":
		if ev.Event.Type == "content_block_delta" && ev.Event.Delta.Type == "text_delta" && ev.Event.Delta.Text != "" {
			text := ev.Event.Delta.Text
			if s.sep && s.text.Len() > 0 {
				text = "\n\n" + text
			}
			s.sep = false
			s.text.WriteString(text)
			if s.onText != nil {
				s.onText(text)
			}
		}
	case "assistant":
		for _, b := range ev.Message.Content {
			// StructuredOutput is how --json-schema replies; it is the
			// answer, not a step on the way to one.
			if b.Type != "tool_use" || b.Name == "StructuredOutput" {
				continue
			}
			s.sep = true
			if s.onTool != nil {
				s.onTool(b.Name, b.Input)
			}
		}
	case "result":
		s.gotResult = true
		s.sessionID = ev.SessionID
		s.result = ev.Result
		if len(ev.StructuredOutput) > 0 && string(ev.StructuredOutput) != "null" {
			s.structured = ev.StructuredOutput
		}
		if ev.IsError || ev.Subtype != "success" {
			s.resultErr = cmp.Or(strings.Join(ev.Errors, "; "), ev.Result, ev.Subtype)
		}
	}
}
