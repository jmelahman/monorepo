package server

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

// gzipMinBytes is the response-size threshold below which compression is
// skipped. Compressing tiny JSON payloads costs more CPU than the few bytes
// saved — and can grow them (an 83 B health response gzips to 92 B).
const gzipMinBytes = 1024

// gzipPool reuses gzip writers to keep allocations off the hot path.
var gzipPool = sync.Pool{
	New: func() any { return gzip.NewWriter(io.Discard) },
}

// compressResponses wraps next with a gzip encoder. It honors
// Accept-Encoding and only compresses payloads larger than gzipMinBytes, so
// small responses don't pay the gzip CPU cost for negligible savings. It
// leaves alone anything that must not be re-encoded: pre-encoded responses,
// event streams (previewed backends may serve SSE through the proxy),
// artifact downloads (already-dense binaries), range responses, and
// hijacked connections.
func compressResponses(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !clientAcceptsGzip(r) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Add("Vary", "Accept-Encoding")
		gw := &gzipWriter{ResponseWriter: w}
		defer gw.finish()
		next.ServeHTTP(gw, r)
	})
}

func clientAcceptsGzip(r *http.Request) bool {
	for part := range strings.SplitSeq(r.Header.Get("Accept-Encoding"), ",") {
		if strings.EqualFold(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]), "gzip") {
			return true
		}
	}
	return false
}

type writeMode int

const (
	modeUndecided   writeMode = iota // buffering, no headers committed
	modePassthrough                  // never compress (SSE, downloads, hijack, …)
	modeGzip                         // compressing
)

// gzipWriter defers the gzip/Content-Encoding decision until either the
// buffer crosses gzipMinBytes (compress) or the handler finishes (write
// plain). This keeps small responses fast and avoids encoding decisions for
// streams.
type gzipWriter struct {
	http.ResponseWriter
	gz        *gzip.Writer
	buf       []byte
	mode      writeMode
	status    int
	wroteHead bool
}

func (g *gzipWriter) WriteHeader(code int) {
	if g.wroteHead {
		return
	}
	g.status = code
	g.wroteHead = true
	// If the committed status or headers forbid compression, pass through
	// immediately. Otherwise wait until we know the size.
	if g.shouldPassthrough() {
		g.mode = modePassthrough
		g.ResponseWriter.WriteHeader(code)
	}
}

func (g *gzipWriter) Write(p []byte) (int, error) {
	if !g.wroteHead {
		g.WriteHeader(http.StatusOK)
	}
	switch g.mode {
	case modePassthrough:
		return g.ResponseWriter.Write(p)
	case modeGzip:
		return g.gz.Write(p)
	}
	// Undecided: buffer until threshold or handler completion.
	if len(g.buf)+len(p) < gzipMinBytes {
		g.buf = append(g.buf, p...)
		return len(p), nil
	}
	g.startGzip()
	if len(g.buf) > 0 {
		if _, err := g.gz.Write(g.buf); err != nil {
			return 0, err
		}
		g.buf = nil
	}
	return g.gz.Write(p)
}

// shouldPassthrough inspects the committed status and response headers for
// responses that must not be compressed.
func (g *gzipWriter) shouldPassthrough() bool {
	// A range response's bytes are offsets into the identity encoding;
	// re-encoding the slice would corrupt resumed downloads.
	if g.status == http.StatusPartialContent {
		return true
	}
	// A protocol switch (WebSocket upgrade — the exec endpoint, or one proxied
	// to a preview) must hit the wire immediately; the connection is about to
	// be hijacked.
	if g.status == http.StatusSwitchingProtocols {
		return true
	}
	h := g.ResponseWriter.Header()
	if h.Get("Content-Encoding") != "" {
		return true
	}
	ct := h.Get("Content-Type")
	if strings.HasPrefix(ct, "text/event-stream") {
		return true
	}
	// Artifact downloads: opaque binaries that are dense already.
	if strings.HasPrefix(ct, "application/octet-stream") {
		return true
	}
	return false
}

func (g *gzipWriter) startGzip() {
	g.mode = modeGzip
	h := g.ResponseWriter.Header()
	h.Set("Content-Encoding", "gzip")
	h.Del("Content-Length") // encoded length differs
	// Byte ranges address the identity encoding; don't advertise them on a
	// gzipped representation.
	h.Del("Accept-Ranges")
	g.gz = gzipPool.Get().(*gzip.Writer)
	g.gz.Reset(g.ResponseWriter)
	g.ResponseWriter.WriteHeader(g.status)
}

// finish commits any buffered writes and closes the gzip stream if active.
// Called via defer from compressResponses.
func (g *gzipWriter) finish() {
	if !g.wroteHead {
		// Handler wrote nothing — the framework sends the implicit 200 with
		// an empty body itself.
		return
	}
	switch g.mode {
	case modeGzip:
		_ = g.gz.Close()
		gzipPool.Put(g.gz)
		g.gz = nil
	case modeUndecided:
		// Small response: flush buffered bytes plain.
		g.mode = modePassthrough
		g.ResponseWriter.WriteHeader(g.status)
		if len(g.buf) > 0 {
			_, _ = g.ResponseWriter.Write(g.buf)
			g.buf = nil
		}
	}
}

// Flush is called by streaming handlers (SSE through the preview proxy, or
// the proxy's own FlushInterval). If we're still buffering, it's a signal
// that the handler wants bytes on the wire now — commit as plain.
func (g *gzipWriter) Flush() {
	if g.mode == modeUndecided && g.wroteHead {
		g.mode = modePassthrough
		g.ResponseWriter.WriteHeader(g.status)
		if len(g.buf) > 0 {
			_, _ = g.ResponseWriter.Write(g.buf)
			g.buf = nil
		}
	}
	if g.mode == modeGzip {
		_ = g.gz.Flush()
	}
	if f, ok := g.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack is required for websocket upgrades through the preview proxy.
// Compression never applies to a hijacked connection, so mark passthrough
// and delegate.
func (g *gzipWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	g.mode = modePassthrough
	hj, ok := g.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("hijack not supported")
	}
	return hj.Hijack()
}

// Unwrap lets http.ResponseController (and libraries following its Unwrap
// convention) reach capabilities this wrapper doesn't intercept — deadline
// control for hijacked exec sessions, notably.
func (g *gzipWriter) Unwrap() http.ResponseWriter { return g.ResponseWriter }
