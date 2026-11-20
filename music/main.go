// Command music serves the homelab's music client: stream to discover, own
// what you keep. The server, its page and the shared design are one binary,
// which the homelab's nginx fronts at /music (see homelab/docker-compose.yml).
package main

import (
	"cmp"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed web design
var embedded embed.FS

func main() {
	if len(os.Args) < 2 || os.Args[1] != "serve" {
		fmt.Fprintln(os.Stderr, "usage: music serve [flags]   (music serve -h for the flags)")
		os.Exit(2)
	}
	fl := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fl.String("addr", "127.0.0.1:8095", "address to listen on")
	webDir := fl.String("web", "", "serve the page from this folder instead of the built-in copy, for development")
	music := fl.String("music", defaultMusic(), "the owned library, where what is bought lands")
	state := fl.String("state", defaultState(), "where the purchase queue and its session live")
	apk := fl.String("apk", "", "the Android client's APK, served at api/apk when it exists")
	_ = fl.Parse(os.Args[2:]) // ExitOnError

	page, err := fs.Sub(embedded, "web")
	if err != nil {
		fail(err)
	}
	design, err := fs.Sub(embedded, "design")
	if err != nil {
		fail(err)
	}
	var tags *etags
	if *webDir != "" {
		page = os.DirFS(*webDir)
		design = os.DirFS(filepath.Join(*webDir, "..", "design"))
	}
	web := withDesign{page: page, design: design}
	if *webDir == "" {
		tags = &etags{fsys: web}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			OK    bool   `json:"ok"`
			Music string `json:"music"`
		}{OK: true, Music: *music})
	})
	mux.HandleFunc("GET /api/library", func(w http.ResponseWriter, _ *http.Request) {
		artists, err := walkLibrary(*music)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(artists)
	})
	mux.HandleFunc("GET /api/audio", func(w http.ResponseWriter, r *http.Request) {
		name, err := safeJoin(*music, r.URL.Query().Get("path"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if info, err := os.Stat(name); err != nil || info.IsDir() || !isAudio(name) {
			http.NotFound(w, r)
			return
		}
		// ServeFile answers range requests, so seeking works and the
		// browser only pulls what it has played.
		http.ServeFile(w, r, name)
	})
	mux.HandleFunc("GET /api/apk", func(w http.ResponseWriter, r *http.Request) {
		if *apk == "" {
			http.NotFound(w, r)
			return
		}
		info, err := os.Stat(*apk)
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}
		// A download, not something to play inline.
		w.Header().Set("Content-Disposition", `attachment; filename="Music.apk"`)
		http.ServeFile(w, r, *apk)
	})
	files := http.FileServerFS(web)
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The page changes with the binary; don't let a browser hold an old one.
		w.Header().Set("Cache-Control", "no-cache")
		if t := tags.of(r.URL.Path); t != "" {
			w.Header().Set("ETag", t)
		}
		files.ServeHTTP(w, r)
	}))

	slog.Info("music", "url", "http://"+*addr, "music", *music, "state", *state)
	hs := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fail(err)
	}
}

// defaultMusic is the owned library, next to the media the homelab's jellyfin
// serves, so what is bought lands where jellyfin picks it up too.
func defaultMusic() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "music"
	}
	return filepath.Join(home, "media", "music")
}

func defaultState() string {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, "music")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "music")
	}
	return filepath.Join(home, ".local", "state", "music")
}

// track is one file of the owned library, with its path relative to the
// library's root so the audio endpoint can ask for it by name.
type track struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// album groups its tracks.
type album struct {
	Name   string  `json:"name"`
	Tracks []track `json:"tracks"`
}

// artist groups its albums. The library's folders carry the store's
// Artist/Album layout, so the tree itself is the catalog; singles that
// sit directly under an artist land in a "Singles" album.
type artist struct {
	Name   string  `json:"name"`
	Albums []album `json:"albums"`
}

// walkLibrary reads the owned library as artists, albums and tracks,
// sorted, one pass of the folders it is made of.
func walkLibrary(root string) ([]artist, error) {
	if root == "" {
		return nil, errors.New("no library folder: pass --music")
	}
	artistDirs, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var artists []artist
	for _, ad := range artistDirs {
		if !ad.IsDir() {
			continue
		}
		a := artist{Name: ad.Name()}
		entries, err := os.ReadDir(filepath.Join(root, ad.Name()))
		if err != nil {
			continue
		}
		var singles album
		singles.Name = "Singles"
		for _, ed := range entries {
			if ed.IsDir() {
				files, err := os.ReadDir(filepath.Join(root, ad.Name(), ed.Name()))
				if err != nil {
					continue
				}
				var al album
				al.Name = ed.Name()
				for _, fd := range files {
					if t, ok := trackOf(ad.Name(), ed.Name(), fd); ok {
						al.Tracks = append(al.Tracks, t)
					}
				}
				if len(al.Tracks) > 0 {
					slices.SortFunc(al.Tracks, func(x, y track) int { return natural(x.Name, y.Name) })
					a.Albums = append(a.Albums, al)
				}
			} else if t, ok := trackOf(ad.Name(), "", ed); ok {
				singles.Tracks = append(singles.Tracks, t)
			}
		}
		if len(singles.Tracks) > 0 {
			slices.SortFunc(singles.Tracks, func(x, y track) int { return natural(x.Name, y.Name) })
			a.Albums = append(a.Albums, singles)
		}
		if len(a.Albums) > 0 {
			slices.SortFunc(a.Albums, func(x, y album) int {
				return cmp.Compare(strings.ToLower(x.Name), strings.ToLower(y.Name))
			})
			artists = append(artists, a)
		}
	}
	slices.SortFunc(artists, func(x, y artist) int {
		return cmp.Compare(strings.ToLower(x.Name), strings.ToLower(y.Name))
	})
	return artists, nil
}

// trackOf reads one directory entry as a track, skipping folders and
// formats the player does not serve.
func trackOf(artist, album string, entry os.DirEntry) (track, bool) {
	if entry.IsDir() || !isAudio(entry.Name()) {
		return track{}, false
	}
	info, err := entry.Info()
	if err != nil {
		return track{}, false
	}
	var dir string
	if album != "" {
		dir = path.Join(artist, album)
	} else {
		dir = artist
	}
	return track{
		Name: strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())),
		Path: path.Join(dir, entry.Name()),
		Size: info.Size(),
	}, true
}

// isAudio reports whether a file is one the player can serve.
func isAudio(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".mp3", ".m4a", ".aac", ".flac", ".ogg", ".opus", ".wav", ".wma":
		return true
	}
	return false
}

// safeJoin resolves a slash-separated path from a request under the
// library's root, refusing anything that would escape it: the path is
// made absolute before it is cleaned, so ".." can only ever climb back
// down inside the root.
func safeJoin(root, p string) (string, error) {
	clean := path.Clean("/" + p)
	if clean == "/" {
		return "", errors.New("empty path")
	}
	joined := filepath.Join(root, filepath.FromSlash(clean))
	abs, err := filepath.Abs(joined)
	if err != nil {
		return "", err
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(abs+string(filepath.Separator), rootAbs+string(filepath.Separator)) {
		return "", errors.New("outside the library")
	}
	return abs, nil
}

// natural orders track names like "02." and "10." by their number rather
// than their spelling.
func natural(a, b string) int {
	na, aok := leadingInt(a)
	nb, bok := leadingInt(b)
	if aok && bok && na != nb {
		return cmp.Compare(na, nb)
	}
	return cmp.Compare(strings.ToLower(a), strings.ToLower(b))
}

// leadingInt reads the number before a track name's first period.
func leadingInt(s string) (int, bool) {
	head, _, found := strings.Cut(s, ".")
	if !found {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(head))
	return n, err == nil
}

// withDesign serves the page with the shared fonts and tokens under design/.
// They sit beside web/ rather than in it because the monorepo syncs the same
// design/ into every project that uses them (.config/git-orchard/shared/design).
type withDesign struct {
	page, design fs.FS
}

func (w withDesign) Open(name string) (fs.File, error) {
	if name == "design" {
		return w.design.Open(".")
	}
	if rest, ok := strings.CutPrefix(name, "design/"); ok {
		return w.design.Open(rest)
	}
	return w.page.Open(name)
}

// etags tags the built-in files by their content. An embed.FS has no
// modification times, so without a tag a browser can't ask whether a file
// changed and downloads it again every time.
type etags struct {
	fsys fs.FS
	mu   sync.Mutex
	tags map[string]string
}

func (e *etags) of(urlPath string) string {
	if e == nil {
		return ""
	}
	name := strings.TrimPrefix(path.Clean("/"+urlPath), "/")
	if name == "" {
		name = "index.html"
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if t, ok := e.tags[name]; ok {
		return t
	}
	t := ""
	if b, err := fs.ReadFile(e.fsys, name); err == nil {
		sum := sha256.Sum256(b)
		t = `"` + hex.EncodeToString(sum[:12]) + `"`
	}
	if e.tags == nil {
		e.tags = map[string]string{}
	}
	e.tags[name] = t
	return t
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "music:", err)
	os.Exit(1)
}
