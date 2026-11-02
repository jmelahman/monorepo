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

// isolatedHome points $HOME at a temp directory, so a test can't link into the
// real ~/.claude/skills, and optionally gives it a ~/.claude.
func isolatedHome(t *testing.T, withClaude bool) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if withClaude {
		if err := os.Mkdir(filepath.Join(home, ".claude"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func TestSkillInstallDefaultsToSharedAgentsDir(t *testing.T) {
	home := isolatedHome(t, false)
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
			isolatedHome(t, false)
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
	isolatedHome(t, false)
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

func TestSkillInstallLinksIntoClaude(t *testing.T) {
	home := isolatedHome(t, true)
	dotfiles := t.TempDir()
	if _, err := runSkillInstall(t, "--dir", filepath.Join(dotfiles, ".agents")); err != nil {
		t.Fatalf("kanban skill install: %v", err)
	}

	link := filepath.Join(home, ".claude", "skills", "kanban-init")
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dotfiles, ".agents", "skills", "kanban-init"); target != want {
		t.Errorf("link target = %q, want %q", target, want)
	}
	if _, err := os.Stat(filepath.Join(link, "SKILL.md")); err != nil {
		t.Errorf("reading the skill through the link: %v", err)
	}

	out, err := runSkillInstall(t, "--dir", filepath.Join(dotfiles, ".agents"))
	if err != nil {
		t.Fatalf("reinstall: %v", err)
	}
	if strings.Contains(out, "linked") {
		t.Errorf("reinstall relinked an existing link: %q", out)
	}
}

func TestSkillInstallSkipsClaudeLink(t *testing.T) {
	t.Run("without a ~/.claude", func(t *testing.T) {
		home := isolatedHome(t, false)
		if _, err := runSkillInstall(t); err != nil {
			t.Fatalf("kanban skill install: %v", err)
		}
		if _, err := os.Stat(filepath.Join(home, ".claude")); !os.IsNotExist(err) {
			t.Errorf("~/.claude was created: stat err = %v", err)
		}
	})
	t.Run("with --link-claude=false", func(t *testing.T) {
		home := isolatedHome(t, true)
		if _, err := runSkillInstall(t, "--link-claude=false"); err != nil {
			t.Fatalf("kanban skill install: %v", err)
		}
		if _, err := os.Lstat(filepath.Join(home, ".claude", "skills")); !os.IsNotExist(err) {
			t.Errorf("~/.claude/skills was created: lstat err = %v", err)
		}
	})
	t.Run("when installing into ~/.claude", func(t *testing.T) {
		home := isolatedHome(t, true)
		if _, err := runSkillInstall(t, "--dir", filepath.Join(home, ".claude")); err != nil {
			t.Fatalf("kanban skill install: %v", err)
		}
		info, err := os.Lstat(filepath.Join(home, ".claude", "skills", "kanban-init"))
		if err != nil {
			t.Fatal(err)
		}
		if !info.IsDir() {
			t.Errorf("skill directory mode = %v, want a real directory", info.Mode())
		}
	})
}

func TestSkillInstallLinkConflicts(t *testing.T) {
	t.Run("a real directory is never replaced", func(t *testing.T) {
		home := isolatedHome(t, true)
		kept := filepath.Join(home, ".claude", "skills", "kanban-init", "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(kept), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(kept, []byte("my skill"), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{nil, {"--force"}} {
			if _, err := runSkillInstall(t, args...); err == nil || !strings.Contains(err.Error(), "isn't a symlink") {
				t.Fatalf("install %v: err = %v, want a refusal", args, err)
			}
		}
		if got, _ := os.ReadFile(kept); string(got) != "my skill" {
			t.Errorf("real directory's skill = %q", got)
		}
		if _, err := os.Stat(filepath.Join(home, ".agents")); !os.IsNotExist(err) {
			t.Errorf("refused install still wrote the skill: stat err = %v", err)
		}
	})
	t.Run("a foreign link needs --force", func(t *testing.T) {
		home := isolatedHome(t, true)
		link := filepath.Join(home, ".claude", "skills", "kanban-init")
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			t.Fatal(err)
		}
		foreign := t.TempDir()
		if err := os.Symlink(foreign, link); err != nil {
			t.Fatal(err)
		}
		if _, err := runSkillInstall(t); err == nil || !strings.Contains(err.Error(), "--force") {
			t.Fatalf("install: err = %v, want one naming --force", err)
		}
		if _, err := runSkillInstall(t, "--force"); err != nil {
			t.Fatalf("install --force: %v", err)
		}
		if _, err := os.Stat(filepath.Join(link, "SKILL.md")); err != nil {
			t.Errorf("reading the skill through the replaced link: %v", err)
		}
		if entries, _ := os.ReadDir(foreign); len(entries) != 0 {
			t.Errorf("--force wrote through the old link: %v", entries)
		}
	})
}

// A session container can write to ~/.claude, so anything below the skills
// directory may have been planted to redirect the install's write on the host.
func TestSkillInstallNeverWritesThroughSymlinks(t *testing.T) {
	for _, args := range [][]string{nil, {"--force"}} {
		name := "without --force"
		if args != nil {
			name = "with --force"
		}
		t.Run(name, func(t *testing.T) {
			t.Run("SKILL.md links to a file", func(t *testing.T) {
				dir := t.TempDir()
				victim := filepath.Join(t.TempDir(), "bashrc")
				if err := os.WriteFile(victim, []byte("precious"), 0o644); err != nil {
					t.Fatal(err)
				}
				plantSkillFileLink(t, dir, victim)
				if _, err := runSkillInstall(t, append([]string{"--dir", dir}, args...)...); err == nil {
					t.Error("install followed the link")
				}
				if got, _ := os.ReadFile(victim); string(got) != "precious" {
					t.Errorf("link target was overwritten with %q", got)
				}
			})
			t.Run("SKILL.md dangles", func(t *testing.T) {
				dir := t.TempDir()
				victim := filepath.Join(t.TempDir(), "created")
				plantSkillFileLink(t, dir, victim)
				if _, err := runSkillInstall(t, append([]string{"--dir", dir}, args...)...); err == nil {
					t.Error("install followed the link")
				}
				if _, err := os.Lstat(victim); !os.IsNotExist(err) {
					t.Errorf("link target was created: lstat err = %v", err)
				}
			})
			t.Run("the skill directory links elsewhere", func(t *testing.T) {
				isolatedHome(t, false)
				dir := t.TempDir()
				victim := t.TempDir()
				link := filepath.Join(dir, "skills", "kanban-init")
				if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(victim, link); err != nil {
					t.Fatal(err)
				}
				if _, err := runSkillInstall(t, append([]string{"--dir", dir}, args...)...); err == nil {
					t.Error("install followed the link")
				}
				if entries, _ := os.ReadDir(victim); len(entries) != 0 {
					t.Errorf("link target was written to: %v", entries)
				}
			})
			t.Run("~/.claude is the install dir and already holds the link", func(t *testing.T) {
				home := isolatedHome(t, true)
				if _, err := runSkillInstall(t); err != nil {
					t.Fatalf("first install: %v", err)
				}
				if _, err := runSkillInstall(t, append([]string{"--dir", filepath.Join(home, ".claude")}, args...)...); err == nil || !strings.Contains(err.Error(), "real directory") {
					t.Errorf("install: err = %v, want a refusal", err)
				}
			})
		})
	}
}

func plantSkillFileLink(t *testing.T, dir, target string) {
	t.Helper()
	isolatedHome(t, false)
	skillDir := filepath.Join(dir, "skills", "kanban-init")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(skillDir, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
}
