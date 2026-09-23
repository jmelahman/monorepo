package metrics

import "github.com/prometheus/client_golang/prometheus"

// Histogram buckets tuned for sub-second local operations. Top bucket
// (10s) catches the GitHub poller's 20s timeout edge, but anything past
// that gets bucketed as +Inf which is fine — those are alerts on their own.
var latencyBuckets = []float64{
	.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10,
}

// DB query metrics.
//
// `op` is one of select/insert/update/delete/exec/query and `target` is the
// first identifier after the verb (table or "pragma"/"begin"/"commit"/"...").
// Together they keep cardinality bounded — see fingerprint() in sqldriver.go.
var (
	dbQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kanban_db_query_duration_seconds",
			Help:    "Latency of SQL operations against the embedded SQLite database.",
			Buckets: latencyBuckets,
		},
		[]string{"op", "target"},
	)
	dbQueryErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kanban_db_query_errors_total",
			Help: "SQL operations that returned a non-nil error (excludes sql.ErrNoRows).",
		},
		[]string{"op", "target"},
	)
)

// Local git CLI metrics.
//
// `operation` is a small, fixed set of human-readable labels (worktree_add,
// worktree_remove, diff, fetch) naming the git command kanban shelled out to.
// Latency is dominated by disk I/O and, for fetch, a remote round-trip, so it
// reuses latencyBuckets — the 10s top bucket lines up with the fetch timeout.
var (
	gitCommandDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kanban_git_command_duration_seconds",
			Help:    "Latency of local git CLI operations invoked by the kanban server.",
			Buckets: latencyBuckets,
		},
		[]string{"operation"},
	)
	gitCommandErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kanban_git_command_errors_total",
			Help: "Local git CLI operations that returned a non-nil error.",
		},
		[]string{"operation"},
	)
)

// GitHub API metrics.
//
// `endpoint` is the URL path with owner/repo/etc. normalized to placeholders
// so a noisy fleet of repos doesn't blow up cardinality. `resource` mirrors
// GitHub's X-RateLimit-Resource header ("core", "search", "graphql", ...).
var (
	githubRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kanban_github_api_requests_total",
			Help: "GitHub REST API requests issued by the kanban server.",
		},
		[]string{"endpoint", "method", "status"},
	)
	githubRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kanban_github_api_request_duration_seconds",
			Help:    "Latency of GitHub REST API requests.",
			Buckets: latencyBuckets,
		},
		[]string{"endpoint", "method"},
	)
	githubRateLimitRemaining = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kanban_github_rate_limit_remaining",
			Help: "Most recent X-RateLimit-Remaining value seen, per resource bucket.",
		},
		[]string{"resource"},
	)
	githubRateLimitLimit = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kanban_github_rate_limit_limit",
			Help: "Most recent X-RateLimit-Limit value seen, per resource bucket.",
		},
		[]string{"resource"},
	)
	githubRateLimitResetTimestamp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kanban_github_rate_limit_reset_timestamp_seconds",
			Help: "Most recent X-RateLimit-Reset value (Unix seconds) seen, per resource bucket.",
		},
		[]string{"resource"},
	)
)

// HTTP server metrics. `route` is the ServeMux pattern (Go 1.22+ Request.Pattern)
// so dynamic IDs don't fan out.
var (
	httpRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kanban_http_requests_total",
			Help: "HTTP requests handled by the kanban server.",
		},
		[]string{"route", "method", "status"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "kanban_http_request_duration_seconds",
			Help:    "Latency of HTTP requests handled by the kanban server.",
			Buckets: latencyBuckets,
		},
		[]string{"route", "method"},
	)
)
