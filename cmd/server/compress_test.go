package server

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// doCompressed drives compressResponses with an Accept-Encoding: gzip
// request against handler and returns the recorded response.
func doCompressed(t *testing.T, handler http.HandlerFunc, header http.Header) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	rec := httptest.NewRecorder()
	compressResponses(handler).ServeHTTP(rec, req)
	return rec
}

func gunzip(t *testing.T, r io.Reader) string {
	t.Helper()
	zr, err := gzip.NewReader(r)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	b, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCompressLargeResponse(t *testing.T) {
	body := strings.Repeat("local-preview ", 200) // ~2.8KB, over the threshold
	rec := doCompressed(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, body)
	}, nil)
	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got := rec.Header().Get("Vary"); got != "Accept-Encoding" {
		t.Errorf("Vary = %q, want Accept-Encoding", got)
	}
	if rec.Body.Len() >= len(body) {
		t.Errorf("compressed body is %d bytes, plain was %d", rec.Body.Len(), len(body))
	}
	if got := gunzip(t, rec.Body); got != body {
		t.Errorf("roundtrip mismatch: %d bytes, want %d", len(got), len(body))
	}
}

func TestSmallResponseStaysPlain(t *testing.T) {
	rec := doCompressed(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"status":"ok"}`)
	}, nil)
	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want none", got)
	}
	if rec.Body.String() != `{"status":"ok"}` {
		t.Errorf("body = %q", rec.Body.String())
	}
}

// The threshold decision must span writes: many small writes that add up
// past the threshold still compress.
func TestThresholdSpansWrites(t *testing.T) {
	rec := doCompressed(t, func(w http.ResponseWriter, r *http.Request) {
		for range 100 {
			io.WriteString(w, strings.Repeat("x", 32))
		}
	}, nil)
	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got := gunzip(t, rec.Body); got != strings.Repeat("x", 3200) {
		t.Errorf("roundtrip mismatch: %d bytes", len(got))
	}
}

func TestClientWithoutGzipGetsPlain(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil) // no Accept-Encoding
	rec := httptest.NewRecorder()
	compressResponses(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, strings.Repeat("y", 4096))
	})).ServeHTTP(rec, req)
	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want none", got)
	}
	if rec.Body.Len() != 4096 {
		t.Errorf("body = %d bytes, want 4096", rec.Body.Len())
	}
}

func TestPassthroughs(t *testing.T) {
	big := strings.Repeat("z", 4096)
	for name, handler := range map[string]http.HandlerFunc{
		"octet-stream downloads": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			io.WriteString(w, big)
		},
		"pre-encoded": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Encoding", "br")
			io.WriteString(w, big)
		},
		"event streams": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			io.WriteString(w, big)
		},
		"range responses": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusPartialContent)
			io.WriteString(w, big)
		},
	} {
		rec := doCompressed(t, handler, nil)
		if got := rec.Header().Get("Content-Encoding"); strings.Contains(got, "gzip") {
			t.Errorf("%s: Content-Encoding = %q, must not gzip", name, got)
		}
		if !strings.Contains(rec.Body.String(), "z") {
			t.Errorf("%s: body not passed through", name)
		}
	}
}

// A flush before the threshold commits the response plain so streamed bytes
// reach the wire immediately (SSE proxied from a preview backend that
// didn't set its Content-Type before the first write).
func TestFlushCommitsPlain(t *testing.T) {
	rec := doCompressed(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "tick\n")
		w.(http.Flusher).Flush()
		io.WriteString(w, strings.Repeat("tock\n", 1000))
	}, nil)
	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, want none after early flush", got)
	}
	if !strings.HasPrefix(rec.Body.String(), "tick\n") {
		t.Errorf("body = %q…", rec.Body.String()[:20])
	}
}

func TestCompressDropsIdentityHeaders(t *testing.T) {
	rec := doCompressed(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", "4096")
		io.WriteString(w, strings.Repeat("w", 4096))
	}, nil)
	if got := rec.Header().Get("Accept-Ranges"); got != "" {
		t.Errorf("Accept-Ranges = %q, want dropped on gzipped response", got)
	}
	if got := rec.Header().Get("Content-Length"); got == "4096" {
		t.Errorf("identity Content-Length survived compression")
	}
}

func TestStatusCodePreserved(t *testing.T) {
	rec := doCompressed(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, strings.Repeat("missing ", 512))
	}, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Errorf("Content-Encoding = %q, want gzip (large 404 body)", got)
	}
}
