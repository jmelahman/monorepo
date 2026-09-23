package typesafe

import (
	"net/http"
	"testing"
	"time"
)

func TestParseRetryAfter(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		header http.Header
		want   time.Duration
		ok     bool
	}{{
		name:   "no header",
		header: http.Header{},
	}, {
		name:   "seconds",
		header: http.Header{"Retry-After": {"3"}},
		want:   3 * time.Second, ok: true,
	}, {
		name:   "fractional seconds",
		header: http.Header{"Retry-After": {"0.5"}},
		want:   500 * time.Millisecond, ok: true,
	}, {
		name:   "milliseconds win over seconds",
		header: http.Header{"Retry-After": {"30"}, "Retry-After-Ms": {"250"}},
		want:   250 * time.Millisecond, ok: true,
	}, {
		name:   "http date",
		header: http.Header{"Retry-After": {now.Add(7 * time.Second).Format(http.TimeFormat)}},
		want:   7 * time.Second, ok: true,
	}, {
		name:   "past http date is zero, not negative",
		header: http.Header{"Retry-After": {now.Add(-time.Hour).Format(http.TimeFormat)}},
		want:   0, ok: true,
	}, {
		name:   "negative seconds ignored",
		header: http.Header{"Retry-After": {"-5"}},
	}, {
		name:   "garbage ignored",
		header: http.Header{"Retry-After": {"soon"}},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := parseRetryAfter(tc.header, now)
			if ok != tc.ok || got != tc.want {
				t.Errorf("parseRetryAfter() = (%s, %t), want (%s, %t)", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestBackoff(t *testing.T) {
	t.Parallel()

	p := DefaultRetryPolicy()
	now := time.Now()

	// Exponential growth, capped at BackoffMax.
	want := []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second, 4 * time.Second, 5 * time.Second, 5 * time.Second}
	for attempt, w := range want {
		if got := p.backoff(attempt, nil, noJitter, now); got != w {
			t.Errorf("backoff(%d) = %s, want %s", attempt, got, w)
		}
	}

	// A large attempt count must not overflow into a negative delay.
	if got := p.backoff(1<<20, nil, noJitter, now); got != p.BackoffMax {
		t.Errorf("backoff(huge) = %s, want %s", got, p.BackoffMax)
	}

	// Jitter is subtracted, so the delay never exceeds the exponential value.
	if got, want := p.backoff(0, nil, func() float64 { return 1 }, now), 375*time.Millisecond; got != want {
		t.Errorf("backoff with full jitter = %s, want %s", got, want)
	}
}

func TestBackoffRetryAfter(t *testing.T) {
	t.Parallel()

	p := DefaultRetryPolicy()
	now := time.Now()

	header := http.Header{"Retry-After": {"2"}}
	if got, want := p.backoff(0, header, noJitter, now), 2*time.Second; got != want {
		t.Errorf("backoff with Retry-After = %s, want %s", got, want)
	}

	// A delay longer than MaxRetryAfter falls back to the computed backoff
	// rather than parking the caller for minutes.
	over := http.Header{"Retry-After": {"3600"}}
	if got, want := p.backoff(0, over, noJitter, now), 500*time.Millisecond; got != want {
		t.Errorf("backoff with over-cap Retry-After = %s, want %s", got, want)
	}

	p.RespectRetryAfter = false
	if got, want := p.backoff(0, header, noJitter, now), 500*time.Millisecond; got != want {
		t.Errorf("backoff with RespectRetryAfter off = %s, want %s", got, want)
	}
}

func TestDefaultRetryPolicyIsIndependent(t *testing.T) {
	t.Parallel()

	first := DefaultRetryPolicy()
	first.Statuses[418] = true
	if DefaultRetryPolicy().Statuses[418] {
		t.Error("mutating a returned policy changed later ones")
	}
	if !first.retriesStatus(529) {
		t.Error("the API's non-standard 529 must be retried")
	}
}

func TestRetryPolicyValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		mutil func(*RetryPolicy)
		bad   bool
	}{
		{name: "default"},
		{name: "negative retries", mutil: func(p *RetryPolicy) { p.MaxRetries = -1 }, bad: true},
		{name: "jitter above one", mutil: func(p *RetryPolicy) { p.BackoffJitter = 1.5 }, bad: true},
		{name: "negative budget", mutil: func(p *RetryPolicy) { p.Budget = -time.Second }, bad: true},
		{name: "non-status code", mutil: func(p *RetryPolicy) { p.Statuses[42] = true }, bad: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := DefaultRetryPolicy()
			if tc.mutil != nil {
				tc.mutil(&p)
			}
			if err := p.validate(); (err != nil) != tc.bad {
				t.Errorf("validate() = %v, want bad=%t", err, tc.bad)
			}
		})
	}
}
