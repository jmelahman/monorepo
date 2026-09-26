package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jmelahman/agilecbt/internal/config"
)

func TestEvalModelsMatrix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.toml")
	err := os.WriteFile(path, []byte(`
[[models]]
name = "local"

[[models]]
name = "hosted"
base_url = "https://example.test/v1"
api_key_env = "TEST_EVAL_KEY"
reasoning_effort = "off"
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{BaseURL: "http://ollama/v1", APIKey: "base", ReasoningEffort: "none"}
	f := evalFlags{matrix: true, matrixFile: path}

	t.Setenv("TEST_EVAL_KEY", "")
	if _, err := evalModels(f, cfg); err == nil {
		t.Fatal("want an error when api_key_env is unset")
	}

	t.Setenv("TEST_EVAL_KEY", "secret")
	models, err := evalModels(f, cfg)
	if err != nil {
		t.Fatal(err)
	}
	local, hosted := models[0], models[1]
	if local.BaseURL != cfg.BaseURL || local.APIKey != "base" || local.ReasoningEffort != "none" {
		t.Errorf("local = %+v, want the coach's endpoint, key and effort", local)
	}
	if hosted.APIKey != "secret" || hosted.ReasoningEffort != "" {
		t.Errorf("hosted = %+v, want the env key and no effort", hosted)
	}
}
