package curator

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/jmelahman/agilecbt/internal/api"
	"github.com/jmelahman/agilecbt/internal/db"
	"github.com/jmelahman/agilecbt/internal/tools"
)

// Config is the coach's model configuration.
type Config struct {
	// LLM is "openai" or "none".
	LLM             string
	BaseURL         string
	APIKey          string
	Model           string
	ReasoningEffort string
}

const (
	llmOpenAI = "openai"
	llmNone   = "none"
)

// overrideKeys maps LLMSettings.Overrides / PATCH fields to settings rows.
var overrideKeys = map[string]string{
	"llm":              db.SettingLLM,
	"base_url":         db.SettingLLMBaseURL,
	"model":            db.SettingLLMModel,
	"reasoning_effort": db.SettingLLMReasoningEffort,
}

// NewConfigured returns a curator whose model settings start from defaults
// (config.toml and the environment) and can be overridden in the app.
func NewConfigured(reg *tools.Registry, defaults Config) (*Curator, error) {
	c := New(reg, nil)
	c.reg = reg
	c.defaults = &defaults
	if err := c.reload(); err != nil {
		return nil, err
	}
	return c, nil
}

// Effective returns the configuration in use.
func (c *Curator) Effective() Config {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cfg
}

// resolve layers the saved overrides over the defaults. A saved API key only
// applies to the base URL it was saved for, and the configured one only to
// the configured base URL, so changing the URL never sends a key elsewhere.
func (c *Curator) resolve() (cfg Config, overrides map[string]string, keySaved bool, err error) {
	stored, err := c.app.Store.GetSettings()
	if err != nil {
		return Config{}, nil, false, err
	}
	cfg, overrides, keySaved = c.layer(stored)
	return cfg, overrides, keySaved, nil
}

// layer applies the stored settings rows over the defaults.
func (c *Curator) layer(stored map[string]string) (cfg Config, overrides map[string]string, keySaved bool) {
	cfg = *c.defaults
	overrides = map[string]string{}
	for field, key := range overrideKeys {
		v, ok := stored[key]
		if !ok {
			continue
		}
		overrides[field] = v
		switch field {
		case "llm":
			cfg.LLM = v
		case "base_url":
			cfg.BaseURL = v
		case "model":
			cfg.Model = v
		case "reasoning_effort":
			cfg.ReasoningEffort = v
		}
	}
	cfg.APIKey = ""
	if k := stored[db.SettingLLMAPIKey]; k != "" && stored[db.SettingLLMAPIKeyURL] == cfg.BaseURL {
		cfg.APIKey, keySaved = k, true
	} else if cfg.BaseURL == c.defaults.BaseURL {
		cfg.APIKey = c.defaults.APIKey
	}
	return cfg, overrides, keySaved
}

// reload rebuilds the backend from the current settings.
func (c *Curator) reload() error {
	cfg, _, _, err := c.resolve()
	if err != nil {
		return err
	}
	var backend Backend
	if cfg.LLM != llmNone {
		backend = &OpenAI{
			BaseURL:         cfg.BaseURL,
			APIKey:          cfg.APIKey,
			Model:           cfg.Model,
			ReasoningEffort: cfg.ReasoningEffort,
			Registry:        c.reg,
		}
	}
	c.mu.Lock()
	c.cfg, c.backend = cfg, backend
	c.mu.Unlock()
	c.cache.reset()
	return nil
}

func fields(cfg Config) api.LLMFields {
	return api.LLMFields{
		LLM: cfg.LLM, BaseURL: cfg.BaseURL, Model: cfg.Model,
		ReasoningEffort: cfg.ReasoningEffort, APIKeySet: cfg.APIKey != "",
	}
}

// ErrNotConfigurable means the curator was built with a fixed backend.
var ErrNotConfigurable = errors.New("the coach's model can't be changed here")

// LLMSettings implements api.Curator.
func (c *Curator) LLMSettings() (api.LLMSettings, error) {
	if c.defaults == nil {
		return api.LLMSettings{}, ErrNotConfigurable
	}
	cfg, overrides, keySaved, err := c.resolve()
	if err != nil {
		return api.LLMSettings{}, err
	}
	return api.LLMSettings{
		Effective:   fields(cfg),
		Defaults:    fields(*c.defaults),
		Overrides:   overrides,
		APIKeySaved: keySaved,
	}, nil
}

// UpdateLLM implements api.Curator.
func (c *Curator) UpdateLLM(patch map[string]*string) (api.LLMSettings, error) {
	if c.defaults == nil {
		return api.LLMSettings{}, ErrNotConfigurable
	}
	for field, v := range patch {
		if _, ok := overrideKeys[field]; !ok && field != "api_key" {
			return api.LLMSettings{}, fmt.Errorf("%w: unknown field %q", db.ErrInvalid, field)
		}
		if v == nil {
			continue
		}
		s := strings.TrimSpace(*v)
		switch field {
		case "llm":
			if s != llmOpenAI && s != llmNone {
				return api.LLMSettings{}, fmt.Errorf("%w: llm must be openai or none", db.ErrInvalid)
			}
		case "base_url":
			s = strings.TrimRight(s, "/")
			if s != "" {
				u, err := url.Parse(s)
				if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
					return api.LLMSettings{}, fmt.Errorf("%w: base_url must be an http(s) URL", db.ErrInvalid)
				}
			}
		case "reasoning_effort":
			switch s {
			case "", "none", "minimal", "low", "medium", "high":
			default:
				return api.LLMSettings{}, fmt.Errorf("%w: reasoning_effort must be none, minimal, low, medium, high, or empty", db.ErrInvalid)
			}
		}
		*v = s
	}

	store := c.app.Store
	err := store.Tx(func(s *db.Store) error {
		for field, key := range overrideKeys {
			v, ok := patch[field]
			if !ok {
				continue
			}
			// An empty base URL or model means "use the default"; an empty
			// reasoning effort means "don't send it".
			if v == nil || (*v == "" && field != "reasoning_effort") {
				if err := s.DeleteSetting(key); err != nil {
					return err
				}
				continue
			}
			if err := s.SetSetting(key, *v); err != nil {
				return err
			}
		}
		v, ok := patch["api_key"]
		switch {
		case !ok:
		case v == nil || *v == "":
			for _, k := range []string{db.SettingLLMAPIKey, db.SettingLLMAPIKeyURL} {
				if err := s.DeleteSetting(k); err != nil {
					return err
				}
			}
		default:
			// Bind the key to the base URL now in effect, including a
			// base_url from this same patch.
			stored, err := s.GetSettings()
			if err != nil {
				return err
			}
			cfg, _, _ := c.layer(stored)
			if err := s.SetSetting(db.SettingLLMAPIKey, *v); err != nil {
				return err
			}
			return s.SetSetting(db.SettingLLMAPIKeyURL, cfg.BaseURL)
		}
		return nil
	})
	if err != nil {
		return api.LLMSettings{}, err
	}
	if err := c.reload(); err != nil {
		return api.LLMSettings{}, err
	}
	return c.LLMSettings()
}

// Models implements api.Curator.
func (c *Curator) Models(ctx context.Context) ([]string, error) {
	o, ok := c.current().(*OpenAI)
	if !ok {
		return []string{}, nil
	}
	return o.Models(ctx)
}
