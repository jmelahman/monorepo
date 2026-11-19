// Command music serves the homelab's music client: stream to discover, own
// what you keep. The server, its page and the shared design are one binary,
// which the homelab's nginx fronts at /music (see homelab/docker-compose.yml).
package main

import (
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
