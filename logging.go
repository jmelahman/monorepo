package typesafe

import (
	"log/slog"
	"net/http"
	"strings"
)

// LogLevel is the verbosity of the SDK's logging.
//
// LogInfo logs one line per attempt; LogDebug adds request and response headers
// and bodies. Known credential headers are redacted, but bodies are not.
type LogLevel int

const (
	LogDebug LogLevel = iota
	LogInfo
	LogWarn
	LogError
	// LogOff disables SDK logging.
	LogOff
)

func (l LogLevel) String() string {
	switch l {
	case LogDebug:
		return "debug"
	case LogInfo:
		return "info"
	case LogWarn:
		return "warn"
	case LogError:
		return "error"
	case LogOff:
		return "off"
	}
	return "unknown"
}

// slogLevel maps a LogLevel onto the matching [slog.Level].
func (l LogLevel) slogLevel() slog.Level {
	switch l {
	case LogDebug:
		return slog.LevelDebug
	case LogInfo:
		return slog.LevelInfo
	case LogWarn:
		return slog.LevelWarn
	default:
		return slog.LevelError
	}
}

// ParseLogLevel converts a level name to a [LogLevel]. It accepts "debug",
// "info", "warn", "warning", "error", and "off", ignoring case and surrounding
// space.
func ParseLogLevel(s string) (LogLevel, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return LogDebug, nil
	case "info":
		return LogInfo, nil
	case "warn", "warning":
		return LogWarn, nil
	case "error":
		return LogError, nil
	case "off":
		return LogOff, nil
	}
	return 0, errf("invalid log level %q; want debug, info, warn, error, or off", s)
}

// secretHeaders are logged as a masked credential rather than verbatim.
var secretHeaders = map[string]bool{
	"Authorization":       true,
	"Proxy-Authorization": true,
	"X-Api-Key":           true,
	"Api-Key":             true,
}

// hiddenHeaders are replaced entirely when logged.
var hiddenHeaders = map[string]bool{
	"Cookie":     true,
	"Set-Cookie": true,
}

// redactHeaders renders headers for logging with credentials removed. Every log
// site routes through it, so no call site can leak a key.
func redactHeaders(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for name, values := range h {
		out[name] = redactHeaderValue(name, strings.Join(values, ", "))
	}
	return out
}

func redactHeaderValue(name, value string) string {
	canonical := http.CanonicalHeaderKey(name)
	lower := strings.ToLower(canonical)
	switch {
	case secretHeaders[canonical]:
		return maskCredential(value)
	case hiddenHeaders[canonical],
		strings.Contains(lower, "token"),
		strings.Contains(lower, "secret"):
		return "***"
	}
	return value
}

// maskCredential keeps an authentication scheme and the last four characters of
// a long secret, which is enough to tell two keys apart without revealing either.
func maskCredential(value string) string {
	scheme, secret, hasScheme := strings.Cut(value, " ")
	if !hasScheme {
		secret, scheme = value, ""
	}
	masked := "***"
	if len(secret) > 8 {
		masked += secret[len(secret)-4:]
	}
	if scheme != "" {
		return scheme + " " + masked
	}
	return masked
}
