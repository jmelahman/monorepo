package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseTag(t *testing.T) {
	cases := map[string]string{
		"0.1.8":            "v0.1.8", // goreleaser strips the v
		"v0.1.8":           "v0.1.8",
		"dev":              "",
		"abc1234":          "",
		"abc1234-dirty":    "",
		"v0.1.5-3-gabc123": "", // git describe with distance
		"0.1.8-SNAPSHOT":   "",
		"":                 "",
	}
	for version, want := range cases {
		if got := releaseTag(version); got != want {
			t.Errorf("releaseTag(%q) = %q, want %q", version, got, want)
		}
	}
}

func TestPreCommitStanza(t *testing.T) {
	release := preCommitStanza("0.1.8")
	for _, want := range []string{"repo: " + preCommitRepoURL, "rev: v0.1.8", "id: local-preview-deploy"} {
		if !strings.Contains(release, want) {
			t.Errorf("release stanza missing %q:\n%s", want, release)
		}
	}

	dev := preCommitStanza("dev")
	if !strings.Contains(dev, "rev: vX.Y.Z") {
		t.Errorf("dev stanza should print a placeholder rev:\n%s", dev)
	}
}

// installHookIn runs install-hook from inside dir and returns its output.
func installHookIn(t *testing.T, dir string, dryRun bool) (string, error) {
	t.Helper()
	t.Chdir(dir)
	var out strings.Builder
	err := runInstallHook(t.Context(), &out, dryRun)
	return out.String(), err
}

func TestInstallHookPreCommitRepo(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".pre-commit-config.yaml"), []byte("repos: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := installHookIn(t, dir, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"repo: " + preCommitRepoURL, "id: local-preview-deploy", "--hook-type post-commit"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "hooks", "post-commit")); !os.IsNotExist(err) {
		t.Errorf("pre-commit repos must not get a hook file written (stat err: %v)", err)
	}
}

func TestInstallHookWritesScript(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := installHookIn(t, dir, false); err != nil {
		t.Fatal(err)
	}
	hookPath := filepath.Join(dir, ".git", "hooks", "post-commit")
	body, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), hookMarker) {
		t.Errorf("hook missing marker:\n%s", body)
	}
	fi, err := os.Stat(hookPath)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&0o111 == 0 {
		t.Errorf("hook not executable: %v", fi.Mode())
	}

	// A hand-written hook (no marker) must be refused, not overwritten.
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\necho mine\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := installHookIn(t, dir, false); err == nil {
		t.Error("expected refusal on hand-written hook")
	}
	body, _ = os.ReadFile(hookPath)
	if !strings.Contains(string(body), "echo mine") {
		t.Error("hand-written hook was overwritten")
	}
}
