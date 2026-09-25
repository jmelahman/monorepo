// Package config resolves where the application keeps its on-disk state and
// reads the environment-driven settings (auth secret, LLM backend).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// appName is the directory name used under the XDG data home. Keep it in
// sync with the binary name in .goreleaser.yaml and pyproject.toml.
const appName = "agilecbt"

// LLM backends for the in-app curator.
const (
	LLMClaudeCode = "claude-code"
	LLMOllama     = "ollama"
	LLMAnthropic  = "anthropic"
	LLMNone       = "none"
)

// Config holds the resolved runtime configuration.
type Config struct {
	DataDir string
	// Secret protects the API when non-empty ($APP_SECRET).
	Secret string
	// LLM selects the curator backend ($APP_LLM, default claude-code).
	LLM string
	// Model overrides the backend's default model ($APP_MODEL).
	Model string
	// OllamaHost is the Ollama base URL ($OLLAMA_HOST).
	OllamaHost string
	// SelfURL is how a local subprocess (claude -p) reaches this server's
	// /mcp endpoint ($APP_SELF_URL); derived from --addr when empty.
	SelfURL string
}

// FromEnv reads the environment-driven settings. It does not touch disk.
func FromEnv() (Config, error) {
	c := Config{
		Secret:     os.Getenv("APP_SECRET"),
		LLM:        os.Getenv("APP_LLM"),
		Model:      os.Getenv("APP_MODEL"),
		OllamaHost: os.Getenv("OLLAMA_HOST"),
		SelfURL:    os.Getenv("APP_SELF_URL"),
	}
	if c.LLM == "" {
		c.LLM = LLMClaudeCode
	}
	switch c.LLM {
	case LLMClaudeCode, LLMOllama, LLMAnthropic, LLMNone:
	default:
		return Config{}, fmt.Errorf("APP_LLM=%q: want claude-code, ollama, anthropic, or none", c.LLM)
	}
	if c.OllamaHost == "" {
		c.OllamaHost = "http://localhost:11434"
	} else if !strings.Contains(c.OllamaHost, "://") {
		// Ollama itself accepts a bare host:port here.
		c.OllamaHost = "http://" + c.OllamaHost
	}
	return c, nil
}

// Load reads the environment, resolves the data directory (flag override >
// $APP_DATA_DIR > $XDG_DATA_HOME/agilecbt > ~/.local/share/agilecbt), and
// ensures it exists.
func Load(dataDirOverride string) (Config, error) {
	cfg, err := FromEnv()
	if err != nil {
		return Config{}, err
	}
	dir := dataDirOverride
	if dir == "" {
		dir = os.Getenv("APP_DATA_DIR")
	}
	if dir == "" {
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			dir = filepath.Join(xdg, appName)
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return Config{}, fmt.Errorf("resolve home dir: %w", err)
			}
			dir = filepath.Join(home, ".local", "share", appName)
		}
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Config{}, fmt.Errorf("resolve data dir: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return Config{}, fmt.Errorf("create data dir: %w", err)
	}
	cfg.DataDir = abs
	return cfg, nil
}

// DBPath returns the SQLite database path inside the data directory.
func (c Config) DBPath() string {
	return filepath.Join(c.DataDir, appName+".db")
}
