package curator

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jmelahman/agilecbt/internal/db"
)

// mcpServerName is what the coach's tools are registered as in Claude Code,
// which names them mcp__<server>__<tool>.
const mcpServerName = "agilecbt"

// maxTurns bounds one reply's tool rounds, as maxToolRounds does for the
// OpenAI backend, so a failing tool can't loop against the plan's limits.
const maxTurns = 12

// ClaudeModels are the model aliases Claude Code accepts, offered in
// Settings → Coach.
var ClaudeModels = []string{"opus", "sonnet", "haiku", "fable"}

// ClaudeCode runs the curator through the Claude Code CLI (`claude -p`), so
// replies count against the signed-in Claude subscription rather than an
// API key. Each turn is one non-interactive run with Claude Code's own tools
// off; the coach's tools reach the app over its MCP endpoint.
//
// With a WorkDir, a check-in's turns continue one Claude Code session
// (--resume), so earlier turns come from the prompt cache. Without one, or
// whenever the session no longer matches the stored history (a restart, a
// failed turn, a settings change), the turn starts a new session with the
// history as a transcript. Crisis turns always start fresh and aren't kept.
//
// Claude Code saves each session's transcript under ~/.claude/projects;
// ClaudeCode deletes a session's file once it's replaced or its check-in is
// deleted, and PruneClaudeSessions clears the rest when the server starts.
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
	// WorkDir is where Claude Code runs and so where it files its sessions
	// (under ~/.claude/projects). Empty turns sessions off.
	WorkDir string
	// OnToolCall, when set, observes every tool call the model makes.
	OnToolCall func(ToolCall)

	mu sync.Mutex
	// sessions maps a check-in to its Claude Code session.
	sessions map[int64]claudeSession
}

type claudeSession struct {
	id string
	// messages is how many stored messages the session has seen. A turn
	// resumes it only when the history is exactly that long and is what
	// the session saw: earlier messages and the reply hash to seen, and the
	// stored user message ends the prompt it was sent.
	messages int
	seen     string
	prompt   string
}

// continues says whether history is what the session has seen.
func (s claudeSession) continues(history []db.Message) bool {
	n := len(history)
	if n != s.messages || n < 2 || history[n-2].Role != "user" || !strings.HasSuffix(s.prompt, history[n-2].Text) {
		return false
	}
	return s.seen == digest(append(history[:n-2:n-2], history[n-1]))
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

// Turn implements Backend.
func (c *ClaudeCode) Turn(ctx context.Context, req TurnRequest, onText func(string)) (TurnResult, error) {
	var history []db.Message
	for _, m := range req.History {
		if m.Role == "user" || m.Role == "assistant" {
			history = append(history, m)
		}
	}
	o := claudeRun{system: req.System, onText: onText}
	if !req.NoTools && c.MCPURL != "" {
		o.mcpURL = c.MCPURL
		if req.CheckinID != 0 {
			// Attribute tool calls to this check-in, as undoable chips.
			o.mcpURL += "?checkin=" + strconv.FormatInt(req.CheckinID, 10)
		}
	}
	// A crisis turn has no tools, so it can't continue a session whose
	// history has tool calls; it starts fresh and isn't kept, which also
	// keeps its safety directive out of later turns.
	keep := c.WorkDir != "" && req.CheckinID != 0 && !req.NoTools
	prev, hadPrev := c.session(req.CheckinID)
	if keep {
		o.persist = true
		if hadPrev && prev.continues(history) {
			// Each turn brings a fresh context block; the session still has
			// the old ones.
			o.resume, o.prompt = prev.id, "(This message's <context> replaces any earlier one.)\n\n"+req.User
		}
	}
	if o.resume == "" {
		o.prompt = transcript(history, req.User)
	}
	text, id, err := c.run(ctx, o)
	if o.resume != "" && err != nil && text == "" && strings.Contains(err.Error(), "No conversation found") {
		// Claude Code lost the session; start over from the transcript.
		o.resume, o.prompt = "", transcript(history, req.User)
		text, id, err = c.run(ctx, o)
	}
	if hadPrev && (!keep || err != nil || id != prev.id) {
		c.Forget(req.CheckinID)
	}
	if keep && err == nil && id != "" {
		c.mu.Lock()
		if c.sessions == nil {
			c.sessions = map[int64]claudeSession{}
		}
		// The app stores this turn's message and reply next.
		seen := digest(append(slices.Clone(history), db.Message{Role: "assistant", Text: text}))
		c.sessions[req.CheckinID] = claudeSession{id: id, messages: len(history) + 2, seen: seen, prompt: req.User}
		c.mu.Unlock()
	} else if keep && id != "" {
		c.removeSession(id)
	}
	return TurnResult{Text: text}, err
}

// Forget drops a check-in's session and deletes its transcript from disk.
func (c *ClaudeCode) Forget(checkinID int64) {
	c.mu.Lock()
	s, ok := c.sessions[checkinID]
	delete(c.sessions, checkinID)
	c.mu.Unlock()
	if ok {
		c.removeSession(s.id)
	}
}

func (c *ClaudeCode) removeSession(id string) {
	if dir := sessionDir(c.WorkDir); dir != "" && id != "" && !strings.ContainsAny(id, `/\.`) {
		os.Remove(filepath.Join(dir, id+".jsonl"))
		os.RemoveAll(filepath.Join(dir, id))
	}
}

// PruneClaudeSessions deletes the transcripts Claude Code kept for sessions
// run in workDir. Sessions are only resumed within one server run, so at
// startup every one of them is stale.
func PruneClaudeSessions(workDir string) {
	if dir := sessionDir(workDir); dir != "" {
		os.RemoveAll(dir)
	}
}

var nonAlnum = regexp.MustCompile(`[^a-zA-Z0-9]`)

// sessionDir is where Claude Code files sessions run in workDir: its
// projects folder, named after the path with every other character a dash.
func sessionDir(workDir string) string {
	if workDir == "" {
		return ""
	}
	abs, err := filepath.Abs(workDir)
	if err != nil {
		return ""
	}
	config := os.Getenv("CLAUDE_CONFIG_DIR")
	if config == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		config = filepath.Join(home, ".claude")
	}
	return filepath.Join(config, "projects", nonAlnum.ReplaceAllString(abs, "-"))
}

// digest identifies a run of messages.
func digest(msgs []db.Message) string {
	h := sha256.New()
	for _, m := range msgs {
		fmt.Fprintf(h, "%s\x00%d\x00%s\x00", m.Role, len(m.Text), m.Text)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (c *ClaudeCode) session(checkinID int64) (claudeSession, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.sessions[checkinID]
	return s, ok
}

// transcript puts earlier messages into the prompt, for a new session.
func transcript(history []db.Message, user string) string {
	if len(history) == 0 {
		return user
	}
	var p strings.Builder
	p.WriteString("The conversation so far:\n\n<transcript>\n")
	for _, m := range history {
		fmt.Fprintf(&p, "<%s>\n%s\n</%s>\n", m.Role, m.Text, m.Role)
	}
	p.WriteString("</transcript>\n\nReply to the user's new message:\n\n")
	p.WriteString(user)
	return p.String()
}

// Complete implements Backend.
func (c *ClaudeCode) Complete(ctx context.Context, system, user string) (string, error) {
	text, _, err := c.run(ctx, claudeRun{system: system, prompt: user, onText: func(string) {}})
	return text, err
}

// claudeRun is one `claude -p` call.
type claudeRun struct {
	system, prompt string
	// mcpURL gives the model the app's tools when set.
	mcpURL string
	// persist saves the session in WorkDir; resume continues that session.
	persist bool
	resume  string
	onText  func(string)
}

// run is one `claude -p` call: the prompt goes in on stdin and the reply
// streams back as stream-json events. It returns the session id.
func (c *ClaudeCode) run(ctx context.Context, o claudeRun) (text, sessionID string, err error) {
	// A scratch directory holds the system prompt and the MCP config, which
	// has the secret, out of the process list. Claude Code runs there too
	// unless the session is kept, which keeps it from picking up a CLAUDE.md.
	dir, err := os.MkdirTemp("", "agilecbt-claude-")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(dir)
	sysFile := filepath.Join(dir, "system.md")
	if err := os.WriteFile(sysFile, []byte(o.system), 0o600); err != nil {
		return "", "", err
	}
	args := []string{
		"-p", "--output-format", "stream-json", "--verbose", "--include-partial-messages",
		"--system-prompt-file", sysFile,
		// No built-in tools, user/project settings, or account MCP servers.
		"--tools", "", "--setting-sources", "",
		"--strict-mcp-config", "--permission-mode", "dontAsk",
		"--max-turns", strconv.Itoa(maxTurns),
	}
	cwd := dir
	switch {
	case o.persist:
		if err := os.MkdirAll(c.WorkDir, 0o700); err != nil {
			return "", "", err
		}
		cwd = c.WorkDir
		if o.resume != "" {
			args = append(args, "--resume", o.resume)
		}
	default:
		args = append(args, "--no-session-persistence")
	}
	if m := c.model(); m != "" {
		args = append(args, "--model", m)
	}
	if e := c.effort(); e != "" {
		args = append(args, "--effort", e)
	}
	if o.mcpURL != "" {
		server := map[string]any{"type": "http", "url": o.mcpURL}
		if c.Secret != "" {
			server["headers"] = map[string]string{"Authorization": "Bearer " + c.Secret}
		}
		b, err := json.Marshal(map[string]any{"mcpServers": map[string]any{mcpServerName: server}})
		if err != nil {
			return "", "", err
		}
		mcpFile := filepath.Join(dir, "mcp.json")
		if err := os.WriteFile(mcpFile, b, 0o600); err != nil {
			return "", "", err
		}
		args = append(args, "--mcp-config", mcpFile, "--allowedTools", "mcp__"+mcpServerName)
	}

	cmd := exec.CommandContext(ctx, c.command(), args...)
	cmd.Dir = cwd
	cmd.Env = claudeEnv(os.Environ())
	// Don't wait forever on a child that keeps stdout open after claude
	// exits.
	cmd.WaitDelay = 5 * time.Second
	cmd.Stdin = strings.NewReader(o.prompt)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", err
	}
	if err := cmd.Start(); err != nil {
		return "", "", fmt.Errorf("start Claude Code: %w", err)
	}
	s := newClaudeStream(o.onText, c.OnToolCall)
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		s.handle(sc.Bytes())
	}
	scanErr := sc.Err()
	if scanErr != nil {
		// Keep claude from blocking on a full pipe so Wait can return.
		_, _ = io.Copy(io.Discard, stdout)
	}
	waitErr := cmd.Wait()
	text = strings.TrimSpace(s.text.String())
	switch {
	case ctx.Err() != nil:
		return text, "", ctx.Err()
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
		return text, "", errors.New("Claude Code: " + lastLine(cmp.Or(detail, "exited without a reply")))
	}
	if text == "" {
		// No partial messages arrived; the reply is only in the result.
		text = strings.TrimSpace(s.result)
		if text != "" {
			o.onText(text)
		}
	}
	return text, s.sessionID, nil
}

// claudeEnv is the server's environment without what would tie claude to
// a Claude Code session the server was started from, or bill an API key
// instead of the signed-in subscription.
func claudeEnv(environ []string) []string {
	var env []string
	for _, kv := range environ {
		name, _, _ := strings.Cut(kv, "=")
		switch {
		case name == "CLAUDECODE", name == "CLAUDE_PID", name == "CLAUDE_EFFORT",
			name == "ANTHROPIC_API_KEY", name == "ANTHROPIC_AUTH_TOKEN":
			continue
		case strings.HasPrefix(name, "CLAUDE_CODE_") && name != "CLAUDE_CODE_OAUTH_TOKEN" && !strings.HasPrefix(name, "CLAUDE_CODE_USE_"):
			continue
		}
		env = append(env, kv)
	}
	return env
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
	sessionID string
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
	SessionID string   `json:"session_id"`
	Result    string   `json:"result"`
	IsError   bool     `json:"is_error"`
	Errors    []string `json:"errors"`
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
		s.sessionID = ev.SessionID
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
