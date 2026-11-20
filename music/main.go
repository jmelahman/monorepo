// Command music serves the homelab's music client: stream to discover, own
// what you keep. The server, its page and the shared design are one binary,
// which the homelab's nginx fronts at /music (see homelab/docker-compose.yml).
package main

import (
	"cmp"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"os/exec"
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
	mux.HandleFunc("GET /api/app", func(w http.ResponseWriter, _ *http.Request) {
		if *apk == "" {
			http.NotFound(w, new(http.Request))
			return
		}
		version := ""
		if b, err := os.ReadFile(*apk + ".version"); err == nil {
			version = strings.TrimSpace(string(b))
		}
		info, err := os.Stat(*apk)
		if err != nil || info.IsDir() {
			http.NotFound(w, new(http.Request))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			Version string `json:"version"`
			Size    int64  `json:"size"`
		}{Version: version, Size: info.Size()})
	})
	mux.HandleFunc("GET /api/yt/search", func(w http.ResponseWriter, r *http.Request) {
		if q := r.URL.Query().Get("q"); q == "" {
			http.Error(w, "missing q", http.StatusBadRequest)
			return
		}
		tracks, err := ytFlat(r.Context(), "ytsearch15:"+r.URL.Query().Get("q"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tracks)
	})
	mux.HandleFunc("GET /api/yt/supermix", func(w http.ResponseWriter, r *http.Request) {
		mix, err := ytSupermix(r.Context(), *music)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mix)
	})
	mux.HandleFunc("GET /api/yt/stream", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("v")
		if id == "" {
			http.NotFound(w, r)
			return
		}
		url, err := ytResolve(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		// The stream is signed to this server, so the phone plays it
		// through here; ranges pass straight through so seeking works.
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, url, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if rg := r.Header.Get("Range"); rg != "" {
			req.Header.Set("Range", rg)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, "the stream could not be fetched", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		for _, h := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges"} {
			if v := resp.Header.Get(h); v != "" {
				w.Header().Set(h, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
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

// ytTrack is one playable YouTube track, as the frontends receive it.
type ytTrack struct {
	VideoID  string `json:"videoId"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Duration int    `json:"duration"` // seconds
}

// ytFlat runs yt-dlp over a URL and reads its flat playlist JSON lines,
// one object per track.
func ytFlat(ctx context.Context, url string) ([]ytTrack, error) {
	cctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, "yt-dlp", "--flat-playlist", "-j", "--no-warnings", url).Output()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp: %w", err)
	}
	var tracks []ytTrack
	for line := range strings.SplitSeq(string(out), "\n") {
		var raw struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			Channel  string `json:"channel"`
			Uploader string `json:"uploader"`
			Duration int    `json:"duration"`
		}
		if json.Unmarshal([]byte(line), &raw) != nil || raw.ID == "" || raw.Title == "" {
			continue
		}
		artist := raw.Channel
		if artist == "" {
			artist = raw.Uploader
		}
		tracks = append(tracks, ytTrack{VideoID: raw.ID, Title: raw.Title, Artist: artist, Duration: raw.Duration})
	}
	return tracks, nil
}

// ytResolve finds the audio stream behind a video, caching briefly: the
// URLs are signed to this server and expire.
var ytStreams = struct {
	sync.RWMutex
	urls map[string]struct {
		url     string
		expires time.Time
	}
}{urls: map[string]struct {
	url     string
	expires time.Time
}{}}

func ytResolve(ctx context.Context, videoID string) (string, error) {
	ytStreams.RLock()
	hit, ok := ytStreams.urls[videoID]
	ytStreams.RUnlock()
	if ok && time.Now().Before(hit.expires) {
		return hit.url, nil
	}
	cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, "yt-dlp", "-g", "-f", "bestaudio", "--no-warnings",
		"https://music.youtube.com/watch?v="+videoID).Output()
	if err != nil {
		return "", fmt.Errorf("yt-dlp: %w", err)
	}
	url, _, _ := strings.Cut(string(out), "\n")
	url = strings.TrimSpace(url)
	if url == "" {
		return "", errors.New("no audio stream found")
	}
	ytStreams.Lock()
	ytStreams.urls[videoID] = struct {
		url     string
		expires time.Time
	}{url, time.Now().Add(10 * time.Minute)}
	ytStreams.Unlock()
	return url, nil
}

// ytSupermix builds a mix like the store apps' own: a seed from the
// owned library, expanded through YouTube's radio into a run of tracks
// the library doesn't have to own yet.
func ytSupermix(ctx context.Context, musicDir string) ([]ytTrack, error) {
	artists, err := walkLibrary(musicDir)
	if err != nil {
		return nil, err
	}
	if len(artists) == 0 {
		return nil, errors.New("the owned library is empty; nothing to seed a mix from")
	}
	artist := artists[rand.IntN(len(artists))]
	album := artist.Albums[rand.IntN(len(artist.Albums))]
	seed := album.Tracks[rand.IntN(len(album.Tracks))]

	// Find the seed on YouTube, then let the radio do the rest.
	found, err := ytFlat(ctx, "ytsearch1:"+artist.Name+" "+seed.Name)
	if err != nil || len(found) == 0 {
		return nil, fmt.Errorf("no YouTube match for %s — %s", artist.Name, seed.Name)
	}
	mix, err := ytFlat(ctx, "https://www.youtube.com/watch?v="+found[0].VideoID+"&list=RD"+found[0].VideoID)
	if err != nil {
		return nil, err
	}
	// The radio starts on its own seed; the mix is what follows it.
	if len(mix) > 50 {
		mix = mix[:50]
	}
	return mix, nil
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
