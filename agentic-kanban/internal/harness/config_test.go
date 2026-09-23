package harness

import (
	"os"
	"path/filepath"
	"testing"
)

func TestForSession(t *testing.T) {
	dir := t.TempDir()
	userPath := filepath.Join(dir, "config.toml")
	t.Setenv("KANBAN_CONFIG", userPath)
	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(path, id string) {
		t.Helper()
		if id == "" {
			_ = os.Remove(path)
			return
		}
		if err := os.WriteFile(path, []byte("[harness]\nid = \""+id+"\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	projectPath := filepath.Join(repo, ".kanban.toml")

	for _, tc := range []struct {
		name, session, user, project, want string
	}{
		{name: "nothing_set_uses_default", want: Default().ID},
		{name: "project_default", project: "pi", want: "pi"},
		{name: "user_beats_project", user: "claude", project: "pi", want: "claude"},
		{name: "session_beats_config", session: "pi", user: "claude", project: "claude", want: "pi"},
		{name: "unknown_session_pick_falls_back", session: "gone", project: "pi", want: "pi"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			write(userPath, tc.user)
			write(projectPath, tc.project)
			if got := ForSession(tc.session, repo).ID; got != tc.want {
				t.Errorf("ForSession(%q) = %q; want %q", tc.session, got, tc.want)
			}
		})
	}
}
