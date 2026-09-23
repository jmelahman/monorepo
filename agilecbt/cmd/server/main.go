package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/jmelahman/fullstack-template/internal/api"
	"github.com/jmelahman/fullstack-template/internal/config"
	"github.com/jmelahman/fullstack-template/internal/db"
)

// version is populated at build time via -ldflags -X (see Dockerfile /
// .goreleaser.yaml). Use `git describe --tags --always --dirty` so a single
// string carries tag, distance, short sha, and dirty marker. Falls back to
// runtime/debug VCS info for dev builds without ldflags.
var version = ""

// BuildInfo describes the running binary.
type BuildInfo struct {
	Version string `json:"version"`
}

// Build returns the build metadata for the running binary, falling back to
// runtime/debug VCS info when ldflags weren't set.
func Build() BuildInfo {
	if version != "" {
		return BuildInfo{Version: version}
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		var rev string
		var modified bool
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.modified":
				modified = s.Value == "true"
			}
		}
		if rev != "" {
			short := rev
			if len(short) > 7 {
				short = short[:7]
			}
			if modified {
				short += "-dirty"
			}
			return BuildInfo{Version: short}
		}
	}
	return BuildInfo{Version: "dev"}
}

func Root() *cobra.Command {
	var addr string
	var dataDir string
	var inMemory bool

	cmd := &cobra.Command{
		Use:     "app",
		Short:   "Fullstack application template",
		Version: Build().Version,
	}

	serve := &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(addr, dataDir, inMemory)
		},
	}
	serve.Flags().StringVar(&addr, "addr", ":8080", "HTTP listen address")
	serve.Flags().StringVar(&dataDir, "data-dir", "", "Override data directory (default: $APP_DATA_DIR or XDG)")
	serve.Flags().BoolVar(&inMemory, "in-memory", false, "Use an ephemeral in-memory SQLite database; all data is discarded on shutdown")
	cmd.AddCommand(serve)

	addClientCommands(cmd)

	return cmd
}

func run(addr, dataDirOverride string, inMemory bool) error {
	cfg, err := config.Load(dataDirOverride)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	dbPath := cfg.DBPath()
	if inMemory {
		log.Printf("WARNING: --in-memory set; using ephemeral SQLite, all data is lost on shutdown")
		dbPath = ":memory:"
	}
	store, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer store.Close()

	mux := api.NewMux(api.Deps{
		Store: store,
		Build: api.BuildInfo(Build()),
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           recoverPanics(logRequests(mux)),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// recoverPanics is the outermost middleware. It catches panics from any
// downstream handler so the process survives, logs the trace, and writes a
// 500 response.
func recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			log.Printf("panic recovered: %v\n%s", rec, debug.Stack())
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}()
		next.ServeHTTP(w, r)
	})
}

func logRequests(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		h.ServeHTTP(rec, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start))
	})
}
