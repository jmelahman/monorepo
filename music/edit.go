package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"
)

// The library's names are its folders' and files' names, and everything
// that remembers a track remembers its path: plays, the tracks kept out
// of the mixes, playlists, and the copies the phone and the browser
// keep. An edit renames on disk, carries the server's own records over,
// and notes each move in a log the clients replay to move their copies.
//
// An album or artist renamed onto one that already exists is merged into
// it. A track the target already has, the same size, is a duplicate: the
// moving copy goes to the library's .trash, so nothing is lost. Two
// different tracks of one name stop the edit before anything moves.

// trashDir is where an edit sets duplicates aside, within the library so
// moving there is a rename. walkLibrary skips folders that start with a
// dot.
const trashDir = ".trash"

// moveRecord is one entry of the log: a track's path, or with Album an
// album's "Artist/Album" (as the phone keeps albums), from and to.
type moveRecord struct {
	Seq   int    `json:"seq"`
	From  string `json:"from"`
	To    string `json:"to"`
	Album bool   `json:"album,omitempty"`
}

type editor struct {
	music string
	shelf *shelf
	file  string // the log

	editing sync.Mutex // one edit at a time, from its plan to its end
	mu      sync.Mutex // the log
	moves   []moveRecord
}

func newEditor(music, state string, shelf *shelf) (*editor, error) {
	e := &editor{music: music, shelf: shelf, file: filepath.Join(state, "moves.json")}
	if err := loadJSON(e.file, &e.moves); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *editor) since(seq int) (int, []moveRecord) {
	e.mu.Lock()
	defer e.mu.Unlock()
	i, _ := slices.BinarySearchFunc(e.moves, seq+1, func(m moveRecord, s int) int { return m.Seq - s })
	last := 0
	if n := len(e.moves); n > 0 {
		last = e.moves[n-1].Seq
	}
	return last, slices.Clone(e.moves[i:])
}

// editRequest is one edit. Exactly one of Track, Album or Artist names
// what is edited:
//   - Track, a track's path: To names its new file name, less the
//     extension, and ToArtist and ToAlbum where it goes ("" keeps it
//     where it is; ToAlbum "Singles" or "" with a new ToArtist is loose).
//   - Album with Artist: the album's folder, renamed to To and moved to
//     ToArtist when given.
//   - Artist alone: the artist's folder, renamed to To.
type editRequest struct {
	Track    string `json:"track,omitempty"`
	Artist   string `json:"artist,omitempty"`
	Album    string `json:"album,omitempty"`
	To       string `json:"to"`
	ToArtist string `json:"toArtist,omitempty"`
	ToAlbum  string `json:"toAlbum,omitempty"`
}

// editPlan is what an edit will do, shown before it is done.
type editPlan struct {
	From       string   `json:"from"` // the folder or file, relative to the library
	To         string   `json:"to"`
	Merge      bool     `json:"merge,omitempty"` // into a folder that is already there
	Tracks     int      `json:"tracks"`
	Duplicates []string `json:"duplicates,omitempty"` // set aside in .trash
	Conflicts  []string `json:"conflicts,omitempty"`  // stop the edit

	files  []fileMove
	albums [][2]string
}

type fileMove struct {
	from, to string // relative, slash-separated
	dup      bool
}

var errEdit = errors.New("edit refused")

type editError struct{ msg string }

func (e editError) Error() string { return e.msg }
func (e editError) Unwrap() error { return errEdit }

func refuse(format string, a ...any) error { return editError{fmt.Sprintf(format, a...)} }

// validName is a name one folder or file can take.
func validName(s string) error {
	switch {
	case s == "":
		return refuse("a name can't be empty")
	case s == "." || s == ".." || strings.HasPrefix(s, "."):
		return refuse("a name can't start with a dot")
	case strings.ContainsAny(s, "/\\"):
		return refuse("a name can't have a slash in it")
	case strings.IndexFunc(s, unicode.IsControl) >= 0:
		return refuse("a name can't have tabs or other control characters in it")
	case len(s) > 240:
		return refuse("that name is too long")
	case s == singlesName:
		return refuse("%q is the name the loose tracks go under", singlesName)
	}
	return nil
}

func (e *editor) abs(rel string) string { return filepath.Join(e.music, filepath.FromSlash(rel)) }

// plan works out an edit without doing it.
func (e *editor) plan(r editRequest) (*editPlan, error) {
	r.To = strings.TrimSpace(r.To)
	r.ToArtist = strings.TrimSpace(r.ToArtist)
	r.ToAlbum = strings.TrimSpace(r.ToAlbum)
	switch {
	case r.Track != "":
		return e.planTrack(r)
	case r.Album != "":
		if r.Album == singlesName && !isDir(e.abs(path.Join(r.Artist, r.Album))) {
			return nil, refuse("the Singles are loose tracks, not a folder; edit them one at a time")
		}
		if err := validName(r.To); err != nil {
			return nil, err
		}
		to := r.Artist
		if r.ToArtist != "" {
			if err := validName(r.ToArtist); err != nil {
				return nil, err
			}
			to = r.ToArtist
		}
		return e.planFolder(path.Join(r.Artist, r.Album), path.Join(to, r.To))
	case r.Artist != "":
		if err := validName(r.To); err != nil {
			return nil, err
		}
		return e.planFolder(r.Artist, r.To)
	}
	return nil, refuse("nothing to edit")
}

// checkRel is a library path that names something already there.
func (e *editor) checkRel(rel string) error {
	if rel == "" || path.Clean(rel) != rel || strings.HasPrefix(rel, "../") || rel == ".." || strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, ".") {
		return refuse("no such thing in the library")
	}
	if _, err := os.Lstat(e.abs(rel)); err != nil {
		return refuse("no such thing in the library")
	}
	return nil
}

func (e *editor) planTrack(r editRequest) (*editPlan, error) {
	if err := e.checkRel(r.Track); err != nil || !isAudio(r.Track) {
		return nil, refuse("no such track")
	}
	parts := strings.Split(r.Track, "/")
	if len(parts) < 2 || len(parts) > 3 {
		return nil, refuse("no such track")
	}
	artist, album := parts[0], ""
	if len(parts) == 3 {
		album = parts[1]
	}
	if r.ToArtist != "" {
		if err := validName(r.ToArtist); err != nil {
			return nil, err
		}
		artist = r.ToArtist
		album = r.ToAlbum
	} else if r.ToAlbum != "" {
		album = r.ToAlbum
	}
	if album == singlesName {
		album = ""
	}
	if album != "" {
		if err := validName(album); err != nil {
			return nil, err
		}
	}
	if err := validName(r.To); err != nil {
		return nil, err
	}
	name := r.To + path.Ext(r.Track)
	to := path.Join(artist, album, name)
	p := &editPlan{From: r.Track, To: to, Tracks: 1}
	if to == r.Track {
		return p, nil
	}
	if _, err := os.Lstat(e.abs(to)); err == nil && !sameFile(e.abs(r.Track), e.abs(to)) {
		p.Conflicts = append(p.Conflicts, to)
	}
	p.files = []fileMove{{from: r.Track, to: to}}
	return p, nil
}

// sameFile is whether two paths are one file, as a case-only rename's
// are on a filesystem that ignores case.
func sameFile(a, b string) bool {
	x, err1 := os.Stat(a)
	y, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(x, y)
}

// planFolder moves the folder from (an artist's or an album's) to to,
// merging when to is already there.
func (e *editor) planFolder(from, to string) (*editPlan, error) {
	if err := e.checkRel(from); err != nil || !isDir(e.abs(from)) {
		return nil, refuse("no such folder")
	}
	p := &editPlan{From: from, To: to}
	if from == to {
		return p, nil
	}
	if strings.HasPrefix(to+"/", from+"/") {
		return nil, refuse("a folder can't go inside itself")
	}
	p.Merge = isDir(e.abs(to)) && !sameFile(e.abs(from), e.abs(to))
	if _, err := os.Lstat(e.abs(to)); err == nil && !p.Merge && !sameFile(e.abs(from), e.abs(to)) {
		return nil, refuse("%s is a file, not a folder", to)
	}
	err := filepath.WalkDir(e.abs(from), func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(e.music, name)
		rel = filepath.ToSlash(rel)
		dst := to + strings.TrimPrefix(rel, from)
		m := fileMove{from: rel, to: dst}
		if isAudio(rel) {
			p.Tracks++
		}
		if p.Merge {
			if there, err := os.Stat(e.abs(dst)); err == nil {
				here, _ := d.Info()
				switch {
				case !isAudio(rel):
					m.dup = true // the folder there keeps its own cover and the like
				case here != nil && here.Size() == there.Size():
					m.dup = true
					p.Duplicates = append(p.Duplicates, rel)
				default:
					p.Conflicts = append(p.Conflicts, dst)
				}
			}
		}
		p.files = append(p.files, m)
		return nil
	})
	if err != nil {
		return nil, err
	}
	// The albums the phone may keep, by the names it keeps them under.
	depth := strings.Count(from, "/")
	if depth == 1 {
		p.albums = append(p.albums, [2]string{from, to})
	} else {
		entries, _ := os.ReadDir(e.abs(from))
		loose := false
		for _, en := range entries {
			if en.IsDir() && !strings.HasPrefix(en.Name(), ".") {
				p.albums = append(p.albums, [2]string{path.Join(from, en.Name()), path.Join(to, en.Name())})
			} else if isAudio(en.Name()) {
				loose = true
			}
		}
		if loose {
			p.albums = append(p.albums, [2]string{path.Join(from, singlesName), path.Join(to, singlesName)})
		}
	}
	return p, nil
}

// apply does a planned edit: the files, then the server's records, then
// the log.
func (e *editor) apply(p *editPlan) error {
	if len(p.Conflicts) > 0 {
		return refuse("%s already has a different track of the same name", strings.Join(p.Conflicts, ", "))
	}
	if len(p.files) == 0 && len(p.albums) == 0 {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if p.Merge || len(p.files) == 1 && p.From == p.files[0].from {
		if err := e.moveFiles(p); err != nil {
			return err
		}
	} else {
		// The whole folder goes in one rename.
		if err := e.mkdirs(path.Dir(p.To)); err != nil {
			return err
		}
		if err := os.Rename(e.abs(p.From), e.abs(p.To)); err != nil {
			return err
		}
	}
	e.prune(path.Dir(p.From))
	// The art found for the folders that moved is looked for afresh.
	covers.Lock()
	for dir := range covers.from {
		if dir == e.abs(p.From) || strings.HasPrefix(dir, e.abs(p.From)+string(filepath.Separator)) || dir == e.abs(path.Dir(p.From)) {
			delete(covers.from, dir)
		}
	}
	covers.Unlock()

	moved := map[string]string{}
	seq := 0
	if n := len(e.moves); n > 0 {
		seq = e.moves[n-1].Seq
	}
	for _, m := range p.files {
		if isAudio(m.from) {
			moved[m.from] = m.to
			seq++
			e.moves = append(e.moves, moveRecord{Seq: seq, From: m.from, To: m.to})
		}
	}
	for _, a := range p.albums {
		seq++
		e.moves = append(e.moves, moveRecord{Seq: seq, From: a[0], To: a[1], Album: true})
	}
	if err := e.shelf.moved(moved); err != nil {
		return err
	}
	return saveJSON(e.file, e.moves)
}

// moveFiles moves a plan's files one at a time, duplicates to the trash,
// then clears the folders they leave empty.
func (e *editor) moveFiles(p *editPlan) error {
	trash := path.Join(trashDir, time.Now().UTC().Format("20060102-150405"))
	for _, m := range p.files {
		dst := m.to
		if m.dup {
			dst = path.Join(trash, m.from)
		}
		if err := e.mkdirs(path.Dir(dst)); err != nil {
			return err
		}
		if err := os.Rename(e.abs(m.from), e.abs(dst)); err != nil {
			return err
		}
	}
	if isDir(e.abs(p.From)) {
		e.clear(p.From)
	}
	return nil
}

// clear removes the folders an edit emptied under rel, then rel and its
// parents while they are empty. Only empty folders go.
func (e *editor) clear(rel string) {
	var dirs []string
	_ = filepath.WalkDir(e.abs(rel), func(name string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			dirs = append(dirs, name)
		}
		return nil
	})
	for _, d := range slices.Backward(dirs) {
		_ = os.Remove(d)
	}
	e.prune(rel)
}

// prune removes rel and its parents while they are empty, up to the
// library itself; it leaves anything else in them alone.
func (e *editor) prune(rel string) {
	for rel != "." && rel != "" {
		if os.Remove(e.abs(rel)) != nil {
			return
		}
		rel = path.Dir(rel)
	}
}

// mkdirs makes a folder in the library and any it needs, each owned as
// its parent is, since the server may run as another user than the one
// whose library it is.
func (e *editor) mkdirs(rel string) error {
	if rel == "." || rel == "" || isDir(e.abs(rel)) {
		return nil
	}
	if err := e.mkdirs(path.Dir(rel)); err != nil {
		return err
	}
	dir := e.abs(rel)
	if err := os.Mkdir(dir, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	if info, err := os.Stat(filepath.Dir(dir)); err == nil {
		if st, ok := info.Sys().(*syscall.Stat_t); ok {
			_ = os.Chown(dir, int(st.Uid), int(st.Gid))
		}
	}
	return nil
}

// moved carries plays, exclusions and playlist entries from each track's
// old path to its new one; a duplicate's plays add to the copy kept.
func (s *shelf) moved(paths map[string]string) error {
	if len(paths) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for from, to := range paths {
		if p, ok := s.plays[from]; ok {
			q := s.plays[to]
			q.N += p.N
			if p.Last.After(q.Last) {
				q.Last = p.Last
			}
			s.plays[to] = q
			delete(s.plays, from)
		}
		if s.excluded[from] {
			s.excluded[to] = true
			delete(s.excluded, from)
		}
	}
	changed := false
	for _, pl := range s.playlists {
		seen := map[string]bool{}
		out := pl.Paths[:0]
		for _, p := range pl.Paths {
			if to, ok := paths[p]; ok {
				p = to
				changed = true
			}
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
		pl.Paths = out
	}
	if err := s.saveMetaLocked(); err != nil {
		return err
	}
	if changed {
		return s.savePlaylistsLocked()
	}
	return nil
}

// routeEdit serves the edits and the log of moves.
func routeEdit(mux *http.ServeMux, e *editor) {
	mux.HandleFunc("POST /api/edit", func(w http.ResponseWriter, r *http.Request) {
		var req editRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		e.editing.Lock()
		p, err := e.plan(req)
		if err == nil && r.URL.Query().Get("dry") == "" {
			err = e.apply(p)
		}
		e.editing.Unlock()
		var refused editError
		switch {
		case errors.As(err, &refused):
			http.Error(w, refused.msg, http.StatusConflict)
		case err != nil:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		default:
			writeJSON(w, p)
		}
	})
	mux.HandleFunc("GET /api/moves", func(w http.ResponseWriter, r *http.Request) {
		since, _ := strconv.Atoi(r.URL.Query().Get("since"))
		seq, moves := e.since(since)
		writeJSON(w, struct {
			Seq   int          `json:"seq"`
			Moves []moveRecord `json:"moves"`
		}{seq, moves})
	})
}
