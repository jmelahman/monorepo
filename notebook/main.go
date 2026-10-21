// Command notebook serves a local NotebookLM: find sources on the web and
// turn them into a podcast episode, with Claude Code doing
// the reading and writing on the signed-in Claude subscription.
package main

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jmelahman/notebook/internal/api"
	"github.com/jmelahman/notebook/internal/claude"
	"github.com/jmelahman/notebook/internal/publish"
	"github.com/jmelahman/notebook/internal/sources"
	"github.com/jmelahman/notebook/internal/speech"
)

//go:embed web
var embedded embed.FS

type hostList []string

func (h *hostList) String() string     { return strings.Join(*h, ",") }
func (h *hostList) Set(v string) error { *h = append(*h, v); return nil }

func main() {
	if len(os.Args) < 2 || os.Args[1] != "serve" {
		fmt.Fprintln(os.Stderr, "usage: notebook serve [flags]   (notebook serve -h for the flags)")
		os.Exit(2)
	}
	fl := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fl.String("addr", "127.0.0.1:8090", "address to listen on")
	webDir := fl.String("web", "", "serve the page from this folder instead of the built-in copy, for development")
	podcasts := fl.String("podcasts", defaultPodcasts(), "folder episodes are published to, one subfolder per notebook")
	ttsURL := fl.String("tts-url", "", "an OpenAI-compatible speech server to speak with instead of this computer, e.g. http://127.0.0.1:8880 for Kokoro-FastAPI")
	ttsModel := fl.String("tts-model", "kokoro", "the speech server's model")
	model := fl.String("model", "", "default Claude model: "+strings.Join(claude.Models, ", ")+", or a model id")
	effort := fl.String("effort", "", "default effort: low, medium, high, xhigh or max")
	state := fl.String("state", defaultState(), "where Claude Code sessions run, so they can be resumed")
	var hosts hostList
	fl.Var(&hosts, "host", "a host name the page is reached at besides localhost (repeatable)")
	_ = fl.Parse(os.Args[2:]) // ExitOnError
	if *ttsURL != "" {
		if u, err := url.Parse(*ttsURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			fail(fmt.Errorf("--tts-url %q: want an http:// or https:// address", *ttsURL))
		}
	}

	web, err := fs.Sub(embedded, "web")
	if err != nil {
		fail(err)
	}
	var tags *etags
	if *webDir != "" {
		web = os.DirFS(*webDir)
	} else {
		tags = &etags{fsys: web}
	}
	srv := &api.Server{
		Claude:    &claude.Runner{Model: *model, Effort: *effort, WorkDir: filepath.Join(*state, "sessions")},
		Sources:   &sources.Client{},
		Publisher: &publish.Publisher{Dir: *podcasts},
		Voices:    &speech.Voices{Dir: filepath.Join(defaultCache(), "voices")},
		TTSURL:    *ttsURL,
		TTSModel:  *ttsModel,
		Hosts:     hosts,
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", srv.Handler())
	files := http.FileServerFS(web)
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The page changes with the binary; don't let a browser hold an old one.
		w.Header().Set("Cache-Control", "no-cache")
		// Cross-origin isolation lets the speech model's WebAssembly run on
		// several threads instead of one.
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Embedder-Policy", "require-corp")
		if t := tags.of(r.URL.Path); t != "" {
			w.Header().Set("ETag", t)
		}
		files.ServeHTTP(w, r)
	}))

	slog.Info("notebook", "url", "http://"+*addr, "podcasts", *podcasts)
	hs := &http.Server{Addr: *addr, Handler: srv.Guard(mux), ReadHeaderTimeout: 10 * time.Second}
	if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fail(err)
	}
}

// defaultPodcasts is the folder the homelab's Audiobookshelf serves as
// /podcasts, when run from the monorepo or a project in it.
func defaultPodcasts() string {
	for _, p := range []string{"homelab", "../homelab"} {
		if _, err := os.Stat(filepath.Join(p, "docker-compose.yml")); err == nil {
			if abs, err := filepath.Abs(filepath.Join(p, "podcasts")); err == nil {
				return abs
			}
		}
	}
	return ""
}

func defaultState() string {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, "notebook")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "notebook")
	}
	return filepath.Join(home, ".local", "state", "notebook")
}

func defaultCache() string {
	if d, err := os.UserCacheDir(); err == nil {
		return filepath.Join(d, "notebook")
	}
	return filepath.Join(os.TempDir(), "notebook")
}

// etags tags the built-in files by their content. An embed.FS has no
// modification times, so without a tag a browser can't ask whether a file
// changed and downloads it again every time: the speech model's 22 MB of
// WebAssembly once per worker.
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
	fmt.Fprintln(os.Stderr, "notebook:", err)
	os.Exit(1)
}
