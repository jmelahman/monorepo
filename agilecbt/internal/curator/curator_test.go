package curator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

// fakeOpenAI replays scripted /chat/completions streams and records
// requests.
type fakeOpenAI struct {
	mu       sync.Mutex
	models   string
	replies  [][]string // SSE data payloads per call
	requests []map[string]any
	auth     []string
}

func (f *fakeOpenAI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.auth = append(f.auth, r.Header.Get("Authorization"))
	f.mu.Unlock()
	switch r.URL.Path {
	case "/v1/models":
		fmt.Fprint(w, f.models)
	case "/v1/chat/completions":
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.requests = append(f.requests, body)
		lines := f.replies[0]
		f.replies = f.replies[1:]
		f.mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, ": keep-alive\n\n")
		for _, l := range lines {
			fmt.Fprintf(w, "data: %s\n\n", l)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	default:
		http.NotFound(w, r)
	}
}

func delta(d string) string { return `{"choices":[{"index":0,"delta":` + d + `}]}` }

func TestOpenAIToolLoop(t *testing.T) {
	reg := newRegistry(t)
	a := reg.App()
	c := newCheckin(t, a)

	fake := &fakeOpenAI{
		models: `{"object":"list","data":[{"id":"qwen3.8:27b"}]}`,
		replies: [][]string{
			{
				delta(`{"role":"assistant","content":"Let me add that. "}`),
				// Arguments arrive in fragments across chunks.
				delta(`{"tool_calls":[{"index":0,"id":"call_a","type":"function","function":{"name":"create_step","arguments":"{\"title\":\"Short"}}]}`),
				delta(`{"tool_calls":[{"index":0,"function":{"arguments":" walk\",\"lane\":\"today\"}"}}]}`),
			},
			{
				delta(`{"content":"Done. "}`),
				delta(`{"content":"A walk is on Today."}`),
			},
		},
	}
	srv := httptest.NewServer(fake)
	defer srv.Close()

	cur := New(reg, &OpenAI{BaseURL: srv.URL + "/v1", APIKey: "k3y", Model: "qwen3.8:27b", ReasoningEffort: "none", Registry: reg})
	if st := cur.Status(context.Background()); !st.Available || st.Backend != "openai" {
		t.Fatalf("status: %+v", st)
	}

	var actions []db.Action
	cancel := a.Broker.Subscribe(c.ID, func(act db.Action) { actions = append(actions, act) })
	defer cancel()
	var streamed strings.Builder
	err := cur.Chat(context.Background(), c.ID, "I could walk today", func(ev string, data any) {
		if ev == "text" {
			streamed.WriteString(data.(map[string]string)["text"])
		}
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
	want := "Let me add that. \n\nDone. A walk is on Today."
	if len(msgs) != 2 || msgs[0].Text != "I could walk today" || msgs[1].Text != want || streamed.String() != want {
		t.Fatalf("messages: %+v, streamed %q", msgs, streamed.String())
	}
	for _, h := range fake.auth {
		if h != "Bearer k3y" {
			t.Errorf("authorization = %q", h)
		}
	}

	// The first request carries system prompt, context, and tools; the
	// second carries the tool call and its result.
	first := fake.requests[0]
	if first["reasoning_effort"] != "none" || first["model"] != "qwen3.8:27b" || first["stream"] != true {
		t.Errorf("request params: %v %v %v", first["reasoning_effort"], first["model"], first["stream"])
	}
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
	call := second[len(second)-2].(map[string]any)
	calls, _ := call["tool_calls"].([]any)
	if len(calls) != 1 || !strings.Contains(fmt.Sprint(calls[0]), `"title":"Short walk"`) {
		t.Errorf("assistant tool call not sent back: %+v", call)
	}
	if last := second[len(second)-1].(map[string]any); last["role"] != "tool" || last["tool_call_id"] != "call_a" || !strings.Contains(last["content"].(string), "Short walk") {
		t.Errorf("tool result not sent back: %+v", last)
	}

	// Retro drafting is one tool-free call.
	fake.replies = [][]string{{delta(`{"content":"{\"went_well\":\"You walked.\",\"try_next\":\"Walk again.\"}"}`)}}
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
	if _, ok := fake.requests[2]["tools"]; ok {
		t.Errorf("retro draft sent tools")
	}
}

func TestOpenAIStatus(t *testing.T) {
	for _, tc := range []struct {
		models, model string
		ok            bool
		detail        string
	}{
		{`{"data":[{"id":"qwen3.8:27b"}]}`, "llama3", false, "ollama pull llama3"},
		{`{"data":[{"id":"llama3:latest"}]}`, "llama3", true, "llama3 at"},
		{`not json`, "anything", true, "anything at"},
	} {
		srv := httptest.NewServer(&fakeOpenAI{models: tc.models})
		ok, detail := (&OpenAI{BaseURL: srv.URL + "/v1", Model: tc.model}).Status(context.Background())
		srv.Close()
		if ok != tc.ok || !strings.Contains(detail, tc.detail) {
			t.Errorf("%s: status = %v %q", tc.models, ok, detail)
		}
	}
	for _, tc := range []struct {
		code   int
		ok     bool
		detail string
	}{
		{http.StatusUnauthorized, false, "needs an API key"},
		{http.StatusServiceUnavailable, false, "503 Service Unavailable"},
		{http.StatusTooManyRequests, false, "429 Too Many Requests"},
		{http.StatusNotFound, true, "m at"},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.code)
		}))
		ok, detail := (&OpenAI{BaseURL: srv.URL, Model: "m"}).Status(context.Background())
		srv.Close()
		if ok != tc.ok || !strings.Contains(detail, tc.detail) {
			t.Errorf("%d: status = %v %q", tc.code, ok, detail)
		}
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
