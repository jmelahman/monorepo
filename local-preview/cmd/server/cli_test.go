package server

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/jmelahman/local-preview/internal/client"
)

// isolateConfig points the config-dir lookup at a fresh temp dir, so tests
// never read (or write) the developer's real ~/.config/preview.
func isolateConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("PREVIEW_CONFIG_DIR", dir)
	return dir
}

// writeClientConfig drops a config.toml into the isolated config dir.
func writeClientConfig(t *testing.T, dir, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

// runLeaf executes a parent command with an inherited --server flag and
// returns what resolveURL reports inside the leaf's RunE — the same shape as
// the real CLI subcommands.
func runLeaf(t *testing.T, args ...string) string {
	t.Helper()
	url, err := runLeafErr(t, args...)
	if err != nil {
		t.Fatal(err)
	}
	return url
}

func runLeafErr(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var serverURL string
	var got string
	var gotErr error
	parent := &cobra.Command{Use: "root"}
	addServerFlag(parent, &serverURL)
	leaf := &cobra.Command{
		Use:           "leaf",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			got, gotErr = resolveURL(cmd, serverURL)
			return gotErr
		},
	}
	parent.AddCommand(leaf)
	parent.SetArgs(append([]string{"leaf"}, args...))
	parent.SetOut(io.Discard)
	parent.SetErr(io.Discard)
	if err := parent.Execute(); err != nil {
		return "", err
	}
	return got, gotErr
}

func TestResolveURL(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		isolateConfig(t)
		if got := runLeaf(t); got != defaultServerURL {
			t.Errorf("resolveURL = %q, want default", got)
		}
	})

	t.Run("env wins over default", func(t *testing.T) {
		isolateConfig(t)
		t.Setenv("PREVIEW_URL", "http://env:1234")
		if got := runLeaf(t); got != "http://env:1234" {
			t.Errorf("resolveURL = %q, want env value", got)
		}
	})

	t.Run("explicit flag wins over env", func(t *testing.T) {
		isolateConfig(t)
		t.Setenv("PREVIEW_URL", "http://env:1234")
		if got := runLeaf(t, "--server", "http://flag:5678"); got != "http://flag:5678" {
			t.Errorf("resolveURL = %q, want flag value", got)
		}
	})

	t.Run("config file wins over default", func(t *testing.T) {
		dir := isolateConfig(t)
		writeClientConfig(t, dir, "server = \"https://preview.example.com\"\n")
		if got := runLeaf(t); got != "https://preview.example.com" {
			t.Errorf("resolveURL = %q, want the configured server", got)
		}
	})

	t.Run("env wins over config file", func(t *testing.T) {
		dir := isolateConfig(t)
		writeClientConfig(t, dir, "server = \"https://preview.example.com\"\n")
		t.Setenv("PREVIEW_URL", "http://env:1234")
		if got := runLeaf(t); got != "http://env:1234" {
			t.Errorf("resolveURL = %q, want env value", got)
		}
	})

	t.Run("explicit flag wins over config file", func(t *testing.T) {
		dir := isolateConfig(t)
		writeClientConfig(t, dir, "server = \"https://preview.example.com\"\n")
		if got := runLeaf(t, "--server", "http://flag:5678"); got != "http://flag:5678" {
			t.Errorf("resolveURL = %q, want flag value", got)
		}
	})

	// A config file naming the wrong server must not silently degrade to
	// localhost: the command would "succeed" against the wrong instance.
	t.Run("broken config file errors", func(t *testing.T) {
		for name, contents := range map[string]string{
			"malformed":   "server = \n",
			"unknown key": "srv = \"https://preview.example.com\"\n",
			"no scheme":   "server = \"preview.example.com\"\n",
		} {
			t.Run(name, func(t *testing.T) {
				dir := isolateConfig(t)
				writeClientConfig(t, dir, contents)
				if got, err := runLeafErr(t); err == nil {
					t.Errorf("resolveURL = %q, want an error", got)
				}
			})
		}
	})

	// An explicit --server is honored even when the config file is broken —
	// the flag never consults it.
	t.Run("explicit flag skips a broken config file", func(t *testing.T) {
		dir := isolateConfig(t)
		writeClientConfig(t, dir, "srv = \"https://preview.example.com\"\n")
		if got := runLeaf(t, "--server", "http://flag:5678"); got != "http://flag:5678" {
			t.Errorf("resolveURL = %q, want flag value", got)
		}
	})
}

// TestEffectiveTokenFallsBackToGH: with no $PREVIEW_TOKEN and no configured
// token, the CLI presents the GitHub CLI's stored credential — the
// zero-setup path for an SSO-gated server. Explicit sources still win.
func TestEffectiveTokenFallsBackToGH(t *testing.T) {
	// A fake gh on PATH that prints a token.
	bin := t.TempDir()
	fake := filepath.Join(bin, "gh")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho gho_from_gh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("PREVIEW_CONFIG_DIR", t.TempDir()) // no config file, no token
	t.Setenv("PREVIEW_TOKEN", "")

	tok, err := effectiveToken()
	if err != nil || tok != "gho_from_gh" {
		t.Fatalf("token = %q, %v; want the gh fallback", tok, err)
	}

	// $PREVIEW_TOKEN beats gh.
	t.Setenv("PREVIEW_TOKEN", "ghp_explicit")
	if tok, _ := effectiveToken(); tok != "ghp_explicit" {
		t.Fatalf("token = %q, want $PREVIEW_TOKEN to win", tok)
	}
}

// TestEffectiveTokenNoGH: a machine without gh (or signed out) sends no
// token, exactly as before the fallback existed.
func TestEffectiveTokenNoGH(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no gh anywhere
	t.Setenv("PREVIEW_CONFIG_DIR", t.TempDir())
	t.Setenv("PREVIEW_TOKEN", "")
	if tok, err := effectiveToken(); err != nil || tok != "" {
		t.Fatalf("token = %q, %v; want empty", tok, err)
	}
}

// TestMatchRepo covers cwd→repo inference: local-path sources, remote-URL
// identity across ssh/https/.git spellings, and the directory-name fallback
// (~/code/onyx → repo "onyx"), in that priority order.
func TestMatchRepo(t *testing.T) {
	repos := []client.Repo{
		{Name: "onyx", Source: "https://github.com/onyx-dot-app/onyx"},
		{Name: "local", Source: "/srv/checkouts/local"},
	}
	cases := []struct {
		name, top, origin, want string
		ok                      bool
	}{
		{"path match", "/srv/checkouts/local", "", "local", true},
		{"ssh origin vs https source", "/home/me/code/work", "git@github.com:onyx-dot-app/onyx", "onyx", true},
		{"ssh with .git", "/x", "git@github.com:Onyx-Dot-App/onyx.git", "onyx", true},
		{"ssh:// scheme", "/x", "ssh://git@github.com/onyx-dot-app/onyx.git", "onyx", true},
		{"https with .git", "/x", "https://github.com/onyx-dot-app/onyx.git", "onyx", true},
		{"dirname fallback", "/home/me/code/onyx", "git@github.com:me/my-fork", "onyx", true},
		{"no match", "/home/me/code/other", "git@github.com:me/other", "", false},
		{"different repo same host", "/x", "https://github.com/onyx-dot-app/else", "", false},
	}
	for _, tc := range cases {
		got, ok := matchRepo(repos, tc.top, tc.origin)
		if got != tc.want || ok != tc.ok {
			t.Errorf("%s: matchRepo = %q,%v; want %q,%v", tc.name, got, ok, tc.want, tc.ok)
		}
	}
	// A local-path origin string must never collide with a URL identity.
	if id := normalizeGitURL("/srv/checkouts/local"); id != "" {
		t.Errorf("local path normalized to %q, want empty", id)
	}
}
