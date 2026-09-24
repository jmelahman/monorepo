package harness

import (
	"strings"
	"testing"
)

func TestRenderCommitScript(t *testing.T) {
	claude := Get("claude")

	t.Run("interpolates the work dir", func(t *testing.T) {
		// The agent's cwd is the devcontainer's workspaceFolder descended
		// into the board's project_dir; a hardcoded /workspace would run the
		// commit-message pass from the wrong directory in a monorepo.
		got, err := claude.RenderCommitScript("summarize", "/workspace/services/api")
		if err != nil {
			t.Fatalf("RenderCommitScript: %v", err)
		}
		if !strings.HasPrefix(got, "cd '/workspace/services/api' &&") {
			t.Errorf("script = %q; want it to cd into the work dir first", got)
		}
	})

	t.Run("defaults to /workspace", func(t *testing.T) {
		got, err := claude.RenderCommitScript("summarize", "")
		if err != nil {
			t.Fatalf("RenderCommitScript: %v", err)
		}
		if !strings.HasPrefix(got, "cd '/workspace' &&") {
			t.Errorf("script = %q; want the /workspace default", got)
		}
	})

	t.Run("still shell-quotes the prompt", func(t *testing.T) {
		got, err := claude.RenderCommitScript("it's fine; rm -rf /", "/workspace")
		if err != nil {
			t.Fatalf("RenderCommitScript: %v", err)
		}
		// The apostrophe must be escaped, not left to close the quote early
		// and hand the rest of the prompt to the shell.
		if !strings.Contains(got, `-p 'it'\''s fine; rm -rf /'`) {
			t.Errorf("script = %q; want the prompt shell-quoted intact", got)
		}
	})

	t.Run("quotes a work dir with a space", func(t *testing.T) {
		got, err := claude.RenderCommitScript("x", "/workspace/my project")
		if err != nil {
			t.Fatalf("RenderCommitScript: %v", err)
		}
		if !strings.Contains(got, `cd '/workspace/my project'`) {
			t.Errorf("script = %q; want the work dir quoted", got)
		}
	})

	t.Run("empty template disables the script", func(t *testing.T) {
		got, err := Get("pi").RenderCommitScript("x", "/workspace/api")
		if err != nil {
			t.Fatalf("RenderCommitScript: %v", err)
		}
		if got != "" {
			t.Errorf("script = %q; want empty", got)
		}
	})
}
