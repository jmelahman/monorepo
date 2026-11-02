package skills

import (
	"os"
	"path/filepath"
	"testing"
)

// These call the writers directly to cover a symlink swapped in after Install's
// checks have passed, which the checks themselves can't catch.

func TestWriteSkillStaysInsideSkillsDir(t *testing.T) {
	skillsDir, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(skillsDir, "kanban-init")); err != nil {
		t.Fatal(err)
	}
	if err := writeSkill(skillsDir, "kanban-init", []byte("skill")); err == nil {
		t.Error("writeSkill followed a link out of the skills directory")
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Errorf("wrote outside the skills directory: %v", entries)
	}
}

func TestWriteSkillReplacesALinkedFileRatherThanItsTarget(t *testing.T) {
	skillsDir := t.TempDir()
	victim := filepath.Join(t.TempDir(), "bashrc")
	if err := os.WriteFile(victim, []byte("precious"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(skillsDir, "kanban-init", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, dest); err != nil {
		t.Fatal(err)
	}
	if err := writeSkill(skillsDir, "kanban-init", []byte("skill")); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(victim); string(got) != "precious" {
		t.Errorf("link target was overwritten with %q", got)
	}
	if info, err := os.Lstat(dest); err != nil || !info.Mode().IsRegular() {
		t.Errorf("SKILL.md = %v, %v, want a regular file", info, err)
	}
}

func TestReplaceLinkRefusesARealDirectory(t *testing.T) {
	linkDir := t.TempDir()
	kept := filepath.Join(linkDir, "kanban-init", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(kept), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(kept, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := replaceLink(linkDir, "kanban-init", t.TempDir()); err == nil {
		t.Error("replaceLink replaced a real directory")
	}
	if got, _ := os.ReadFile(kept); string(got) != "mine" {
		t.Errorf("directory's contents = %q", got)
	}
	if _, err := os.Lstat(filepath.Join(linkDir, ".kanban-init.tmp")); !os.IsNotExist(err) {
		t.Errorf("temporary link left behind: lstat err = %v", err)
	}
}
