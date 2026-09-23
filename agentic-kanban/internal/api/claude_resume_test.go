package api

import (
	"os"
	"path/filepath"
	"testing"
)

// claudeTranscriptExists is the guard that prevents wsPTY from running
// `claude --resume <uuid>` against a UUID whose JSONL was never persisted —
// e.g. when SessionStart fires on `claude` boot but the container is
// restarted before any prompt is submitted, leaving a dead UUID in the DB.
func TestClaudeTranscriptExists(t *testing.T) {
	const uuid = "abcdef01-2345-6789-abcd-0123456789ab"

	t.Run("missing_when_no_jsonl_on_disk", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		if claudeTranscriptExists(uuid) {
			t.Fatal("expected false when ~/.claude/projects is empty")
		}
	})

	t.Run("found_under_any_project_subdir", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		// Claude encodes the cwd into the project subdir name; from inside
		// the session container that's `/workspace` → `-workspace`. We
		// glob across subdirs rather than coupling to that encoding.
		dir := filepath.Join(home, ".claude", "projects", "-workspace")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, uuid+".jsonl"), nil, 0o644); err != nil {
			t.Fatalf("write jsonl: %v", err)
		}
		if !claudeTranscriptExists(uuid) {
			t.Fatal("expected true once jsonl is on disk")
		}
	})

	t.Run("ignores_unrelated_uuids", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		dir := filepath.Join(home, ".claude", "projects", "-workspace")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		other := "11111111-2222-3333-4444-555555555555"
		if err := os.WriteFile(filepath.Join(dir, other+".jsonl"), nil, 0o644); err != nil {
			t.Fatalf("write jsonl: %v", err)
		}
		if claudeTranscriptExists(uuid) {
			t.Fatal("expected false when only an unrelated uuid is on disk")
		}
	})
}
