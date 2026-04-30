package server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/jmelahman/kanban/internal/api"
	"github.com/jmelahman/kanban/internal/config"
	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/docker"
	"github.com/jmelahman/kanban/internal/hooks"
	"github.com/jmelahman/kanban/internal/session"
)

func Root() *cobra.Command {
	var addr string
	var dataDir string
	var portRangeStart int
	var portRangeEnd int

	cmd := &cobra.Command{
		Use:   "kanban",
		Short: "Kanban board for managing Claude Code sessions",
	}

	serve := &cobra.Command{
		Use:   "serve",
		Short: "Start the kanban HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(addr, dataDir, portRangeStart, portRangeEnd)
		},
	}
	serve.Flags().StringVar(&addr, "addr", ":7474", "HTTP listen address")
	serve.Flags().StringVar(&dataDir, "data-dir", "", "Override data directory (default: $KANBAN_DATA_DIR or XDG)")
	serve.Flags().IntVar(&portRangeStart, "port-range-start", 13000, "First host port available for proxy allocation")
	serve.Flags().IntVar(&portRangeEnd, "port-range-end", 13099, "Last host port available for proxy allocation (inclusive)")

	cmd.AddCommand(serve)
	return cmd
}

func run(addr, dataDirOverride string, portStart, portEnd int) error {
	cfg, err := config.Load(dataDirOverride, portStart, portEnd)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	store, err := db.Open(cfg.DBPath())
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer store.Close()

	dockerClient, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("docker client: %w", err)
	}
	defer dockerClient.Close()

	hookRunner := hooks.NewRunner(store)
	sessionMgr := session.NewManager(store, dockerClient, hookRunner)

	mux := api.NewMux(api.Deps{
		Store:    store,
		Docker:   dockerClient,
		Sessions: sessionMgr,
		Hooks:    hookRunner,
		Config:   cfg,
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           logRequests(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("kanban listening on %s", addr)
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

func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := s.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("hijack not supported")
	}
	return hj.Hijack()
}

func logRequests(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		h.ServeHTTP(rec, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start))
	})
}
