// Package safety flags crisis messages (suicide, self-harm, harm to others,
// immediate danger) before the coach replies, so the app can enforce the
// crisis response instead of trusting the chat model to notice.
//
// It has two tiers: an in-process lexicon that is always on, and an optional
// classification call to any OpenAI-compatible model for what the lexicon
// can't see (passive ideation, indirect phrasing, a bare "no" after "are you
// safe?").
package safety

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jmelahman/agilecbt/internal/db"
)

// Category is what a message was flagged as.
type Category string

const (
	None       Category = "none"
	Suicide    Category = "suicide"
	SelfHarm   Category = "self_harm"
	HarmOthers Category = "harm_others"
	InDanger   Category = "in_danger"
)

// Categories lists the crisis categories, most urgent first.
var Categories = []Category{InDanger, Suicide, SelfHarm, HarmOthers}

func (c Category) valid() bool {
	return c == None || c == Suicide || c == SelfHarm || c == HarmOthers || c == InDanger
}

// Sources of a Result.
const (
	SourceLexicon = "lexicon"
	SourceLLM     = "llm"
)

// Result is the verdict on one message.
type Result struct {
	Category Category `json:"category"`
	// Source is SourceLexicon or SourceLLM; empty when nothing was flagged.
	Source string `json:"source,omitempty"`
	// Reason is the matched phrase or the classifier's explanation.
	Reason string `json:"reason,omitempty"`
}

// Flagged reports whether the message needs the crisis response.
func (r Result) Flagged() bool { return r.Category != "" && r.Category != None }

// Completer is a single tool-free model call, such as curator.OpenAI.
type Completer interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

// DefaultTimeout bounds the classifier call. It runs before every coach turn,
// so a slow or hung classifier must not hold the reply up for long.
const DefaultTimeout = 20 * time.Second

// Classifier flags crisis messages.
type Classifier struct {
	// LLM is the model tier; nil means lexicon only.
	LLM Completer
	// Timeout bounds the LLM call (DefaultTimeout when zero).
	Timeout time.Duration
}

// historyTurns is how many earlier messages the model tier sees, enough for
// "are you safe right now?" / "no".
const historyTurns = 4

// Classify checks the new message text, with recent history for context.
// The lexicon runs first; on a hit the model isn't asked. When the model
// fails or times out, Classify falls back to the lexicon's verdict: failing
// closed would put every turn into crisis mode while the model is down.
func (c *Classifier) Classify(ctx context.Context, history []db.Message, text string) Result {
	if r := Match(text); r.Flagged() || c == nil || c.LLM == nil {
		return r
	}
	timeout := c.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := c.LLM.Complete(ctx, SystemPrompt, Input(history, text))
	if err == nil {
		var r Result
		if r, err = Parse(out); err == nil {
			return r
		}
	}
	log.Printf("safety: classifier failed, using the lexicon's verdict: %v", err)
	return Result{Category: None}
}

// Input renders the classifier's user message.
func Input(history []db.Message, text string) string {
	var b strings.Builder
	var recent []db.Message
	for _, m := range history {
		if m.Role == "user" || m.Role == "assistant" {
			recent = append(recent, m)
		}
	}
	if len(recent) > historyTurns {
		recent = recent[len(recent)-historyTurns:]
	}
	if len(recent) > 0 {
		b.WriteString("<earlier>\n")
		for _, m := range recent {
			role := "person"
			if m.Role == "assistant" {
				role = "coach"
			}
			fmt.Fprintf(&b, "[%s] %s\n", role, clip(strings.TrimSpace(m.Text), 600))
		}
		b.WriteString("</earlier>\n\n")
	}
	fmt.Fprintf(&b, "<message>\n%s\n</message>", strings.TrimSpace(text))
	return b.String()
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
