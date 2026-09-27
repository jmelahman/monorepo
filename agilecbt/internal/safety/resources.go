package safety

import (
	"regexp"
	"strings"
)

var (
	numberRE = regexp.MustCompile(`\d(?:[ \-.]?\d){2,}`)
	domainRE = regexp.MustCompile(`(?i)\b(?:https?://)?(?:www\.)?((?:[a-z0-9-]+\.)+(?:com|org|net|gov|uk|ie|au|ca|nz|us|io|info|help))\b`)
	nonDigit = regexp.MustCompile(`\D`)
)

// emergency numbers don't count as sharing a crisis line: "call 999" alone
// leaves out the Samaritans line the person configured.
var emergency = map[string]bool{"911": true, "999": true, "112": true, "000": true}

// Markers returns what identifies the configured resources in a reply: their
// phone numbers (digits only) and web domains, leaving out bare emergency
// numbers. With neither, it falls back to the resource lines themselves.
func Markers(resources string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, n := range numberRE.FindAllString(resources, -1) {
		if d := nonDigit.ReplaceAllString(n, ""); !emergency[d] {
			add(d)
		}
	}
	for _, m := range domainRE.FindAllStringSubmatch(resources, -1) {
		add(strings.ToLower(m[1]))
	}
	if len(out) > 0 {
		return out
	}
	for _, l := range strings.Split(resources, "\n") {
		add(strings.ToLower(strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(l), "-*• "))))
	}
	return out
}

// Mentions reports whether reply shares at least one of the resources.
func Mentions(reply, resources string) bool {
	markers := Markers(resources)
	if len(markers) == 0 {
		return true
	}
	numbers := map[string]bool{}
	for _, n := range numberRE.FindAllString(reply, -1) {
		numbers[nonDigit.ReplaceAllString(n, "")] = true
	}
	lower := strings.ToLower(reply)
	for _, m := range markers {
		if numbers[m] || (nonDigit.MatchString(m) && strings.Contains(lower, m)) {
			return true
		}
	}
	return false
}

var emergencyRE = regexp.MustCompile(`(?i)\b(911|999|112|000|emergency)\b`)

// MentionsEmergency reports whether reply points to emergency services.
func MentionsEmergency(reply string) bool { return emergencyRE.MatchString(reply) }
