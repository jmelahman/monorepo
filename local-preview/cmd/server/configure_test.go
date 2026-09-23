package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runConfigureCmd executes `preview configure` with args and the given stdin,
// returning its combined output.
func runConfigureCmd(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	cmd := configureCmd()
	var out bytes.Buffer
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	err := cmd.Execute()
	return out.String(), err
}

// readConfig returns the raw config.toml written into the isolated dir.
func readConfig(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestConfigureSetsServer(t *testing.T) {
	dir := isolateConfig(t)
	// A health-serving stub stands in for a real instance, so the verify
	// step exercises its success path without touching the network.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/health" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","version":"v1.2.3","preview_domain":"preview.example.com"}`))
	}))
	defer srv.Close()

	out, err := runConfigureCmd(t, "", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "server set to "+srv.URL) {
		t.Errorf("output = %q, want the saved server", out)
	}
	if !strings.Contains(out, "version v1.2.3") || !strings.Contains(out, "*.preview.example.com") {
		t.Errorf("output = %q, want the verified server's health", out)
	}
	if got := readConfig(t, dir); !strings.Contains(got, srv.URL) {
		t.Errorf("config.toml = %q, want the server URL", got)
	}

	// The stored value is what subsequent commands resolve to.
	if got := runLeaf(t); got != srv.URL {
		t.Errorf("resolveURL = %q, want %q", got, srv.URL)
	}
}

// An unreachable server is a warning: configuring before starting the
// server is a legitimate order to do things in.
func TestConfigureUnreachableServerStillSaves(t *testing.T) {
	dir := isolateConfig(t)
	out, err := runConfigureCmd(t, "", "http://127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "warning: could not reach") {
		t.Errorf("output = %q, want a reachability warning", out)
	}
	if got := readConfig(t, dir); !strings.Contains(got, "http://127.0.0.1:1") {
		t.Errorf("config.toml = %q, want the server saved anyway", got)
	}
}

func TestConfigurePrompts(t *testing.T) {
	dir := isolateConfig(t)
	out, err := runConfigureCmd(t, "http://127.0.0.1:1\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Server URL [") {
		t.Errorf("output = %q, want a prompt", out)
	}
	if got := readConfig(t, dir); !strings.Contains(got, "http://127.0.0.1:1") {
		t.Errorf("config.toml = %q, want the prompted server", got)
	}
}

// An empty answer keeps the current setting rather than clearing it.
func TestConfigurePromptEmptyKeepsCurrent(t *testing.T) {
	dir := isolateConfig(t)
	writeClientConfig(t, dir, "server = \"http://127.0.0.1:1\"\n")
	if _, err := runConfigureCmd(t, "\n"); err != nil {
		t.Fatal(err)
	}
	if got := readConfig(t, dir); !strings.Contains(got, "http://127.0.0.1:1") {
		t.Errorf("config.toml = %q, want the previous server kept", got)
	}
}

// Non-interactive with nothing on stdin: fail with an actionable message
// rather than saving an empty server.
func TestConfigureNoInputErrors(t *testing.T) {
	isolateConfig(t)
	_, err := runConfigureCmd(t, "")
	if err == nil {
		t.Fatal("configure succeeded, want an error")
	}
	if !strings.Contains(err.Error(), "preview configure <url>") {
		t.Errorf("error = %v, want the usage hint", err)
	}
}

func TestConfigureRejectsInvalidURL(t *testing.T) {
	dir := isolateConfig(t)
	if _, err := runConfigureCmd(t, "", "preview.example.com"); err == nil {
		t.Fatal("configure succeeded, want an error")
	}
	if _, err := os.Stat(filepath.Join(dir, "config.toml")); err == nil {
		t.Error("config.toml written despite an invalid URL")
	}
}

func TestConfigureShow(t *testing.T) {
	dir := isolateConfig(t)

	out, err := runConfigureCmd(t, "", "--show")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "not created yet") || !strings.Contains(out, "(unset)") {
		t.Errorf("output = %q, want the unconfigured state", out)
	}
	if !strings.Contains(out, defaultServerURL+" (from default)") {
		t.Errorf("output = %q, want the default as effective", out)
	}

	writeClientConfig(t, dir, "server = \"https://preview.example.com\"\n")
	if out, err = runConfigureCmd(t, "", "--show"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "https://preview.example.com (from config file)") {
		t.Errorf("output = %q, want the configured server as effective", out)
	}

	// $PREVIEW_URL wins, and --show says so rather than reporting the file.
	t.Setenv("PREVIEW_URL", "http://env:1234")
	if out, err = runConfigureCmd(t, "", "--show"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "configured server: https://preview.example.com") {
		t.Errorf("output = %q, want the stored server reported", out)
	}
	if !strings.Contains(out, "http://env:1234 (from $PREVIEW_URL)") {
		t.Errorf("output = %q, want the env var as effective", out)
	}
}

func TestConfigureUnset(t *testing.T) {
	dir := isolateConfig(t)
	writeClientConfig(t, dir, "server = \"https://preview.example.com\"\n")

	out, err := runConfigureCmd(t, "", "--unset")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, defaultServerURL) {
		t.Errorf("output = %q, want the fallback named", out)
	}
	if got := runLeaf(t); got != defaultServerURL {
		t.Errorf("resolveURL = %q, want the default after unset", got)
	}

	// Unsetting twice is a no-op, not an error.
	if out, err = runConfigureCmd(t, "", "--unset"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "no server configured") {
		t.Errorf("output = %q, want the already-unset message", out)
	}
}

func TestConfigureFlagConflicts(t *testing.T) {
	isolateConfig(t)
	for _, args := range [][]string{
		{"--show", "--unset"},
		{"--show", "https://preview.example.com"},
		{"--unset", "https://preview.example.com"},
	} {
		if _, err := runConfigureCmd(t, "", args...); err == nil {
			t.Errorf("configure %v succeeded, want an error", args)
		}
	}
}

// A shell with $PREVIEW_URL set would otherwise silently ignore what was
// just configured.
func TestConfigureWarnsAboutEnvOverride(t *testing.T) {
	isolateConfig(t)
	t.Setenv("PREVIEW_URL", "http://env:1234")
	out, err := runConfigureCmd(t, "", "http://127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "$PREVIEW_URL is set to http://env:1234") {
		t.Errorf("output = %q, want the override warning", out)
	}
}
