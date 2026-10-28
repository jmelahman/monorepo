package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmelahman/lockstep/internal/config"
)

func TestRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr    error
		name       string
		wantOut    string
		args       []string
		wantAnyErr bool
	}{
		{name: "help", args: []string{"-h"}, wantOut: "Usage:"},
		{name: "version", args: []string{"--version"}, wantOut: "version"},
		{name: "unknown command", args: []string{"nope"}, wantAnyErr: true},
		{name: "arguments", args: []string{"check", "x"}, wantAnyErr: true},
		{name: "repository without config", args: []string{"check", "-C", newRepo(t, nil)}, wantErr: config.ErrNoConfig},
		{name: "outside a repository", args: []string{"check", "-C", t.TempDir()}, wantAnyErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr strings.Builder
			err := run(tt.args, &stdout, &stderr)
			switch {
			case tt.wantAnyErr:
				if err == nil {
					t.Fatalf("run(%q) = nil, want an error", tt.args)
				}
			case !errors.Is(err, tt.wantErr):
				t.Fatalf("run(%q) error = %v, want %v", tt.args, err, tt.wantErr)
			}
			if !strings.Contains(stdout.String(), tt.wantOut) {
				t.Errorf("run(%q) stdout = %q, want it to contain %q", tt.args, stdout.String(), tt.wantOut)
			}
		})
	}
}

// TestCheckAndSync takes a repository whose copies lag their pins through
// check, sync, and check again. Its hook revs are tags, so sync needs no
// network.
func TestCheckAndSync(t *testing.T) {
	t.Parallel()

	dir := newRepo(t, map[string]string{
		config.File: `
[pins.uv]
source = "pre-commit"
repo = "https://github.com/astral-sh/uv-pre-commit"
action = "astral-sh/setup-uv"

[pins.prek]
source = "uv.lock"
action = "j178/prek-action"
input = "prek-version"
`,
		"uv.lock": `
[[package]]
name = "prek"
version = "0.5.7"
`,
		".pre-commit-config.yaml": `repos:
  - repo: https://github.com/astral-sh/uv-pre-commit
    rev: 0.13.5
    hooks:
      - id: uv-sync
  - repo: local
    hooks:
      - id: pytest
        entry: uv run pytest
        language: python
        additional_dependencies: [uv==0.9.15] # keep this comment
`,
		"sub/.pre-commit-config.yaml": `repos:
  - repo: https://github.com/astral-sh/uv-pre-commit
    rev: 0.12.19
    hooks:
      - id: uv-sync
`,
		".github/workflows/ci.yml": `jobs:
  test:
    steps:
      - uses: astral-sh/setup-uv@08807647e7069bb48b6ef5acd8ec9567f424441b
        with:
          version: "0.12.3"
      - uses: j178/prek-action@44924fe9ef4a6f88253c4b6b1bf11c689d3ce53d
        with:
          prek-version: '0.5.4'
`,
	})

	var stdout strings.Builder
	if err := run([]string{"check", "-C", dir}, &stdout, &stdout); !errors.Is(err, errOutOfDate) {
		t.Fatalf("check = %v, want %v", err, errOutOfDate)
	}
	for _, want := range []string{
		".pre-commit-config.yaml:11: uv 0.9.15, want 0.13.5",
		"sub/.pre-commit-config.yaml:3: uv 0.12.19, want 0.13.5",
		".github/workflows/ci.yml:6: uv 0.12.3, want 0.13.5",
		".github/workflows/ci.yml:9: prek 0.5.4, want 0.5.7",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("check output %q lacks %q", stdout.String(), want)
		}
	}

	if err := run([]string{"sync", "-C", dir}, &stdout, &stdout); !errors.Is(err, errUpdated) {
		t.Fatalf("sync = %v, want %v", err, errUpdated)
	}
	for name, want := range map[string]string{
		".pre-commit-config.yaml":     "additional_dependencies: [uv==0.13.5] # keep this comment\n",
		"sub/.pre-commit-config.yaml": "rev: 0.13.5\n",
		".github/workflows/ci.yml":    "version: \"0.13.5\"\n",
	} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), want) {
			t.Errorf("after sync, %s = %q, want it to contain %q", name, data, want)
		}
	}

	stdout.Reset()
	if err := run([]string{"check", "-C", dir}, &stdout, &stdout); err != nil {
		t.Errorf("check after sync = %v, output %q", err, stdout.String())
	}
}

// newRepo returns a git repository holding files, all tracked.
func newRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}} {
		cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v: %s", args[0], err, out)
		}
	}
	return dir
}
