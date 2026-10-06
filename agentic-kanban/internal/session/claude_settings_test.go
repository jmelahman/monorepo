package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteClaudeSettings_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	if err := writeClaudeSettings(dir); err != nil {
		t.Fatalf("writeClaudeSettings: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.local.json"))
	if err != nil {
		t.Fatalf("read written settings: %v", err)
	}
	if string(got) != claudeSettings {
		t.Errorf("written file does not match template")
	}
}

func TestWriteClaudeSettings_PreservesExistingFile(t *testing.T) {
	dir := t.TempDir()
	settingsDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	existing := `{"hooks": {"UserPromptSubmit": [{"hooks": [{"type": "command", "command": "/usr/local/bin/my-hook"}]}]}}`
	path := filepath.Join(settingsDir, "settings.local.json")
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	if err := writeClaudeSettings(dir); err != nil {
		t.Fatalf("writeClaudeSettings: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != existing {
		t.Errorf("existing file was modified: got %q want %q", string(got), existing)
	}
}

// A worktree created before the PostToolUse hook holds the old generated
// file. It isn't hand-authored, so it must pick up the new hooks.
func TestWriteClaudeSettings_UpgradesLegacyGeneratedFile(t *testing.T) {
	dir := t.TempDir()
	settingsDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(settingsDir, "settings.local.json")
	if err := os.WriteFile(path, []byte(legacyClaudeSettings), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	if err := writeClaudeSettings(dir); err != nil {
		t.Fatalf("writeClaudeSettings: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != claudeSettings {
		t.Errorf("legacy generated file was not upgraded")
	}
}

func TestClaudeSettingsAreValidJSON(t *testing.T) {
	for name, raw := range map[string]string{"current": claudeSettings, "legacy": legacyClaudeSettings} {
		var v struct {
			Hooks map[string]json.RawMessage `json:"hooks"`
		}
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			t.Fatalf("%s settings: %v", name, err)
		}
		_, hasPost := v.Hooks["PostToolUse"]
		if hasPost != (name == "current") {
			t.Errorf("%s settings: PostToolUse present = %v", name, hasPost)
		}
	}
}
