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
// saved — measured locally, 263 B responses became ~60% slower under gzip.
const gzipMinBytes = 1024

// gzipPool reuses gzip writers to keep allocations off the hot path.
var gzipPool = sync.Pool{
	New: func() any { return gzip.NewWriter(io.Discard) },
}

// compressResponses wraps next with a gzip encoder. It honors Accept-Encoding,
// skips streaming responses (text/event-stream) and hijacked connections
// (websockets), and only compresses payloads larger than gzipMinBytes so small
// responses don't pay the gzip CPU cost for negligible bandwidth savings.
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
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		if strings.EqualFold(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]), "gzip") {
			return true
		}
	}
	return false
}

type writeMode int

const (
	modeUndecided  writeMode = iota // buffering, no headers committed
	modePassthrough                 // never compress (SSE, hijack, etc.)
	modeGzip                        // compressing
)

// gzipWriter defers the gzip/Content-Encoding decision until either the buffer
// crosses gzipMinBytes (compress) or the handler finishes (write plain). This
// keeps small responses fast and avoids encoding decisions for streams.
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
	// If the handler set a Content-Type we know forbids compression, commit
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

// shouldPassthrough inspects committed response headers to detect streaming
// or pre-encoded responses that must not be compressed.
func (g *gzipWriter) shouldPassthrough() bool {
	h := g.ResponseWriter.Header()
	if h.Get("Content-Encoding") != "" {
		return true
	}
	if strings.HasPrefix(h.Get("Content-Type"), "text/event-stream") {
		return true
	}
	return false
}

func (g *gzipWriter) startGzip() {
	g.mode = modeGzip
	h := g.ResponseWriter.Header()
	h.Set("Content-Encoding", "gzip")
	h.Del("Content-Length") // encoded length differs
	g.gz = gzipPool.Get().(*gzip.Writer)
	g.gz.Reset(g.ResponseWriter)
	g.ResponseWriter.WriteHeader(g.status)
}

// finish commits any buffered writes and closes the gzip stream if active.
// Called via defer from compressResponses.
func (g *gzipWriter) finish() {
	if !g.wroteHead {
		// Handler wrote nothing — nothing to do; the framework will send 200
		// with an empty body if necessary.
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

// Flush is called by streaming handlers (SSE). If we're still buffering, this
// is a signal that the handler wants bytes on the wire now — commit as plain.
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

// Hijack is required for websocket upgrades. Compression never applies to a
// hijacked connection, so mark passthrough and delegate.
func (g *gzipWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	g.mode = modePassthrough
	hj, ok := g.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("hijack not supported")
	}
	return hj.Hijack()
}
