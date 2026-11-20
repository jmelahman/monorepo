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
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
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

	// A stop signal ends the buy worker and drains the requests in flight;
	// a buy cut short runs again on the next start.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
		writeJSON(w, struct {
			OK    bool   `json:"ok"`
			Music string `json:"music"`
		}{OK: true, Music: *music})
	})
	shelf, err := newShelf(*state)
	if err != nil {
		fail(err)
	}
	routeLibrary(mux, *music, shelf)
	routeShelf(mux, shelf, *music)
	routeMixes(mux, &mixer{
		music: *music, shelf: shelf,
		cookies: os.Getenv("MUSIC_YTM_COOKIES"), supermix: os.Getenv("MUSIC_YTM_SUPERMIX"),
	})
	routeApp(mux, *apk)
	routeYouTube(mux)
	routeBuys(ctx, mux, os.Getenv("MUSIC_STORE"), os.Getenv("MUSIC_STORE_PROXY"), *music, *state)
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
	go func() {
		<-ctx.Done()
		// Streams can run for minutes; give them a moment, not forever.
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = hs.Shutdown(sctx)
	}()
	if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fail(err)
	}
}

// writeJSON answers with v as JSON.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// routeLibrary serves the owned library: its listing, its audio and its
// albums' art.
func routeLibrary(mux *http.ServeMux, music string, shelf *shelf) {
	mux.HandleFunc("GET /api/library", func(w http.ResponseWriter, _ *http.Request) {
		artists, err := walkLibrary(music)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		shelf.decorate(artists)
		writeJSON(w, artists)
	})
	mux.HandleFunc("GET /api/audio", func(w http.ResponseWriter, r *http.Request) {
		name, err := safeJoin(music, r.URL.Query().Get("path"))
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
	mux.HandleFunc("GET /api/cover", func(w http.ResponseWriter, r *http.Request) {
		// An album's art by the names the library lists it under; the
		// Singles an artist's loose tracks are gathered into have no
		// folder of their own, so their art is looked for among them.
		q := r.URL.Query()
		artist, album := q.Get("artist"), q.Get("album")
		if strings.ContainsAny(artist+album, "/\\") {
			http.NotFound(w, r)
			return
		}
		artistDir, err := safeJoin(music, artist)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		dir, err := safeJoin(artistDir, album)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		loose := false
		if info, err := os.Stat(dir); album == singlesName && (err != nil || !info.IsDir()) {
			dir, loose = artistDir, true
		}
		serveCover(w, r, dir, loose)
	})
}

// routeApp serves the Android client's APK and what version it is, so
// installed clients can offer to update themselves.
func routeApp(mux *http.ServeMux, apk string) {
	present := func() (os.FileInfo, bool) {
		if apk == "" {
			return nil, false
		}
		info, err := os.Stat(apk)
		return info, err == nil && !info.IsDir()
	}
	mux.HandleFunc("GET /api/apk", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := present(); !ok {
			http.NotFound(w, r)
			return
		}
		// A download, not something to play inline.
		w.Header().Set("Content-Disposition", `attachment; filename="Music.apk"`)
		http.ServeFile(w, r, apk)
	})
	mux.HandleFunc("GET /api/app", func(w http.ResponseWriter, r *http.Request) {
		info, ok := present()
		if !ok {
			http.NotFound(w, r)
			return
		}
		version := ""
		if b, err := os.ReadFile(apk + ".version"); err == nil {
			version = strings.TrimSpace(string(b))
		}
		writeJSON(w, struct {
			Version string `json:"version"`
			Size    int64  `json:"size"`
		}{Version: version, Size: info.Size()})
	})
}

// routeYouTube serves the streamed half: search and the audio behind a
// video, proxied. The supermix is with the other mix, in mix.go.
func routeYouTube(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/yt/search", func(w http.ResponseWriter, r *http.Request) {
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		if q == "" {
			http.Error(w, "missing q", http.StatusBadRequest)
			return
		}
		tracks, err := ytFlat(r.Context(), "ytsearch15:"+q, 15)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, tracks)
	})
	mux.HandleFunc("GET /api/yt/stream", ytStream)
}

// ytStream proxies a video's audio. The stream is signed to this server,
// so the phone plays it through here; ranges pass straight through so
// seeking works. A cached URL that has gone stale is resolved afresh once.
func ytStream(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("v")
	if !ytID.MatchString(id) {
		http.NotFound(w, r)
		return
	}
	for attempt := 0; ; attempt++ {
		stream, err := ytResolve(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, stream.url, nil)
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
		if (resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusGone) && attempt == 0 {
			resp.Body.Close()
			ytForget(id, stream)
			continue
		}
		defer resp.Body.Close()
		for _, h := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges"} {
			if v := resp.Header.Get(h); v != "" {
				w.Header().Set(h, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
		return
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
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Added    int64  `json:"added"` // when the file landed, in Unix seconds
	Plays    int    `json:"plays,omitempty"`
	Excluded bool   `json:"excluded,omitempty"` // kept out of the mixes
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
		singles.Name = singlesName
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
			// A folder that is itself called Singles takes the loose ones
			// in, rather than the artist listing two albums of one name.
			if i := slices.IndexFunc(a.Albums, func(al album) bool { return al.Name == singlesName }); i >= 0 {
				singles.Tracks = append(a.Albums[i].Tracks, singles.Tracks...)
				a.Albums = slices.Delete(a.Albums, i, i+1)
			}
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

// singlesName is the album an artist's loose tracks are gathered into.
const singlesName = "Singles"

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
		Name:  strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())),
		Path:  path.Join(dir, entry.Name()),
		Size:  info.Size(),
		Added: info.ModTime().Unix(),
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

// leadingInt reads the number a track name leads with, whatever follows
// it: "02. ", "02 - " and "2 " all number the track.
func leadingInt(s string) (int, bool) {
	end := strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' })
	if end < 0 {
		end = len(s)
	}
	n, err := strconv.Atoi(s[:end])
	return n, err == nil
}

// ytTrack is one playable YouTube track, as the frontends receive it.
type ytTrack struct {
	VideoID  string `json:"videoId"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Duration int    `json:"duration"` // seconds
	Thumb    string `json:"thumb"`    // the video's still, cropped square by the clients
}

// ytFlat runs yt-dlp over a URL and reads its flat playlist JSON lines,
// one object per track.
func ytFlat(ctx context.Context, url string, limit int, extra ...string) ([]ytTrack, error) {
	cctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	args := append([]string{"--flat-playlist", "-j", "--no-warnings", "--playlist-end", strconv.Itoa(limit)}, extra...)
	out, err := ytdlp(cctx, append(args, url)...)
	if err != nil {
		return nil, err
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
		tracks = append(tracks, ytTrack{
			VideoID: raw.ID, Title: raw.Title, Artist: artist, Duration: raw.Duration,
			Thumb: "https://i.ytimg.com/vi/" + raw.ID + "/mqdefault.jpg",
		})
	}
	return tracks, nil
}

// ytdlp runs yt-dlp and returns what it printed, or the last thing it
// complained about, which says more than an exit status.
func ytdlp(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	cmd.WaitDelay = 5 * time.Second
	out, err := cmd.Output()
	if err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			if lines := strings.Split(strings.TrimSpace(string(exit.Stderr)), "\n"); lines[len(lines)-1] != "" {
				return nil, fmt.Errorf("yt-dlp: %s", lines[len(lines)-1])
			}
		}
		return nil, fmt.Errorf("yt-dlp: %w", err)
	}
	return out, nil
}

// ytStreams caches the audio stream behind each video briefly: the URLs
// are signed to this server and expire. A video being resolved has a
// pending entry the other requests for it wait on, as a player asks for
// several ranges at once and each would otherwise run yt-dlp.
var ytStreams = struct {
	sync.Mutex
	urls map[string]*ytStreamURL
}{urls: map[string]*ytStreamURL{}}

type ytStreamURL struct {
	ready   chan struct{} // closed once url and err are set
	url     string
	err     error
	expires time.Time
}

// ytID is what a YouTube video id looks like; anything else is refused
// before it reaches yt-dlp.
var ytID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// ytForget drops a cached stream URL that the far end has refused,
// unless another request has already put a fresh one in its place.
func ytForget(videoID string, stale *ytStreamURL) {
	ytStreams.Lock()
	if ytStreams.urls[videoID] == stale {
		delete(ytStreams.urls, videoID)
	}
	ytStreams.Unlock()
}

// ytResolve finds the audio stream behind a video. It runs apart from
// any one request, so a player hanging up on its first range doesn't
// fail the others waiting on the same resolve.
func ytResolve(videoID string) (*ytStreamURL, error) {
	now := time.Now()
	ytStreams.Lock()
	e, ok := ytStreams.urls[videoID]
	if !ok || now.After(e.expires) {
		// Expired entries go as new ones come, so the cache stays the
		// size of what was played in the last ten minutes.
		for id, old := range ytStreams.urls {
			if now.After(old.expires) {
				delete(ytStreams.urls, id)
			}
		}
		e = &ytStreamURL{ready: make(chan struct{}), expires: now.Add(10 * time.Minute)}
		ytStreams.urls[videoID] = e
		go e.resolve(videoID)
	}
	ytStreams.Unlock()
	<-e.ready
	if e.err != nil {
		return nil, e.err
	}
	return e, nil
}

func (e *ytStreamURL) resolve(videoID string) {
	defer close(e.ready)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	out, err := ytdlp(ctx, "-g", "-f", "bestaudio", "--no-warnings", "https://music.youtube.com/watch?v="+videoID)
	if err == nil {
		e.url, _, _ = strings.Cut(string(out), "\n")
		e.url = strings.TrimSpace(e.url)
		if e.url == "" {
			err = errors.New("no audio stream found")
		}
	}
	if err != nil {
		e.err = err
		// A failure isn't cached; the next play tries again.
		ytForget(videoID, e)
	}
}

// trackNumber is the "01 " or "1. " a file name leads with.
var trackNumber = regexp.MustCompile(`^\d+[\s._-]*`)

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
	b, err := fs.ReadFile(e.fsys, name)
	if err != nil {
		// Not remembered: any path can be asked for, and the misses
		// would grow the map without end.
		return ""
	}
	sum := sha256.Sum256(b)
	t := `"` + hex.EncodeToString(sum[:12]) + `"`
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
