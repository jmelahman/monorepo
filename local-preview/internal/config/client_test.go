package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("PREVIEW_CONFIG_DIR", dir)
	if err := os.WriteFile(filepath.Join(dir, clientConfigName), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadClientConfig(t *testing.T) {
	writeConfig(t, "server = \"https://preview.example.com\"\n")
	cfg, err := LoadClientConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server != "https://preview.example.com" {
		t.Errorf("Server = %q", cfg.Server)
	}
}

// No file is the normal state for a fresh install, not an error.
func TestLoadClientConfigMissing(t *testing.T) {
	t.Setenv("PREVIEW_CONFIG_DIR", t.TempDir())
	cfg, err := LoadClientConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server != "" {
		t.Errorf("Server = %q, want empty", cfg.Server)
	}
}

func TestLoadClientConfigErrors(t *testing.T) {
	for name, contents := range map[string]string{
		"malformed toml": "server = \n",
		"unknown key":    "srv = \"https://preview.example.com\"\n",
		"missing scheme": "server = \"preview.example.com\"\n",
		"missing host":   "server = \"https://\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			writeConfig(t, contents)
			if _, err := LoadClientConfig(); err == nil {
				t.Error("LoadClientConfig succeeded, want an error")
			}
		})
	}
}

func TestSaveClientConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PREVIEW_CONFIG_DIR", filepath.Join(dir, "nested"))
	path, err := SaveClientConfig(ClientConfig{Server: "https://preview.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config not written: %v", err)
	}
	cfg, err := LoadClientConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server != "https://preview.example.com" {
		t.Errorf("Server = %q after round trip", cfg.Server)
	}
}

// Unsetting writes an empty server rather than leaving the old one behind.
func TestSaveClientConfigUnset(t *testing.T) {
	writeConfig(t, "server = \"https://preview.example.com\"\n")
	if _, err := SaveClientConfig(ClientConfig{}); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadClientConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server != "" {
		t.Errorf("Server = %q, want empty", cfg.Server)
	}
}

// The config file lives beside the manifests directory, both under the same
// config root.
func TestDirLayout(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PREVIEW_CONFIG_DIR", dir)
	if got, want := ClientConfigPath(), filepath.Join(dir, "config.toml"); got != want {
		t.Errorf("ClientConfigPath = %q, want %q", got, want)
	}
	if got, want := ManifestsDir(), filepath.Join(dir, "manifests"); got != want {
		t.Errorf("ManifestsDir = %q, want %q", got, want)
	}
}

func TestValidateServerURL(t *testing.T) {
	for _, ok := range []string{"http://localhost:8080", "https://preview.example.com"} {
		if err := ValidateServerURL(ok); err != nil {
			t.Errorf("ValidateServerURL(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"preview.example.com", "ftp://preview.example.com", "https://", ""} {
		if err := ValidateServerURL(bad); err == nil {
			t.Errorf("ValidateServerURL(%q) = nil, want an error", bad)
		} else if !strings.Contains(err.Error(), "invalid server URL") {
			t.Errorf("ValidateServerURL(%q) error = %v", bad, err)
		}
	}
}
