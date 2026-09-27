package safety

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jmelahman/agilecbt/internal/db"
)

func TestMatch(t *testing.T) {
	for _, tc := range []struct {
		text string
		want Category
	}{
		{"not good. I've been thinking about killing myself.", Suicide},
		{"lol if this week goes like the last one I'll just unalive myself haha", Suicide},
		{"thanks. also I've been saving up my pills, just in case. anyway", Suicide},
		{"everyone would be better off without me", Suicide},
		{"I don’t want to wake up tomorrow", Suicide},
		{"I want to die", Suicide},
		{"whatever. I cut myself again last night, that's the only thing that helps", SelfHarm},
		{"I keep wanting to hurt myself", SelfHarm},
		{"I'm so angry at my brother I want to hurt him. like actually hurt him, not just yell", HarmOthers},
		{"my partner is screaming and just threw a chair. I'm hiding in the bathroom", InDanger},
		{"he keeps threatening me", InDanger},
		{"my dad is drunk again and he hit me, I'm in my room", InDanger},
		{"kms if I have to do one more spreadsheet lmao", Suicide},
		{"I'm so tired of existing", Suicide},
		{"the urge to cut is really strong tonight", SelfHarm},
		{"I want to kill my brother. I'm serious, not joking", HarmOthers},
		{"I've been planning how to kill him", HarmOthers},

		{"this deadline is killing me, I have three things due today", None},
		{"I could kill for a coffee", None},
		{"my phone died mid-call so I'm redoing it", None},
		{"I cut myself shaving this morning, ouch", None},
		{"I accidentally burned myself on the pan", None},
		{"dying to get this done", None},
		{"watched a movie about suicide prevention, heavy but good", None},
		{"I want to hurt my chances less by prepping", None},
		{"the ball hit me in the face at practice", None},
		{"today is a year since I stopped self-harming", None},
		{"I'm tired of living out of boxes since the move", None},
		{"I'm gonna kill my brother if he eats my leftovers again", None},
	} {
		if got := Match(tc.text).Category; got != tc.want {
			t.Errorf("Match(%q) = %s, want %s", tc.text, got, tc.want)
		}
	}
}

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		out  string
		want Category
		err  bool
	}{
		{`{"category":"none","reason":"stress"}`, None, false},
		{`{"category": "suicide", "reason": "passive ideation"}`, Suicide, false},
		{"```json\n{\"category\":\"self-harm\"}\n```", SelfHarm, false},
		{`<think>they said {x}</think>{"category":"In Danger"}`, InDanger, false},
		{`Sure: {"category":"harm_others"} hope that helps {}`, HarmOthers, false},
		{`{"category":"crisis"}`, "", true},
		{`no json here`, "", true},
	} {
		r, err := Parse(tc.out)
		if (err != nil) != tc.err {
			t.Errorf("Parse(%q) err = %v, want err %v", tc.out, err, tc.err)
			continue
		}
		if !tc.err && r.Category != tc.want {
			t.Errorf("Parse(%q) = %s, want %s", tc.out, r.Category, tc.want)
		}
	}
}

type fakeLLM struct {
	out   string
	err   error
	delay time.Duration
	calls int
	input string
}

func (f *fakeLLM) Complete(ctx context.Context, system, user string) (string, error) {
	f.calls++
	f.input = user
	select {
	case <-time.After(f.delay):
	case <-ctx.Done():
		return "", ctx.Err()
	}
	return f.out, f.err
}

func TestClassify(t *testing.T) {
	ctx := context.Background()
	history := []db.Message{
		{Role: "assistant", Text: "That sounds really heavy. Are you safe right now?"},
	}

	// The lexicon answers without asking the model.
	llm := &fakeLLM{out: `{"category":"none"}`}
	c := &Classifier{LLM: llm}
	if r := c.Classify(ctx, nil, "I want to kill myself"); r.Category != Suicide || r.Source != SourceLexicon || llm.calls != 0 {
		t.Errorf("lexicon hit = %+v after %d calls", r, llm.calls)
	}

	// The model catches what the lexicon can't, with history for context.
	llm = &fakeLLM{out: `{"category":"suicide","reason":"not safe"}`}
	c = &Classifier{LLM: llm}
	if r := c.Classify(ctx, history, "no"); r.Category != Suicide || r.Source != SourceLLM {
		t.Errorf("model verdict = %+v", r)
	}
	if !strings.Contains(llm.input, "Are you safe right now?") || !strings.Contains(llm.input, "<message>\nno\n</message>") {
		t.Errorf("classifier input missing history or message:\n%s", llm.input)
	}

	// Failing or slow models fall back to the lexicon (fail open).
	for _, f := range []*fakeLLM{
		{err: errors.New("down")},
		{out: "I can't help with that"},
		{out: `{"category":"suicide"}`, delay: time.Second},
	} {
		c = &Classifier{LLM: f, Timeout: 20 * time.Millisecond}
		if r := c.Classify(ctx, history, "no"); r.Flagged() {
			t.Errorf("failed model flagged %+v", r)
		}
	}

	// Lexicon only.
	var nilc *Classifier
	if r := nilc.Classify(ctx, nil, "I cut myself last night"); r.Category != SelfHarm {
		t.Errorf("nil classifier = %+v", r)
	}
}

const defaultResources = `If you might act on thoughts of harming yourself, please reach out now:

- US: call or text 988 (Suicide & Crisis Lifeline), or text HOME to 741741.
- UK & Ireland: call Samaritans on 116 123.
- Elsewhere: find a local line at https://findahelpline.com.
- If you are in immediate danger, call your local emergency number.`

const ukResources = `- Samaritans (UK and Ireland): call 116 123, any time.
- Shout: text SHOUT to 85258.
- Emergency: 999.`

func TestMentions(t *testing.T) {
	if got := Markers(defaultResources); strings.Join(got, ",") != "988,741741,116123,findahelpline.com" {
		t.Errorf("Markers(default) = %v", got)
	}
	if got := Markers(ukResources); strings.Join(got, ",") != "116123,85258" {
		t.Errorf("Markers(uk) = %v", got)
	}
	for _, tc := range []struct {
		reply, resources string
		want             bool
	}{
		{"Please call or text 988 now.", defaultResources, true},
		{"You can find a line at findahelpline.com.", defaultResources, true},
		{"Samaritans are on 116123.", defaultResources, true},
		{"Please reach out to someone.", defaultResources, false},
		{"Call 999 if you're in danger.", ukResources, false},
		{"Call Samaritans on 116 123.", ukResources, true},
		{"Text 9887 for fun", defaultResources, false},
		{"Maybe call my sister tonight.", "Call my sister", true},
		{"Maybe call someone.", "Call my sister", false},
	} {
		if got := Mentions(tc.reply, tc.resources); got != tc.want {
			t.Errorf("Mentions(%q) = %v, want %v", tc.reply, got, tc.want)
		}
	}
}

func TestEnsure(t *testing.T) {
	suicide := Result{Category: Suicide}
	if got := Ensure(suicide, "Are you safe? Call 988.", defaultResources); got != "" {
		t.Errorf("Ensure added %q to a reply that has the resources", got)
	}
	if got := Ensure(suicide, "Are you safe?", defaultResources); !strings.Contains(got, "988") {
		t.Errorf("Ensure = %q, want the resources", got)
	}
	danger := Result{Category: InDanger}
	got := Ensure(danger, "Get somewhere safe. Samaritans: 116 123.", ukResources)
	if !strings.Contains(got, "emergency number") {
		t.Errorf("Ensure(in danger) = %q, want an emergency pointer", got)
	}
	if got := Ensure(danger, "Call 999 now, then Samaritans on 116 123.", ukResources); got != "" {
		t.Errorf("Ensure(in danger) added %q", got)
	}
	if f := Fallback(danger, ukResources); !strings.Contains(f, "999") || !strings.Contains(f, "116 123") {
		t.Errorf("Fallback = %q", f)
	}
}
