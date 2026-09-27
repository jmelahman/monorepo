package safety

import (
	"fmt"
	"strings"
)

// Directive is appended to a flagged turn's user message so the coach knows
// the app switched it into the crisis response, whatever it would have
// noticed on its own.
func Directive(r Result, resources string) string {
	var focus string
	switch r.Category {
	case InDanger:
		focus = "They may be in danger right now. Lead with getting safe: calling emergency services or getting somewhere safe. Keep it short and practical enough to read in a hurry."
	case HarmOthers:
		focus = "They may want to seriously hurt someone. Respond with care, not judgment, and check on safety right now, theirs and the other person's."
	case SelfHarm:
		focus = "They may be hurting themselves. Ask directly whether they are safe right now."
	default:
		focus = "They may be thinking about suicide, even if it was phrased as a joke or in passing. Ask directly whether they are safe right now."
	}
	return fmt.Sprintf(`[Safety check: the app flagged this message as possible %s. Follow the Safety section now. %s Take it seriously, share the crisis resources below, and encourage reaching a real person now. Tools are off for this reply; don't plan, move steps or talk about the board.]

Crisis resources:
%s`, strings.ReplaceAll(string(r.Category), "_", " "), focus, strings.TrimSpace(resources))
}

// Fallback is the reply when the coach fails or says nothing on a flagged
// turn.
func Fallback(r Result, resources string) string {
	lead := "I'm really glad you told me, and I'm taking it seriously. Are you safe right now?"
	switch r.Category {
	case InDanger:
		lead = "If you're in danger right now, call your local emergency number (911 in the US, 999 in the UK, 112 in Europe) or get somewhere safe first. Are you safe right now?"
	case HarmOthers:
		lead = "I'm glad you told me. Are you and the people around you safe right now? If anyone is in danger, call your local emergency number."
	}
	return lead + "\n\n" + strings.TrimSpace(resources)
}

// Ensure returns what to append to a flagged reply so the resources always
// reach the person: the resources when the reply shares none of them, and an
// emergency pointer when someone may be in danger and the reply has none.
func Ensure(r Result, reply, resources string) string {
	var add []string
	if !Mentions(reply, resources) {
		add = append(add, strings.TrimSpace(resources))
	}
	if (r.Category == InDanger || r.Category == HarmOthers) && !MentionsEmergency(reply) && !MentionsEmergency(strings.Join(add, "\n")) {
		add = append(add, "If anyone is in immediate danger, call your local emergency number.")
	}
	if len(add) == 0 {
		return ""
	}
	return "\n\n" + strings.Join(add, "\n\n")
}
