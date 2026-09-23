package workerapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/jmelahman/local-preview/internal/execstream"
	"github.com/jmelahman/local-preview/internal/supervise"
)

// Supervisor is the worker-local orchestration surface the API exposes. It is
// exactly the subset of supervise.Manager the control node drives remotely, so
// *supervise.Manager satisfies it and tests can substitute a fake.
type Supervisor interface {
	EnsureRunning(ctx context.Context, k supervise.Key, repoName string) (int, error)
	OfferWireSpec(k supervise.Key, s supervise.WireSpec)
	Stop(k supervise.Key, reason string)
	Status(k supervise.Key) string
	LastFailure(k supervise.Key) (supervise.Failure, bool)
	Report(ctx context.Context) []supervise.ProcReport
	RunLog(repoName, side, hash string, attempt int, offset int64) (supervise.RunLog, error)
	Exec(ctx context.Context, k supervise.Key, opts supervise.ExecOptions, stream io.ReadWriter) error
	Running() int
	MaxWarm() int
	SetMaxWarm(n int)
	DrainEvents() []supervise.ProcEventRecord
	MinWarm() int
	SetMinWarm(n int)
	IdleOverride() time.Duration
	SetIdleOverride(d time.Duration)
	HitStats() (warm, cold int64)
}

// Server exposes a Supervisor over HTTP behind a shared-secret check. Mount
// Handler() on a private listener only.
type Server struct {
	sup      Supervisor
	secret   string
	draining atomic.Bool
}

// NewServer wraps sup.
func NewServer(sup Supervisor, secret string) *Server {
	return &Server{sup: sup, secret: secret}
}

// SetDraining marks the worker draining (or not). A draining worker keeps
// serving what is already warm but reports Draining in its heartbeat so the
// control node's placement stops sending it new work — the pre-terminate half
// of an ASG scale-in lifecycle hook.
func (s *Server) SetDraining(v bool) { s.draining.Store(v) }

// Handler returns the authenticated worker-API mux.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(pathEnsure, s.handleEnsure)
	mux.HandleFunc(pathStop, s.handleStop)
	mux.HandleFunc(pathStatus, s.handleStatus)
	mux.HandleFunc(pathHeartbeat, s.handleHeartbeat)
	mux.HandleFunc(pathDrain, s.handleDrain)
	mux.HandleFunc(pathReport, s.handleReport)
	mux.HandleFunc(pathRunLog, s.handleRunLog)
	mux.HandleFunc(pathConfigure, s.handleConfigure)
	mux.HandleFunc(pathExec, s.handleExec)
	return s.authed(mux)
}

// authed rejects any request without the shared secret, in constant time.
func (s *Server) authed(next http.Handler) http.Handler {
	return bearerAuth(s.secret, next)
}

// bearerAuth wraps next in a constant-time "Authorization: Bearer <secret>"
// check, rejecting an empty secret outright. Shared by both directions of the
// protocol (the worker's Server and the control node's ControlServer).
func bearerAuth(secret string, next http.Handler) http.Handler {
	want := "Bearer " + secret
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get(AuthHeader)
		if secret == "" || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleEnsure(w http.ResponseWriter, r *http.Request) {
	var req ensureReq
	if !decode(w, r, &req) {
		return
	}
	// Offer the control-resolved specs before the ensure so the Manager's
	// spec lookup finds them; its own (empty) DB is still consulted first.
	if req.Spec != nil {
		s.sup.OfferWireSpec(req.Key.toKey(), *req.Spec)
	}
	if req.PeerSpec != nil && req.Key.Peer != "" {
		s.sup.OfferWireSpec(supervise.BackendKey(req.Key.RepoID, req.Key.Peer), *req.PeerSpec)
	}
	port, err := s.sup.EnsureRunning(r.Context(), req.Key.toKey(), req.Repo)
	if err != nil {
		// A failed start is a normal outcome (crash, missing artifact), reported
		// to the control node as a 502 with the detail so its proxy can render
		// the same "failed to start" page a local start would.
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, ensureResp{Port: port})
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	var req stopReq
	if !decode(w, r, &req) {
		return
	}
	s.sup.Stop(req.Key.toKey(), req.Reason)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	var req statusReq
	if !decode(w, r, &req) {
		return
	}
	k := req.Key.toKey()
	resp := statusResp{Status: s.sup.Status(k)}
	if resp.Status == supervise.StatusCrashed {
		if f, ok := s.sup.LastFailure(k); ok {
			resp.Error = f.Detail
		}
	}
	writeJSON(w, resp)
}

func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	procs := s.sup.Report(r.Context())
	resp := reportResp{Procs: make([]wireProc, 0, len(procs))}
	for _, p := range procs {
		resp.Procs = append(resp.Procs, wireProc{
			Key: fromKey(p.Key), Repo: p.Repo, Status: p.Status, Error: p.Error, Stats: p.Stats,
			LastTouch: p.LastTouch,
		})
	}
	writeJSON(w, resp)
}

func (s *Server) handleRunLog(w http.ResponseWriter, r *http.Request) {
	var req runLogReq
	if !decode(w, r, &req) {
		return
	}
	chunk, err := s.sup.RunLog(req.Repo, req.Side, req.Hash, req.Attempt, req.Offset)
	if err != nil && chunk.Attempt == 0 {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, chunk)
}

func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	warm, cold := s.sup.HitStats()
	writeJSON(w, Heartbeat{
		Running:            s.sup.Running(),
		MaxWarm:            s.sup.MaxWarm(),
		MinWarm:            s.sup.MinWarm(),
		IdleTimeoutSeconds: int(s.sup.IdleOverride() / time.Second),
		Draining:           s.draining.Load(),
		WarmHits:           warm,
		ColdStarts:         cold,
		// Drained here, delivered at-most-once: a response lost in transit
		// loses its batch, an accepted trade for statistics.
		Events: s.sup.DrainEvents(),
	})
}

func (s *Server) handleConfigure(w http.ResponseWriter, r *http.Request) {
	var req WorkerConfig
	if !decode(w, r, &req) {
		return
	}
	if req.MaxWarm != nil {
		s.sup.SetMaxWarm(*req.MaxWarm)
	}
	if req.MinWarm != nil {
		s.sup.SetMinWarm(*req.MinWarm)
	}
	if req.IdleTimeoutSeconds != nil {
		s.sup.SetIdleOverride(time.Duration(*req.IdleTimeoutSeconds) * time.Second)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDrain(w http.ResponseWriter, r *http.Request) {
	var req drainReq
	if !decode(w, r, &req) {
		return
	}
	s.draining.Store(req.Draining)
	w.WriteHeader(http.StatusNoContent)
}

// handleExec upgrades the request to an execstream WebSocket and runs one
// exec session against the local supervisor. The key and session options
// arrive as query parameters; failures after the upgrade arrive as FrameError
// on the stream, exactly as the apex endpoint reports them.
func (s *Server) handleExec(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	repoID, err := strconv.ParseInt(q.Get("repo_id"), 10, 64)
	if err != nil {
		http.Error(w, "bad repo_id", http.StatusBadRequest)
		return
	}
	k := supervise.Key{
		RepoID: repoID,
		Side:   supervise.Side(q.Get("side")),
		Hash:   q.Get("hash"),
		Peer:   q.Get("peer"),
	}
	opts := supervise.ExecOptions{
		Cmd:   q["cmd"],
		TTY:   q.Get("tty") == "1",
		Stdin: q.Get("stdin") == "1",
		Term:  q.Get("term"),
	}
	if len(opts.Cmd) == 0 {
		http.Error(w, "missing cmd parameter", http.StatusBadRequest)
		return
	}
	conn, err := execstream.Accept(w, r)
	if err != nil {
		return // Accept wrote the handshake failure
	}
	defer conn.Close()
	// The request context dies with the hijacked request; the session's
	// lifetime is the connection itself.
	if err := s.sup.Exec(context.Background(), k, opts, conn); err != nil {
		execstream.NewWriter(conn).WriteFrame(execstream.FrameError, []byte(err.Error())) //nolint:errcheck // conn may already be gone
	}
}

// maxWorkerBody caps request bodies before decoding. The largest request is
// an ensure carrying two run-config JSON blobs (argv + env from the manifest,
// a few KiB in practice), so 1 MiB is ample headroom while still keeping a
// compromised or misconfigured-network peer from forcing unbounded allocation
// against this RCE-adjacent surface.
const maxWorkerBody = 1 << 20 // 1 MiB

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxWorkerBody)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
