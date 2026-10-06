package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/jmelahman/kanban/web"
)

// backendURLEnv names the kanban server that `kanban web` proxies to.
const backendURLEnv = "KANBAN_BACKEND_URL"

// newWebCmd builds `kanban web`: the frontend served from this process, with
// the API and WebSockets proxied to a `kanban serve` running elsewhere.
func newWebCmd() *cobra.Command {
	var addr string
	var backend string
	cmd := &cobra.Command{
		Use:   "web",
		Short: "Serve the web UI and proxy its API calls to another kanban server",
		Long: "Serve the web UI from this machine while the boards, sessions and\n" +
			"containers stay on another `kanban serve`. Requests under /api/ and\n" +
			"/ws/ are forwarded to --backend; nothing is stored locally.\n\n" +
			"kanban has no login: anyone who can reach this address gets a shell\n" +
			"in the backend's session containers. Don't expose it publicly.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if env := os.Getenv(backendURLEnv); env != "" && !cmd.Flags().Changed("backend") {
				backend = env
			}
			if backend == "" {
				return fmt.Errorf("--backend (or $%s) is required", backendURLEnv)
			}
			target, err := parseBackendURL(backend)
			if err != nil {
				return err
			}
			return runWeb(addr, target)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", ":7474", "HTTP listen address")
	cmd.Flags().StringVar(&backend, "backend", "", "Base URL of the kanban server to proxy to, e.g. http://devbox:7474. Env: $"+backendURLEnv)
	return cmd
}

func parseBackendURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse backend URL: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("backend URL %q must look like http://host:port", raw)
	}
	return u, nil
}

// newWebHandler serves the embedded frontend and forwards /api/ and /ws/ to
// the kanban server at backend. The page stays same-origin with its API, so
// the frontend's relative fetch/EventSource paths, its location.host-based
// WebSocket URL and the service worker all work unchanged.
func newWebHandler(backend *url.URL) http.Handler {
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(backend)
			// SetURL rewrites Host to the backend's. Put the browser's back:
			// the backend only accepts a PTY WebSocket whose Origin matches
			// Host, and the browser's Origin names this server.
			// See REGRESSIONS.md: "Proxying `/ws` must preserve `Host`".
			pr.Out.Host = pr.In.Host
			// Rewrite has already dropped the caller's own X-Forwarded-*
			// headers; this records only the address we actually saw.
			pr.SetXForwarded()
		},
		// Flush every write so live updates and streamed logs aren't held
		// back. ReverseProxy already does this for the streaming responses it
		// recognises; this makes it unconditional.
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			// A closed tab ends its event stream this way; that isn't a
			// backend problem and there is nobody left to answer.
			if r.Context().Err() != nil {
				return
			}
			log.Printf("proxy %s %s: %v", r.Method, r.URL.Path, err)
			http.Error(w, "kanban backend unreachable", http.StatusBadGateway)
		},
	}
	static := compressResponses(web.Handler())
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ws/") {
			proxy.ServeHTTP(w, r)
			return
		}
		static.ServeHTTP(w, r)
	})
}

func runWeb(addr string, backend *url.URL) error {
	// Every request inherits this context, so cancelling it on shutdown ends
	// the long-lived event streams that Shutdown would otherwise wait out.
	baseCtx, cancelRequests := context.WithCancel(context.Background())
	defer cancelRequests()
	srv := &http.Server{
		Addr:              addr,
		Handler:           logRequests(newWebHandler(backend)),
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return baseCtx },
	}

	go func() {
		log.Printf("kanban web listening on %s, backend %s", addr, backend)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("shutting down")
	cancelRequests()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}
