package main

import (
	"cmp"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	mrand "math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// libTrack is a track with the artist and album it is filed under, as
// playlists and mixes hand them out: flat, from anywhere in the library.
type libTrack struct {
	track
	Artist string `json:"artist"`
	Album  string `json:"album"`
}

// flatten lists every track of the library with where it is filed.
func flatten(artists []artist) []libTrack {
	var all []libTrack
	for _, a := range artists {
		for _, al := range a.Albums {
			for _, t := range al.Tracks {
				all = append(all, libTrack{track: t, Artist: a.Name, Album: al.Name})
			}
		}
	}
	return all
}

// plays counts how often a track was played, and when it last was.
type plays struct {
	N    int       `json:"n"`
	Last time.Time `json:"last"`
}

// rule is one condition of a smart playlist. Text fields (artist, album,
// title) take contains, is, not and starts; plays takes >, < and =; added
// and played take within and before, in days.
type rule struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value string `json:"value"`
}

// smart is what a smart playlist is made of: its rules, whether a track
// must meet all of them or any, the order and how many it holds.
type smart struct {
	Match string `json:"match"` // all or any
	Rules []rule `json:"rules"`
	Sort  string `json:"sort"` // artist, added, plays, played or random
	Limit int    `json:"limit"`
}

// playlist is either a list of tracks, kept in its order, or a smart one
// whose tracks are whatever meets its rules when it is asked for.
type playlist struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Paths   []string  `json:"paths,omitempty"`
	Smart   *smart    `json:"smart,omitempty"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

// playlistView is a playlist as the clients see it: with its tracks when
// one is asked for, and only their count in a listing.
type playlistView struct {
	playlist
	Count  int        `json:"count"`
	Tracks []libTrack `json:"tracks,omitempty"`
}

// shelf keeps what the listener adds to the library: playlists, how often
// each track was played, and the tracks kept out of the mixes. Owned
// tracks go by their path; streamed ones by "yt:" and their video id.
type shelf struct {
	dir string

	mu        sync.Mutex
	playlists []*playlist
	plays     map[string]plays
	excluded  map[string]bool
}

func newShelf(state string) (*shelf, error) {
	s := &shelf{dir: state, plays: map[string]plays{}, excluded: map[string]bool{}}
	if err := loadJSON(filepath.Join(state, "playlists.json"), &s.playlists); err != nil {
		return nil, err
	}
	var meta struct {
		Plays    map[string]plays `json:"plays"`
		Excluded []string         `json:"excluded"`
	}
	if err := loadJSON(filepath.Join(state, "library.json"), &meta); err != nil {
		return nil, err
	}
	if meta.Plays != nil {
		s.plays = meta.Plays
	}
	for _, k := range meta.Excluded {
		s.excluded[k] = true
	}
	return s, nil
}

func (s *shelf) saveMetaLocked() error {
	ex := make([]string, 0, len(s.excluded))
	for k := range s.excluded {
		ex = append(ex, k)
	}
	slices.Sort(ex)
	return saveJSON(filepath.Join(s.dir, "library.json"), struct {
		Plays    map[string]plays `json:"plays"`
		Excluded []string         `json:"excluded"`
	}{s.plays, ex})
}

func (s *shelf) savePlaylistsLocked() error {
	return saveJSON(filepath.Join(s.dir, "playlists.json"), s.playlists)
}

// decorate fills in the plays and the exclusions the library listing
// carries beside its tracks.
func (s *shelf) decorate(artists []artist) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range artists {
		for _, al := range a.Albums {
			for i := range al.Tracks {
				t := &al.Tracks[i]
				t.Plays = s.plays[t.Path].N
				t.Excluded = s.excluded[t.Path]
			}
		}
	}
}

func (s *shelf) played(path string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.plays[path]
	p.N++
	p.Last = at
	s.plays[path] = p
	return s.saveMetaLocked()
}

func (s *shelf) setExcluded(key string, excluded bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if excluded {
		s.excluded[key] = true
	} else {
		delete(s.excluded, key)
	}
	return s.saveMetaLocked()
}

func (s *shelf) isExcluded(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.excluded[key]
}

func (s *shelf) excludedKeys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := make([]string, 0, len(s.excluded))
	for k := range s.excluded {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func (s *shelf) playsOf(path string) plays {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.plays[path]
}

// resolve lists a playlist's tracks from the library as it is now: a
// plain one's in its order, skipping any that have gone; a smart one's
// by its rules.
func (s *shelf) resolve(p *playlist, all []libTrack, now time.Time) []libTrack {
	if p.Smart == nil {
		byPath := make(map[string]libTrack, len(all))
		for _, t := range all {
			byPath[t.Path] = t
		}
		out := []libTrack{}
		for _, path := range p.Paths {
			if t, ok := byPath[path]; ok {
				out = append(out, t)
			}
		}
		return out
	}
	sm := p.Smart
	out := []libTrack{}
	for _, t := range all {
		if s.matches(sm, t, now) {
			out = append(out, t)
		}
	}
	s.sortTracks(out, sm.Sort, p.ID, now)
	if sm.Limit > 0 && len(out) > sm.Limit {
		out = out[:sm.Limit]
	}
	return out
}

func (s *shelf) matches(sm *smart, t libTrack, now time.Time) bool {
	if len(sm.Rules) == 0 {
		return true
	}
	anyOf := sm.Match == "any"
	for _, r := range sm.Rules {
		ok := s.meets(r, t, now)
		if anyOf && ok {
			return true
		}
		if !anyOf && !ok {
			return false
		}
	}
	return !anyOf
}

func (s *shelf) meets(r rule, t libTrack, now time.Time) bool {
	switch r.Field {
	case "artist", "album", "title":
		have := map[string]string{"artist": t.Artist, "album": t.Album, "title": t.Name}[r.Field]
		have, want := strings.ToLower(have), strings.ToLower(strings.TrimSpace(r.Value))
		switch r.Op {
		case "contains":
			return strings.Contains(have, want)
		case "is":
			return have == want
		case "not":
			return !strings.Contains(have, want)
		case "starts":
			return strings.HasPrefix(have, want)
		}
	case "plays":
		n, _ := strconv.Atoi(r.Value)
		have := s.plays[t.Path].N
		switch r.Op {
		case ">":
			return have > n
		case "<":
			return have < n
		case "=":
			return have == n
		}
	case "added", "played":
		days, _ := strconv.Atoi(r.Value)
		cutoff := now.AddDate(0, 0, -days)
		var at time.Time
		if r.Field == "added" {
			at = time.Unix(t.Added, 0)
		} else {
			at = s.plays[t.Path].Last // zero when never played: before any cutoff
		}
		switch r.Op {
		case "within":
			return at.After(cutoff)
		case "before":
			return !at.After(cutoff)
		}
	}
	return false
}

// sortTracks orders a smart playlist's tracks. Random is shuffled the
// same way all day, so a phone keeping the playlist doesn't download a
// new set every time it asks.
func (s *shelf) sortTracks(ts []libTrack, by, id string, now time.Time) {
	switch by {
	case "added":
		slices.SortStableFunc(ts, func(a, b libTrack) int { return cmp.Compare(b.Added, a.Added) })
	case "plays":
		slices.SortStableFunc(ts, func(a, b libTrack) int { return cmp.Compare(s.plays[b.Path].N, s.plays[a.Path].N) })
	case "played":
		slices.SortStableFunc(ts, func(a, b libTrack) int {
			return s.plays[b.Path].Last.Compare(s.plays[a.Path].Last)
		})
	case "random":
		rng := dailyRand(id, now)
		rng.Shuffle(len(ts), func(i, j int) { ts[i], ts[j] = ts[j], ts[i] })
	}
	// artist, the default, is the library's own order already.
}

// dailyRand is a source that is the same for one name all day.
func dailyRand(name string, now time.Time) *mrand.Rand {
	h := fnv.New64a()
	_, _ = h.Write([]byte(name + now.Format(time.DateOnly)))
	seed := h.Sum64()
	return mrand.New(mrand.NewPCG(seed, seed>>1|1))
}

// check refuses a playlist the clients could not have meant.
func (p *playlist) check() error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return errors.New("a playlist needs a name")
	}
	if p.Smart == nil {
		return nil
	}
	p.Paths = nil
	sm := p.Smart
	if sm.Match != "any" {
		sm.Match = "all"
	}
	if !slices.Contains([]string{"", "artist", "added", "plays", "played", "random"}, sm.Sort) {
		return fmt.Errorf("unknown sort %q", sm.Sort)
	}
	if sm.Limit < 0 || sm.Limit > 5000 {
		return errors.New("the limit is from 0 (none) to 5000")
	}
	ops := map[string][]string{
		"artist": {"contains", "is", "not", "starts"},
		"album":  {"contains", "is", "not", "starts"},
		"title":  {"contains", "is", "not", "starts"},
		"plays":  {">", "<", "="},
		"added":  {"within", "before"},
		"played": {"within", "before"},
	}
	for _, r := range sm.Rules {
		allowed, ok := ops[r.Field]
		if !ok {
			return fmt.Errorf("unknown field %q", r.Field)
		}
		if !slices.Contains(allowed, r.Op) {
			return fmt.Errorf("%s can't be compared with %q", r.Field, r.Op)
		}
		if r.Field == "plays" || r.Field == "added" || r.Field == "played" {
			if _, err := strconv.Atoi(r.Value); err != nil {
				return fmt.Errorf("%s wants a number, not %q", r.Field, r.Value)
			}
		}
	}
	return nil
}

func (s *shelf) find(id string) *playlist {
	i := slices.IndexFunc(s.playlists, func(p *playlist) bool { return p.ID == id })
	if i < 0 {
		return nil
	}
	return s.playlists[i]
}

// routeShelf serves the playlists, the play counts and the exclusions.
func routeShelf(mux *http.ServeMux, s *shelf, music string) {
	library := func(w http.ResponseWriter) ([]libTrack, bool) {
		artists, err := walkLibrary(music)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return nil, false
		}
		return flatten(artists), true
	}
	view := func(p *playlist, all []libTrack, withTracks bool) playlistView {
		ts := s.resolve(p, all, time.Now())
		v := playlistView{playlist: *p, Count: len(ts)}
		if withTracks {
			v.Tracks = ts
		}
		return v
	}
	mux.HandleFunc("GET /api/playlists", func(w http.ResponseWriter, _ *http.Request) {
		all, ok := library(w)
		if !ok {
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		out := make([]playlistView, 0, len(s.playlists))
		for _, p := range s.playlists {
			out = append(out, view(p, all, false))
		}
		writeJSON(w, out)
	})
	mux.HandleFunc("GET /api/playlists/{id}", func(w http.ResponseWriter, r *http.Request) {
		all, ok := library(w)
		if !ok {
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		p := s.find(r.PathValue("id"))
		if p == nil {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, view(p, all, true))
	})
	// Create, or replace one: the body is the whole playlist.
	save := func(w http.ResponseWriter, r *http.Request, id string) {
		var in playlist
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "the playlist could not be read", http.StatusBadRequest)
			return
		}
		if err := in.check(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		all, ok := library(w)
		if !ok {
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		now := time.Now()
		p := s.find(id)
		switch {
		case id == "":
			p = &playlist{ID: newID(), Created: now}
			s.playlists = append(s.playlists, p)
		case p == nil:
			http.NotFound(w, r)
			return
		}
		p.Name, p.Paths, p.Smart, p.Updated = in.Name, in.Paths, in.Smart, now
		if err := s.savePlaylistsLocked(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, view(p, all, true))
	}
	mux.HandleFunc("POST /api/playlists", func(w http.ResponseWriter, r *http.Request) { save(w, r, "") })
	mux.HandleFunc("PUT /api/playlists/{id}", func(w http.ResponseWriter, r *http.Request) {
		save(w, r, r.PathValue("id"))
	})
	mux.HandleFunc("POST /api/playlists/{id}/add", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Paths []string `json:"paths"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || len(in.Paths) == 0 {
			http.Error(w, "paths required", http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		p := s.find(r.PathValue("id"))
		if p == nil {
			http.NotFound(w, r)
			return
		}
		if p.Smart != nil {
			http.Error(w, "a smart playlist picks its own tracks", http.StatusConflict)
			return
		}
		for _, path := range in.Paths {
			if !slices.Contains(p.Paths, path) {
				p.Paths = append(p.Paths, path)
			}
		}
		p.Updated = time.Now()
		if err := s.savePlaylistsLocked(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("DELETE /api/playlists/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		n := len(s.playlists)
		s.playlists = slices.DeleteFunc(s.playlists, func(p *playlist) bool { return p.ID == r.PathValue("id") })
		if len(s.playlists) == n {
			http.NotFound(w, r)
			return
		}
		if err := s.savePlaylistsLocked(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("POST /api/played", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Path string `json:"path"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "path required", http.StatusBadRequest)
			return
		}
		// Only what is in the library is counted, so the file can't be
		// grown with made-up paths.
		name, err := safeJoin(music, in.Path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if info, err := os.Stat(name); err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}
		if err := s.played(in.Path, time.Now()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/excluded", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, s.excludedKeys())
	})
	mux.HandleFunc("PUT /api/excluded", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Key      string `json:"key"`
			Excluded bool   `json:"excluded"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Key == "" || len(in.Key) > 1024 {
			http.Error(w, "key required", http.StatusBadRequest)
			return
		}
		if id, ok := strings.CutPrefix(in.Key, "yt:"); ok && !ytID.MatchString(id) {
			http.Error(w, "not a video id", http.StatusBadRequest)
			return
		}
		if err := s.setExcluded(in.Key, in.Excluded); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// loadJSON reads a saved file into v; a file not yet saved leaves v as
// it is.
func loadJSON(file string, v any) error {
	b, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) || (err == nil && len(b) == 0) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// saveJSON writes v through a temporary file, so a crash leaves the old
// copy or the new one, never half of either.
func saveJSON(file string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}
