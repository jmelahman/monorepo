package metrics

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// WrapGitHubTransport returns an http.RoundTripper that counts requests and
// records rate-limit headers for every response. base may be nil; in that
// case http.DefaultTransport is wrapped.
func WrapGitHubTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &githubTransport{base: base}
}

type githubTransport struct{ base http.RoundTripper }

func (t *githubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	endpoint := normalizeGitHubPath(req.URL.Path)
	method := req.Method

	resp, err := t.base.RoundTrip(req)
	dur := time.Since(start).Seconds()
	githubRequestDuration.WithLabelValues(endpoint, method).Observe(dur)

	status := "error"
	if resp != nil {
		status = strconv.Itoa(resp.StatusCode)
		recordRateLimit(resp.Header)
	}
	githubRequests.WithLabelValues(endpoint, method, status).Inc()
	return resp, err
}

func recordRateLimit(h http.Header) {
	resource := h.Get("X-RateLimit-Resource")
	if resource == "" {
		// Responses without the resource header are unauthenticated or from
		// non-API endpoints (e.g. raw.githubusercontent.com) — bucket as "core"
		// since that's GitHub's default budget.
		resource = "core"
	}
	if v := h.Get("X-RateLimit-Remaining"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			githubRateLimitRemaining.WithLabelValues(resource).Set(f)
		}
	}
	if v := h.Get("X-RateLimit-Limit"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			githubRateLimitLimit.WithLabelValues(resource).Set(f)
		}
	}
	if v := h.Get("X-RateLimit-Reset"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			githubRateLimitResetTimestamp.WithLabelValues(resource).Set(f)
		}
	}
}

// normalizeGitHubPath replaces variable URL segments with placeholders so
// the endpoint label has bounded cardinality. We recognise the GitHub
// patterns the kanban server actually hits (repos, search) and fall back to
// the raw path for the rest — the fallback set should be empty in practice.
func normalizeGitHubPath(p string) string {
	// Drop GitHub Enterprise prefix /api/v3 if present.
	p = strings.TrimPrefix(p, "/api/v3")
	if p == "" {
		return "/"
	}
	segs := strings.Split(strings.TrimPrefix(p, "/"), "/")
	switch {
	case len(segs) >= 3 && segs[0] == "repos":
		// /repos/:owner/:repo[/...]
		out := []string{"", "repos", ":owner", ":repo"}
		out = append(out, segs[3:]...)
		// Numeric ids inside the remainder become :id (e.g. pulls/123/files).
		for i := 4; i < len(out); i++ {
			if isNumeric(out[i]) {
				out[i] = ":id"
			}
		}
		return strings.Join(out, "/")
	case len(segs) >= 2 && segs[0] == "repositories" && isNumeric(segs[1]):
		// GitHub's pagination `Link` headers come back as
		// /repositories/{numeric_id}/... even though the first-page request
		// used /repos/{owner}/{repo}/.... Collapse onto the same label so
		// /repos/:owner/:repo/pulls and its paginated follow-ups share one
		// series in the counter — otherwise we'd grow one label per repo.
		out := []string{"", "repos", ":owner", ":repo"}
		out = append(out, segs[2:]...)
		for i := 4; i < len(out); i++ {
			if isNumeric(out[i]) {
				out[i] = ":id"
			}
		}
		return strings.Join(out, "/")
	case len(segs) >= 2 && segs[0] == "search":
		return "/search/" + segs[1]
	case len(segs) >= 1 && segs[0] == "rate_limit":
		return "/rate_limit"
	default:
		return p
	}
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
