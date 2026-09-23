//go:build embed

package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Embedded files carry no modtime (no Last-Modified, no ETag), so cache
// behavior rides entirely on the Cache-Control split: hashed assets are
// immutable, everything else must revalidate.
func TestHandlerCacheHeaders(t *testing.T) {
	h := Handler()
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		return rec
	}

	rec := get("/")
	if rec.Code != http.StatusOK {
		t.Fatalf("index: %d", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("index Cache-Control = %q, want no-cache", got)
	}
	// SPA fallback routes serve index.html and must stay fresh too.
	if got := get("/some/route").Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("spa fallback Cache-Control = %q, want no-cache", got)
	}
	if got := get("/assets/anything.js").Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Errorf("asset Cache-Control = %q, want immutable", got)
	}
}
