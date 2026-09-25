// Package api defines the HTTP surface: JSON endpoints under /api/, the MCP
// endpoint at /mcp, and the embedded web frontend at /.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/jmelahman/agilecbt/internal/app"
	"github.com/jmelahman/agilecbt/internal/db"
	"github.com/jmelahman/agilecbt/web"
)

// BuildInfo describes the running binary; mirrored from cmd/server so the
// api package doesn't import it.
type BuildInfo struct {
	Version string `json:"version"`
}

// LLMStatus reports whether the curator's LLM backend is usable.
type LLMStatus struct {
	Backend   string `json:"backend"`
	Available bool   `json:"available"`
	Detail    string `json:"detail,omitempty"`
}

// Curator is the AI check-in coach. Implemented by internal/curator.
type Curator interface {
	Status(ctx context.Context) LLMStatus
	// Chat runs one user turn on a check-in, calling emit for each streamed
	// event ("text" deltas, "error"). It stores both sides of the exchange.
	Chat(ctx context.Context, checkinID int64, text string, emit func(event string, data any)) error
	// DraftRetro writes an AI draft for a week's retro and returns it.
	DraftRetro(ctx context.Context, weekID int64) (db.Retro, error)
}

// Deps carries the dependencies handlers need.
type Deps struct {
	App   *app.App
	Build BuildInfo
	// Secret enables auth when non-empty.
	Secret string
	// Curator is optional; nil means AI chat is unavailable.
	Curator Curator
	// MCP serves /mcp when non-nil.
	MCP http.Handler
}

// NewMux returns the full application handler: API routes, MCP, and the SPA.
func NewMux(d Deps) http.Handler {
	mux := http.NewServeMux()
	d.routes(mux)
	if d.MCP != nil {
		mux.Handle("/mcp", d.MCP)
	}
	mux.Handle("/", web.Handler())
	return d.requireAuth(mux)
}

// apiFunc is a JSON handler: it returns a value to encode, or an error that
// fail maps to a status code. A nil value with a nil error means 204.
type apiFunc func(r *http.Request) (any, error)

// handle adapts an apiFunc. POSTs that return a value answer 201.
func handle(fn apiFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v, err := fn(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		if v == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if ok, isOK := v.(okBody); isOK {
			writeJSON(w, http.StatusOK, ok.v)
			return
		}
		status := http.StatusOK
		if r.Method == http.MethodPost {
			status = http.StatusCreated
		}
		writeJSON(w, status, v)
	}
}

// okBody forces a 200 for POSTs that act rather than create.
type okBody struct{ v any }

// ok wraps an action-style POST result so it answers 200 rather than 201.
func ok[T any](v T, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	return okBody{v}, nil
}

// errUnavailable means the AI curator can't be reached right now.
var errUnavailable = errors.New("AI curator is unavailable")

// errBadRequest marks request-shape problems (bad JSON, bad ids).
type errBadRequest struct{ msg string }

func (e errBadRequest) Error() string { return e.msg }

func badRequest(format string, args ...any) error {
	return errBadRequest{fmt.Sprintf(format, args...)}
}

// fail maps domain errors to HTTP statuses.
func fail(w http.ResponseWriter, r *http.Request, err error) {
	var br errBadRequest
	switch {
	case errors.As(err, &br):
		httpError(w, http.StatusBadRequest, br.msg)
	case errors.Is(err, db.ErrInvalid):
		httpError(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), db.ErrInvalid.Error()+": "))
	case errors.Is(err, db.ErrNotFound):
		httpError(w, http.StatusNotFound, "not found")
	case errors.Is(err, errUnavailable):
		httpError(w, http.StatusServiceUnavailable, err.Error())
	default:
		internalError(w, r.Method+" "+r.URL.Path, err)
	}
}

// decode reads a JSON body into dst, rejecting unknown fields so typos in
// scripts fail loudly instead of silently doing nothing.
func decode(r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return badRequest("invalid JSON body: %v", err)
	}
	return nil
}

// pathID parses the {id} path value.
func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return 0, badRequest("invalid id %q", r.PathValue("id"))
	}
	return id, nil
}

// queryInt parses an optional integer query parameter.
func queryInt(r *http.Request, name string, def int) (int, error) {
	s := r.URL.Query().Get(name)
	if s == "" {
		return def, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, badRequest("invalid %s %q", name, s)
	}
	return n, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}

// httpError writes a JSON error body so API clients never have to parse
// plain-text errors.
func httpError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// internalError logs the underlying error and hides it from the client.
func internalError(w http.ResponseWriter, op string, err error) {
	log.Printf("%s: %v", op, err)
	httpError(w, http.StatusInternalServerError, "internal server error")
}
