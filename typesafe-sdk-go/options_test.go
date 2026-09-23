package typesafe

import (
	"strings"
	"testing"
	"time"
)

// These tests set environment variables, so they must not run in parallel.

func TestNewReadsEnvironment(t *testing.T) {
	t.Setenv(EnvAPIKey, "sk-env")
	t.Setenv(EnvBaseURL, "https://example.test/api/")
	t.Setenv(EnvDefaultModel, "jev-2")
	t.Setenv(EnvLogLevel, "debug")

	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.apiKey != "sk-env" {
		t.Errorf("apiKey = %q, want sk-env", c.apiKey)
	}
	if got, want := c.BaseURL(), "https://example.test/api"; got != want {
		t.Errorf("BaseURL() = %q, want %q (trailing slash trimmed)", got, want)
	}
	if got := c.DefaultModel(); got != "jev-2" {
		t.Errorf("DefaultModel() = %q, want jev-2", got)
	}
	if c.logLevel != LogDebug {
		t.Errorf("logLevel = %v, want debug", c.logLevel)
	}
}

func TestOptionsBeatEnvironment(t *testing.T) {
	t.Setenv(EnvAPIKey, "sk-env")
	t.Setenv(EnvDefaultModel, "jev-env")

	c, err := New(WithAPIKey("sk-opt"), WithDefaultModel("jev-opt"), WithLogLevel(LogOff))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.apiKey != "sk-opt" || c.DefaultModel() != "jev-opt" || c.logLevel != LogOff {
		t.Errorf("options did not take precedence: %q %q %v", c.apiKey, c.DefaultModel(), c.logLevel)
	}
}

func TestBlankEnvironmentIsIgnored(t *testing.T) {
	t.Setenv(EnvAPIKey, "sk-env")
	t.Setenv(EnvBaseURL, "   ")
	t.Setenv(EnvDefaultModel, "")

	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.BaseURL() != DefaultBaseURL {
		t.Errorf("BaseURL() = %q, want the default", c.BaseURL())
	}
	if c.DefaultModel() != DefaultModel {
		t.Errorf("DefaultModel() = %q, want the default", c.DefaultModel())
	}
}

func TestNewRequiresAPIKey(t *testing.T) {
	t.Setenv(EnvAPIKey, "")

	_, err := New()
	if err == nil {
		t.Fatal("New() = nil error, want a missing-key error")
	}
	if !strings.Contains(err.Error(), EnvAPIKey) {
		t.Errorf("New() = %q, want it to name %s", err, EnvAPIKey)
	}
}

func TestNewRejectsInvalidValues(t *testing.T) {
	t.Setenv(EnvAPIKey, "sk-env")

	tests := []struct {
		name string
		opt  Option
	}{
		{"zero timeout", WithTimeout(0)},
		{"negative timeout", WithTimeout(-time.Second)},
		{"nil http client", WithHTTPClient(nil)},
		{"nil logger", WithLogger(nil)},
		{"out of range log level", WithLogLevel(LogLevel(99))},
		{"invalid retry policy", WithRetry(RetryPolicy{MaxRetries: -1})},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := New(tc.opt); err == nil {
				t.Errorf("New(%s) = nil error, want one", tc.name)
			}
		})
	}
}

func TestInvalidEnvLogLevelIsAnError(t *testing.T) {
	t.Setenv(EnvAPIKey, "sk-env")
	t.Setenv(EnvLogLevel, "loud")

	_, err := New()
	if err == nil {
		t.Fatal("New() = nil error, want an invalid log level error")
	}
	if !strings.Contains(err.Error(), EnvLogLevel) {
		t.Errorf("New() = %q, want it to name %s", err, EnvLogLevel)
	}
}

func TestWithRetryCopiesStatuses(t *testing.T) {
	t.Setenv(EnvAPIKey, "sk-env")

	p := DefaultRetryPolicy()
	c, err := New(WithRetry(p))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	p.Statuses[418] = true
	if c.retry.Statuses[418] {
		t.Error("mutating the caller's policy changed the client's")
	}
}
