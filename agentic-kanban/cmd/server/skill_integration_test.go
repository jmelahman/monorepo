package server

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runSkillInstall(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	root := Root()
	root.SetArgs(append([]string{"skill", "install"}, args...))
	root.SetOut(&out)
	root.SetErr(io.Discard)
	err := root.Execute()
	return out.String(), err
}

func TestSkillInstallDefaultsToSharedAgentsDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if _, err := runSkillInstall(t); err != nil {
		t.Fatalf("kanban skill install: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(home, ".agents", "skills", "kanban-init", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	// The directory name has to match the skill's name, and the skill must not
	// cost context until it's asked for.
	for _, want := range []string{"name: kanban-init\n", "disable-model-invocation: true\n"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("SKILL.md is missing %q", want)
		}
	}
}

func TestSkillInstallDir(t *testing.T) {
	for _, tc := range []struct{ name, dir string }{
		{"agent directory", "dotfiles/.agents"},
		{"skills directory", "dotfiles/.agents/skills"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			t.Chdir(base)
			if _, err := runSkillInstall(t, "--dir", tc.dir); err != nil {
				t.Fatalf("kanban skill install: %v", err)
			}
			path := filepath.Join(base, "dotfiles", ".agents", "skills", "kanban-init", "SKILL.md")
			if _, err := os.Stat(path); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestSkillInstallKeepsEditsWithoutForce(t *testing.T) {
	dir := t.TempDir()
	if _, err := runSkillInstall(t, "--dir", dir); err != nil {
		t.Fatalf("first install: %v", err)
	}
	out, err := runSkillInstall(t, "--dir", dir)
	if err != nil {
		t.Fatalf("reinstalling an unchanged skill: %v", err)
	}
	if !strings.Contains(out, "up to date") {
		t.Errorf("reinstall output = %q, want it to say up to date", out)
	}

	path := filepath.Join(dir, "skills", "kanban-init", "SKILL.md")
	if err := os.WriteFile(path, []byte("my edits"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runSkillInstall(t, "--dir", dir); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("installing over edits: err = %v, want one naming --force", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "my edits" {
		t.Errorf("edits were replaced with %q", got)
	}

	if _, err := runSkillInstall(t, "--dir", dir, "--force"); err != nil {
		t.Fatalf("install --force: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) == "my edits" {
		t.Error("--force left the edits in place")
	}
}
