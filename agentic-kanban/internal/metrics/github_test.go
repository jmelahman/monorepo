package metrics

import "testing"

func TestNormalizeGitHubPath(t *testing.T) {
	cases := map[string]string{
		// First-page form.
		"/repos/octo/repo/pulls":              "/repos/:owner/:repo/pulls",
		"/repos/octo/repo/actions/runs":       "/repos/:owner/:repo/actions/runs",
		"/repos/octo/repo/actions/runs/42/jobs": "/repos/:owner/:repo/actions/runs/:id/jobs",
		// Pagination follow-ups arrive as /repositories/{numeric_id}/...;
		// must collapse onto the same label as the first-page form so we
		// don't grow one series per repo.
		"/repositories/633262635/pulls":            "/repos/:owner/:repo/pulls",
		"/repositories/633262635/actions/runs":     "/repos/:owner/:repo/actions/runs",
		"/repositories/633262635/actions/runs/42/jobs": "/repos/:owner/:repo/actions/runs/:id/jobs",
		// GHE prefix is stripped.
		"/api/v3/repos/octo/repo/pulls": "/repos/:owner/:repo/pulls",
		// Other known paths.
		"/search/issues": "/search/issues",
		"/rate_limit":    "/rate_limit",
		// Unknown path is returned as-is (low-cardinality assumption: the
		// kanban server only hits the patterns above).
		"/users/octo": "/users/octo",
	}
	for in, want := range cases {
		if got := normalizeGitHubPath(in); got != want {
			t.Errorf("normalizeGitHubPath(%q) = %q; want %q", in, got, want)
		}
	}
}
