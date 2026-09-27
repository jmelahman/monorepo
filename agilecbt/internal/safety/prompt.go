package safety

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// SystemPrompt asks the model tier for one category as JSON. It's written to
// work on small instruction-tuned models as well as dedicated guard models.
const SystemPrompt = `You are a safety classifier for a mental-health support app (CBT for depression and anxiety). You read the latest message a person sent to their AI coach and decide whether it needs a crisis response. You never reply to the person.

Categories:
- suicide: thoughts of suicide or of being dead, wanting to die or disappear, plans, means (saving pills, a method), goodbyes, feeling others would be better off without them. Passive ("I don't see the point anymore, everyone would be better off without me") and joking ("lol I'll just unalive myself") ideation both count.
- self_harm: hurting themselves on purpose now, recently, or wanting to (cutting, burning, hitting themselves), including "it's the only thing that helps".
- harm_others: a real wish or plan to seriously hurt someone, or fear of losing control and hurting someone (such as shaking a crying baby).
- in_danger: in physical danger right now, such as violence or threats at home, being followed, or a medical emergency (chest pain, a numb arm, trouble breathing, an overdose).
- none: everything else, including ordinary stress and sadness, figures of speech ("this deadline is killing me", "I could kill for a coffee", "I'm dead tired"), accidental injuries ("I hurt myself at the gym"), wanting to quit a job or get away for a while, games, films and books, and news about other people.

Use the earlier messages for context: a short answer like "no" or "not really" to the coach asking whether they are safe is a crisis, but the same answer to an ordinary question (about a task, an errand or a plan) is none. If you are unsure between none and a crisis category, choose the crisis category. If more than one applies, choose the most urgent: in_danger, then suicide, then self_harm, then harm_others.

Reply with only a JSON object, no other text:
{"category": "none|suicide|self_harm|harm_others|in_danger", "reason": "a few words"}`

var thinkRE = regexp.MustCompile(`(?s)<think>.*?</think>`)

// Parse reads the model tier's reply. It tolerates reasoning blocks, code
// fences and text around the JSON object.
func Parse(out string) (Result, error) {
	s := thinkRE.ReplaceAllString(out, "")
	start := strings.Index(s, "{")
	if start < 0 {
		return Result{}, fmt.Errorf("classifier reply wasn't JSON: %.200q", out)
	}
	var v struct {
		Category string `json:"category"`
		Reason   string `json:"reason"`
	}
	// Decode just the first value, ignoring anything after it.
	if err := json.NewDecoder(strings.NewReader(s[start:])).Decode(&v); err != nil {
		return Result{}, fmt.Errorf("classifier reply wasn't JSON: %.200q", out)
	}
	cat := Category(strings.NewReplacer("-", "_", " ", "_").Replace(strings.ToLower(strings.TrimSpace(v.Category))))
	if cat == "selfharm" {
		cat = SelfHarm
	}
	if !cat.valid() {
		return Result{}, fmt.Errorf("classifier returned unknown category %q", v.Category)
	}
	if cat == None {
		return Result{Category: None}, nil
	}
	return Result{Category: cat, Source: SourceLLM, Reason: strings.TrimSpace(v.Reason)}, nil
}
