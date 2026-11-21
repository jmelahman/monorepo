package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// library writes a library of empty tracks at the given paths.
func library(t *testing.T, paths ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, p := range paths {
		f := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func names(ts []libTrack) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Name
	}
	return out
}

func TestSmartPlaylistRules(t *testing.T) {
	root := library(t, "Air/Moon Safari/01 La Femme.mp3", "Air/Talkie/02 Cherry.mp3", "Beck/Sea/01 Golden.mp3")
	old := time.Now().AddDate(0, 0, -40)
	if err := os.Chtimes(filepath.Join(root, "Air/Talkie/02 Cherry.mp3"), old, old); err != nil {
		t.Fatal(err)
	}
	artists, err := walkLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	all := flatten(artists)
	s, err := newShelf(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for range 3 {
		if err := s.played("Beck/Sea/01 Golden.mp3", now); err != nil {
			t.Fatal(err)
		}
	}

	for _, tc := range []struct {
		sm   smart
		want []string
	}{
		{smart{Rules: []rule{{"artist", "is", "air"}}}, []string{"01 La Femme", "02 Cherry"}},
		{smart{Rules: []rule{{"artist", "is", "Air"}, {"added", "within", "30"}}}, []string{"01 La Femme"}},
		{smart{Match: "any", Rules: []rule{{"album", "starts", "tal"}, {"plays", ">", "2"}}}, []string{"02 Cherry", "01 Golden"}},
		{smart{Rules: []rule{{"played", "before", "7"}}}, []string{"01 La Femme", "02 Cherry"}},
		{smart{Sort: "plays", Limit: 1}, []string{"01 Golden"}},
		{smart{Rules: []rule{{"title", "not", "o"}}, Sort: "added"}, []string{"01 La Femme", "02 Cherry"}},
	} {
		p := &playlist{ID: "p", Name: "x", Smart: &tc.sm}
		if err := p.check(); err != nil {
			t.Fatal(err)
		}
		if got := names(s.resolve(p, all, now)); !slices.Equal(got, tc.want) {
			t.Errorf("%+v: got %q, want %q", tc.sm, got, tc.want)
		}
	}
}

func TestPlaylistCheck(t *testing.T) {
	for _, p := range []playlist{
		{Name: " "},
		{Name: "x", Smart: &smart{Rules: []rule{{"mood", "is", "x"}}}},
		{Name: "x", Smart: &smart{Rules: []rule{{"plays", "contains", "1"}}}},
		{Name: "x", Smart: &smart{Rules: []rule{{"added", "within", "soon"}}}},
		{Name: "x", Smart: &smart{Sort: "loudest"}},
		{Name: "x", Smart: &smart{Limit: -1}},
	} {
		if err := p.check(); err == nil {
			t.Errorf("%+v passed the check", p)
		}
	}
}

func TestLibraryMix(t *testing.T) {
	paths := make([]string, 0, 150)
	for a := range 30 {
		for n := range 5 {
			paths = append(paths, fmt.Sprintf("Artist %d/Album/%02d Song.mp3", a, n))
		}
	}
	root := library(t, paths...)
	artists, err := walkLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	all := flatten(artists)
	s, err := newShelf(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for n := range 5 {
		if err := s.setExcluded(fmt.Sprintf("Artist 0/Album/%02d Song.mp3", n), true); err != nil {
			t.Fatal(err)
		}
	}
	m := &mixer{music: root, shelf: s}
	now := time.Now()
	mix := m.library(all, now)
	if len(mix) != mixLength {
		t.Fatalf("want %d tracks, got %d", mixLength, len(mix))
	}
	per := map[string]int{}
	for i, tr := range mix {
		if tr.Artist == "Artist 0" {
			t.Fatalf("an excluded track made the mix: %s", tr.Path)
		}
		if per[tr.Artist]++; per[tr.Artist] > mixPerArtist {
			t.Fatalf("%s has more than %d tracks", tr.Artist, mixPerArtist)
		}
		if i > 0 && mix[i-1].Artist == tr.Artist {
			t.Fatalf("%s twice in a row at %d", tr.Artist, i)
		}
	}
	// The same all day: a phone keeping it doesn't churn.
	if again := m.library(all, now); !slices.Equal(names(again), names(mix)) {
		t.Fatal("the mix changed within the day")
	}
}

func TestShelfRoutes(t *testing.T) {
	root := library(t, "A/B/01 One.mp3", "A/B/02 Two.mp3")
	state := t.TempDir()
	s, err := newShelf(state)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	routeShelf(mux, s, root)
	routeLibrary(mux, root, s)
	do := func(method, url, body string, want int) []byte {
		t.Helper()
		r := httptest.NewRequest(method, url, strings.NewReader(body))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: %d %s, want %d", method, url, w.Code, w.Body, want)
		}
		return w.Body.Bytes()
	}

	var p playlistView
	if err := json.Unmarshal(do("POST", "/api/playlists", `{"name":"Mine","paths":["A/B/02 Two.mp3"]}`, 200), &p); err != nil {
		t.Fatal(err)
	}
	do("POST", "/api/playlists/"+p.ID+"/add", `{"paths":["A/B/01 One.mp3","A/B/02 Two.mp3"]}`, 204)
	if err := json.Unmarshal(do("GET", "/api/playlists/"+p.ID, "", 200), &p); err != nil {
		t.Fatal(err)
	}
	if got := names(p.Tracks); !slices.Equal(got, []string{"02 Two", "01 One"}) {
		t.Fatalf("tracks %q", got)
	}
	do("POST", "/api/playlists", `{"name":"Bad","smart":{"rules":[{"field":"mood"}]}}`, 400)

	do("POST", "/api/played", `{"path":"A/B/01 One.mp3"}`, 204)
	do("POST", "/api/played", `{"path":"A/B/nothing.mp3"}`, 404)
	do("PUT", "/api/excluded", `{"key":"A/B/02 Two.mp3","excluded":true}`, 204)
	do("PUT", "/api/excluded", `{"key":"yt:not-an-id","excluded":true}`, 400)
	var artists []artist
	if err := json.Unmarshal(do("GET", "/api/library", "", 200), &artists); err != nil {
		t.Fatal(err)
	}
	if ts := artists[0].Albums[0].Tracks; ts[0].Plays != 1 || !ts[1].Excluded {
		t.Fatalf("library %+v", ts)
	}

	// All of it survives a restart.
	again, err := newShelf(state)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.playlists) != 1 || again.plays["A/B/01 One.mp3"].N != 1 || !again.excluded["A/B/02 Two.mp3"] {
		t.Fatalf("not saved: %+v", again)
	}
	do("DELETE", "/api/playlists/"+p.ID, "", 204)
	do("GET", "/api/playlists/"+p.ID, "", 404)
}

// The supermix is offered only once YouTube Music is set up; the compose
// file mounts an empty cookies file until it is.
func TestSupermixNeedsTheAccount(t *testing.T) {
	cookies := filepath.Join(t.TempDir(), "cookies.txt")
	if err := os.WriteFile(cookies, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	m := &mixer{cookies: cookies, supermix: "RDTMAK"}
	mux := http.NewServeMux()
	routeMixes(mux, m)
	offered := func() bool {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/mix", nil))
		var got struct{ Supermix bool }
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		return got.Supermix
	}
	if offered() {
		t.Fatal("offered with an empty cookies file")
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/yt/supermix", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404 without the account, got %d", rec.Code)
	}
	if err := os.WriteFile(cookies, []byte("# Netscape HTTP Cookie File\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !offered() {
		t.Fatal("not offered with the account set up")
	}
	m.supermix = ""
	if offered() {
		t.Fatal("offered without a playlist id")
	}
}
