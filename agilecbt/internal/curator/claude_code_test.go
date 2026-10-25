package curator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jmelahman/agilecbt/internal/db"
)

// scriptedClaude writes a stand-in for the claude binary that records its
// args, stdin, working directory and MCP config into dir and prints events,
// one per line. Given --resume, it prints resume.jsonl instead, when the
// test has written one.
func scriptedClaude(t *testing.T, events ...string) (cmd, dir string) {
	t.Helper()
	dir = t.TempDir()
	out := filepath.Join(dir, "events.jsonl")
	if err := os.WriteFile(out, []byte(strings.Join(events, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	script := `#!/bin/sh
printf '%s\n' "$@" > "` + dir + `/args"
cat > "` + dir + `/stdin"
pwd > "` + dir + `/cwd"
echo x >> "` + dir + `/runs"
events="` + out + `"
while [ $# -gt 0 ]; do
	case "$1" in
		--resume) [ -f "` + dir + `/resume.jsonl" ] && events="` + dir + `/resume.jsonl" ;;
		--mcp-config) cp "$2" "` + dir + `/mcp.json" ;;
		--system-prompt-file) cp "$2" "` + dir + `/system" ;;
	esac
	shift
done
cat "$events"
`
	cmd = filepath.Join(dir, "claude")
	if err := os.WriteFile(cmd, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return cmd, dir
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestClaudeCodeTurn(t *testing.T) {
	cmd, dir := scriptedClaude(t,
		`{"type":"system","subtype":"init"}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"Let me look."}}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"mcp__agilecbt__list_goals","input":{"status":"active"}}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t1","content":[{"type":"text","text":"[]"}]}]}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"No goals yet."}}}`,
		`{"type":"result","subtype":"success","is_error":false,"result":"No goals yet."}`,
	)
	var calls []ToolCall
	c := &ClaudeCode{
		Command: cmd, Model: "sonnet", ReasoningEffort: "none",
		MCPURL: "http://127.0.0.1:8080/mcp", Secret: "s3cret",
		OnToolCall: func(tc ToolCall) { calls = append(calls, tc) },
	}
	var streamed strings.Builder
	res, err := c.Turn(context.Background(), TurnRequest{
		CheckinID: 7,
		System:    "be kind",
		History:   []db.Message{{Role: "assistant", Text: "How are you arriving?"}, {Role: "user", Text: "tired"}},
		User:      "what are my goals?",
	}, func(s string) { streamed.WriteString(s) })
	if err != nil {
		t.Fatal(err)
	}
	if want := "Let me look.\n\nNo goals yet."; res.Text != want || streamed.String() != want {
		t.Fatalf("text = %q, streamed %q", res.Text, streamed.String())
	}
	if len(calls) != 1 || calls[0].Name != "list_goals" || calls[0].Result != "[]" || string(calls[0].Args) != `{"status":"active"}` {
		t.Fatalf("calls = %+v", calls)
	}

	args := strings.Split(readFile(t, filepath.Join(dir, "args")), "\n")
	for _, pair := range [][2]string{{"--model", "sonnet"}, {"--effort", "low"}, {"--tools", ""}, {"--allowedTools", "mcp__agilecbt"}} {
		i := slices.Index(args, pair[0])
		if i < 0 || args[i+1] != pair[1] {
			t.Fatalf("want %s %q in %q", pair[0], pair[1], args)
		}
	}
	if readFile(t, filepath.Join(dir, "system")) != "be kind" {
		t.Fatal("system prompt not passed")
	}
	stdin := readFile(t, filepath.Join(dir, "stdin"))
	if !strings.Contains(stdin, "<assistant>\nHow are you arriving?\n</assistant>\n<user>\ntired\n</user>") || !strings.HasSuffix(stdin, "what are my goals?") {
		t.Fatalf("prompt = %q", stdin)
	}
	var mcp struct {
		MCPServers map[string]struct {
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(dir, "mcp.json"))), &mcp); err != nil {
		t.Fatal(err)
	}
	if s := mcp.MCPServers["agilecbt"]; s.URL != "http://127.0.0.1:8080/mcp?checkin=7" || s.Headers["Authorization"] != "Bearer s3cret" {
		t.Fatalf("mcp config = %+v", mcp)
	}
	if strings.Contains(strings.Join(args, " "), "s3cret") {
		t.Fatal("secret on the command line")
	}
}

func TestClaudeCodeNoToolsAndModelFallback(t *testing.T) {
	cmd, dir := scriptedClaude(t, `{"type":"result","subtype":"success","result":"Here for you."}`)
	// An Ollama model left over from the openai backend isn't passed on.
	c := &ClaudeCode{Command: cmd, Model: "gemma4:26b", MCPURL: "http://127.0.0.1:8080/mcp"}
	res, err := c.Turn(context.Background(), TurnRequest{User: "hi", NoTools: true}, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "Here for you." {
		t.Fatalf("text = %q", res.Text)
	}
	args := readFile(t, filepath.Join(dir, "args"))
	if strings.Contains(args, "--mcp-config") || strings.Contains(args, "--model") || strings.Contains(args, "--effort") {
		t.Fatalf("args = %q", args)
	}
	if stdin := readFile(t, filepath.Join(dir, "stdin")); stdin != "hi" {
		t.Fatalf("prompt = %q", stdin)
	}
}

func TestClaudeCodeErrors(t *testing.T) {
	cmd, _ := scriptedClaude(t, `{"type":"result","subtype":"success","is_error":true,"result":"Claude AI usage limit reached"}`)
	_, err := (&ClaudeCode{Command: cmd}).Complete(context.Background(), "sys", "draft")
	if err == nil || !strings.Contains(err.Error(), "usage limit") {
		t.Fatalf("err = %v", err)
	}

	cmd, _ = scriptedClaude(t, `not json`)
	if _, err := (&ClaudeCode{Command: cmd}).Complete(context.Background(), "sys", "draft"); err == nil {
		t.Fatal("want an error when no result arrives")
	}

	ok, detail := (&ClaudeCode{Command: filepath.Join(t.TempDir(), "missing")}).Status(context.Background())
	if ok || !strings.Contains(detail, "isn't installed") {
		t.Fatalf("status = %v %q", ok, detail)
	}
}

func TestClaudeCodeResumesSessions(t *testing.T) {
	cmd, dir := scriptedClaude(t,
		`{"type":"system","subtype":"init","session_id":"s-1"}`,
		`{"type":"result","subtype":"success","result":"Hello.","session_id":"s-1"}`,
	)
	work := filepath.Join(t.TempDir(), "claude-code")
	c := &ClaudeCode{Command: cmd, WorkDir: work, MCPURL: "http://127.0.0.1:8080/mcp"}
	turn := func(history []db.Message, user string) {
		t.Helper()
		if _, err := c.Turn(context.Background(), TurnRequest{CheckinID: 3, History: history, User: user}, func(string) {}); err != nil {
			t.Fatal(err)
		}
	}
	opener := make([]db.Message, 1, 3)
	opener[0] = db.Message{Role: "assistant", Text: "How are you arriving?"}
	turn(opener, "tired")
	args := readFile(t, filepath.Join(dir, "args"))
	if strings.Contains(args, "--resume") || strings.Contains(args, "--no-session-persistence") {
		t.Fatalf("first turn args = %q", args)
	}
	if cwd := strings.TrimSpace(readFile(t, filepath.Join(dir, "cwd"))); cwd != work {
		t.Fatalf("cwd = %q, want %q", cwd, work)
	}
	if stdin := readFile(t, filepath.Join(dir, "stdin")); !strings.Contains(stdin, "How are you arriving?") {
		t.Fatalf("first turn prompt = %q", stdin)
	}

	// The next turn continues the session with just the new message.
	history := append(opener, db.Message{Role: "user", Text: "tired"}, db.Message{Role: "assistant", Text: "Hello."})
	turn(history, "what now?")
	args = readFile(t, filepath.Join(dir, "args"))
	if !strings.Contains(args, "--resume\ns-1\n") {
		t.Fatalf("second turn args = %q", args)
	}
	if stdin := readFile(t, filepath.Join(dir, "stdin")); !strings.HasSuffix(stdin, "\n\nwhat now?") || strings.Contains(stdin, "<transcript>") {
		t.Fatalf("second turn prompt = %q", stdin)
	}

	// History the session hasn't seen (another check-in's edit, a failed
	// turn) starts over from the transcript.
	turn(history[:2], "again")
	if args := readFile(t, filepath.Join(dir, "args")); strings.Contains(args, "--resume") {
		t.Fatalf("mismatched history resumed: %q", args)
	}

	// So does history of the right length that isn't what the session saw,
	// such as a deleted check-in imported again under the same id.
	turn(opener, "tired")
	other := append(slices.Clone(opener), db.Message{Role: "user", Text: "someone else"}, db.Message{Role: "assistant", Text: "Hello."})
	turn(other, "what now?")
	if args := readFile(t, filepath.Join(dir, "args")); strings.Contains(args, "--resume") {
		t.Fatalf("different history resumed: %q", args)
	}

	// Without a check-in or a WorkDir, nothing is kept.
	if _, err := (&ClaudeCode{Command: cmd}).Turn(context.Background(), TurnRequest{CheckinID: 3, User: "hi"}, func(string) {}); err != nil {
		t.Fatal(err)
	}
	if args := readFile(t, filepath.Join(dir, "args")); !strings.Contains(args, "--no-session-persistence") {
		t.Fatalf("args = %q", args)
	}
}

func TestClaudeCodeLostSessionRetriesFresh(t *testing.T) {
	cmd, dir := scriptedClaude(t, `{"type":"result","subtype":"success","result":"Fresh.","session_id":"s-2"}`)
	if err := os.WriteFile(filepath.Join(dir, "resume.jsonl"),
		[]byte(`{"type":"result","subtype":"error_during_execution","is_error":true,"errors":["No conversation found with session ID: gone"]}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := &ClaudeCode{Command: cmd, WorkDir: t.TempDir()}
	history := []db.Message{{Role: "assistant", Text: "Hi"}, {Role: "user", Text: "hey"}, {Role: "assistant", Text: "Hello."}}
	c.sessions = map[int64]claudeSession{5: {id: "gone", messages: 3, prompt: "hey", seen: digest([]db.Message{history[0], history[2]})}}
	res, err := c.Turn(context.Background(), TurnRequest{CheckinID: 5, History: history, User: "still there?"}, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "Fresh." {
		t.Fatalf("text = %q", res.Text)
	}
	if runs := strings.Count(readFile(t, filepath.Join(dir, "runs")), "x"); runs != 2 {
		t.Fatalf("runs = %d, want a retry", runs)
	}
	if stdin := readFile(t, filepath.Join(dir, "stdin")); !strings.Contains(stdin, "<user>\nhey\n</user>") {
		t.Fatalf("retry prompt = %q", stdin)
	}
	if s, _ := c.session(5); s.id != "s-2" || s.messages != 5 {
		t.Fatalf("session = %+v", s)
	}
}

func TestClaudeCodeCrisisTurnStartsFresh(t *testing.T) {
	cmd, dir := scriptedClaude(t, `{"type":"result","subtype":"success","result":"I'm here.","session_id":"s-9"}`)
	work := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	sessions := sessionDir(work)
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(sessions, "s-1.jsonl")
	if err := os.WriteFile(old, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := &ClaudeCode{Command: cmd, WorkDir: work, MCPURL: "http://127.0.0.1:8080/mcp"}
	history := []db.Message{{Role: "assistant", Text: "Hi"}, {Role: "user", Text: "hey"}, {Role: "assistant", Text: "Hello."}}
	c.sessions = map[int64]claudeSession{5: {id: "s-1", messages: 3, prompt: "hey", seen: digest([]db.Message{history[0], history[2]})}}
	if _, err := c.Turn(context.Background(), TurnRequest{CheckinID: 5, History: history, User: "help", NoTools: true}, func(string) {}); err != nil {
		t.Fatal(err)
	}
	args := readFile(t, filepath.Join(dir, "args"))
	if strings.Contains(args, "--resume") || !strings.Contains(args, "--no-session-persistence") || strings.Contains(args, "--mcp-config") {
		t.Fatalf("crisis turn args = %q", args)
	}
	if stdin := readFile(t, filepath.Join(dir, "stdin")); !strings.Contains(stdin, "<user>\nhey\n</user>") {
		t.Fatalf("crisis turn prompt = %q", stdin)
	}
	if _, ok := c.session(5); ok {
		t.Fatal("crisis turn kept a session")
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("the replaced session's transcript is still on disk: %v", err)
	}
}

func TestClaudeCodeForgetAndPrune(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	work := filepath.Join(t.TempDir(), "claude-code")
	sessions := sessionDir(work)
	if !strings.HasSuffix(sessions, strings.ReplaceAll(work, "/", "-")) {
		t.Fatalf("sessionDir = %q", sessions)
	}
	if err := os.MkdirAll(filepath.Join(sessions, "s-1"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"s-1.jsonl", "s-2.jsonl"} {
		if err := os.WriteFile(filepath.Join(sessions, f), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	c := &ClaudeCode{WorkDir: work, sessions: map[int64]claudeSession{7: {id: "s-1"}}}
	c.Forget(7)
	if _, err := os.Stat(filepath.Join(sessions, "s-1.jsonl")); !os.IsNotExist(err) {
		t.Fatal("Forget left the transcript")
	}
	if _, err := os.Stat(filepath.Join(sessions, "s-2.jsonl")); err != nil {
		t.Fatal("Forget removed another session's transcript")
	}
	PruneClaudeSessions(work)
	if _, err := os.Stat(sessions); !os.IsNotExist(err) {
		t.Fatal("PruneClaudeSessions left transcripts")
	}
}

func TestClaudeEnv(t *testing.T) {
	got := claudeEnv([]string{
		"PATH=/bin", "HOME=/home/me", "CLAUDECODE=1", "CLAUDE_EFFORT=max", "CLAUDE_CODE_SESSION_ID=abc",
		"CLAUDE_CODE_MESSAGING_TOKEN=t", "ANTHROPIC_API_KEY=sk", "CLAUDE_CODE_OAUTH_TOKEN=o",
		"CLAUDE_CODE_USE_BEDROCK=1", "CLAUDE_CONFIG_DIR=/c",
	})
	want := []string{"PATH=/bin", "HOME=/home/me", "CLAUDE_CODE_OAUTH_TOKEN=o", "CLAUDE_CODE_USE_BEDROCK=1", "CLAUDE_CONFIG_DIR=/c"}
	if !slices.Equal(got, want) {
		t.Fatalf("claudeEnv = %q, want %q", got, want)
	}
}

func TestClaudeCodeResultOnlyReplyStreams(t *testing.T) {
	cmd, _ := scriptedClaude(t, `{"type":"result","subtype":"success","result":"Only here.","session_id":"s-1"}`)
	var streamed strings.Builder
	res, err := (&ClaudeCode{Command: cmd}).Turn(context.Background(), TurnRequest{User: "hi"}, func(d string) { streamed.WriteString(d) })
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "Only here." || streamed.String() != "Only here." {
		t.Fatalf("text = %q, streamed = %q", res.Text, streamed.String())
	}
}

func TestSelfMCPURL(t *testing.T) {
	for addr, want := range map[string]string{
		":8080":          "http://127.0.0.1:8080/mcp",
		"0.0.0.0:9000":   "http://127.0.0.1:9000/mcp",
		"[::]:8080":      "http://127.0.0.1:8080/mcp",
		"localhost:8095": "http://localhost:8095/mcp",
		"10.0.0.5:8080":  "http://10.0.0.5:8080/mcp",
	} {
		if got := SelfMCPURL(addr); got != want {
			t.Errorf("SelfMCPURL(%q) = %q, want %q", addr, got, want)
		}
	}
}
