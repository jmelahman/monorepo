package api

import (
	"path/filepath"
	"testing"
)

// The agent writes relative plan paths from its own working directory, so a
// board scoped to a subproject needs the plans dir joined onto the project
// root — otherwise the Plans tab is permanently empty. Absolute values
// (including the `~/.claude/plans` default) stay global.
func TestPlansDirUnder(t *testing.T) {
	projectRoot := filepath.Join("/wt", "services", "api")
	cases := []struct {
		name       string
		configured string
		root       string
		want       string
	}{
		{
			name:       "relative joins onto the project root",
			configured: "plans",
			root:       projectRoot,
			want:       filepath.Join(projectRoot, "plans"),
		},
		{
			name:       "dot-relative joins too",
			configured: "./plans",
			root:       projectRoot,
			want:       filepath.Join(projectRoot, "plans"),
		},
		{
			name:       "absolute stays global",
			configured: "/home/u/.claude/plans",
			root:       projectRoot,
			want:       "/home/u/.claude/plans",
		},
		{
			name:       "no root leaves it alone",
			configured: "plans",
			root:       "",
			want:       "plans",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := plansDirUnder(tc.configured, tc.root); got != tc.want {
				t.Errorf("plansDirUnder(%q, %q) = %q; want %q", tc.configured, tc.root, got, tc.want)
			}
		})
	}
}
