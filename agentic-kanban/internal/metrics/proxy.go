package metrics

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sync"
)

// PrometheusProxyHandler returns a reverse-proxy handler that forwards
// requests under /prometheus/ to a Prometheus server reachable inside the
// docker network. The target is taken from $KANBAN_PROMETHEUS_URL; when the
// env var is empty the handler returns 503 so the route works in dev where
// no Prometheus is running.
//
// The Prometheus side must be started with --web.external-url and
// --web.route-prefix so the UI generates correct links — see compose.yaml.
func PrometheusProxyHandler() http.Handler {
	return &promProxy{}
}

type promProxy struct {
	once sync.Once
	rp   *httputil.ReverseProxy
	err  error
}

func (p *promProxy) lazyInit() {
	raw := os.Getenv("KANBAN_PROMETHEUS_URL")
	if raw == "" {
		return
	}
	target, err := url.Parse(raw)
	if err != nil {
		p.err = err
		return
	}
	p.rp = httputil.NewSingleHostReverseProxy(target)
}

func (p *promProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.once.Do(p.lazyInit)
	if p.rp == nil {
		http.Error(w, "prometheus not configured (set KANBAN_PROMETHEUS_URL)", http.StatusServiceUnavailable)
		return
	}
	p.rp.ServeHTTP(w, r)
}
