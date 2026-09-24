package tasks

import "testing"

func TestSubstituteVSCodeVars(t *testing.T) {
	const ws = "/workspace/services/api"
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "workspaceFolder", in: "${workspaceFolder}/bin", want: "/workspace/services/api/bin"},
		{name: "workspaceRoot alias", in: "${workspaceRoot}", want: "/workspace/services/api"},
		{name: "basename", in: "${workspaceFolderBasename}", want: "api"},
		{name: "unknown var left literal", in: "${notAVar}", want: "${notAVar}"},
		{name: "multiple", in: "${workspaceFolder}:${workspaceFolderBasename}", want: "/workspace/services/api:api"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := substituteVSCodeVars(tc.in, ws); got != tc.want {
				t.Errorf("substituteVSCodeVars(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestResolveContainerPath(t *testing.T) {
	// A task's cwd has to resolve against the same directory the task is run
	// from. For a monorepo board that is the subproject, not /workspace — a
	// root-level tasks.json entry with a relative cwd would otherwise land in
	// the wrong tree. Docker exec also rejects any non-absolute WorkingDir.
	const ws = "/workspace/services/api"
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty defaults to the workspace folder", in: "", want: ws},
		{name: "whitespace only", in: "   ", want: ws},
		{name: "relative joins onto the workspace folder", in: "web", want: "/workspace/services/api/web"},
		{name: "substituted then joined", in: "${workspaceFolder}/web", want: "/workspace/services/api/web"},
		{name: "absolute passes through", in: "/opt/tool", want: "/opt/tool"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveContainerPath(tc.in, ws); got != tc.want {
				t.Errorf("resolveContainerPath(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}
