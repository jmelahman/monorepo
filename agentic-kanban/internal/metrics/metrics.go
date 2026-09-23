// Package metrics exposes a shared Prometheus registry plus instrumentation
// helpers (a SQL driver wrapper, an HTTP RoundTripper for the GitHub client,
// and a server-side middleware). All collectors are registered on the
// package-level registry so a single GET /metrics handler surfaces them.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Registry is the package-level registry. Tests can swap it via the helpers
// in metrics_test.go; production code uses it through Handler() and the
// instrumentation constructors.
var Registry = prometheus.NewRegistry()

func init() {
	Registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	Registry.MustRegister(
		dbQueryDuration,
		dbQueryErrors,
		gitCommandDuration,
		gitCommandErrors,
		githubRequests,
		githubRequestDuration,
		githubRateLimitRemaining,
		githubRateLimitLimit,
		githubRateLimitResetTimestamp,
		httpRequests,
		httpRequestDuration,
	)
}

// Handler returns an http.Handler that serves the Prometheus exposition
// format for the package registry. Mount it at /metrics.
func Handler() http.Handler {
	return promhttp.HandlerFor(Registry, promhttp.HandlerOpts{
		Registry: Registry,
	})
}
