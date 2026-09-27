package curator

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jmelahman/agilecbt/internal/safety"
)

func TestChatCrisisTurn(t *testing.T) {
	for _, tc := range []struct {
		name    string
		replies [][]string
		// wantAppended is whether the app had to add the resources.
		wantAppended bool
	}{
		{
			name:    "coach shares the resources",
			replies: [][]string{{delta(`{"role":"assistant","content":"I'm glad you told me. Are you safe right now? You can call or text 988."}`)}},
		},
		{
			name:         "coach leaves them out",
			replies:      [][]string{{delta(`{"role":"assistant","content":"That sounds so hard. Are you safe right now?"}`)}},
			wantAppended: true,
		},
		{
			name:         "coach says nothing",
			replies:      [][]string{{}},
			wantAppended: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reg := newRegistry(t)
			a := reg.App()
			c := newCheckin(t, a)
			fake := &fakeOpenAI{replies: tc.replies}
			srv := httptest.NewServer(fake)
			defer srv.Close()
			// New uses the lexicon alone, so the only model call is the turn.
			cur := New(reg, &OpenAI{BaseURL: srv.URL + "/v1", Model: "m", Registry: reg})

			var flagged []safety.Result
			var streamed strings.Builder
			err := cur.Chat(context.Background(), c.ID, "I've been thinking about killing myself", func(ev string, data any) {
				switch ev {
				case "safety":
					flagged = append(flagged, data.(safety.Result))
				case "text":
					streamed.WriteString(data.(map[string]string)["text"])
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(flagged) != 1 || flagged[0].Category != safety.Suicide {
				t.Fatalf("safety events: %+v", flagged)
			}
			req := fake.requests[0]
			if _, ok := req["tools"]; ok {
				t.Errorf("crisis turn sent tools")
			}
			sent := req["messages"].([]any)
			if user := sent[len(sent)-1].(map[string]any)["content"].(string); !strings.Contains(user, "[Safety check") {
				t.Errorf("user turn missing the safety directive:\n%s", user)
			}
			msgs, err := a.Store.ListMessages(c.ID)
			if err != nil {
				t.Fatal(err)
			}
			reply := msgs[len(msgs)-1]
			if reply.Role != "assistant" || reply.Safety != string(safety.Suicide) {
				t.Fatalf("reply: %+v", reply)
			}
			if strings.TrimSpace(streamed.String()) != reply.Text {
				t.Errorf("streamed %q, saved %q", streamed.String(), reply.Text)
			}
			if !strings.Contains(reply.Text, "988") {
				t.Errorf("reply has no crisis line: %q", reply.Text)
			}
			if appended := strings.Contains(reply.Text, "Suicide & Crisis Lifeline"); appended != tc.wantAppended {
				t.Errorf("resources appended = %v, want %v: %q", appended, tc.wantAppended, reply.Text)
			}
			if msgs[len(msgs)-2].Safety != "" {
				t.Errorf("user message flagged: %+v", msgs[len(msgs)-2])
			}
		})
	}
}

func TestChatOrdinaryTurnKeepsTools(t *testing.T) {
	reg := newRegistry(t)
	a := reg.App()
	c := newCheckin(t, a)
	fake := &fakeOpenAI{replies: [][]string{{delta(`{"role":"assistant","content":"Three things is a lot. Which one first?"}`)}}}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	cur := New(reg, &OpenAI{BaseURL: srv.URL + "/v1", Model: "m", Registry: reg})
	err := cur.Chat(context.Background(), c.ID, "this deadline is killing me", func(ev string, _ any) {
		if ev == "safety" {
			t.Errorf("figure of speech flagged")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := fake.requests[0]["tools"]; !ok {
		t.Errorf("ordinary turn sent no tools")
	}
	msgs, _ := a.Store.ListMessages(c.ID)
	if got := msgs[len(msgs)-1]; got.Safety != "" || strings.Contains(got.Text, "988") {
		t.Errorf("reply: %+v", got)
	}
}

func TestClassifierFor(t *testing.T) {
	c := &Curator{}
	base := Config{LLM: llmOpenAI, BaseURL: "http://coach/v1", APIKey: "coach-key", Model: "big", ReasoningEffort: "none"}
	for _, tc := range []struct {
		name             string
		cfg              func(Config) Config
		wantURL, wantKey string
		wantModel        string
		wantNoModelTier  bool
	}{
		{name: "default is the lexicon alone", cfg: func(c Config) Config { return c }, wantNoModelTier: true},
		{name: "model only", cfg: func(c Config) Config { c.SafetyModel = "small"; return c }, wantURL: "http://coach/v1", wantKey: "coach-key", wantModel: "small"},
		{name: "other endpoint drops the coach's key", cfg: func(c Config) Config {
			c.SafetyModel, c.SafetyBaseURL = "guard", "http://guard/v1"
			return c
		}, wantURL: "http://guard/v1", wantKey: "", wantModel: "guard"},
		{name: "other endpoint with its own key", cfg: func(c Config) Config {
			c.SafetyModel, c.SafetyBaseURL, c.SafetyAPIKey = "guard", "http://guard/v1", "guard-key"
			return c
		}, wantURL: "http://guard/v1", wantKey: "guard-key", wantModel: "guard"},
		{name: "coach off", cfg: func(c Config) Config { c.LLM, c.SafetyModel = llmNone, "small"; return c }, wantNoModelTier: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cl := c.classifierFor(tc.cfg(base))
			if tc.wantNoModelTier {
				if cl.LLM != nil {
					t.Fatalf("model tier = %+v, want none", cl.LLM)
				}
				return
			}
			o := cl.LLM.(*OpenAI)
			if o.BaseURL != tc.wantURL || o.APIKey != tc.wantKey || o.Model != tc.wantModel {
				t.Errorf("classifier = %s %q %s, want %s %q %s", o.BaseURL, o.APIKey, o.Model, tc.wantURL, tc.wantKey, tc.wantModel)
			}
		})
	}
}
