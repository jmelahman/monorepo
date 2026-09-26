package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolvePrecedence(t *testing.T) {
	for _, k := range []string{"APP_SECRET", "APP_LLM", "APP_MODEL", "APP_LLM_BASE_URL", "APP_LLM_API_KEY", "APP_LLM_REASONING_EFFORT", "APP_CRISIS_RESOURCES"} {
		// t.Setenv then Unsetenv restores the original value after the test.
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	dir := t.TempDir()
	user := filepath.Join(dir, "user", "config.toml")
	project := filepath.Join(dir, "project", "config.toml")
	writeFile(t, user, `llm = "none"
model = "user-model"
reasoning_effort = ""
secret = "s3cret"
data_dir = "data"
`)
	writeFile(t, project, `llm = "openai"
base_url = "http://ollama:11434/v1/"
crisis_resources = '''
Call my sister: 555-0100.
'''
`)
	t.Setenv("APP_MODEL", "env-model")

	c, dataDir, err := resolve([]string{user, project, filepath.Join(dir, "missing.toml")})
	if err != nil {
		t.Fatal(err)
	}
	if c.LLM != "openai" || c.Model != "env-model" || c.Secret != "s3cret" || c.BaseURL != "http://ollama:11434/v1" || c.ReasoningEffort != "" {
		t.Fatalf("resolved %+v", c)
	}
	if want := filepath.Join(dir, "user", "data"); dataDir != want {
		t.Fatalf("data dir = %q, want %q", dataDir, want)
	}
	if c.CrisisResources != "Call my sister: 555-0100." {
		t.Fatalf("crisis resources = %q", c.CrisisResources)
	}
	if len(c.Files) != 2 {
		t.Fatalf("files = %v", c.Files)
	}

	// A set-but-empty variable still wins, e.g. to turn auth off.
	t.Setenv("APP_SECRET", "")
	if c, _, _ = resolve([]string{user}); c.Secret != "" {
		t.Fatalf("secret = %q, want empty", c.Secret)
	}
}

func TestResolveRejectsUnknownKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, path, `LLM_MODEL = "qwen"`)
	if _, _, err := resolve([]string{path}); err == nil || !strings.Contains(err.Error(), "LLM_MODEL") {
		t.Fatalf("err = %v, want unknown key LLM_MODEL", err)
	}
}

func TestResolveDefaults(t *testing.T) {
	for _, k := range []string{"APP_LLM", "APP_MODEL", "APP_LLM_BASE_URL", "APP_LLM_REASONING_EFFORT"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	c, _, err := resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.LLM != LLMOpenAI || c.BaseURL != DefaultBaseURL || c.Model != DefaultModel || c.ReasoningEffort != "none" {
		t.Fatalf("defaults %+v", c)
	}
}

func TestResolveRejectsRemovedBackends(t *testing.T) {
	t.Setenv("APP_LLM", "claude-code")
	if _, _, err := resolve(nil); err == nil || !strings.Contains(err.Error(), "removed") {
		t.Fatalf("err = %v, want removed backend", err)
	}
}
