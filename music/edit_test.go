package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// editFixture is a small library with plays, an exclusion and a
// playlist, served with the edits.
func editFixture(t *testing.T, files map[string]string) (music string, s *shelf, mux *http.ServeMux) {
	t.Helper()
	music, state := t.TempDir(), t.TempDir()
	for name, body := range files {
		p := filepath.Join(music, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s, err := newShelf(state)
	if err != nil {
		t.Fatal(err)
	}
	e, err := newEditor(music, state, s)
	if err != nil {
		t.Fatal(err)
	}
	mux = http.NewServeMux()
	routeEdit(mux, e)
	return music, s, mux
}

func edit(t *testing.T, mux *http.ServeMux, req editRequest, dry bool) (*httptest.ResponseRecorder, editPlan) {
	t.Helper()
	b, _ := json.Marshal(req)
	target := "/api/edit"
	if dry {
		target += "?dry=1"
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodPost, target, bytes.NewReader(b)))
	var p editPlan
	if w.Code == http.StatusOK {
		_ = json.Unmarshal(w.Body.Bytes(), &p)
	}
	return w, p
}

func moves(t *testing.T, mux *http.ServeMux, since int) (int, []moveRecord) {
	t.Helper()
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/moves?since="+strconv.Itoa(since), nil))
	var out struct {
		Seq   int          `json:"seq"`
		Moves []moveRecord `json:"moves"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Seq, out.Moves
}

func exists(music, rel string) bool {
	_, err := os.Lstat(filepath.Join(music, filepath.FromSlash(rel)))
	return err == nil
}

func TestEditAlbum(t *testing.T) {
	music, s, mux := editFixture(t, map[string]string{
		"Kanye West/MBDTF/01 Dark Fantasy.mp3": "one",
		"Kanye West/MBDTF/cover.jpg":           "art",
		"Kanye West/Yeezus/01 On Sight.mp3":    "two",
	})
	if err := s.played("Kanye West/MBDTF/01 Dark Fantasy.mp3", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := s.setExcluded("Kanye West/MBDTF/01 Dark Fantasy.mp3", true); err != nil {
		t.Fatal(err)
	}
	s.playlists = []*playlist{{ID: "p", Name: "P", Paths: []string{"Kanye West/Yeezus/01 On Sight.mp3", "Kanye West/MBDTF/01 Dark Fantasy.mp3"}}}

	req := editRequest{Artist: "Kanye West", Album: "MBDTF", To: "My Beautiful Dark Twisted Fantasy"}
	w, p := edit(t, mux, req, true)
	if w.Code != http.StatusOK || p.Tracks != 1 || p.Merge {
		t.Fatalf("dry run: %d %s %+v", w.Code, w.Body, p)
	}
	if !exists(music, "Kanye West/MBDTF") {
		t.Fatal("a dry run moved the album")
	}
	if w, _ := edit(t, mux, req, false); w.Code != http.StatusOK {
		t.Fatalf("edit: %d %s", w.Code, w.Body)
	}
	to := "Kanye West/My Beautiful Dark Twisted Fantasy/01 Dark Fantasy.mp3"
	if exists(music, "Kanye West/MBDTF") || !exists(music, to) || !exists(music, "Kanye West/My Beautiful Dark Twisted Fantasy/cover.jpg") {
		t.Error("the folder didn't move whole")
	}
	if s.playsOf(to).N != 1 || !s.isExcluded(to) || s.isExcluded("Kanye West/MBDTF/01 Dark Fantasy.mp3") {
		t.Error("plays and the exclusion didn't follow the track")
	}
	if got := s.playlists[0].Paths; got[1] != to {
		t.Errorf("the playlist has %q", got)
	}
	seq, ms := moves(t, mux, 0)
	want := []moveRecord{
		{Seq: 1, From: "Kanye West/MBDTF/01 Dark Fantasy.mp3", To: to},
		{Seq: 2, From: "Kanye West/MBDTF", To: "Kanye West/My Beautiful Dark Twisted Fantasy", Album: true},
	}
	if seq != 2 || !slices.Equal(ms, want) {
		t.Errorf("moves = %d %+v", seq, ms)
	}
	if _, ms := moves(t, mux, 1); len(ms) != 1 || ms[0].Seq != 2 {
		t.Errorf("moves since 1 = %+v", ms)
	}

	// The shelf and the log are as saved.
	again, err := newShelf(s.dir)
	if err != nil || again.playsOf(to).N != 1 {
		t.Errorf("reloaded plays: %v", err)
	}
}

func TestEditMerge(t *testing.T) {
	music, s, mux := editFixture(t, map[string]string{
		"Chvrches/Bones/01 Clearest Blue.mp3":  "same",
		"Chvrches/Bones/02 Leave a Trace.mp3":  "only here",
		"Chvrches/Bones/cover.jpg":             "old art",
		"Chvrches/Stray.mp3":                   "loose",
		"CHVRCHES/Bones/01 Clearest Blue.mp3":  "same",
		"CHVRCHES/Bones/cover.jpg":             "new art",
		"CHVRCHES/Other/01 Get Away.mp3":       "x",
		"Linkin Park/Meteora/01 Foreword.mp3":  "a",
		"Linkin_Park/Meteora/01 Foreword.mp3":  "different",
		"Linkin_Park/Meteora/02 Dont Stay.mp3": "b",
	})
	_ = s.played("Chvrches/Bones/01 Clearest Blue.mp3", time.Now())
	_ = s.played("CHVRCHES/Bones/01 Clearest Blue.mp3", time.Now())

	w, p := edit(t, mux, editRequest{Artist: "Chvrches", To: "CHVRCHES"}, false)
	if w.Code != http.StatusOK || !p.Merge || len(p.Duplicates) != 1 {
		t.Fatalf("merge: %d %s %+v", w.Code, w.Body, p)
	}
	if exists(music, "Chvrches") {
		t.Error("the emptied artist is still there")
	}
	for _, f := range []string{"CHVRCHES/Bones/02 Leave a Trace.mp3", "CHVRCHES/Stray.mp3", "CHVRCHES/Other/01 Get Away.mp3"} {
		if !exists(music, f) {
			t.Errorf("%s is missing", f)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(music, "CHVRCHES/Bones/cover.jpg")); string(b) != "new art" {
		t.Errorf("the art there was replaced: %q", b)
	}
	trash, _ := filepath.Glob(filepath.Join(music, trashDir, "*", "Chvrches", "Bones", "01 Clearest Blue.mp3"))
	if len(trash) != 1 {
		t.Error("the duplicate isn't in the trash")
	}
	if n := s.playsOf("CHVRCHES/Bones/01 Clearest Blue.mp3").N; n != 2 {
		t.Errorf("the kept copy has %d plays, want both", n)
	}
	_, ms := moves(t, mux, 0)
	var albums []string
	for _, m := range ms {
		if m.Album {
			albums = append(albums, m.From+">"+m.To)
		}
	}
	slices.Sort(albums)
	if want := []string{"Chvrches/Bones>CHVRCHES/Bones", "Chvrches/Singles>CHVRCHES/Singles"}; !slices.Equal(albums, want) {
		t.Errorf("album moves = %q", albums)
	}
	// The trash isn't the library's.
	artists, _ := walkLibrary(music)
	for _, a := range artists {
		if strings.HasPrefix(a.Name, ".") {
			t.Errorf("the library lists %q", a.Name)
		}
	}

	// Two different tracks of one name stop the merge before anything
	// moves.
	w, p = edit(t, mux, editRequest{Artist: "Linkin_Park", To: "Linkin Park"}, true)
	if w.Code != http.StatusOK || len(p.Conflicts) != 1 {
		t.Fatalf("dry run: %d %+v", w.Code, p)
	}
	if w, _ := edit(t, mux, editRequest{Artist: "Linkin_Park", To: "Linkin Park"}, false); w.Code != http.StatusConflict {
		t.Errorf("a conflicting merge answered %d", w.Code)
	}
	if !exists(music, "Linkin_Park/Meteora/02 Dont Stay.mp3") {
		t.Error("a refused merge moved something")
	}
}

func TestEditTrack(t *testing.T) {
	music, s, mux := editFixture(t, map[string]string{
		"Kendrick Lamar/Section_80/01 Fu.mp3": "a",
		"Kendrick Lamar/Section_80/02 B.mp3":  "b",
		"Kendrick Lamar/Loose.mp3":            "c",
	})
	s.playlists = []*playlist{{ID: "p", Paths: []string{"Kendrick Lamar/Section_80/01 Fu.mp3"}}}

	if w, _ := edit(t, mux, editRequest{Track: "Kendrick Lamar/Section_80/01 Fu.mp3", To: "01 Fuck Your Ethnicity"}, false); w.Code != http.StatusOK {
		t.Fatalf("rename: %d %s", w.Code, w.Body)
	}
	if !exists(music, "Kendrick Lamar/Section_80/01 Fuck Your Ethnicity.mp3") {
		t.Error("the track wasn't renamed")
	}
	if s.playlists[0].Paths[0] != "Kendrick Lamar/Section_80/01 Fuck Your Ethnicity.mp3" {
		t.Errorf("the playlist has %q", s.playlists[0].Paths)
	}
	// A loose track into an album, and a track out of one.
	if w, _ := edit(t, mux, editRequest{Track: "Kendrick Lamar/Loose.mp3", To: "03 Loose", ToAlbum: "Section_80"}, false); w.Code != http.StatusOK {
		t.Fatalf("into an album: %d %s", w.Code, w.Body)
	}
	if w, _ := edit(t, mux, editRequest{Track: "Kendrick Lamar/Section_80/02 B.mp3", To: "B", ToArtist: "Someone"}, false); w.Code != http.StatusOK {
		t.Fatalf("to another artist: %d %s", w.Code, w.Body)
	}
	if !exists(music, "Kendrick Lamar/Section_80/03 Loose.mp3") || !exists(music, "Someone/B.mp3") {
		t.Error("the tracks didn't land")
	}
	// Onto a track that is there already.
	if w, _ := edit(t, mux, editRequest{Track: "Kendrick Lamar/Section_80/03 Loose.mp3", To: "01 Fuck Your Ethnicity"}, false); w.Code != http.StatusConflict {
		t.Errorf("onto another track: %d", w.Code)
	}

	for _, bad := range []editRequest{
		{Track: "Kendrick Lamar/Section_80/03 Loose.mp3", To: "../x"},
		{Track: "Kendrick Lamar/Section_80/03 Loose.mp3", To: ".hidden"},
		{Track: "Kendrick Lamar/Section_80/03 Loose.mp3", To: ""},
		{Track: "../etc/passwd", To: "x"},
		{Track: "Kendrick Lamar/Section_80", To: "x"},
		{Artist: "Kendrick Lamar", Album: "Section_80", To: "Singles"},
		{Artist: "Kendrick Lamar", Album: "Nope", To: "x"},
		{Artist: "Kendrick Lamar", To: "Kendrick Lamar/Inside"},
		{Artist: "..", To: "x"},
	} {
		if w, _ := edit(t, mux, bad, false); w.Code != http.StatusConflict {
			t.Errorf("%+v answered %d %s", bad, w.Code, w.Body)
		}
	}
}
