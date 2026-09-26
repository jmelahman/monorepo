// Package config resolves where the application keeps its on-disk state and
// its settings (auth secret, LLM backend), from config.toml files and the
// environment.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// appName is the directory name used under the XDG data home. Keep it in
// sync with the binary name in .goreleaser.yaml and pyproject.toml.
const appName = "agilecbt"

// LLM backends for the in-app curator.
const (
	// LLMOpenAI is any OpenAI-compatible chat completions API: Ollama's /v1,
	// OpenRouter, llama.cpp, vLLM, OpenAI itself.
	LLMOpenAI = "openai"
	LLMNone   = "none"
)

// Defaults for the OpenAI-compatible backend: a local Ollama.
const (
	DefaultBaseURL         = "http://localhost:11434/v1"
	DefaultModel           = "qwen3.8:27b"
	DefaultReasoningEffort = "none"
)

// Config holds the resolved runtime configuration.
type Config struct {
	DataDir string
	// Secret protects the API when non-empty ($APP_SECRET).
	Secret string
	// LLM selects the curator backend ($APP_LLM, default openai).
	LLM string
	// BaseURL is the OpenAI-compatible API root, ending before
	// /chat/completions ($APP_LLM_BASE_URL).
	BaseURL string
	// APIKey is sent as a bearer token when set ($APP_LLM_API_KEY).
	APIKey string
	// Model is the model id at BaseURL ($APP_MODEL).
	Model string
	// ReasoningEffort is sent as reasoning_effort
	// ($APP_LLM_REASONING_EFFORT); "none" turns thinking off, "" omits it.
	ReasoningEffort string
	// CrisisResources replaces the built-in crisis lines the coach shares
	// ($APP_CRISIS_RESOURCES); empty keeps the default.
	CrisisResources string
	// Files lists the config.toml files that were read, lowest precedence
	// first.
	Files []string
}

// fileConfig is one config.toml. Every key is optional and mirrors an
// environment variable, which wins when set.
type fileConfig struct {
	LLM             *string `toml:"llm"`
	BaseURL         *string `toml:"base_url"`
	APIKey          *string `toml:"api_key"`
	Model           *string `toml:"model"`
	ReasoningEffort *string `toml:"reasoning_effort"`
	Secret          *string `toml:"secret"`
	// CrisisResources is free text, usually a multi-line ''' string.
	CrisisResources *string `toml:"crisis_resources"`
	// DataDir is relative to the file's directory unless absolute.
	DataDir *string `toml:"data_dir"`
}

// Files returns where config.toml is looked for, lowest precedence first:
// the user's ($XDG_CONFIG_HOME/agilecbt, default ~/.config/agilecbt), then
// the working directory's .config/agilecbt.
func Files() []string {
	var out []string
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, ".config")
		}
	}
	if dir != "" {
		out = append(out, filepath.Join(dir, appName, "config.toml"))
	}
	return append(out, filepath.Join(".config", appName, "config.toml"))
}

// fromFiles merges the config files that exist into c. data_dir is returned
// separately since it's resolved in Load.
func fromFiles(c *Config, paths []string) (dataDir string, err error) {
	seen := map[string]bool{}
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return "", err
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true
		var f fileConfig
		md, err := toml.DecodeFile(abs, &f)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("%s: %w", abs, err)
		}
		if extra := md.Undecoded(); len(extra) > 0 {
			return "", fmt.Errorf("%s: unknown keys %v (want llm, base_url, api_key, model, reasoning_effort, secret, crisis_resources, data_dir)", abs, extra)
		}
		for dst, src := range map[*string]*string{
			&c.LLM: f.LLM, &c.BaseURL: f.BaseURL, &c.APIKey: f.APIKey, &c.Model: f.Model,
			&c.ReasoningEffort: f.ReasoningEffort, &c.Secret: f.Secret, &c.CrisisResources: f.CrisisResources,
		} {
			if src != nil {
				*dst = *src
			}
		}
		if f.DataDir != nil {
			dataDir = *f.DataDir
			if dataDir != "" && !filepath.IsAbs(dataDir) {
				dataDir = filepath.Join(filepath.Dir(abs), dataDir)
			}
		}
		c.Files = append(c.Files, abs)
	}
	return dataDir, nil
}

// resolve reads the settings from config files and the environment. An
// environment variable that is set, even to "", overrides the files.
func resolve(files []string) (Config, string, error) {
	// Set before the files so an explicit "" can still clear it.
	c := Config{ReasoningEffort: DefaultReasoningEffort}
	dataDir, err := fromFiles(&c, files)
	if err != nil {
		return Config{}, "", err
	}
	for dst, key := range map[*string]string{
		&c.Secret: "APP_SECRET", &c.LLM: "APP_LLM", &c.Model: "APP_MODEL",
		&c.BaseURL: "APP_LLM_BASE_URL", &c.APIKey: "APP_LLM_API_KEY",
		&c.ReasoningEffort: "APP_LLM_REASONING_EFFORT",
		&c.CrisisResources: "APP_CRISIS_RESOURCES",
	} {
		if v, ok := os.LookupEnv(key); ok {
			*dst = v
		}
	}
	if c.LLM == "" {
		c.LLM = LLMOpenAI
	}
	switch c.LLM {
	case LLMOpenAI, LLMNone:
	case "ollama", "claude-code", "anthropic":
		return Config{}, "", fmt.Errorf("llm %q was removed: use llm = %q with base_url (Ollama is %s), or %q", c.LLM, LLMOpenAI, DefaultBaseURL, LLMNone)
	default:
		return Config{}, "", fmt.Errorf("llm %q: want openai or none", c.LLM)
	}
	c.CrisisResources = strings.TrimSpace(c.CrisisResources)
	c.BaseURL = strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if c.BaseURL == "" {
		c.BaseURL = DefaultBaseURL
	}
	if c.Model == "" {
		c.Model = DefaultModel
	}
	return c, dataDir, nil
}

// Load reads the config files and environment, resolves the data directory
// (flag override > $APP_DATA_DIR > data_dir in config.toml >
// $XDG_DATA_HOME/agilecbt > ~/.local/share/agilecbt), and ensures it exists.
func Load(dataDirOverride string) (Config, error) {
	cfg, fileDataDir, err := resolve(Files())
	if err != nil {
		return Config{}, err
	}
	dir := dataDirOverride
	if dir == "" {
		dir = os.Getenv("APP_DATA_DIR")
	}
	if dir == "" {
		dir = fileDataDir
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
