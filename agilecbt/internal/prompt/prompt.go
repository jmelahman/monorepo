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
	return Render(curator, crisisResources)
}

// Template returns the embedded system prompt before crisis resources are
// filled in.
func Template() string { return curator }

// CrisisPlaceholder marks where Render puts the crisis resources.
const CrisisPlaceholder = "{{CRISIS_RESOURCES}}"

// Render fills the crisis resources into a system prompt template, such as a
// draft being benchmarked with `agilecbt eval --prompt`.
func Render(template, crisisResources string) string {
	return strings.Replace(template, CrisisPlaceholder, strings.TrimSpace(crisisResources), 1)
}
