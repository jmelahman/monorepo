// Package prompt holds the curator's system prompt, shared by the in-app
// curator and the MCP daily_checkin prompt.
package prompt

import (
	_ "embed"
	"strings"
)

//go:embed curator.md
var curator string

// Curator returns the system prompt with the user's crisis resources filled
// in. The text is otherwise stable so providers can cache it.
func Curator(crisisResources string) string {
	return strings.Replace(curator, "{{CRISIS_RESOURCES}}", strings.TrimSpace(crisisResources), 1)
}
