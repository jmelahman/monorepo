package metrics

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"
)

// HTTPMiddleware records counter + duration histogram per route, where
// "route" is the Go 1.22+ ServeMux pattern (e.g. "GET /api/boards/{id}").
// Requests that match no pattern report route="other" so we don't blow up
// cardinality on probes for arbitrary paths.
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &metricsRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rec, r)

		route := r.Pattern
		if route == "" {
			route = "other"
		}
		method := r.Method
		status := strconv.Itoa(rec.status)
		httpRequests.WithLabelValues(route, method, status).Inc()
		httpRequestDuration.WithLabelValues(route, method).Observe(time.Since(start).Seconds())
	})
}

type metricsRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (m *metricsRecorder) WriteHeader(code int) {
	if !m.wroteHeader {
		m.status = code
		m.wroteHeader = true
	}
	m.ResponseWriter.WriteHeader(code)
}

func (m *metricsRecorder) Write(b []byte) (int, error) {
	if !m.wroteHeader {
		m.wroteHeader = true
	}
	return m.ResponseWriter.Write(b)
}

func (m *metricsRecorder) Flush() {
	if f, ok := m.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (m *metricsRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := m.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("hijack not supported")
	}
	return hj.Hijack()
}
