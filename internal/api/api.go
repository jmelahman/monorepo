// Package api defines the HTTP surface: JSON endpoints under /api/ plus the
// embedded web frontend at /.
package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/jmelahman/fullstack-template/internal/db"
	"github.com/jmelahman/fullstack-template/web"
)

// BuildInfo describes the running binary; mirrored from cmd/server so the
// api package doesn't import it.
type BuildInfo struct {
	Version string `json:"version"`
}

// Deps carries the dependencies handlers need.
type Deps struct {
	Store *db.Store
	Build BuildInfo
}

// NewMux returns the full application handler: API routes plus the SPA.
func NewMux(d Deps) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", d.handleHealth)
	mux.HandleFunc("GET /api/items", d.handleListItems)
	mux.HandleFunc("POST /api/items", d.handleCreateItem)
	mux.HandleFunc("DELETE /api/items/{id}", d.handleDeleteItem)
	mux.Handle("/", web.Handler())
	return mux
}

func (d Deps) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": d.Build.Version,
	})
}

func (d Deps) handleListItems(w http.ResponseWriter, r *http.Request) {
	items, err := d.Store.ListItems()
	if err != nil {
		internalError(w, "list items", err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (d Deps) handleCreateItem(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		httpError(w, http.StatusBadRequest, "title is required")
		return
	}
	item, err := d.Store.CreateItem(req.Title)
	if err != nil {
		internalError(w, "create item", err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (d Deps) handleDeleteItem(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpError(w, http.StatusBadRequest, "invalid item id")
		return
	}
	switch err := d.Store.DeleteItem(id); {
	case errors.Is(err, db.ErrNotFound):
		httpError(w, http.StatusNotFound, "item not found")
	case err != nil:
		internalError(w, "delete item", err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
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
