package main

import (
	"cmp"
	"math"
	"net/http"
	"slices"
	"strings"
	"time"
)

// The mixes made for you: a genre's, a mood's, and a mood's within a
// genre, from the labels and the files' genre tags. Each draws as the
// library mix does, from its own tracks. Which ones the home page shows
// turns over daily, leaning to what is played most.

// madeMin is the fewest tracks, and madeArtists the fewest artists, a
// made mix wants: fewer would play the same few songs.
const (
	madeMin     = 30
	madeArtists = 3
	madeShown   = 8
)

// madeMix is a mix as the home page lists it.
type madeMix struct {
	ID     string      `json:"id"` // genre:G, mood:M or mood:M+genre:G
	Title  string      `json:"title"`
	Tracks int         `json:"tracks"`
	Covers [][2]string `json:"covers"` // [artist, album] of a few of its albums
	Blurb  string      `json:"blurb"`  // a few of its artists

	plays int
}

// madeID parts an id into the mood and genre it wants; ok is false for
// one that isn't an id.
func madeID(id string) (mood, genre string, ok bool) {
	for part := range strings.SplitSeq(id, "+") {
		k, v, found := strings.Cut(part, ":")
		switch {
		case !found || v == "":
			return "", "", false
		case k == "mood" && mood == "":
			mood = v
		case k == "genre" && genre == "":
			genre = v
		default:
			return "", "", false
		}
	}
	return mood, genre, mood != "" || genre != ""
}

// madePerArtist is how many of one artist a made mix may hold: as the
// library mix's, unless a mix of few artists would come up short.
func madePerArtist(pool []libTrack, size int) int {
	artists := map[string]bool{}
	for _, t := range pool {
		artists[t.Artist] = true
	}
	return max(mixPerArtist, mixPerArtist*size/mixLength, (size+len(artists)-1)/len(artists)*3/2)
}

func madeTitle(mood, genre string) string {
	var words []string
	if mood != "" {
		words = append(words, genreTitle(mood))
	}
	if genre != "" {
		words = append(words, genreTitle(genre))
	}
	return strings.Join(words, " ") + " Mix"
}

// madeTracks are the tracks a mix draws from, the excluded left out.
func (m *mixer) madeTracks(all []libTrack, mood, genre string) []libTrack {
	var out []libTrack
	for _, t := range all {
		if !t.Excluded && (mood == "" || slices.Contains(t.Moods, mood)) && (genre == "" || slices.Contains(t.Genres, genre)) {
			out = append(out, t)
		}
	}
	return out
}

// made lists the mixes the library can make today, the most fitting
// first, madeShown of them.
func (m *mixer) made(all []libTrack, now time.Time) []madeMix {
	type group struct {
		tracks  []libTrack
		artists map[string]int
	}
	groups := map[string]*group{}
	add := func(id string, t libTrack) {
		g := groups[id]
		if g == nil {
			g = &group{artists: map[string]int{}}
			groups[id] = g
		}
		g.tracks = append(g.tracks, t)
		g.artists[t.Artist]++
	}
	for _, t := range all {
		if t.Excluded {
			continue
		}
		for _, g := range t.Genres {
			add("genre:"+g, t)
		}
		for _, md := range t.Moods {
			add("mood:"+md, t)
			for _, g := range t.Genres[:min(2, len(t.Genres))] {
				add("mood:"+md+"+genre:"+g, t)
			}
		}
	}

	var out []madeMix
	for id, g := range groups {
		if len(g.tracks) < madeMin || len(g.artists) < madeArtists {
			continue
		}
		// A mix nearly the whole library says nothing.
		if len(g.tracks) > len(all)*2/3 {
			continue
		}
		mood, genre, _ := madeID(id)
		mm := madeMix{ID: id, Title: madeTitle(mood, genre), Tracks: len(g.tracks)}
		artists := make([]string, 0, len(g.artists))
		for a := range g.artists {
			artists = append(artists, a)
		}
		for _, t := range g.tracks {
			mm.plays += t.Plays
		}
		slices.SortFunc(artists, func(a, b string) int {
			return cmp.Or(cmp.Compare(g.artists[b], g.artists[a]), strings.Compare(a, b))
		})
		mm.Blurb = strings.Join(artists[:min(3, len(artists))], ", ")
		seen := map[string]bool{}
		for _, t := range m.draw("made/"+id, g.tracks, now, 12, 3) {
			if k := albumKey(t.Artist, t.Album); !seen[k] && len(mm.Covers) < 4 {
				seen[k] = true
				mm.Covers = append(mm.Covers, [2]string{t.Artist, t.Album})
			}
		}
		out = append(out, mm)
	}

	// Today's: a weighted draw, a mix's tracks' plays making it likelier,
	// with a mood-in-a-genre mix first where one fits, as those say most.
	slices.SortFunc(out, func(a, b madeMix) int { return strings.Compare(a.ID, b.ID) }) // the day's draw, whatever the map's order
	rng := dailyRand("made-mixes", now)
	keys := map[string]float64{}
	for _, mm := range out {
		w := 1 + math.Log1p(float64(mm.plays))
		if strings.Contains(mm.ID, "+") {
			w *= 1.5
		}
		keys[mm.ID] = math.Pow(rng.Float64(), 1/w)
	}
	slices.SortFunc(out, func(a, b madeMix) int { return cmpFloat(keys[b.ID], keys[a.ID]) })
	// No two of one genre or one mood in a row of the shelf's few.
	var shown []madeMix
	used := map[string]int{}
	for _, mm := range out {
		if len(shown) == madeShown {
			break
		}
		parts := strings.Split(mm.ID, "+")
		if slices.ContainsFunc(parts, func(p string) bool { return used[p] >= 2 }) {
			continue
		}
		for _, p := range parts {
			used[p]++
		}
		shown = append(shown, mm)
	}
	return shown
}

func routeMade(mux *http.ServeMux, m *mixer) {
	library := func(w http.ResponseWriter) ([]libTrack, bool) {
		artists, err := walkLibrary(m.music)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return nil, false
		}
		all := flatten(artists)
		m.shelf.enrich(m.music, all)
		return all, true
	}
	mux.HandleFunc("GET /api/mixes", func(w http.ResponseWriter, _ *http.Request) {
		all, ok := library(w)
		if !ok {
			return
		}
		writeJSON(w, m.made(all, time.Now()))
	})
	// A made mix's tracks, any the library can make and not only those
	// the home page shows today.
	mux.HandleFunc("GET /api/mixes/{id}", func(w http.ResponseWriter, r *http.Request) {
		mood, genre, ok := madeID(r.PathValue("id"))
		if !ok {
			http.Error(w, "no such mix", http.StatusNotFound)
			return
		}
		all, ok := library(w)
		if !ok {
			return
		}
		pool := m.madeTracks(all, mood, genre)
		if len(pool) == 0 {
			http.Error(w, "no such mix", http.StatusNotFound)
			return
		}
		size := mixSize(r)
		writeJSON(w, struct {
			Title  string     `json:"title"`
			Tracks []libTrack `json:"tracks"`
		}{madeTitle(mood, genre), m.draw("made/"+r.PathValue("id"), pool, time.Now(), size, madePerArtist(pool, size))})
	})
}
