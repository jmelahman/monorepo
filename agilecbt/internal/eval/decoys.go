package eval

import (
	"encoding/json"
	"slices"

	"github.com/jmelahman/agilecbt/internal/tools"
)

// decoyDefs are plausible tools for this app that no scenario needs. Adding
// them measures how a model's tool use holds up as the tool list grows.
// They aren't registered, so a call returns an unknown-tool error.
var decoyDefs = []struct{ name, desc, arg string }{
	{"archive_value", "Archive a value the person no longer wants on their roadmap.", "value_id"},
	{"export_week", "Export a week's check-ins and steps as a shareable summary.", "week_id"},
	{"set_reminder", "Schedule a reminder notification for a step at a given time.", "step_id"},
	{"list_reminders", "List upcoming reminder notifications.", ""},
	{"rename_lane", "Rename a board lane for this person.", "lane"},
	{"get_streaks", "Get check-in and step-completion streaks.", ""},
	{"merge_goals", "Merge two goals into one, moving their steps.", "goal_id"},
	{"duplicate_step", "Copy a step, e.g. to repeat it tomorrow.", "step_id"},
	{"list_retros", "List past weekly retrospectives.", ""},
	{"get_retro", "Get one week's retrospective.", "week_id"},
	{"set_theme", "Change the app's color theme.", "theme"},
	{"get_settings", "Get the person's app settings.", ""},
	{"list_checkins", "List check-ins in a date range.", "from"},
	{"tag_step", "Add a tag to a step for filtering.", "step_id"},
	{"list_tags", "List tags used on steps.", ""},
	{"get_value", "Get one value with its goals.", "value_id"},
	{"reorder_values", "Change the order values are shown in.", "value_ids"},
	{"snooze_step", "Hide a step until a later date.", "step_id"},
	{"get_insights", "Summarize trends in mood and completed steps.", ""},
	{"log_sleep", "Record last night's hours of sleep.", "hours"},
	{"log_medication", "Record that a medication was taken.", "name"},
	{"share_progress", "Share a progress summary with a trusted person.", "contact"},
	{"list_habits", "List recurring habits.", ""},
	{"create_habit", "Create a recurring habit.", "title"},
	{"pause_habit", "Pause a recurring habit.", "habit_id"},
	{"get_step", "Get one step with its notes and history.", "step_id"},
	{"search_notes", "Search the coach's saved notes.", "query"},
	{"list_goals_by_value", "List the goals linked to one value.", "value_id"},
	{"set_energy_budget", "Set how much energy today's plan may use.", "budget"},
	{"start_timer", "Start a focus timer for a step.", "step_id"},
}

// decoyTools returns the first n decoy tools.
func decoyTools(n int) []tools.Tool {
	n = min(n, len(decoyDefs))
	out := make([]tools.Tool, n)
	for i, d := range decoyDefs[:n] {
		props := map[string]any{}
		var required []string
		if d.arg != "" {
			props[d.arg] = map[string]any{"type": "string", "description": "The " + d.arg + "."}
			required = []string{d.arg}
		}
		schema := map[string]any{"type": "object", "properties": props, "additionalProperties": false}
		if required != nil {
			schema["required"] = required
		}
		raw, _ := json.Marshal(schema)
		out[i] = tools.Tool{Name: d.name, Description: d.desc, InputSchema: raw, Mutates: true}
	}
	return out
}

// MaxDecoys is how many decoy tools exist.
func MaxDecoys() int { return len(decoyDefs) }

// toolset is what the model is offered: the registry (or the --tools
// subset) plus decoys, and the decoys' names.
func toolset(reg *tools.Registry, opts Options) ([]tools.Tool, []string) {
	var out []tools.Tool
	for _, t := range reg.List() {
		if len(opts.Tools) == 0 || slices.Contains(opts.Tools, t.Name) {
			out = append(out, t)
		}
	}
	var names []string
	for _, d := range decoyTools(opts.Decoys) {
		out = append(out, d)
		names = append(names, d.Name)
	}
	return out, names
}

// ToolPayload measures the tool definitions sent on every turn.
type ToolPayload struct {
	Count  int `json:"count"`
	Decoys int `json:"decoys"`
	Bytes  int `json:"bytes"`
	// Tokens is a rough estimate (bytes / 4).
	Tokens int `json:"approx_tokens"`
}

func payload(list []tools.Tool, decoys int) ToolPayload {
	type fn struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	}
	type def struct {
		Type     string `json:"type"`
		Function fn     `json:"function"`
	}
	defs := make([]def, len(list))
	for i, t := range list {
		defs[i] = def{"function", fn{t.Name, t.Description, t.InputSchema}}
	}
	raw, _ := json.Marshal(defs)
	return ToolPayload{Count: len(list), Decoys: decoys, Bytes: len(raw), Tokens: len(raw) / 4}
}
