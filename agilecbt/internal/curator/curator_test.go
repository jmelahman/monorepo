package curator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jmelahman/agilecbt/internal/app"
	"github.com/jmelahman/agilecbt/internal/db"
	"github.com/jmelahman/agilecbt/internal/tools"
)

func newRegistry(t *testing.T) *tools.Registry {
	t.Helper()
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return tools.New(app.New(store))
}

func newCheckin(t *testing.T, a *app.App) db.Checkin {
	t.Helper()
	today := a.Today()
	mood := 4
	c, err := a.Store.CreateCheckin(db.CheckinPatch{Date: &today, Mood: &mood})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// fakeOllama replays scripted /api/chat responses and records requests.
type fakeOllama struct {
	mu       sync.Mutex
	replies  [][]string // NDJSON lines per call
	requests []map[string]any
}

func (f *fakeOllama) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/tags":
		fmt.Fprint(w, `{"models":[{"name":"qwen3:14b"}]}`)
	case "/api/chat":
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.requests = append(f.requests, body)
		lines := f.replies[0]
		f.replies = f.replies[1:]
		f.mu.Unlock()
		for _, l := range lines {
			fmt.Fprintln(w, l)
		}
	default:
		http.NotFound(w, r)
	}
}

func TestOllamaToolLoop(t *testing.T) {
	reg := newRegistry(t)
	a := reg.App()
	c := newCheckin(t, a)

	fake := &fakeOllama{replies: [][]string{
		{
			`{"message":{"role":"assistant","content":"Let me add that. "},"done":false}`,
			`{"message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"create_step","arguments":{"title":"Short walk","lane":"today"}}}]},"done":false}`,
			`{"message":{"role":"assistant","content":""},"done":true}`,
		},
		{
			`{"message":{"role":"assistant","content":"Done — "},"done":false}`,
			`{"message":{"role":"assistant","content":"a walk is on Today."},"done":true}`,
		},
	}}
	srv := httptest.NewServer(fake)
	defer srv.Close()

	cur := New(reg, &Ollama{Host: srv.URL, Registry: reg})
	if st := cur.Status(context.Background()); !st.Available || st.Backend != "ollama" {
		t.Fatalf("status: %+v", st)
	}

	var events []string
	var actions []db.Action
	cancel := a.Broker.Subscribe(c.ID, func(act db.Action) { actions = append(actions, act) })
	defer cancel()
	err := cur.Chat(context.Background(), c.ID, "I could walk today", func(ev string, data any) {
		events = append(events, ev)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].Tool != "create_step" || actions[0].Source != "curator" {
		t.Fatalf("actions: %+v", actions)
	}
	msgs, err := a.Store.ListMessages(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 || msgs[0].Text != "I could walk today" || msgs[1].Text != "Let me add that. \n\nDone — a walk is on Today." {
		t.Fatalf("messages: %+v", msgs)
	}

	// The first request carries system prompt, context, and tools; the
	// second carries the tool result.
	first := fake.requests[0]
	if len(first["tools"].([]any)) != len(reg.List()) {
		t.Errorf("tools not sent")
	}
	sent := first["messages"].([]any)
	if sys := sent[0].(map[string]any); sys["role"] != "system" || !strings.Contains(sys["content"].(string), "988") {
		t.Errorf("system prompt missing crisis resources")
	}
	if user := sent[len(sent)-1].(map[string]any)["content"].(string); !strings.Contains(user, "<context>") || !strings.Contains(user, "mood 4") {
		t.Errorf("context block missing: %s", user)
	}
	second := fake.requests[1]["messages"].([]any)
	if last := second[len(second)-1].(map[string]any); last["role"] != "tool" || !strings.Contains(last["content"].(string), "Short walk") {
		t.Errorf("tool result not sent back: %+v", last)
	}
}

func TestOllamaStatusMissingModel(t *testing.T) {
	srv := httptest.NewServer(&fakeOllama{})
	defer srv.Close()
	ok, detail := (&Ollama{Host: srv.URL, Model: "llama3"}).Status(context.Background())
	if ok || !strings.Contains(detail, "ollama pull llama3") {
		t.Fatalf("status = %v %q", ok, detail)
	}
}

// fakeClaude writes a stand-in `claude` script that logs its arguments and
// stdin, then replays stream-json output.
func fakeClaude(t *testing.T) (bin, logPath string) {
	t.Helper()
	dir := t.TempDir()
	logPath = filepath.Join(dir, "calls.log")
	bin = filepath.Join(dir, "claude")
	script := `#!/bin/sh
log="` + logPath + `"
echo "ARGS: $*" >> "$log"
if [ "$1" = "auth" ]; then echo '{"loggedIn":true,"authMethod":"claude.ai"}'; exit 0; fi
echo "STDIN: $(cat)" >> "$log"
for a in "$@"; do case "$prev" in --mcp-config) echo "MCP: $(cat "$a")" >> "$log";; esac; prev="$a"; done
case "$*" in
*stream-json*)
cat <<'EOF'
{"type":"system","subtype":"init","session_id":"sess-1"}
{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}}
{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Good morning."}}}
{"type":"stream_event","event":{"type":"content_block_start","index":1,"content_block":{"type":"tool_use"}}}
{"type":"stream_event","event":{"type":"content_block_start","index":2,"content_block":{"type":"text","text":""}}}
{"type":"stream_event","event":{"type":"content_block_delta","index":2,"delta":{"type":"text_delta","text":"How are you arriving?"}}}
{"type":"result","subtype":"success","is_error":false,"result":"How are you arriving?","session_id":"sess-1"}
EOF
;;
*) echo '{"type":"result","subtype":"success","is_error":false,"result":"{\"went_well\":\"You walked.\",\"try_next\":\"Walk again.\"}"}';;
esac
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, logPath
}

func TestClaudeCodeTurns(t *testing.T) {
	reg := newRegistry(t)
	a := reg.App()
	c := newCheckin(t, a)
	bin, logPath := fakeClaude(t)
	cc := &ClaudeCode{Bin: bin, MCPURL: "http://127.0.0.1:9/mcp", Secret: "s3cret", Dir: t.TempDir()}
	cur := New(reg, cc)

	if st := cur.Status(context.Background()); !st.Available || !strings.Contains(st.Detail, "claude.ai") {
		t.Fatalf("status: %+v", st)
	}

	var text strings.Builder
	emit := func(ev string, data any) {
		if ev == "text" {
			text.WriteString(data.(map[string]string)["text"])
		}
	}
	if err := cur.Chat(context.Background(), c.ID, "hello", emit); err != nil {
		t.Fatal(err)
	}
	if text.String() != "Good morning.\n\nHow are you arriving?" {
		t.Fatalf("streamed text = %q", text.String())
	}
	got, err := a.Store.GetCheckin(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.LLMSession == "" {
		t.Fatal("session id not saved")
	}
	if err := cur.Chat(context.Background(), c.ID, "tired", emit); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	log := string(raw)
	for _, want := range []string{
		"--tools  --setting-sources  --strict-mcp-config",
		"--allowedTools mcp__agilecbt",
		"--session-id ",
		"--resume sess-1",
		`"Authorization":"Bearer s3cret"`,
		fmt.Sprintf(`checkin=%d`, c.ID),
		"STDIN: <context>",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("claude calls missing %q:\n%s", want, log)
		}
	}
	if strings.Contains(log, "ARGS: "+"s3cret") || strings.Contains(strings.Split(log, "MCP:")[0], "s3cret") {
		t.Errorf("secret leaked into argv")
	}
	// The per-turn MCP config file is cleaned up.
	if m, _ := filepath.Glob(filepath.Join(cc.Dir, "mcp-*.json")); len(m) != 0 {
		t.Errorf("leftover MCP configs: %v", m)
	}

	// Retro drafting uses the one-shot JSON mode.
	w, err := a.CurrentWeek()
	if err != nil {
		t.Fatal(err)
	}
	r, err := cur.DraftRetro(context.Background(), w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if r.WentWell != "You walked." || r.TryNext != "Walk again." || !strings.Contains(r.AIDraft, "**What went well**") {
		t.Fatalf("retro: %+v", r)
	}
}

func TestParseRetroDraft(t *testing.T) {
	d := parseRetroDraft("Sure!\n```json\n{\"went_well\":\"a\",\"was_hard\":\"b\",\"try_next\":\"c\",\"patterns\":\"d\"}\n```")
	if d.wentWell != "a" || d.tryNext != "c" || !strings.Contains(d.text, "**Patterns I noticed**\nd") {
		t.Fatalf("draft: %+v", d)
	}
	if d := parseRetroDraft("just prose"); d.text != "just prose" || d.wentWell != "" {
		t.Fatalf("prose fallback: %+v", d)
	}
}

func TestChatRejectsConcurrentTurns(t *testing.T) {
	reg := newRegistry(t)
	c := newCheckin(t, reg.App())
	block := make(chan struct{})
	cur := New(reg, blockingBackend{block})
	done := make(chan error)
	go func() { done <- cur.Chat(context.Background(), c.ID, "one", func(string, any) {}) }()
	// Wait until the first turn holds the check-in.
	for {
		cur.mu.Lock()
		busy := cur.busy[c.ID]
		cur.mu.Unlock()
		if busy {
			break
		}
	}
	if err := cur.Chat(context.Background(), c.ID, "two", func(string, any) {}); err != ErrBusy {
		t.Fatalf("second turn: %v, want ErrBusy", err)
	}
	close(block)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

type blockingBackend struct{ block chan struct{} }

func (blockingBackend) Name() string                          { return "blocking" }
func (blockingBackend) Status(context.Context) (bool, string) { return true, "" }
func (b blockingBackend) Turn(context.Context, TurnRequest, func(string)) (TurnResult, error) {
	<-b.block
	return TurnResult{Text: "ok"}, nil
}

func (blockingBackend) Complete(context.Context, string, string) (string, error) {
	return "", nil
}
