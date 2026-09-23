package typesafe

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Environment variables read when the matching option is not supplied. A value
// that is empty or only whitespace is ignored.
const (
	EnvAPIKey       = "TYPESAFE_API_KEY"
	EnvBaseURL      = "TYPESAFE_BASE_URL"
	EnvDefaultModel = "TYPESAFE_DEFAULT_MODEL"
	EnvLogLevel     = "TYPESAFE_LOG_LEVEL"
)

// SDK defaults used when neither an option nor an environment variable applies.
const (
	// DefaultBaseURL is the root of the TypeSafe API.
	DefaultBaseURL = "https://api.typesafe.ai"
	// DefaultModel is the model used when a request does not name one.
	DefaultModel = "jev-latest"
	// DefaultLogLevel is the verbosity used when TYPESAFE_LOG_LEVEL is unset.
	DefaultLogLevel = LogWarn
)

// An Option configures a [Client].
type Option func(*config) error

type config struct {
	apiKey       string
	baseURL      string
	defaultModel string
	timeout      time.Duration
	retry        RetryPolicy
	headers      http.Header
	httpClient   *http.Client
	logger       *slog.Logger
	logLevel     LogLevel
	logLevelSet  bool

	// Test seams, set through export_test.go.
	rand  func() float64
	sleep func(context.Context, time.Duration) error
}

// WithAPIKey sets the API key. It falls back to TYPESAFE_API_KEY.
func WithAPIKey(key string) Option {
	return func(c *config) error {
		c.apiKey = key
		return nil
	}
}

// WithBaseURL sets the API root. It falls back to TYPESAFE_BASE_URL, then
// [DefaultBaseURL]. Trailing slashes are removed.
func WithBaseURL(raw string) Option {
	return func(c *config) error {
		c.baseURL = raw
		return nil
	}
}

// WithDefaultModel sets the model used by requests that do not name one. It
// falls back to TYPESAFE_DEFAULT_MODEL, then [DefaultModel].
func WithDefaultModel(model string) Option {
	return func(c *config) error {
		c.defaultModel = model
		return nil
	}
}

// WithTimeout sets the timeout for a single attempt. There is no total budget
// across retries by default; use a deadline on the request context for that.
// Default: [DefaultTimeout].
func WithTimeout(d time.Duration) Option {
	return func(c *config) error {
		if d <= 0 {
			return errf("timeout must be positive, got %s", d)
		}
		c.timeout = d
		return nil
	}
}

// WithRetry replaces the retry policy. Start from [DefaultRetryPolicy] and
// change the fields you need; the zero [RetryPolicy] disables retries.
func WithRetry(p RetryPolicy) Option {
	return func(c *config) error {
		if err := p.validate(); err != nil {
			return err
		}
		c.retry = p.clone()
		return nil
	}
}

// WithDefaultHeaders adds headers to every request. Per-call headers take
// precedence, and neither can override the headers the SDK sets itself.
func WithDefaultHeaders(h http.Header) Option {
	return func(c *config) error {
		c.headers = h.Clone()
		return nil
	}
}

// WithHTTPClient sets the HTTP client used for requests. This is the seam for
// proxies, custom transports, and tests. Default: a client with no timeout of
// its own, since the SDK applies a per-attempt timeout.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *config) error {
		if hc == nil {
			return errf("HTTP client must not be nil")
		}
		c.httpClient = hc
		return nil
	}
}

// WithLogger sets the logger. Default: [slog.Default].
func WithLogger(l *slog.Logger) Option {
	return func(c *config) error {
		if l == nil {
			return errf("logger must not be nil")
		}
		c.logger = l
		return nil
	}
}

// WithLogLevel sets the SDK's log verbosity. It falls back to
// TYPESAFE_LOG_LEVEL, then [DefaultLogLevel].
func WithLogLevel(l LogLevel) Option {
	return func(c *config) error {
		if l < LogDebug || l > LogOff {
			return errf("invalid log level %d", int(l))
		}
		c.logLevel = l
		c.logLevelSet = true
		return nil
	}
}

// resolve applies environment fallbacks and validates the result.
func (c *config) resolve() error {
	c.apiKey = firstNonEmpty(c.apiKey, readEnv(EnvAPIKey))
	if c.apiKey == "" {
		return errf("no API key was provided; pass WithAPIKey or set %s", EnvAPIKey)
	}

	c.baseURL = strings.TrimRight(firstNonEmpty(c.baseURL, readEnv(EnvBaseURL), DefaultBaseURL), "/")
	if _, err := url.Parse(c.baseURL); err != nil {
		return errf("invalid base URL %q: %v", c.baseURL, err)
	}

	c.defaultModel = firstNonEmpty(c.defaultModel, readEnv(EnvDefaultModel), DefaultModel)

	if !c.logLevelSet {
		if raw := readEnv(EnvLogLevel); raw != "" {
			level, err := ParseLogLevel(raw)
			if err != nil {
				return errf("%s: %v", EnvLogLevel, strings.TrimPrefix(err.Error(), "typesafe: "))
			}
			c.logLevel = level
		}
	}
	return nil
}

// readEnv reads a trimmed environment value, treating blank as unset.
func readEnv(name string) string { return strings.TrimSpace(os.Getenv(name)) }

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// A RequestOption overrides client settings for a single call.
type RequestOption func(*requestConfig) error

type requestConfig struct {
	timeout time.Duration
	retry   RetryPolicy
	headers http.Header
}

// WithRequestTimeout overrides the per-attempt timeout for one call.
func WithRequestTimeout(d time.Duration) RequestOption {
	return func(rc *requestConfig) error {
		if d <= 0 {
			return errf("timeout must be positive, got %s", d)
		}
		rc.timeout = d
		return nil
	}
}

// WithRequestRetry replaces the retry policy for one call.
func WithRequestRetry(p RetryPolicy) RequestOption {
	return func(rc *requestConfig) error {
		if err := p.validate(); err != nil {
			return err
		}
		rc.retry = p.clone()
		return nil
	}
}

// WithRequestHeaders adds headers to one call, on top of the client's default
// headers. They cannot override the headers the SDK sets itself.
func WithRequestHeaders(h http.Header) RequestOption {
	return func(rc *requestConfig) error {
		rc.headers = h.Clone()
		return nil
	}
}
