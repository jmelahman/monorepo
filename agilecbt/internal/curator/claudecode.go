package curator

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// ClaudeCode runs the curator through headless Claude Code (`claude -p`),
// which uses the machine's Claude login (e.g. a Max subscription). Its only
// tools are this app's /mcp endpoint; built-in tools are disabled.
type ClaudeCode struct {
	// Bin is the claude executable (default "claude").
	Bin string
	// MCPURL is this server's /mcp endpoint as reachable from this machine.
	MCPURL string
	// Secret is sent as a bearer token to /mcp when auth is on.
	Secret string
	// Model optionally overrides Claude Code's default model.
	Model string
	// Dir is the working directory for claude, which scopes its session
	// storage. It must be stable across restarts for --resume to work.
	Dir string
}

// Name implements Backend.
func (c *ClaudeCode) Name() string { return "claude-code" }

func (c *ClaudeCode) bin() string {
	if c.Bin != "" {
		return c.Bin
	}
	return "claude"
}

// Status implements Backend: claude must be installed and logged in.
func (c *ClaudeCode) Status(ctx context.Context) (bool, string) {
	path, err := exec.LookPath(c.bin())
	if err != nil {
		return false, "the claude CLI is not installed or not on PATH"
	}
	out, err := exec.CommandContext(ctx, path, "auth", "status").Output()
	if err != nil {
		return false, "could not check Claude Code login (run `claude auth status`)"
	}
	var st struct {
		LoggedIn   bool   `json:"loggedIn"`
		AuthMethod string `json:"authMethod"`
	}
	if json.Unmarshal(out, &st) != nil {
		// Older CLIs print text; assume logged in and let a turn report
		// otherwise.
		return true, "claude CLI found"
	}
	if !st.LoggedIn {
		return false, "Claude Code is not logged in (run `claude` once and log in)"
	}
	return true, "Claude Code (" + st.AuthMethod + ")"
}

// baseArgs are shared by every invocation: no built-in tools, no user or
// project settings, and nothing that could prompt for permission.
func (c *ClaudeCode) baseArgs(system string) []string {
	args := []string{"-p", "--system-prompt", system, "--tools", "", "--setting-sources", "",
		"--strict-mcp-config", "--permission-prompts", "none"}
	if c.Model != "" {
		args = append(args, "--model", c.Model)
	}
	return args
}

// Turn implements Backend.
func (c *ClaudeCode) Turn(ctx context.Context, req TurnRequest, onText func(string)) (TurnResult, error) {
	if err := os.MkdirAll(c.Dir, 0o700); err != nil {
		return TurnResult{}, err
	}
	mcpConfig, cleanup, err := c.writeMCPConfig(req.CheckinID)
	if err != nil {
		return TurnResult{}, err
	}
	defer cleanup()

	args := append(c.baseArgs(req.System),
		"--output-format", "stream-json", "--verbose", "--include-partial-messages",
		"--mcp-config", mcpConfig, "--allowedTools", "mcp__agilecbt")
	session := req.Session
	if session == "" {
		session = uuid.NewString()
		args = append(args, "--session-id", session)
	} else {
		args = append(args, "--resume", session)
	}

	cmd := exec.CommandContext(ctx, c.bin(), args...)
	cmd.Dir = c.Dir
	cmd.Stdin = strings.NewReader(req.User)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return TurnResult{}, err
	}
	if err := cmd.Start(); err != nil {
		return TurnResult{}, fmt.Errorf("start claude: %w", err)
	}

	var text strings.Builder
	var result *ccResult
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		var line ccLine
		if json.Unmarshal(sc.Bytes(), &line) != nil {
			continue
		}
		switch line.Type {
		case "stream_event":
			ev := line.Event
			switch {
			case ev.Type == "content_block_start" && ev.ContentBlock.Type == "text" && text.Len() > 0:
				// A new text block after a tool call; keep paragraphs apart.
				text.WriteString("\n\n")
				onText("\n\n")
			case ev.Type == "content_block_delta" && ev.Delta.Type == "text_delta":
				text.WriteString(ev.Delta.Text)
				onText(ev.Delta.Text)
			}
		case "result":
			result = &ccResult{IsError: line.IsError, Result: line.Result, SessionID: line.SessionID, Subtype: line.Subtype}
		}
	}
	waitErr := cmd.Wait()

	if ctx.Err() != nil {
		return TurnResult{Text: text.String()}, ctx.Err()
	}
	if result == nil {
		return TurnResult{Text: text.String()}, fmt.Errorf("claude exited without a result (%v): %s", waitErr, tail(stderr.String()))
	}
	if result.IsError {
		msg := result.Result
		if msg == "" {
			msg = result.Subtype
		}
		// Don't save a session id that never started.
		return TurnResult{Text: text.String()}, fmt.Errorf("claude: %s", msg)
	}
	if result.SessionID != "" {
		session = result.SessionID
	}
	return TurnResult{Text: text.String(), Session: session}, nil
}

// Complete implements Backend with a one-shot, session-less, tool-free call.
func (c *ClaudeCode) Complete(ctx context.Context, system, user string) (string, error) {
	if err := os.MkdirAll(c.Dir, 0o700); err != nil {
		return "", err
	}
	args := append(c.baseArgs(system), "--output-format", "json", "--no-session-persistence")
	cmd := exec.CommandContext(ctx, c.bin(), args...)
	cmd.Dir = c.Dir
	cmd.Stdin = strings.NewReader(user)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	var res ccLine
	if jerr := json.Unmarshal(out, &res); jerr != nil {
		if err == nil {
			err = jerr
		}
		return "", fmt.Errorf("claude: %v: %s", err, tail(stderr.String()))
	}
	if res.IsError {
		return "", fmt.Errorf("claude: %s", res.Result)
	}
	return res.Result, nil
}

// writeMCPConfig writes a private, per-turn MCP config pointing claude at
// this server. A file (not an argument) keeps the secret out of `ps`.
func (c *ClaudeCode) writeMCPConfig(checkinID int64) (string, func(), error) {
	u, err := url.Parse(c.MCPURL)
	if err != nil {
		return "", nil, fmt.Errorf("bad MCP URL %q: %w", c.MCPURL, err)
	}
	q := u.Query()
	q.Set("checkin", strconv.FormatInt(checkinID, 10))
	u.RawQuery = q.Encode()
	server := map[string]any{"type": "http", "url": u.String()}
	if c.Secret != "" {
		server["headers"] = map[string]string{"Authorization": "Bearer " + c.Secret}
	}
	b, err := json.Marshal(map[string]any{"mcpServers": map[string]any{"agilecbt": server}})
	if err != nil {
		return "", nil, err
	}
	f, err := os.CreateTemp(c.Dir, "mcp-*.json")
	if err != nil {
		return "", nil, err
	}
	path := f.Name()
	cleanup := func() { os.Remove(path) }
	if _, err := f.Write(b); err != nil {
		f.Close()
		cleanup()
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	return filepath.Clean(path), cleanup, nil
}

// ccLine is the subset of claude's stream-json/json output we read.
type ccLine struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	IsError   bool   `json:"is_error"`
	Result    string `json:"result"`
	SessionID string `json:"session_id"`
	Event     struct {
		Type         string `json:"type"`
		ContentBlock struct {
			Type string `json:"type"`
		} `json:"content_block"`
		Delta struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"delta"`
	} `json:"event"`
}

type ccResult struct {
	IsError   bool
	Result    string
	SessionID string
	Subtype   string
}

func tail(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 500 {
		s = "…" + s[len(s)-500:]
	}
	if s == "" {
		return "(no output)"
	}
	return s
}
