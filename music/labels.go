package main

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// Labels say what the library sounds like, beyond the folders' names:
// its genres, its moods and its energy. They are kept for an artist and
// for an album, an album's own winning over its artist's, field by
// field; a track with no genre labelled goes by its file's genre tag.
// Smart playlists and the mixes go by them. They are the server's own
// record, so the files are never rewritten for them.

// moods are the moods a label may give, few enough that a mix of each
// has something in it.
var moods = []string{
	"aggressive", "chill", "dark", "dreamy", "epic", "euphoric",
	"melancholy", "playful", "romantic", "upbeat", "uplifting", "warm",
}

// energies are how hard a label says the music goes.
var energies = []string{"low", "medium", "high"}

// labels is what is said of an artist or an album. A field left empty
// says nothing, and the artist's (or the file's) is used instead.
type labels struct {
	Genres []string `json:"genres,omitempty"` // most telling first, as normalizeGenre has them
	Moods  []string `json:"moods,omitempty"`
	Energy string   `json:"energy,omitempty"`
}

func (l labels) empty() bool {
	return len(l.Genres) == 0 && len(l.Moods) == 0 && l.Energy == ""
}

// labelBook is the labels as labels.json keeps them: artists by name,
// albums by "Artist/Album".
type labelBook struct {
	Artists map[string]labels `json:"artists"`
	Albums  map[string]labels `json:"albums"`
}

func albumKey(artist, album string) string { return artist + "/" + album }

// clean checks a label as given and puts it the way it is kept.
func (l labels) clean() (labels, error) {
	var out labels
	for _, g := range l.Genres {
		if g = normalizeGenre(g); g != "" && !slices.Contains(out.Genres, g) {
			out.Genres = append(out.Genres, g)
		}
	}
	for _, m := range l.Moods {
		m = strings.ToLower(strings.TrimSpace(m))
		if !slices.Contains(moods, m) {
			return labels{}, fmt.Errorf("%q isn't a mood; the moods are %s", m, strings.Join(moods, ", "))
		}
		if !slices.Contains(out.Moods, m) {
			out.Moods = append(out.Moods, m)
		}
	}
	if e := strings.ToLower(strings.TrimSpace(l.Energy)); e != "" {
		if !slices.Contains(energies, e) {
			return labels{}, fmt.Errorf("%q isn't an energy; it is low, medium or high", l.Energy)
		}
		out.Energy = e
	}
	if len(out.Genres) > 6 || len(out.Moods) > 4 {
		return labels{}, errors.New("at most 6 genres and 4 moods")
	}
	return out, nil
}

func (s *shelf) saveLabelsLocked() error {
	return saveJSON(filepath.Join(s.dir, "labels.json"), s.labels)
}

// labelOf is what is said of an artist, or with an album, of the album
// alone.
func (s *shelf) labelOf(artist, album string) labels {
	s.mu.Lock()
	defer s.mu.Unlock()
	if album == "" {
		return s.labels.Artists[artist]
	}
	return s.labels.Albums[albumKey(artist, album)]
}

// setLabel replaces what is said of an artist, or with an album, of
// the album; an empty label forgets it.
func (s *shelf) setLabel(artist, album string, l labels) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	book := s.labels.Artists
	key := artist
	if album != "" {
		book, key = s.labels.Albums, albumKey(artist, album)
	}
	if l.empty() {
		delete(book, key)
	} else {
		book[key] = l
	}
	return s.saveLabelsLocked()
}

// enrich gives each track its year and its labels as they fall to it:
// the album's, else the artist's, else for the genre the file's tag.
func (s *shelf) enrich(music string, all []libTrack) {
	for i := range all {
		f := s.files.of(music, all[i].track)
		all[i].Duration, all[i].Year = f.secs, f.year
		all[i].fileGenre = normalizeGenre(f.genre)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range all {
		t := &all[i]
		a, al := s.labels.Artists[t.Artist], s.labels.Albums[albumKey(t.Artist, t.Album)]
		t.Genres = pick(al.Genres, a.Genres)
		if len(t.Genres) == 0 && t.fileGenre != "" {
			t.Genres = []string{t.fileGenre}
		}
		t.Moods = pick(al.Moods, a.Moods)
		t.Energy = cmp.Or(al.Energy, a.Energy)
		t.Plays = s.plays[t.Path].N
		t.Excluded = s.excluded[t.Path]
	}
}

func pick(a, b []string) []string {
	if len(a) > 0 {
		return a
	}
	return b
}

// movedLabelsLocked carries an album's and an artist's labels to where
// an edit moved their tracks, unless the new home has its own. The old
// ones are kept: a single track moving out doesn't unsay its album.
func (s *shelf) movedLabelsLocked(paths map[string]string) bool {
	changed := false
	for from, to := range paths {
		fa, fal := filedUnder(from)
		ta, tal := filedUnder(to)
		if fa != ta {
			if l, ok := s.labels.Artists[fa]; ok {
				if _, has := s.labels.Artists[ta]; !has {
					s.labels.Artists[ta] = l
					changed = true
				}
			}
		}
		if fk, tk := albumKey(fa, fal), albumKey(ta, tal); fk != tk {
			if l, ok := s.labels.Albums[fk]; ok {
				if _, has := s.labels.Albums[tk]; !has {
					s.labels.Albums[tk] = l
					changed = true
				}
			}
		}
	}
	return changed
}

// filedUnder is the artist and album a track's path files it under.
func filedUnder(p string) (artist, album string) {
	dir := path.Dir(p)
	artist, album, ok := strings.Cut(dir, "/")
	if !ok {
		return dir, singlesName
	}
	return artist, album
}

// labelsCopy is the book as it stands, for the MCP's listings.
func (s *shelf) labelsCopy() labelBook {
	s.mu.Lock()
	defer s.mu.Unlock()
	return labelBook{Artists: maps.Clone(s.labels.Artists), Albums: maps.Clone(s.labels.Albums)}
}
