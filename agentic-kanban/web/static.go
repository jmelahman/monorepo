//go:build !embed

package web

import (
	"net/http"
	"strings"
)

// Handler returns a placeholder when the frontend is not embedded. Build with
// `-tags embed` (and a populated web/dist) to serve the real SPA.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ws/") {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "frontend not built (rebuild with -tags embed)", http.StatusServiceUnavailable)
	})
}
