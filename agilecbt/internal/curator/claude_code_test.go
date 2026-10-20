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
// args, stdin and MCP config into dir and prints events, one per line.
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
while [ $# -gt 0 ]; do
	case "$1" in
		--mcp-config) cp "$2" "` + dir + `/mcp.json" ;;
		--system-prompt-file) cp "$2" "` + dir + `/system" ;;
	esac
	shift
done
cat "` + out + `"
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
