package curator

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jmelahman/agilecbt/internal/db"
)

func ptr(s string) *string { return &s }

func TestConfiguredOverridesAndKeyBinding(t *testing.T) {
	reg := newRegistry(t)
	a := reg.App()
	defaults := Config{LLM: "openai", BaseURL: "http://ollama:11434/v1", APIKey: "config-key", Model: "qwen", ReasoningEffort: "none"}
	cur, err := NewConfigured(reg, defaults)
	if err != nil {
		t.Fatal(err)
	}
	if eff := cur.Effective(); eff != defaults {
		t.Fatalf("effective = %+v", eff)
	}

	// Moving to another URL drops the configured key rather than sending
	// it there.
	s, err := cur.UpdateLLM(map[string]*string{"base_url": ptr("https://openrouter.ai/api/v1/"), "model": ptr("anthropic/claude-sonnet-5")})
	if err != nil {
		t.Fatal(err)
	}
	if s.Effective.BaseURL != "https://openrouter.ai/api/v1" || s.Effective.APIKeySet || cur.Effective().APIKey != "" {
		t.Fatalf("after url change: %+v", s)
	}
	if s.Overrides["model"] != "anthropic/claude-sonnet-5" || s.Defaults.Model != "qwen" || !s.Defaults.APIKeySet {
		t.Fatalf("layers: %+v", s)
	}

	// A saved key is bound to the URL in effect when it was saved.
	if s, err = cur.UpdateLLM(map[string]*string{"api_key": ptr("sk-or-1")}); err != nil {
		t.Fatal(err)
	}
	if !s.APIKeySaved || cur.Effective().APIKey != "sk-or-1" {
		t.Fatalf("saved key not used: %+v", s)
	}
	if raw, _ := json.Marshal(s); strings.Contains(string(raw), "sk-or-1") {
		t.Fatalf("key leaked in settings: %s", raw)
	}
	if s, err = cur.UpdateLLM(map[string]*string{"base_url": ptr("http://evil.example/v1")}); err != nil {
		t.Fatal(err)
	}
	if s.APIKeySaved || cur.Effective().APIKey != "" {
		t.Fatalf("key followed the url: %+v", s)
	}
	// A key sent with a new URL is bound to that URL.
	if s, err = cur.UpdateLLM(map[string]*string{"base_url": ptr("https://b.example/v1"), "api_key": ptr("sk-b")}); err != nil {
		t.Fatal(err)
	}
	if !s.APIKeySaved || cur.Effective().APIKey != "sk-b" {
		t.Fatalf("key not bound to the new url: %+v", s)
	}

	// Exports never carry the key.
	d, err := a.Export()
	if err != nil {
		t.Fatal(err)
	}
	for k := range d.Settings {
		if db.IsSecretSetting(k) {
			t.Fatalf("export has %s", k)
		}
	}

	// Null reverts to the config, restoring its key; "" keeps
	// reasoning_effort out of requests.
	if s, err = cur.UpdateLLM(map[string]*string{"base_url": nil, "model": nil, "reasoning_effort": ptr("")}); err != nil {
		t.Fatal(err)
	}
	if eff := cur.Effective(); eff.BaseURL != defaults.BaseURL || eff.APIKey != "config-key" || eff.Model != "qwen" || eff.ReasoningEffort != "" {
		t.Fatalf("reverted: %+v", eff)
	}

	// Turning the coach off takes effect immediately.
	if _, err = cur.UpdateLLM(map[string]*string{"llm": ptr("none")}); err != nil {
		t.Fatal(err)
	}
	if st := cur.Status(context.Background()); st.Backend != "none" || st.Available {
		t.Fatalf("status when off: %+v", st)
	}
	c := newCheckin(t, a)
	if err := cur.Chat(context.Background(), c.ID, "hi", func(string, any) {}); !errors.Is(err, ErrOff) {
		t.Fatalf("chat when off: %v", err)
	}

	for _, bad := range []map[string]*string{
		{"llm": ptr("claude-code")},
		{"base_url": ptr("ftp://x")},
		{"reasoning_effort": ptr("max")},
		{"secret": ptr("x")},
		{"secret": nil},
	} {
		if _, err := cur.UpdateLLM(bad); !errors.Is(err, db.ErrInvalid) {
			t.Errorf("%v: err = %v", bad, err)
		}
	}
}

func TestConfiguredModelsAndStatus(t *testing.T) {
	reg := newRegistry(t)
	srv := httptest.NewServer(&fakeOpenAI{models: `{"data":[{"id":"b"},{"id":"a"}]}`})
	defer srv.Close()
	cur, err := NewConfigured(reg, Config{LLM: "openai", BaseURL: srv.URL + "/v1", Model: "a"})
	if err != nil {
		t.Fatal(err)
	}
	ids, err := cur.Models(context.Background())
	if err != nil || strings.Join(ids, ",") != "a,b" {
		t.Fatalf("models = %v, %v", ids, err)
	}
	if st := cur.Status(context.Background()); !st.Available {
		t.Fatalf("status: %+v", st)
	}
	// Changing the model re-probes instead of serving the cached status.
	if _, err := cur.UpdateLLM(map[string]*string{"model": ptr("c")}); err != nil {
		t.Fatal(err)
	}
	if st := cur.Status(context.Background()); st.Available || !strings.Contains(st.Detail, "ollama pull c") {
		t.Fatalf("status after change: %+v", st)
	}
}
