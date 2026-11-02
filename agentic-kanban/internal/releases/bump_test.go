package releases

import "testing"

func TestSuggestBump(t *testing.T) {
	tests := []struct {
		name     string
		subjects []string
		bump     string
	}{
		{name: "no commits", bump: BumpNone},
		{name: "chores", subjects: []string{"chore: tidy", "docs(kanban): explain", "ci: pin"}, bump: BumpNone},
		{name: "fix", subjects: []string{"chore: tidy", "fix(kanban): stop it"}, bump: "patch"},
		{name: "perf", subjects: []string{"perf: go faster"}, bump: "patch"},
		{name: "revert", subjects: []string{"revert(kanban): undo it"}, bump: "patch"},
		{name: "feat beats fix", subjects: []string{"fix: a", "feat(kanban): b", "fix: c"}, bump: "minor"},
		{name: "breaking", subjects: []string{"feat: a", "fix(kanban)!: b"}, bump: "major"},
		{name: "breaking chore", subjects: []string{"chore!: drop Go 1.25"}, bump: "major"},
		{name: "skips others", subjects: []string{"wip probe", "chore: tidy"}, bump: BumpNone},
		{name: "skips among others", subjects: []string{"nit", "feat: a"}, bump: "minor"},
		{name: "unknown type", subjects: []string{"kanban: fix it"}, bump: BumpNone},
		{name: "unknown breaking type", subjects: []string{"kanban!: fix it"}, bump: BumpNone},
		{name: "no space", subjects: []string{"feat:a"}, bump: BumpNone},
		{name: "no description", subjects: []string{"feat: "}, bump: BumpNone},
		{name: "capitalized", subjects: []string{"Feat: a"}, bump: BumpNone},
		{name: "git's own revert", subjects: []string{`Revert "feat: a"`}, bump: BumpNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if bump := SuggestBump(tt.subjects); bump != tt.bump {
				t.Errorf("SuggestBump(%q) = %q; want %q", tt.subjects, bump, tt.bump)
			}
		})
	}
}
