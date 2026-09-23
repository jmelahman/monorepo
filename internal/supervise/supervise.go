// Package supervise owns preview process lifecycles: one process per
// distinct artifact per repo (backends, and process-mode frontends),
// started on demand, health-checked, and stopped as a unit. A process runs
// either directly on the host (killed as a process group) or, with a
// manifest run_image, inside a labeled container. It also provisions
// backend state directories via lineage forking — the state for a new
// backend hash is copied from the nearest deployed ancestor's backend,
// quiesced first.
//
// The in-memory process table is authoritative while the orchestrator runs;
// process_records rows (host processes) and container labels (containered
// processes) exist only so an unclean exit can be reclaimed at next
// startup.
package supervise

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/devcontainer"
	"github.com/jmelahman/local-preview/internal/dockerapi"
	"github.com/jmelahman/local-preview/internal/gitrepo"
	"github.com/jmelahman/local-preview/internal/manifest"
	"github.com/jmelahman/local-preview/internal/store"
)

// AncestryLimit caps how far the lineage walk searches for a fork source
// before falling back to a fresh state dir.
const AncestryLimit = 500

// stopGrace is how long a SIGTERM'd process gets before SIGKILL.
const stopGrace = 5 * time.Second

// Process states reported by Status.
const (
	StatusIdle     = "idle"     // not running; a start is one request away
	StatusStarting = "starting" // a start is in flight
	StatusRunning  = "running"  // healthy
	StatusCrashed  = "crashed"  // the last run or start attempt failed
)

// Side distinguishes the two supervised process kinds.
type Side string

// Sides.
const (
	SideBackend  Side = "be"
	SideFrontend Side = "fe"
)

// Key identifies a supervised process: one per artifact per repo. Peer is
// the deploy's backend hash for process-mode frontends — a {backend_url}
// env binding makes a frontend instance specific to one backend, while the
// built artifact stays shared per fe_hash.
type Key struct {
	RepoID int64
	Side   Side
	Hash   string
	Peer   string
}

// BackendKey identifies a backend process.
func BackendKey(repoID int64, beHash string) Key {
	return Key{RepoID: repoID, Side: SideBackend, Hash: beHash}
}

// FrontendKey identifies a process-mode frontend paired with the deploy's
// backend.
func FrontendKey(repoID int64, feHash, beHash string) Key {
	return Key{RepoID: repoID, Side: SideFrontend, Hash: feHash, Peer: beHash}
}

// Manager supervises preview processes.
type Manager struct {
	db      *db.Store
	files   *store.Store
	logsDir string

	healthInterval time.Duration
	reapInterval   time.Duration
	maxWarm        atomic.Int64 // soft target for warm processes; 0 = unlimited
	minWarm        atomic.Int64 // floor: this many most-recent processes never idle out
	idleOverride   atomic.Int64 // ns; > 0 overrides every manifest idle_timeout

	// warmHits and coldStarts count EnsureRunning outcomes since this
	// Manager started: a warm hit reused an already-tracked process, a cold
	// start kicked off a new one. Reported in the worker heartbeat so the
	// control node can aggregate a fleet-wide warm ratio.
	warmHits   atomic.Int64
	coldStarts atomic.Int64

	mu        sync.Mutex
	procs     map[Key]*process
	locks     map[Key]*sync.Mutex
	failures  map[Key]Failure  // last unexpected exit / failed start, per key
	wireSpecs map[Key]WireSpec // control-supplied run specs (worker serving)
	stopping  bool             // set by StopAll; refuses new starts during shutdown

	// initRevoked marks backend artifacts whose recorded init-done is no
	// longer trusted on this node: a start that skipped init because the
	// flag said "done" then failed, so the effects init owns (a per-preview
	// database on a shared Postgres, say) may have been reaped out from
	// under the flag. A revocation outranks BOTH the sticky wire cache and
	// the control node's next offer — the control DB may legitimately still
	// say done — until an init actually succeeds here again.
	initRevoked map[Key]bool

	// eventBuf holds process events since the last DrainEvents, shipped to
	// the control node in the worker heartbeat. Events land in this node's
	// own database too, but a worker's database is ephemeral — the control's
	// process_events trail (which feeds the startup-latency percentiles) only
	// sees them via this buffer. Capped: on a node nothing drains (a
	// single-node deployment, or the control's own idle Manager) it fills
	// once and stops growing.
	eventMu  sync.Mutex
	eventBuf []ProcEventRecord

	stateDirWarned bool   // one {state_dir}-over-workers warning per process
	publishIP      string // extra host address container ports publish on

	dockerMu     sync.Mutex
	dockerProbed bool
	dockerCli    *dockerapi.Client
	dockerErr    error

	// netMu serializes deploy-network membership changes: a container's
	// resolve-or-create → start window against a peer's release. Docker only
	// refuses to remove a network once an endpoint is attached, and endpoints
	// attach at container *start*, not create — so without the lock a backend
	// exiting between its frontend's EnsureNetwork and StartContainer would
	// delete the network out from under the frontend.
	netMu sync.Mutex
}

// New returns a Manager. logsDir is the root for per-process run logs.
func New(database *db.Store, files *store.Store, logsDir string) *Manager {
	return &Manager{
		db:             database,
		files:          files,
		logsDir:        logsDir,
		healthInterval: 200 * time.Millisecond,
		reapInterval:   30 * time.Second,
		procs:          make(map[Key]*process),
		locks:          make(map[Key]*sync.Mutex),
		failures:       make(map[Key]Failure),
		wireSpecs:      make(map[Key]WireSpec),
		initRevoked:    make(map[Key]bool),
	}
}

// Failure is why a key stopped serving: an unintentional non-zero exit, or a
// start attempt that never reached healthy. It outlives the process itself
// so a dead service reads "crashed" rather than an indistinguishable "idle",
// and is cleared by the next start attempt or a deliberate stop.
type Failure struct {
	Detail string
	At     time.Time
}

// noteFailure records why the key stopped serving.
func (m *Manager) noteFailure(k Key, detail string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failures[k] = Failure{Detail: detail, At: time.Now()}
}

// noteExit records a healthy process dying on its own — the crash a user
// notices as a preview that stopped answering. A clean exit isn't a failure,
// and a process that never went healthy belongs to the start attempt's own
// failure path, which knows what it was waiting for.
func (m *Manager) noteExit(k Key, p *process) {
	if p.exitOK || !isReady(p) {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	// This runs after the key is forgotten, so a start racing the exit may
	// already own it; that attempt's outcome is the current one, and a
	// report from the process it replaced would outlive its subject.
	if cur, ok := m.procs[k]; ok && cur != p {
		return
	}
	m.failures[k] = Failure{Detail: p.exit, At: time.Now()}
}

// clearFailure drops a recorded failure: a new start attempt or a deliberate
// stop supersedes whatever the last run did.
func (m *Manager) clearFailure(k Key) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.failures, k)
}

// LastFailure returns the failure behind a "crashed" status, if one is
// recorded for the key.
func (m *Manager) LastFailure(k Key) (Failure, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.failures[k]
	return f, ok
}

// LiveArtifacts returns the set of artifact sides with a tracked process
// (starting or running), keyed as "<repoName>\x00<side>\x00<hash>". Cache
// eviction consults it to avoid reclaiming a resident artifact whose files a
// live process may still depend on (a containered preview bind-mounts them at
// start). Keyed by repo name to match the store's on-disk layout.
func (m *Manager) LiveArtifacts() map[string]struct{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]struct{}, len(m.procs))
	for k, p := range m.procs {
		out[p.repoName+"\x00"+string(k.Side)+"\x00"+k.Hash] = struct{}{}
	}
	return out
}

// IsArtifactLive reports whether a specific artifact side currently has a
// tracked process. Cache eviction calls it per candidate immediately before
// deleting, so a process that a start registered after the sweep began is still
// seen — a snapshot taken once per sweep would be TOCTOU against a fresh start
// whose container is about to bind-mount the very files being reclaimed.
func (m *Manager) IsArtifactLive(repoName, side, hash string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, p := range m.procs {
		if p.repoName == repoName && string(k.Side) == side && k.Hash == hash {
			return true
		}
	}
	return false
}

// CrashedKeys returns every key Status would currently report as
// "crashed" — a recorded failure with no process since started under it.
// Runtime state lives only here, so a listing that wants to filter on it
// has to ask for the set and match deploy rows against it.
func (m *Manager) CrashedKeys() []Key {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := make([]Key, 0, len(m.failures))
	for k := range m.failures {
		if m.procs[k] == nil {
			keys = append(keys, k)
		}
	}
	return keys
}

// SetMaxWarm sets the warm *target*: beyond it the reaper prunes the
// least-recently-used processes back down — but only ones that are actually
// idle (untouched for warmActiveWindow). Actively-used previews are never
// stopped to satisfy the target, so a burst of simultaneous visitors is
// served in full and pruned back once it quiets down. Non-positive disables
// the target. Safe at runtime — the control node pushes the
// dashboard-configured value to workers while they serve.
func (m *Manager) SetMaxWarm(n int) {
	m.maxWarm.Store(int64(max(n, 0)))
}

// SetMinWarm sets the warm floor: the n most-recently-used processes are
// exempt from the idle timeout (and from target pruning), so the previews a
// developer is most likely to revisit stay hot indefinitely. 0 (the default)
// keeps today's behavior. Safe at runtime, like SetMaxWarm.
func (m *Manager) SetMinWarm(n int) {
	m.minWarm.Store(int64(max(n, 0)))
}

// MinWarm returns the warm floor. Echoed in the worker heartbeat so the
// control node's reconcile loop can re-push it after a worker reboot.
func (m *Manager) MinWarm() int { return int(m.minWarm.Load()) }

// MaxWarm returns the warm-process cap (0 = unlimited). Reported in the worker
// heartbeat so the control node can size fleet-wide capacity.
func (m *Manager) MaxWarm() int { return int(m.maxWarm.Load()) }

// SetIdleOverride overrides every manifest's idle_timeout with d (0 restores
// per-manifest values). Applied at reap time rather than start time, so a
// dashboard change affects processes that are already running — shortening it
// reaps sooner on the next tick, lengthening it keeps warm previews warm.
func (m *Manager) SetIdleOverride(d time.Duration) {
	m.idleOverride.Store(int64(max(d, 0)))
}

// IdleOverride returns the server-wide idle-timeout override (0 = none).
// Reported in the worker heartbeat so the control node's reconcile loop can
// detect a worker that missed the push (fresh boot).
func (m *Manager) IdleOverride() time.Duration { return time.Duration(m.idleOverride.Load()) }

// SetPublishIP additionally publishes containered processes' ports on the
// given host address. A worker must set it to the address the control node
// dials (its private IP): container ports default to loopback-only, which a
// remote proxy can never reach. Host processes are unaffected — they follow
// the manifest's bind address. Call before serving.
func (m *Manager) SetPublishIP(ip string) { m.publishIP = ip }

// Running returns the number of tracked processes (starting or running) — the
// worker's committed warm slots, reported in the heartbeat.
func (m *Manager) Running() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.procs)
}

// HitStats returns the cumulative warm-hit and cold-start counts since this
// Manager started. Reported in the worker heartbeat so the control node can
// compute a fleet-wide warm ratio.
func (m *Manager) HitStats() (warm, cold int64) {
	return m.warmHits.Load(), m.coldStarts.Load()
}

// StartReaper launches the loop enforcing idle timeouts and the warm cap.
func (m *Manager) StartReaper(ctx context.Context) {
	go func() {
		tick := time.NewTicker(m.reapInterval)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				m.reap()
			}
		}
	}()
}

// docker dials the daemon lazily, once — mirrors build.autoRunner.
func (m *Manager) docker(ctx context.Context) (*dockerapi.Client, error) {
	m.dockerMu.Lock()
	defer m.dockerMu.Unlock()
	if !m.dockerProbed {
		m.dockerProbed = true
		m.dockerCli, m.dockerErr = dockerapi.Connect(ctx)
	}
	return m.dockerCli, m.dockerErr
}

// process is one supervised child — a host child process (cmd set) or a
// container (containerID set). ready is closed once healthy; failed is
// closed if the start attempt errors (err is set first). done is closed
// when the process exits, however that happens.
type process struct {
	repoName    string
	port        int
	cmd         *exec.Cmd
	containerID string
	startedAt   time.Time // when the child/container launched

	lastTouch time.Time     // guarded by Manager.mu; recency for the reaper
	idleAfter time.Duration // idle_timeout from the run config

	statsMu sync.Mutex // guards prevCPU
	prevCPU cpuSample  // prior sample for CPU-percent deltas

	ready  chan struct{}
	failed chan struct{}
	done   chan struct{}
	err    error

	// exit summarizes how the process ended ("exit status 3", "exit 137",
	// "exit ok"); exitOK marks a zero status. Both are written before done
	// closes, so any reader that waited on done sees them.
	exit   string
	exitOK bool

	intentional bool // set (under the key lock) before a deliberate stop
}

func (m *Manager) keyLock(k Key) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.locks[k]
	if !ok {
		l = &sync.Mutex{}
		m.locks[k] = l
	}
	return l
}

// EnsureRunning returns the port of a healthy process for the artifact
// key, starting one if needed. The start itself is never cancelled by
// ctx — a caller that gives up (e.g. a proxy request with a short deadline)
// leaves the start running for the next request to pick up.
func (m *Manager) EnsureRunning(ctx context.Context, k Key, repoName string) (int, error) {
	for range 2 {
		m.mu.Lock()
		if m.stopping {
			m.mu.Unlock()
			return 0, fmt.Errorf("supervisor is shutting down")
		}
		p := m.procs[k]
		if p == nil {
			p = &process{
				repoName: repoName,
				ready:    make(chan struct{}),
				failed:   make(chan struct{}),
				done:     make(chan struct{}),
			}
			m.procs[k] = p
			// This attempt now owns the key's outcome.
			delete(m.failures, k)
			m.coldStarts.Add(1)
			go m.start(k, p)
		} else if isReady(p) {
			// Served by an already-healthy process — the warm experience the
			// hit ratio measures.
			m.warmHits.Add(1)
		} else {
			// Joined an in-flight cold start: no new process launches, but
			// this caller still waits out the start, so it counts cold — a
			// page load firing dozens of asset requests during one cold start
			// must not record dozens of "warm hits".
			m.coldStarts.Add(1)
		}
		p.lastTouch = time.Now()
		// SSR frontends call their backend directly over the deploy
		// network, invisible to the proxy — the pair's recency travels
		// with the frontend's touches.
		if k.Side == SideFrontend && k.Peer != "" {
			if bp := m.procs[BackendKey(k.RepoID, k.Peer)]; bp != nil {
				bp.lastTouch = p.lastTouch
			}
		}
		m.mu.Unlock()

		select {
		case <-p.ready:
			select {
			case <-p.done:
				// Died after becoming healthy; clear and retry once.
				m.forget(k, p)
				continue
			default:
				return p.port, nil
			}
		case <-p.failed:
			m.forget(k, p)
			return 0, p.err
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
	return 0, fmt.Errorf("%s %s: process exited immediately after start", k.Side, k.Hash[:12])
}

// LocalBackends adapts a Manager to the proxy's address-based Backends
// interface: a process supervised here runs on this host, so its address is
// loopback. This is the transport for single-node (--role=all) serving and for
// a worker serving its own local processes; the remote transport
// (workerapi.Client) satisfies the same interface across a network hop.
type LocalBackends struct{ M *Manager }

// EnsureRunning starts (or reuses) the keyed process and returns its loopback
// address.
func (l LocalBackends) EnsureRunning(ctx context.Context, k Key, repoName string) (string, error) {
	port, err := l.M.EnsureRunning(ctx, k, repoName)
	if err != nil {
		return "", err
	}
	return "127.0.0.1:" + strconv.Itoa(port), nil
}

// forget removes p from the table if it's still the tracked entry.
func (m *Manager) forget(k Key, p *process) {
	m.mu.Lock()
	if m.procs[k] == p {
		delete(m.procs, k)
	}
	m.mu.Unlock()
}

// runSpec is the side-independent run contract loaded from the artifact's
// stored run_config.
type runSpec struct {
	argv         []string
	runImage     string
	devc         devcontainer.Config // default runtime when runImage is empty
	env          map[string]string
	healthPath   string
	startTimeout time.Duration
	idleTimeout  time.Duration
	dir          string // artifact dir: host cwd, or container bind + workdir
	stateDir     string // backend only
	networks     []string

	// Backend init contract: one-time steps owed before the first run.
	init        [][]string
	initTimeout time.Duration
	initDone    bool

	// fromWire marks a spec served from the wire cache rather than this
	// node's DB — init completion is then recorded there too.
	fromWire bool
}

// Stored run_config shapes: the manifest section plus the top-level
// networks list and the resolved devcontainer (the side's default runtime
// when it pins no run_image), marshaled together at build time.
type backendRunConfig struct {
	manifest.Backend
	Networks     []string             `json:"networks,omitempty"`
	Devcontainer *devcontainer.Config `json:"devcontainer,omitempty"`
}

type frontendRunConfig struct {
	manifest.Frontend
	Networks     []string             `json:"networks,omitempty"`
	Devcontainer *devcontainer.Config `json:"devcontainer,omitempty"`
}

// WireSpec is the transportable run contract for one supervised process: the
// artifact-row fields a node needs to start it, independent of any control
// DB. It exists because a worker's DB never sees builds — the control node
// resolves the spec from its own DB (ResolveWireSpec) and ships it with the
// ensure request; the worker offers it to its Manager (OfferWireSpec), whose
// spec lookup falls back to it when the DB row is absent. RunConfig is the
// stored run_config JSON; InitDone mirrors init_done_at (backend only). The
// state dir deliberately does not travel: it is stored as an absolute
// control-node path, so each node recomputes it from identity (repo, hash)
// against its own store root.
type WireSpec struct {
	RunConfig string `json:"run_config"`
	InitDone  bool   `json:"init_done,omitempty"`
}

// ResolveWireSpec is the control-side half of the wire: the DB-backed run
// spec for k in transportable form, or an error naming the missing artifact.
func (m *Manager) ResolveWireSpec(k Key) (WireSpec, error) {
	if k.Side == SideFrontend {
		art, err := m.db.GetFrontendArtifact(k.RepoID, k.Hash)
		if err != nil {
			return WireSpec{}, fmt.Errorf("frontend artifact %s not provisioned: %w", shortHash(k.Hash), err)
		}
		return WireSpec{RunConfig: art.RunConfig}, nil
	}
	art, err := m.db.GetBackendArtifact(k.RepoID, k.Hash)
	if err != nil {
		return WireSpec{}, fmt.Errorf("backend artifact %s not provisioned: %w", shortHash(k.Hash), err)
	}
	if strings.Contains(art.RunConfig, "{state_dir}") {
		// Resolving for the wire means a worker will serve this, where state
		// dirs start fresh per node — no lineage forking, no cross-worker
		// state. Loud once so registering such a repo is a decision, not a
		// silent correctness bug.
		m.mu.Lock()
		warned := m.stateDirWarned
		m.stateDirWarned = true
		m.mu.Unlock()
		if !warned {
			log.Printf("WARNING: a manifest uses {state_dir} and previews are served by workers — worker state dirs start fresh per node (no lineage forking); key per-commit state on {hash} in external services instead")
		}
	}
	return WireSpec{RunConfig: art.RunConfig, InitDone: art.InitDoneAt != ""}, nil
}

// OfferWireSpec caches a control-resolved run spec for k. The DB stays the
// authority — spec lookup consults it first — so offering a spec on a node
// that also builds changes nothing. InitDone is sticky: a later offer with
// InitDone=false must not make every cold start here re-run init. The control
// node normally adopts a worker's init result off the ensure response
// (AdoptRemoteInitDone), so this is the backstop for the offers that race
// that write, not the only guard. Stickiness stops at a revocation
// (revokeInitDone): a failed start that skipped init must re-run it here even
// while the offer still says done.
func (m *Manager) OfferWireSpec(k Key, s WireSpec) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if prev, ok := m.wireSpecs[k]; ok && prev.InitDone {
		s.InitDone = true
	}
	if m.initRevoked[k] {
		// A revocation beats both the sticky cache and the offer: this node
		// watched a start that skipped init fail, and the control DB (whose
		// row this offer mirrors) can't know that yet.
		s.InitDone = false
	}
	m.wireSpecs[k] = s
}

func (m *Manager) wireSpec(k Key) (WireSpec, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.wireSpecs[k]
	return s, ok
}

// AdoptRemoteInitDone records in this node's DB a backend init that completed
// on a worker. The caller (workerapi.Client) proves it from a successful
// backend ensure — the worker fails the ensure on init failure — and init is
// once per ARTIFACT, not per node: its effects live in external services
// keyed on {hash}, never in worker-local state ({state_dir} on workers is
// loudly unsupported — see ResolveWireSpec). Without adoption, init_done_at
// stays false forever on a control node that never serves locally, and every
// fresh worker re-runs init.
func (m *Manager) AdoptRemoteInitDone(k Key) error {
	return m.db.MarkBackendInitDone(k.RepoID, k.Hash)
}

// markInitDone records a completed init where the spec came from: the DB row
// on a node that builds, or the wire cache on a worker (whose DB has no row
// to update — OfferWireSpec's stickiness is what keeps init from re-running
// on this node's later cold starts).
func (m *Manager) markInitDone(k Key, spec runSpec) error {
	m.mu.Lock()
	delete(m.initRevoked, k)
	if s, ok := m.wireSpecs[k]; ok {
		s.InitDone = true
		m.wireSpecs[k] = s
	}
	m.mu.Unlock()
	if !spec.fromWire {
		return m.db.MarkBackendInitDone(k.RepoID, k.Hash)
	}
	return nil
}

// initIsRevoked reports whether k's recorded init-done has been revoked here.
func (m *Manager) initIsRevoked(k Key) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.initRevoked[k]
}

// revokeInitDone withdraws k's recorded init-done so the NEXT start re-runs
// the manifest's init steps. It is the self-healing half of the once-per-
// artifact rule: init_done_at records that init's effects exist, but those
// effects can live in a service this node doesn't own (the per-preview
// Postgres database an onyx init creates), and something out there can
// delete them. A start that skipped init and then failed is the signal —
// the flag is claiming an effect that may be gone.
//
// The revocation is in-memory (it must outrank the wire cache and every
// later offer, which mirror a control DB that still says done) and, on a
// node that owns the row, persisted by clearing init_done_at so it survives
// a restart and reaches fresh workers through ResolveWireSpec.
func (m *Manager) revokeInitDone(k Key, spec runSpec, reason string) {
	m.mu.Lock()
	m.initRevoked[k] = true
	if s, ok := m.wireSpecs[k]; ok {
		s.InitDone = false
		m.wireSpecs[k] = s
	}
	m.mu.Unlock()
	if !spec.fromWire {
		if err := m.db.ClearBackendInitDone(k.RepoID, k.Hash); err != nil {
			log.Printf("revoke init done for %s %s: %v", k.Side, shortHash(k.Hash), err)
		}
	}
	m.recordEvent(k.RepoID, k.Hash, "init_revoked", reason)
	log.Printf("init revoked for %s %s: %s; init re-runs on the next start", k.Side, shortHash(k.Hash), reason)
}

// RevokeInitDone is the control-side mirror of revokeInitDone: a worker
// reported that a start which skipped init failed, so this node's record of
// that init is withdrawn (row cleared, revocation cached) and the next
// ensure ships InitDone=false. Symmetric to AdoptRemoteInitDone, and driven
// by workerapi.Client's InitRevoker hook.
func (m *Manager) RevokeInitDone(k Key) error {
	m.mu.Lock()
	m.initRevoked[k] = true
	if s, ok := m.wireSpecs[k]; ok {
		s.InitDone = false
		m.wireSpecs[k] = s
	}
	m.mu.Unlock()
	m.recordEvent(k.RepoID, k.Hash, "init_revoked", "worker start failed after skipping init")
	log.Printf("init revoked for %s %s: worker start failed after skipping init; init re-runs on the next start", k.Side, shortHash(k.Hash))
	return m.db.ClearBackendInitDone(k.RepoID, k.Hash)
}

// loadRunSpec resolves the artifact's run contract for either side: the DB
// row when this node has it, else the control-supplied wire spec. Both
// sources carry the same run_config JSON and feed the same construction
// below, so the two transports cannot drift.
func (m *Manager) loadRunSpec(k Key, repoName string) (runSpec, error) {
	if k.Side == SideFrontend {
		ws := WireSpec{}
		fromWire := false
		if art, err := m.db.GetFrontendArtifact(k.RepoID, k.Hash); err == nil {
			ws.RunConfig = art.RunConfig
		} else if ws, fromWire = m.wireSpec(k); !fromWire {
			return runSpec{}, fmt.Errorf("frontend artifact %s not provisioned: %w", shortHash(k.Hash), err)
		}
		var cfg frontendRunConfig
		if err := json.Unmarshal([]byte(ws.RunConfig), &cfg); err != nil {
			return runSpec{}, fmt.Errorf("parse run config: %w", err)
		}
		env, err := resolveSecretEnv(cfg.Env)
		if err != nil {
			return runSpec{}, err
		}
		return runSpec{
			argv:         cfg.Run,
			runImage:     cfg.RunImage,
			devc:         derefDevc(cfg.Devcontainer),
			env:          env,
			healthPath:   cfg.HealthPath,
			startTimeout: time.Duration(cfg.StartTimeout),
			idleTimeout:  idleOrDefault(time.Duration(cfg.IdleTimeout)),
			dir:          m.files.FrontendDir(repoName, k.Hash),
			networks:     cfg.Networks,
			fromWire:     fromWire,
		}, nil
	}
	ws := WireSpec{}
	fromWire := false
	stateDir := ""
	if art, err := m.db.GetBackendArtifact(k.RepoID, k.Hash); err == nil {
		ws = WireSpec{RunConfig: art.RunConfig, InitDone: art.InitDoneAt != ""}
		stateDir = art.StateDir
	} else if ws, fromWire = m.wireSpec(k); !fromWire {
		return runSpec{}, fmt.Errorf("backend artifact %s not provisioned: %w", shortHash(k.Hash), err)
	} else {
		// The wire carries no state-dir path: state is node-local, and a
		// worker's starts fresh (lineage forking is a build-time, control-node
		// concern — see the {state_dir} limitation in the worker docs).
		// Recompute against this node's store root and make sure the dir
		// exists: init steps and container bind mounts expect a directory.
		if err := m.files.InitFreshStateDir(repoName, k.Hash); err != nil {
			return runSpec{}, err
		}
		stateDir = m.files.StateDirPath(repoName, k.Hash)
	}
	var cfg backendRunConfig
	if err := json.Unmarshal([]byte(ws.RunConfig), &cfg); err != nil {
		return runSpec{}, fmt.Errorf("parse run config: %w", err)
	}
	env, err := resolveSecretEnv(cfg.Env)
	if err != nil {
		return runSpec{}, err
	}
	return runSpec{
		argv:         cfg.Run,
		runImage:     cfg.RunImage,
		devc:         derefDevc(cfg.Devcontainer),
		env:          env,
		healthPath:   cfg.HealthPath,
		startTimeout: time.Duration(cfg.StartTimeout),
		idleTimeout:  idleOrDefault(time.Duration(cfg.IdleTimeout)),
		dir:          m.files.BackendDir(repoName, k.Hash),
		stateDir:     stateDir,
		networks:     cfg.Networks,
		init:         cfg.Init,
		initTimeout:  time.Duration(cfg.InitTimeout),
		initDone:     ws.InitDone && !m.initIsRevoked(k),
		fromWire:     fromWire,
	}, nil
}

// shortHash truncates a hash for error text without assuming its length.
func shortHash(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

// secretRe matches the {secret:NAME} env placeholder.
var secretRe = regexp.MustCompile(`\{secret:([A-Za-z_][A-Za-z0-9_]*)\}`)

// SecretEnvPrefix namespaces the orchestrator environment variables that
// manifest env values may reference via {secret:NAME}. The prefix is the
// authorization: manifests are repo-controlled, so an unrestricted
// pass-through would let any registered repo's preview.toml exfiltrate
// control-plane env (the SSO client secret, worker shared secret, ...).
// Only variables deliberately published under this prefix are reachable.
const SecretEnvPrefix = "PREVIEW_SECRET_"

// resolveSecretEnv expands {secret:NAME} in env values from the serving
// node's own environment (PREVIEW_SECRET_<NAME>). Expansion is start-time
// and node-local on purpose: the stored run config and the worker wire spec
// carry only the placeholder, so credentials never land in the DB, in
// Terraform-rendered files, or in state — each node reads its own copy from
// its boot-time env file. A referenced secret that is unset fails the start
// loudly rather than exporting an empty credential.
func resolveSecretEnv(env map[string]string) (map[string]string, error) {
	touched := false
	out := make(map[string]string, len(env))
	var missing []string
	for k, v := range env {
		expanded := secretRe.ReplaceAllStringFunc(v, func(m string) string {
			touched = true
			name := SecretEnvPrefix + secretRe.FindStringSubmatch(m)[1]
			val, ok := os.LookupEnv(name)
			if !ok {
				missing = append(missing, name)
			}
			return val
		})
		out[k] = expanded
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("env references undefined secrets: set %s on this node", strings.Join(missing, ", "))
	}
	if !touched {
		return env, nil
	}
	return out, nil
}

// idleOrDefault covers run configs stored before idle enforcement existed.
func idleOrDefault(d time.Duration) time.Duration {
	if d <= 0 {
		return manifest.DefaultIdleTimeout
	}
	return d
}

func derefDevc(d *devcontainer.Config) devcontainer.Config {
	if d == nil {
		return devcontainer.Config{}
	}
	return *d
}

// runtimeEnv is the resolved container runtime for one process start: which
// image (empty = host), the extra cache-volume binds, and the HOME inside
// the container. devc marks the image as the repo devcontainer default
// rather than an explicit run_image.
type runtimeEnv struct {
	image  string
	mounts []devcontainer.Mount
	home   string
	devc   bool
}

// resolveRuntime picks the process runtime. An explicit run_image is a hard
// container contract (starting without a daemon fails later, loudly); the
// devcontainer is a soft default — when no daemon is reachable the process
// runs on the host, and the returned note says so.
func (m *Manager) resolveRuntime(spec runSpec) (runtimeEnv, string) {
	if spec.runImage != "" {
		return runtimeEnv{image: spec.runImage}, ""
	}
	if spec.devc.Image == "" {
		return runtimeEnv{}, ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := m.docker(ctx); err != nil {
		return runtimeEnv{}, fmt.Sprintf("[devcontainer %s unavailable (%v); running on the host]", spec.devc.Image, err)
	}
	return runtimeEnv{
		image:  spec.devc.Image,
		mounts: spec.devc.Mounts,
		home:   spec.devc.Home,
		devc:   true,
	}, ""
}

// start runs the full start sequence: load run config, run init steps if
// the artifact still owes them, probe a port, exec (host or container),
// health-poll. It owns its own timeouts and runs detached from any request
// context. Exactly one start goroutine exists per tracked process.
func (m *Manager) start(k Key, p *process) {
	lock := m.keyLock(k)
	lock.Lock()
	defer lock.Unlock()

	// fail ends the attempt and leaves the reason behind: nothing is serving
	// this key, and Status must say so instead of reading "idle". A process
	// that exited before it was ever healthy is reported here — the exit
	// watcher defers to the start, which knows what it was waiting for.
	fail := func(event string, err error) {
		p.err = err
		m.recordEvent(k.RepoID, k.Hash, event, err.Error())
		m.noteFailure(k, err.Error())
		close(p.failed)
		m.forget(k, p)
	}

	spec, err := m.loadRunSpec(k, p.repoName)
	if err != nil {
		fail("start_attempt", err)
		return
	}
	p.idleAfter = spec.idleTimeout
	if _, err := os.Stat(spec.dir); err != nil {
		// Local disk is a cache of the durable tier. A missing artifact is not
		// necessarily an eviction: it may simply not be resident on this node.
		// Try to hydrate it back before concluding it is gone. With no tier
		// configured (single-node default) Hydrate fails immediately and this
		// stays the original evicted failure.
		if herr := m.files.Hydrate(context.Background(), p.repoName, string(k.Side), k.Hash); herr != nil {
			fail("start_attempt", fmt.Errorf("%s artifact files missing (evicted?): %w", k.Side, herr))
			return
		}
	}
	m.files.NoteAccess(p.repoName, string(k.Side), k.Hash)
	rt, rtNote := m.resolveRuntime(spec)

	// A frontend whose env references {backend_url} needs its peer backend
	// up first — the URL carries the backend's port.
	backendURL := ""
	if k.Side == SideFrontend && envReferences(spec.env, "{backend_url}") {
		if k.Peer == "" {
			fail("start_attempt", fmt.Errorf("{backend_url} referenced but deploy has no backend"))
			return
		}
		bePort, err := m.EnsureRunning(context.Background(), BackendKey(k.RepoID, k.Peer), p.repoName)
		if err != nil {
			fail("start_attempt", fmt.Errorf("start backend for {backend_url}: %w", err))
			return
		}
		if rt.image != "" {
			// The frontend runs containered, so its backend is containered
			// too (manifest-validated for run_image; the devcontainer default
			// applies to both sides alike): the deploy network resolves the
			// backend by alias.
			backendURL = fmt.Sprintf("http://backend:%d", bePort)
		} else {
			backendURL = fmt.Sprintf("http://127.0.0.1:%d", bePort)
		}
	}

	logFile, err := m.openRunLog(p.repoName, string(k.Side)+"-"+k.Hash)
	if err != nil {
		fail("start_attempt", err)
		return
	}
	if rtNote != "" {
		fmt.Fprintln(logFile, rtNote)
	}

	// Init runs at most once per backend artifact: the artifact's code is
	// immutable and nothing else writes its state dir, so a recorded success
	// holds for every later cold start. A failure leaves init_done_at unset
	// and the next start attempt retries from the first step.
	//
	// skippedInit marks the other case — init was owed but the flag said
	// done — because a start that skipped init and then never went healthy
	// is the one signal that the flag may be describing an effect that no
	// longer exists (see revokeInitDone).
	skippedInit := k.Side == SideBackend && len(spec.init) > 0 && spec.initDone
	if k.Side == SideBackend && len(spec.init) > 0 && !spec.initDone {
		m.recordEvent(k.RepoID, k.Hash, "init_attempt", fmt.Sprintf("%d steps", len(spec.init)))
		if err := m.runInit(k, p.repoName, spec, rt, logFile); err != nil {
			logFile.Close()
			fail("init_failed", err)
			return
		}
		if err := m.markInitDone(k, spec); err != nil {
			logFile.Close()
			fail("init_failed", fmt.Errorf("record init success: %w", err))
			return
		}
		m.recordEvent(k.RepoID, k.Hash, "init_done", "")
	}

	port, err := probeFreePort()
	if err != nil {
		logFile.Close()
		fail("start_attempt", err)
		return
	}
	p.port = port

	argv := templateArgv(spec.argv, port, spec.stateDir)
	envPairs := expandEnv(spec.env, p.repoName, k.Hash, port, spec.stateDir, backendURL)

	if rt.image != "" {
		if err := m.startContainer(k, p, spec, rt, argv, envPairs, port, logFile); err != nil {
			logFile.Close()
			fail("start_attempt", err)
			return
		}
	} else {
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Dir = spec.dir
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		cmd.Env = append(os.Environ(), envPairs...)
		newProcessGroup(cmd)
		if err := cmd.Start(); err != nil {
			logFile.Close()
			fail("start_attempt", fmt.Errorf("start %s: %w", k.Side, err))
			return
		}
		p.cmd = cmd
		p.startedAt = time.Now()
		m.recordEvent(k.RepoID, k.Hash, "start_attempt",
			fmt.Sprintf("pid %d port %d", cmd.Process.Pid, port))
		m.db.UpsertProcessRecord(db.ProcessRecord{
			RepoID: k.RepoID, BeHash: k.Hash,
			PID: cmd.Process.Pid, PGID: cmd.Process.Pid, Port: port,
		})

		// Reaper: closes done when the child exits and clears bookkeeping.
		go func() {
			waitErr := cmd.Wait()
			logFile.Close()
			p.exit, p.exitOK = "exit ok", waitErr == nil
			if waitErr != nil {
				p.exit = waitErr.Error()
			}
			close(p.done)
			m.forget(k, p)
			m.db.DeleteProcessRecord(k.RepoID, k.Hash)
			if !p.intentional {
				m.recordEvent(k.RepoID, k.Hash, "exited", p.exit)
				m.noteExit(k, p)
			}
		}()
	}

	if err := m.awaitHealthy(p, spec.healthPath, spec.startTimeout, port); err != nil {
		// Only kill what's still alive: force-removing an already-exited
		// container would cut its log stream before the crash output
		// (reaped with a drain grace) lands in the run log. Whether it died
		// on its own has to be read before the wait below, which makes every
		// process look exited.
		died := isExited(p)
		if !died {
			m.killProcess(p)
		}
		<-p.done
		event := "health_timeout"
		if died {
			// The exit status is a better answer than "never went healthy".
			event, err = "exited", fmt.Errorf("process exited during startup: %s (see run log)", p.exit)
		}
		if skippedInit {
			// Never on an attempt whose init actually ran: that one has
			// already paid for itself, and revoking there would re-run init
			// on every retry of a backend that simply cannot start.
			m.revokeInitDone(k, spec, event)
		}
		fail(event, err)
		return
	}
	m.recordEvent(k.RepoID, k.Hash, "healthy", fmt.Sprintf("port %d", port))
	close(p.ready)
	if m.MaxWarm() > 0 {
		// Enforce the warm cap promptly rather than waiting a reap tick.
		go m.reap()
	}
}

// isReady reports whether p passed its health check.
func isReady(p *process) bool {
	select {
	case <-p.ready:
		return true
	default:
		return false
	}
}

// warmActiveWindow is what "actively used" means to the warm target: a
// process touched within it is never pruned to satisfy the target — a burst
// of simultaneous visitors is served in full and pruned back once quiet.
const warmActiveWindow = 2 * time.Minute

// reap enforces the warm policy: a floor (the min-warm most-recent processes
// never idle out), per-process idle timeouts (or the server-wide override),
// and a soft warm target (beyond it, the least-recently-used *idle* processes
// are pruned — never actively-used ones). Backends inherit their running
// frontend's recency and are never reaped while that frontend runs: the
// frontend holds the backend's port in its env, so a backend restart (new
// port) would strand it.
func (m *Manager) reap() {
	type cand struct {
		k     Key
		touch time.Time
		idle  time.Duration
	}
	now := time.Now()
	m.mu.Lock()
	var cands []cand
	pairedTouch := make(map[Key]time.Time) // backend key → its live frontend's touch
	for k, p := range m.procs {
		if !isReady(p) || isExited(p) {
			continue
		}
		cands = append(cands, cand{k: k, touch: p.lastTouch, idle: p.idleAfter})
		if k.Side == SideFrontend && k.Peer != "" {
			bk := BackendKey(k.RepoID, k.Peer)
			if p.lastTouch.After(pairedTouch[bk]) {
				pairedTouch[bk] = p.lastTouch
			}
		}
	}
	m.mu.Unlock()

	// Most-recent first: the head of this order is what the floor protects,
	// the tail is what pruning takes. A pair's frontend sorts "older" than its
	// backend on equal touches, so it stops first.
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].touch.Equal(cands[j].touch) {
			return cands[i].k.Side == SideBackend && cands[j].k.Side == SideFrontend
		}
		return cands[i].touch.After(cands[j].touch)
	})
	floor := min(m.MinWarm(), len(cands))

	// The server-wide override beats the per-process value stamped at start,
	// so a dashboard change governs already-running previews too.
	override := m.IdleOverride()
	alive := cands[:floor:floor] // the floor is exempt from idle and pruning alike
	for _, c := range cands[floor:] {
		if _, paired := pairedTouch[c.k]; paired {
			alive = append(alive, c) // counted toward the target, never stopped
			continue
		}
		idle := c.idle
		if override > 0 {
			idle = override
		}
		if idle > 0 && now.Sub(c.touch) > idle {
			m.Stop(c.k, fmt.Sprintf("idle %s", now.Sub(c.touch).Round(time.Second)))
			continue
		}
		alive = append(alive, c)
	}

	target := m.MaxWarm()
	if target <= 0 || len(alive) <= target {
		return
	}
	// Prune oldest-first, but only genuinely idle candidates: an active
	// process above the target is demand, not waste — and sustained demand
	// above the target is the autoscaling signal, not something to kill.
	excess := len(alive) - target
	for i := len(alive) - 1; i >= floor && excess > 0; i-- {
		c := alive[i]
		if _, paired := pairedTouch[c.k]; paired {
			continue
		}
		if now.Sub(c.touch) < warmActiveWindow {
			continue // actively used; never pruned for the target
		}
		m.Stop(c.k, fmt.Sprintf("least-recently-used beyond warm target %d", target))
		excess--
	}
}

// expandEnv renders manifest env declarations into KEY=value pairs,
// substituting the run-time placeholders. {hash} is the side's own 12-char
// artifact hash so isolation follows artifact identity like state dirs do.
func expandEnv(env map[string]string, repo, hash string, port int, stateDir, backendURL string) []string {
	if len(env) == 0 {
		return nil
	}
	r := strings.NewReplacer(
		"{port}", strconv.Itoa(port),
		"{state_dir}", stateDir,
		"{repo}", repo,
		"{hash}", hash[:12],
		"{backend_url}", backendURL,
	)
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+r.Replace(v))
	}
	sort.Strings(out)
	return out
}

func envReferences(env map[string]string, placeholder string) bool {
	for _, v := range env {
		if strings.Contains(v, placeholder) {
			return true
		}
	}
	return false
}

// runInit executes the backend's init steps sequentially — on the host, or
// under the resolved container runtime in one-shot containers with the same
// mounts and external networks as the server — streaming output into the run
// log so init output leads the first cold start's log. Only {state_dir} is
// templated in argv (no port exists yet), and env vars referencing {port}
// are omitted for the same reason. The timeout spans all steps together.
func (m *Manager) runInit(k Key, repoName string, spec runSpec, rt runtimeEnv, logFile *os.File) error {
	timeout := spec.initTimeout
	if timeout <= 0 {
		timeout = manifest.DefaultInitTimeout
	}
	env := initEnv(spec.env, repoName, k.Hash, spec.stateDir)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for _, step := range spec.init {
		argv := make([]string, len(step))
		for i, a := range step {
			argv[i] = strings.ReplaceAll(a, "{state_dir}", spec.stateDir)
		}
		fmt.Fprintf(logFile, "[init] %s\n", strings.Join(argv, " "))
		var err error
		if rt.image != "" {
			err = m.runInitContainer(ctx, spec, rt, argv, env, logFile)
		} else {
			cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
			cmd.Dir = spec.dir
			cmd.Stdout = logFile
			cmd.Stderr = logFile
			cmd.Env = append(os.Environ(), env...)
			newProcessGroup(cmd)
			cmd.Cancel = func() error {
				return killGroup(cmd.Process.Pid)
			}
			err = cmd.Run()
		}
		if err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("init %q: timed out after %s", strings.Join(step, " "), timeout)
			}
			return fmt.Errorf("init %q: %w (see run log)", strings.Join(step, " "), err)
		}
	}
	return nil
}

// initEnv renders the manifest env for init steps. No port is assigned at
// init time, so variables referencing {port} are omitted entirely rather
// than given a bogus value.
func initEnv(env map[string]string, repo, hash, stateDir string) []string {
	filtered := make(map[string]string, len(env))
	for k, v := range env {
		if !strings.Contains(v, "{port}") {
			filtered[k] = v
		}
	}
	return expandEnv(filtered, repo, hash, 0, stateDir, "")
}

func isExited(p *process) bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

// awaitHealthy polls the health path until 200, the process exits, or the
// start timeout elapses.
func (m *Manager) awaitHealthy(p *process, healthPath string, timeout time.Duration, port int) error {
	if timeout <= 0 {
		timeout = manifest.DefaultStartTimeout
	}
	deadline := time.After(timeout)
	tick := time.NewTicker(m.healthInterval)
	defer tick.Stop()
	client := &http.Client{Timeout: time.Second}
	url := fmt.Sprintf("http://127.0.0.1:%d%s", port, healthPath)
	for {
		select {
		case <-p.done:
			return fmt.Errorf("process exited during startup (see run log)")
		case <-deadline:
			return fmt.Errorf("process did not become healthy within %s", timeout)
		case <-tick.C:
			resp, err := client.Get(url)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return nil
				}
			}
		}
	}
}

// ProcEventRecord is one process_events row with its timestamp, buffered on a
// worker and shipped to the control node in the heartbeat. OccurredAt is the
// event's true time on the worker: the control replays these into its own
// trail, and the startup percentiles are computed from timestamp deltas —
// stamping them at replay time would read every fleet cold start as ~0s.
type ProcEventRecord struct {
	RepoID     int64     `json:"repo_id"`
	BeHash     string    `json:"be_hash"`
	Event      string    `json:"event"`
	Detail     string    `json:"detail,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
}

// maxEventBuf bounds the heartbeat event buffer. At a 10s heartbeat cadence
// this is far beyond a burst; when it fills (nothing draining), newest events
// are dropped rather than growing without bound.
const maxEventBuf = 256

// recordEvent appends to this node's process_events trail and buffers the
// event for the worker heartbeat (DrainEvents).
func (m *Manager) recordEvent(repoID int64, beHash, event, detail string) {
	m.db.AddProcessEvent(repoID, beHash, event, detail) //nolint:errcheck // observability trail, best-effort
	m.eventMu.Lock()
	if len(m.eventBuf) < maxEventBuf {
		m.eventBuf = append(m.eventBuf, ProcEventRecord{
			RepoID: repoID, BeHash: beHash, Event: event, Detail: detail,
			OccurredAt: time.Now().UTC(),
		})
	}
	m.eventMu.Unlock()
}

// DrainEvents returns the events buffered since the last call and resets the
// buffer. At-most-once: the caller (the worker heartbeat handler) consumes
// them into one response, and a response lost in transit loses its batch —
// an accepted trade for statistics.
func (m *Manager) DrainEvents() []ProcEventRecord {
	m.eventMu.Lock()
	defer m.eventMu.Unlock()
	out := m.eventBuf
	m.eventBuf = nil
	return out
}

// Stop gracefully stops the process for key, if running. Reason lands in
// the process_events trail.
func (m *Manager) Stop(k Key, reason string) {
	lock := m.keyLock(k)
	lock.Lock()
	defer lock.Unlock()
	m.stopLocked(k, reason)
}

// stopLocked stops the tracked process while the caller holds the key lock.
func (m *Manager) stopLocked(k Key, reason string) {
	// Stopping is a deliberate act on this key either way, so it also
	// acknowledges a crash the key was still reporting.
	m.clearFailure(k)
	m.mu.Lock()
	p := m.procs[k]
	m.mu.Unlock()
	if p == nil {
		return
	}
	switch {
	case p.containerID != "":
		p.intentional = true
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if cli, err := m.docker(ctx); err == nil {
			// The daemon owns the SIGTERM→SIGKILL grace window.
			cli.StopContainer(ctx, p.containerID, int(stopGrace/time.Second)) //nolint:errcheck
		}
		select {
		case <-p.done:
		case <-time.After(stopGrace + 10*time.Second):
			m.killProcess(p)
			<-p.done
		}
	case p.cmd != nil && p.cmd.Process != nil:
		p.intentional = true
		pid := p.cmd.Process.Pid
		terminateGroup(pid) //nolint:errcheck
		select {
		case <-p.done:
		case <-time.After(stopGrace):
			m.killProcess(p)
			<-p.done
		}
	default:
		return
	}
	m.forget(k, p)
	m.recordEvent(k.RepoID, k.Hash, "idle_stop", reason)
}

// killProcess force-terminates either process kind.
func (m *Manager) killProcess(p *process) {
	if p.containerID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if cli, err := m.docker(ctx); err == nil {
			cli.RemoveContainer(ctx, p.containerID, true) //nolint:errcheck
		}
		return
	}
	if p.cmd != nil && p.cmd.Process != nil {
		killGroup(p.cmd.Process.Pid) //nolint:errcheck
	}
}

// StopAll gracefully stops every supervised process (orchestrator
// shutdown). Children never outlive the orchestrator by design: the manager
// is marked stopping first, so a start racing in from a background trigger
// (auto-start, an in-flight request) is either refused or lands in the
// table before the snapshot below and is stopped with the rest.
func (m *Manager) StopAll() {
	m.mu.Lock()
	m.stopping = true
	keys := make([]Key, 0, len(m.procs))
	for k := range m.procs {
		keys = append(keys, k)
	}
	m.mu.Unlock()
	var wg sync.WaitGroup
	for _, k := range keys {
		wg.Go(func() { m.Stop(k, "shutdown") })
	}
	wg.Wait()
}

// StopRepo gracefully stops every supervised process belonging to one repo
// (repo deletion).
func (m *Manager) StopRepo(repoID int64, reason string) {
	m.mu.Lock()
	keys := make([]Key, 0, len(m.procs))
	for k := range m.procs {
		if k.RepoID == repoID {
			keys = append(keys, k)
		}
	}
	// Crashed keys have no process left to stop; drop their records so a
	// deleted repo leaves nothing behind.
	for k := range m.failures {
		if k.RepoID == repoID {
			delete(m.failures, k)
		}
	}
	m.mu.Unlock()
	var wg sync.WaitGroup
	for _, k := range keys {
		wg.Go(func() { m.Stop(k, reason) })
	}
	wg.Wait()
}

// StartDeploy warm-starts the processes backing one deploy — the backend
// and, in process mode, the frontend — so a fresh deploy serves its first
// request without a cold start. Processes are shared per artifact hash, so a
// side already running (or mid-start) is joined, not duplicated. Blocks
// until the processes are healthy or failed; ctx bounds only this wait, not
// the starts themselves.
func (m *Manager) StartDeploy(ctx context.Context, row db.DeployRow) error {
	if row.BeHash != "" {
		if _, err := m.EnsureRunning(ctx, BackendKey(row.RepoID, row.BeHash), row.RepoName); err != nil {
			return fmt.Errorf("start backend: %w", err)
		}
	}
	if row.FeHash != "" {
		if _, err := m.db.GetFrontendArtifact(row.RepoID, row.FeHash); err == nil {
			if _, err := m.EnsureRunning(ctx, FrontendKey(row.RepoID, row.FeHash, row.BeHash), row.RepoName); err != nil {
				return fmt.Errorf("start frontend: %w", err)
			}
		}
	}
	return nil
}

// StopDeploy stops the backend and (process-mode) frontend processes backing
// one deploy, if running. Because processes are shared per artifact hash,
// this also quiesces any sibling deploy on the same hash; each cold-starts
// again on its next request. Build artifacts and the deploy row are left
// intact.
func (m *Manager) StopDeploy(row db.DeployRow, reason string) {
	if row.BeHash != "" {
		m.Stop(BackendKey(row.RepoID, row.BeHash), reason)
	}
	if row.FeHash != "" {
		if _, err := m.db.GetFrontendArtifact(row.RepoID, row.FeHash); err == nil {
			m.Stop(FrontendKey(row.RepoID, row.FeHash, row.BeHash), reason)
		}
	}
}

// GCDeploy reclaims the artifacts, backend state, logs, and process
// bookkeeping backing a deploy that no surviving (non-evicted) deploy of the
// same repo still references. Call it AFTER the triggering deploy row has
// been deleted or marked evicted, so the shared-hash checks observe the
// correct surviving set. Content-addressed artifacts are shared, so a hash
// still used by another deploy is left untouched. This is the reclaim
// primitive that both a manual delete and the retention sweep build on.
// Cleanup is best-effort: failures only leave unreachable bytes on disk, so
// they are logged, not returned.
func (m *Manager) GCDeploy(row db.DeployRow) {
	remaining, err := m.db.ListDeploys(db.DeployFilter{Repo: row.RepoName})
	if err != nil {
		log.Printf("gc deploy %d: list deploys: %v", row.ID, err)
		return
	}
	liveBe := map[string]bool{}
	liveFe := map[string]bool{}
	liveDL := map[string]bool{}
	for _, r := range remaining {
		// Evicted deploys hold no artifacts, so they pin nothing.
		if r.Status == db.DeployEvicted {
			continue
		}
		if r.BeHash != "" {
			liveBe[r.BeHash] = true
		}
		if r.FeHash != "" {
			liveFe[r.FeHash] = true
		}
		for _, ref := range r.Artifacts {
			liveDL[ref.Hash] = true
		}
	}

	if row.BeHash != "" && !liveBe[row.BeHash] {
		m.Stop(BackendKey(row.RepoID, row.BeHash), "deploy deleted")
		if err := m.db.DeleteBackendArtifact(row.RepoID, row.BeHash); err != nil {
			log.Printf("gc deploy %d: delete backend artifact: %v", row.ID, err)
		}
		if err := m.files.RemoveBackend(row.RepoName, row.BeHash); err != nil {
			log.Printf("gc deploy %d: remove backend files: %v", row.ID, err)
		}
		m.removeHashLogs(row.RepoName, "be", row.BeHash)
	}
	if row.FeHash != "" && !liveFe[row.FeHash] {
		if _, err := m.db.GetFrontendArtifact(row.RepoID, row.FeHash); err == nil {
			m.Stop(FrontendKey(row.RepoID, row.FeHash, row.BeHash), "deploy deleted")
			if err := m.db.DeleteFrontendArtifact(row.RepoID, row.FeHash); err != nil {
				log.Printf("gc deploy %d: delete frontend artifact: %v", row.ID, err)
			}
		}
		if err := m.files.RemoveFrontend(row.RepoName, row.FeHash); err != nil {
			log.Printf("gc deploy %d: remove frontend files: %v", row.ID, err)
		}
		m.removeHashLogs(row.RepoName, "fe", row.FeHash)
	}
	for _, ref := range row.Artifacts {
		if liveDL[ref.Hash] {
			continue
		}
		if err := m.files.RemoveArtifact(row.RepoName, ref.Hash); err != nil {
			log.Printf("gc deploy %d: remove artifact files: %v", row.ID, err)
		}
		m.removeHashLogs(row.RepoName, "dl", ref.Hash)
	}
}

// removeHashLogs deletes an orphaned hash's build log and run-log directory.
// Logs are keyed per hash (build.Queue.logPath, openRunLog), so once no
// surviving deploy references the hash, nothing can serve them again.
func (m *Manager) removeHashLogs(repoName, kind, hash string) {
	os.Remove(filepath.Join(m.logsDir, repoName, kind, hash+".log"))
	os.RemoveAll(filepath.Join(m.logsDir, repoName, "run", kind+"-"+hash))
}

// Status reports the runtime state of an artifact's process for API views:
// "running", "starting", "crashed" (the last run or start attempt ended
// badly and nothing has superseded it), or "idle" (not running; a start is
// one request away). Crashed is not a wedged state — the next request starts
// the process like any idle one; it exists so a service that died on its own
// is distinguishable from one that was never asked to run.
func (m *Manager) Status(k Key) string {
	m.mu.Lock()
	p := m.procs[k]
	_, crashed := m.failures[k]
	m.mu.Unlock()
	if p == nil {
		if crashed {
			return StatusCrashed
		}
		return StatusIdle
	}
	select {
	case <-p.ready:
		return StatusRunning
	default:
		return StatusStarting
	}
}

// ReclaimOrphans handles leftovers from an unclean exit. Host processes:
// if a process_records pid still leads a matching process group, the group
// is killed (PID reuse makes certain identification impossible; a health
// check always precedes routing, so a stale guess here can never misroute
// traffic). Containers: everything carrying the managed label is removed —
// labels are the container-side crash record. Docker being unreachable
// must never block startup; the sweep is skipped silently then.
func (m *Manager) ReclaimOrphans() {
	records, err := m.db.ListProcessRecords()
	if err == nil {
		for _, r := range records {
			if pgid, err := processGroupOf(r.PID); err == nil && pgid == r.PGID {
				terminateGroup(r.PGID) //nolint:errcheck
				time.Sleep(200 * time.Millisecond)
				killGroup(r.PGID) //nolint:errcheck
				m.recordEvent(r.RepoID, r.BeHash, "exited", "reclaimed orphan after unclean shutdown")
			}
			m.db.DeleteProcessRecord(r.RepoID, r.BeHash)
		}
	}
	m.removeLabeled("", "")
}

// PurgeRepoContainers removes every container and network labeled for the
// repo — repo deletion's container-side cleanup, catching leftovers no
// longer in the in-memory table.
func (m *Manager) PurgeRepoContainers(repoName string) {
	m.removeLabeled("local-preview.repo", repoName)
}

// removeLabeled force-removes managed containers (then networks) matching
// the label filter; an empty label sweeps everything managed.
func (m *Manager) removeLabeled(label, value string) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cli, err := m.docker(ctx)
	if err != nil {
		return
	}
	if label == "" {
		label = dockerapi.ManagedLabel
		value = ""
	}
	if cs, err := cli.ListContainersByLabel(ctx, label, value); err == nil {
		for _, c := range cs {
			cli.RemoveContainer(ctx, c.ID, true) //nolint:errcheck
		}
	}
	if ns, err := cli.ListNetworksByLabel(ctx, label, value); err == nil {
		m.netMu.Lock()
		for _, n := range ns {
			cli.RemoveNetwork(ctx, n.ID) //nolint:errcheck
		}
		m.netMu.Unlock()
	}
}

// releaseNetwork removes a deploy network once its last member is gone —
// last-one-out semantics without any refcount: the daemon refuses the
// removal while an endpoint (the peer side's container) is still attached,
// and whichever of the pair exits second takes the network with it. Errors
// are ignored for that reason; the leak this guards against (one bridge
// network per backend hash ever served, never reclaimed until restart) is
// what exhausted the daemon's ~30 default address pools on a busy worker.
func (m *Manager) releaseNetwork(ctx context.Context, cli *dockerapi.Client, id string) {
	if id == "" {
		return
	}
	m.netMu.Lock()
	defer m.netMu.Unlock()
	cli.RemoveNetwork(ctx, id) //nolint:errcheck
}

// ForkOrInitStateDir provisions the state directory for a newly built
// backend artifact and records the backend_artifacts row. It walks
// first-parent ancestry for the nearest ready deploy whose backend has a
// live state dir, briefly stops that backend (a full stop is the only
// quiesce that needs no assumptions about the app's persistence engine),
// copies, and renames into place. With no usable ancestor the state starts
// fresh. Idempotent: an existing backend_artifacts row short-circuits.
func (m *Manager) ForkOrInitStateDir(ctx context.Context, git gitrepo.Repo, repoID int64, repoName, newBeHash, sha, runConfigJSON string) error {
	if _, err := m.db.GetBackendArtifact(repoID, newBeHash); err == nil {
		// Already provisioned. Refresh the run contract: `networks` isn't
		// part of the artifact hash, so its changes land here rather than
		// in a new row.
		return m.db.UpdateBackendArtifactRunConfig(repoID, newBeHash, runConfigJSON)
	} else if !errors.Is(err, db.ErrNotFound) {
		return err
	}

	forkedFrom := ""
	if src := m.findForkSource(ctx, git, repoID, repoName, newBeHash, sha); src != "" {
		srcKey := BackendKey(repoID, src)
		lock := m.keyLock(srcKey)
		lock.Lock()
		m.stopLocked(srcKey, "quiesce for state fork")
		err := m.files.ForkStateDir(repoName, src, newBeHash)
		lock.Unlock()
		if err != nil {
			return err
		}
		forkedFrom = src
	} else if err := m.files.InitFreshStateDir(repoName, newBeHash); err != nil {
		return err
	}

	return m.db.CreateBackendArtifact(db.BackendArtifact{
		RepoID:     repoID,
		BeHash:     newBeHash,
		ForkedFrom: forkedFrom,
		StateDir:   m.files.StateDirPath(repoName, newBeHash),
		RunConfig:  runConfigJSON,
	})
}

// findForkSource returns the be_hash of the nearest deployed ancestor with
// a live state dir, or "".
func (m *Manager) findForkSource(ctx context.Context, git gitrepo.Repo, repoID int64, repoName, newBeHash, sha string) string {
	ancestors, err := git.FirstParentAncestry(ctx, sha, AncestryLimit)
	if err != nil {
		return ""
	}
	for _, anc := range ancestors {
		d, err := m.db.GetDeployBySHA(repoID, anc)
		if err != nil || d.Status != db.DeployReady || d.BeHash == "" || d.BeHash == newBeHash {
			continue
		}
		if _, err := m.db.GetBackendArtifact(repoID, d.BeHash); err != nil {
			continue
		}
		if !m.files.HasStateDir(repoName, d.BeHash) {
			continue
		}
		return d.BeHash
	}
	return ""
}

func (m *Manager) openRunLog(repoName, beHash string) (*os.File, error) {
	dir := filepath.Join(m.logsDir, repoName, "run", beHash)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	entries, _ := os.ReadDir(dir)
	name := strconv.Itoa(len(entries)+1) + ".log"
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open run log: %w", err)
	}
	return f, nil
}

// probeFreePort asks the OS for a free loopback port. The tiny window
// between Close and the child's bind is accepted: a bind failure surfaces
// as a fast exit and the retry probes a fresh port.
func probeFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("probe free port: %w", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port, nil
}

// templateArgv substitutes {port} and {state_dir} in each argv element.
func templateArgv(argv []string, port int, stateDir string) []string {
	out := make([]string, len(argv))
	for i, a := range argv {
		a = strings.ReplaceAll(a, "{port}", strconv.Itoa(port))
		a = strings.ReplaceAll(a, "{state_dir}", stateDir)
		out[i] = a
	}
	return out
}
