package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// The store's own catalogue can be browsed, so a whole album is bought
// off its page rather than track by track from streams. Looking costs
// nothing; only the tracks picked and confirmed are bought, through the
// queue like any other buy.

// storeLook bounds a search or an album's page.
const storeLook = 45 * time.Second

// albumPick is a track picked off a store album's page.
type albumPick struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
}

// routeStore serves the store's search and album pages from the store
// provider, and takes the tracks picked off one.
func routeStore(mux *http.ServeMux, q *buyQueue) {
	look := func(w http.ResponseWriter, r *http.Request, args ...string) {
		if q == nil {
			http.Error(w, "no store provider configured", http.StatusServiceUnavailable)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), storeLook)
		defer cancel()
		out, err := q.provider(ctx, args...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if !json.Valid(out) {
			http.Error(w, "the store provider's answer could not be read", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(out)
	}
	// "--" ends the flags, so a name that starts with a dash is a name.
	mux.HandleFunc("GET /api/store/search", func(w http.ResponseWriter, r *http.Request) {
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		if query == "" {
			http.Error(w, "q required", http.StatusBadRequest)
			return
		}
		look(w, r, "search", "--", query)
	})
	mux.HandleFunc("GET /api/store/album", func(w http.ResponseWriter, r *http.Request) {
		v := r.URL.Query()
		artist, album := strings.TrimSpace(v.Get("artist")), strings.TrimSpace(v.Get("album"))
		if artist == "" || album == "" {
			http.Error(w, "artist and album required", http.StatusBadRequest)
			return
		}
		look(w, r, "album", "--", artist, album)
	})
	// Buying an album queues the tracks picked, each a buy of its own.
	mux.HandleFunc("POST /api/buy/album", func(w http.ResponseWriter, r *http.Request) {
		if q == nil {
			http.Error(w, "no store provider configured", http.StatusServiceUnavailable)
			return
		}
		var req struct {
			Artist string      `json:"artist"`
			Album  string      `json:"album"`
			Page   string      `json:"page"`
			Tracks []albumPick `json:"tracks"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
		err := json.NewDecoder(r.Body).Decode(&req)
		req.Artist, req.Album, req.Page = strings.TrimSpace(req.Artist), strings.TrimSpace(req.Album), strings.TrimSpace(req.Page)
		if err != nil || req.Artist == "" || req.Album == "" || len(req.Tracks) == 0 {
			http.Error(w, "artist, album and tracks required", http.StatusBadRequest)
			return
		}
		if !storePage(req.Page) {
			http.Error(w, "the page must be the store album's path or http(s) address", http.StatusBadRequest)
			return
		}
		for i, t := range req.Tracks {
			req.Tracks[i].Title = strings.TrimSpace(t.Title)
			if req.Tracks[i].Title == "" || t.Number <= 0 {
				http.Error(w, "each track needs its number and title", http.StatusBadRequest)
				return
			}
		}
		bs, err := q.addAlbum(req.Artist, req.Album, req.Page, req.Tracks)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(bs)
	})
}

// storePage reports whether page names a store page: a path on the
// store, as its search gives, or a whole address.
func storePage(page string) bool {
	u, err := url.Parse(page)
	if err != nil || page == "" {
		return false
	}
	if u.Scheme == "" && u.Host == "" {
		return strings.HasPrefix(u.Path, "/") && !strings.HasPrefix(page, "//")
	}
	return (u.Scheme == "https" || u.Scheme == "http") && u.Host != ""
}
