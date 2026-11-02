package releases

import "testing"

func TestSuggestBump(t *testing.T) {
	tests := []struct {
		name         string
		subjects     []string
		bump         string
		unclassified int
	}{
		{name: "no commits", bump: BumpNone},
		{name: "chores", subjects: []string{"chore: tidy", "docs(kanban): explain", "ci: pin"}, bump: BumpNone},
		{name: "fix", subjects: []string{"chore: tidy", "fix(kanban): stop it"}, bump: "patch"},
		{name: "perf", subjects: []string{"perf: go faster"}, bump: "patch"},
		{name: "revert", subjects: []string{"revert(kanban): undo it"}, bump: "patch"},
		{name: "feat beats fix", subjects: []string{"fix: a", "feat(kanban): b", "fix: c"}, bump: "minor"},
		{name: "breaking", subjects: []string{"feat: a", "fix(kanban)!: b"}, bump: "major"},
		{name: "breaking chore", subjects: []string{"chore!: drop Go 1.25"}, bump: "major"},
		{name: "unclassified", subjects: []string{"wip probe", "chore: tidy"}, bump: "patch", unclassified: 1},
		{name: "unclassified doesn't lower", subjects: []string{"nit", "feat: a"}, bump: "minor", unclassified: 1},
		{name: "unknown type", subjects: []string{"kanban: fix it"}, bump: "patch", unclassified: 1},
		{name: "unknown breaking type", subjects: []string{"kanban!: fix it"}, bump: "patch", unclassified: 1},
		{name: "no space", subjects: []string{"feat:a"}, bump: "patch", unclassified: 1},
		{name: "no description", subjects: []string{"feat: "}, bump: "patch", unclassified: 1},
		{name: "capitalized", subjects: []string{"Feat: a"}, bump: "patch", unclassified: 1},
		{name: "git's own revert", subjects: []string{`Revert "feat: a"`}, bump: "patch", unclassified: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bump, unclassified := SuggestBump(tt.subjects)
			if bump != tt.bump || unclassified != tt.unclassified {
				t.Errorf("SuggestBump(%q) = %q, %d; want %q, %d", tt.subjects, bump, unclassified, tt.bump, tt.unclassified)
			}
		})
	}
}
