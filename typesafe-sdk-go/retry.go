package typesafe

import (
	"context"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

// Default retry and timeout settings, matching the other TypeSafe SDKs.
const (
	// DefaultTimeout is the default timeout for a single attempt.
	DefaultTimeout = 10 * time.Second

	defaultMaxRetries     = 2
	defaultBackoffInitial = 500 * time.Millisecond
	defaultBackoffMax     = 5 * time.Second
	defaultBackoffJitter  = 0.25
	defaultMaxRetryAfter  = 60 * time.Second

	// maxBackoffShift caps the exponent so the backoff cannot overflow.
	maxBackoffShift = 30
)

// RetryPolicy configures how a request is retried.
//
// Use [DefaultRetryPolicy] as a starting point and change the fields you care
// about; the zero RetryPolicy disables retries entirely.
type RetryPolicy struct {
	// MaxRetries is the number of retries after the initial attempt.
	// Zero disables retries. Default: 2.
	MaxRetries int

	// BackoffInitial is the first backoff delay, doubled on each retry up to
	// BackoffMax. Default: 500ms.
	BackoffInitial time.Duration

	// BackoffMax caps the backoff delay. Default: 5s.
	BackoffMax time.Duration

	// BackoffJitter is the fraction of each delay randomly subtracted, from 0
	// to 1, so that concurrent clients do not retry in lockstep. Default: 0.25.
	BackoffJitter float64

	// Statuses holds the HTTP status codes to retry.
	// Default: 408, 429, and 500 through 599.
	Statuses map[int]bool

	// RespectRetryAfter honors the Retry-After and retry-after-ms response
	// headers in place of the computed backoff. Default: true.
	RespectRetryAfter bool

	// MaxRetryAfter is the longest server-requested delay to honor. A longer
	// delay falls back to the computed backoff. Default: 60s.
	MaxRetryAfter time.Duration

	// RetryConnectionErrors retries requests that failed to reach the API,
	// including a response body that ended early. Default: true.
	RetryConnectionErrors bool

	// RetryTimeouts retries attempts that exceeded their timeout. Default: true.
	RetryTimeouts bool

	// Budget caps the total elapsed time across all attempts and delays.
	// Zero, the default, disables it; prefer a deadline on the context, which
	// is always honored.
	Budget time.Duration
}

// DefaultRetryPolicy returns the SDK's default retry policy. Each call returns a
// fresh Statuses map, so changing it cannot affect other clients.
func DefaultRetryPolicy() RetryPolicy {
	statuses := map[int]bool{
		http.StatusRequestTimeout:  true,
		http.StatusTooManyRequests: true,
	}
	for code := 500; code < 600; code++ {
		statuses[code] = true
	}
	return RetryPolicy{
		MaxRetries:            defaultMaxRetries,
		BackoffInitial:        defaultBackoffInitial,
		BackoffMax:            defaultBackoffMax,
		BackoffJitter:         defaultBackoffJitter,
		Statuses:              statuses,
		RespectRetryAfter:     true,
		MaxRetryAfter:         defaultMaxRetryAfter,
		RetryConnectionErrors: true,
		RetryTimeouts:         true,
	}
}

func (p RetryPolicy) validate() error {
	if p.MaxRetries < 0 {
		return errf("retry MaxRetries must not be negative, got %d", p.MaxRetries)
	}
	if p.BackoffInitial < 0 {
		return errf("retry BackoffInitial must not be negative, got %s", p.BackoffInitial)
	}
	if p.BackoffMax < 0 {
		return errf("retry BackoffMax must not be negative, got %s", p.BackoffMax)
	}
	if p.BackoffJitter < 0 || p.BackoffJitter > 1 {
		return errf("retry BackoffJitter must be between 0 and 1, got %v", p.BackoffJitter)
	}
	if p.MaxRetryAfter < 0 {
		return errf("retry MaxRetryAfter must not be negative, got %s", p.MaxRetryAfter)
	}
	if p.Budget < 0 {
		return errf("retry Budget must not be negative, got %s", p.Budget)
	}
	for code := range p.Statuses {
		if code < 100 || code > 999 {
			return errf("retry Statuses must contain HTTP status codes, got %d", code)
		}
	}
	return nil
}

// retriesStatus reports whether the policy retries a status code.
func (p RetryPolicy) retriesStatus(code int) bool { return p.Statuses[code] }

// clone copies the policy so a caller mutating Statuses afterwards cannot affect
// an in-flight request.
func (p RetryPolicy) clone() RetryPolicy {
	statuses := make(map[int]bool, len(p.Statuses))
	for code, on := range p.Statuses {
		statuses[code] = on
	}
	p.Statuses = statuses
	return p
}

// backoff returns the delay before a retry. attempt is zero-based, so the first
// retry uses attempt 0. header is the response that triggered the retry, or nil
// for a connection failure.
func (p RetryPolicy) backoff(attempt int, header http.Header, rnd func() float64, now time.Time) time.Duration {
	if p.RespectRetryAfter && header != nil {
		if d, ok := parseRetryAfter(header, now); ok && d <= p.MaxRetryAfter {
			return d
		}
	}
	shift := min(attempt, maxBackoffShift)
	delay := p.BackoffInitial << shift
	if delay > p.BackoffMax || delay < 0 {
		delay = p.BackoffMax
	}
	// Jitter is subtracted, so the delay lands in [1-jitter, 1] of the
	// exponential value and never exceeds BackoffMax.
	return time.Duration(float64(delay) * (1 - rnd()*p.BackoffJitter))
}

// parseRetryAfter reads a server-requested retry delay, preferring the
// millisecond-precision retry-after-ms header over Retry-After, which may be a
// number of seconds or an HTTP date. A negative delay is ignored.
func parseRetryAfter(header http.Header, now time.Time) (time.Duration, bool) {
	if raw := header.Get("retry-after-ms"); raw != "" {
		if ms, err := strconv.ParseFloat(raw, 64); err == nil && ms >= 0 {
			return time.Duration(ms * float64(time.Millisecond)), true
		}
	}

	raw := header.Get("Retry-After")
	if raw == "" {
		return 0, false
	}
	if secs, err := strconv.ParseFloat(raw, 64); err == nil {
		if secs < 0 {
			return 0, false
		}
		return time.Duration(secs * float64(time.Second)), true
	}
	if date, err := http.ParseTime(raw); err == nil {
		return max(date.Sub(now), 0), true
	}
	return 0, false
}

// sleepContext waits for d, returning early if ctx ends.
func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// defaultRand is the jitter source. math/rand/v2's global generator is seeded
// randomly and safe for concurrent use, so no client-owned source is needed.
func defaultRand() float64 { return rand.Float64() }
