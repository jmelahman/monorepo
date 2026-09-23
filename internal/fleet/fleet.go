// Package fleet is the control node's view of the elastic worker tier: a
// heartbeat-tracked registry plus placement. It implements proxy.Backends by
// choosing a worker for a supervise.Key and delegating to that worker's
// transport — so the proxy routes to a fleet exactly as it routes to a single
// local Manager, the same "one orchestrator, two transports" property extended
// to N workers.
//
// Placement is rendezvous (highest-random-weight) hashing on (repo, hash):
// cache-affine (the same artifact lands on the same worker, so its local cache
// and any warm process are reused) and minimally disruptive when workers join
// or leave (only the keys whose top-weighted worker changed move). A
// process-mode frontend is co-placed with its backend — it hashes on the
// backend's hash — because the pair shares a per-deploy docker network that
// only exists on one node. A worker that is draining or at capacity is skipped
// for new placements, falling back to the least-loaded fresh worker.
package fleet

import (
	"context"
	"errors"
	"hash/fnv"
	"io"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jmelahman/local-preview/internal/supervise"
	"github.com/jmelahman/local-preview/internal/workerapi"
)

// ErrNoWorker means no worker could take the placement (none registered, all
// stale, or all draining). The proxy renders it like any other start failure.
var ErrNoWorker = errors.New("no worker available")

// Backend is one worker's control-side transport. *workerapi.Client satisfies
// it; tests substitute a fake.
type Backend interface {
	EnsureRunning(ctx context.Context, k supervise.Key, repoName string) (string, error)
	Heartbeat(ctx context.Context) (workerapi.Heartbeat, error)
	Report(ctx context.Context) ([]supervise.ProcReport, error)
	RunLog(ctx context.Context, repo, side, hash string, attempt int, offset int64) (supervise.RunLog, error)
	Exec(ctx context.Context, k supervise.Key, opts supervise.ExecOptions, stream io.ReadWriter) error
	Stop(ctx context.Context, k supervise.Key, reason string) error
	Configure(ctx context.Context, cfg workerapi.WorkerConfig) error
}

type workerState struct {
	id string
	be Backend
	// instanceID is the worker's cloud instance-id (empty for a hand-wired or
	// non-cloud worker). The autoscaler uses it to scale-in-protect a worker
	// that is currently serving previews.
	instanceID string

	mu        sync.Mutex
	hb        workerapi.Heartbeat
	lastBeat  time.Time
	reachable bool
	// warmBase/coldBase bank the hit counters a worker reported before it
	// restarted (its counters are since-boot, and reboots are routine on a
	// spot tier) — added to the live heartbeat values so fleet totals stay
	// monotonic for as long as the control node runs.
	warmBase, coldBase int64
}

func (w *workerState) snapshot() (hb workerapi.Heartbeat, lastBeat time.Time, reachable bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.hb, w.lastBeat, w.reachable
}

// hitTotals returns the worker's lifetime hit counters: the banked history of
// previous boots plus its current since-boot values.
func (w *workerState) hitTotals() (warm, cold int64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.warmBase + w.hb.WarmHits, w.coldBase + w.hb.ColdStarts
}

// Registry tracks workers and places keys onto them.
type Registry struct {
	mu         sync.RWMutex
	workers    map[string]*workerState
	staleAfter time.Duration

	// Merged fleet report cache — see runtime.go.
	reportMu    sync.Mutex
	reportAt    time.Time
	reportCache map[supervise.Key]procEntry

	// The dashboard-configured runtime settings the heartbeat loop reconciles
	// onto every worker (a rebooted worker comes back with its boot flags
	// until the next poll pushes these). -1 = unset.
	desiredMaxWarm atomic.Int64
	desiredMinWarm atomic.Int64
	desiredIdleSec atomic.Int64

	// minWarmActive gates the min-warm floor by wall clock (nil = always
	// active). Outside the window every worker is pushed a floor of 0, so
	// idle previews drain, workers empty, and the ASG can return to zero.
	// Set once before StartHeartbeats; not safe to change after.
	minWarmActive func(time.Time) bool

	// eventSink, when set, receives the process-event batches workers ship in
	// their heartbeats (SetEventSink). Events are recorded where a process
	// RUNS — a worker's own ephemeral database — so without shipping them the
	// control node's startup-latency percentiles read empty in fleet mode.
	eventSink func([]supervise.ProcEventRecord)

	// unservedDemand records the placement keys of EnsureRunning calls that
	// found no worker (ErrNoWorker) since the last DrainDemand. It is the
	// scale-from-zero signal: LoadRatio reads 1 with an empty fleet (see its
	// doc), which can't distinguish "idle at zero" from "saturated", so demand
	// — a request that arrived with nowhere to run — is what tells the
	// autoscaler to lift the fleet off zero.
	//
	// Keys, not a raw counter, deliberately: demand must count previews
	// waiting for capacity, not requests. An impatient refresh loop (or a
	// pre-warm retrying while a node boots) hits EnsureRunning many times for
	// one preview, and raw counts inflated the metric into the scale-out
	// policy's higher steps — production launched three nodes for what was one
	// deploy's worth of demand. Guarded by demandMu; only the ErrNoWorker path
	// ever takes the lock, so placement's happy path stays lock-free.
	demandMu       sync.Mutex
	unservedDemand map[string]struct{}
}

// New returns an empty registry. A worker whose last heartbeat is older than
// staleAfter is treated as gone for placement.
func New(staleAfter time.Duration) *Registry {
	r := &Registry{
		workers:        map[string]*workerState{},
		staleAfter:     staleAfter,
		unservedDemand: map[string]struct{}{},
	}
	r.desiredMaxWarm.Store(-1)
	r.desiredMinWarm.Store(-1)
	r.desiredIdleSec.Store(-1)
	return r
}

// SetMaxWarm sets the fleet-wide per-worker warm cap: applied to every worker
// on the next heartbeat poll and re-applied whenever a worker's reported cap
// drifts (a reboot restores its boot flag). Implements the dashboard's warm
// setting for a fleet.
func (r *Registry) SetMaxWarm(n int) {
	r.desiredMaxWarm.Store(int64(max(n, 0)))
}

// SetMinWarm sets the fleet-wide warm floor: the n most-recently-touched
// processes ACROSS THE FLEET never idle out. Unlike SetMaxWarm's per-worker
// cap, the floor is not applied verbatim to every worker — the heartbeat loop
// ranks the fleet's processes by recency and pushes each worker only its
// share (minWarmQuotas), so "min 12" protects 12 processes total, however
// many workers exist.
func (r *Registry) SetMinWarm(n int) {
	r.desiredMinWarm.Store(int64(max(n, 0)))
}

// SetMinWarmWindow gates the min-warm floor by wall clock: outside active
// hours the floor is 0 (idle previews drain and the fleet can scale to zero);
// scale-out from demand is unaffected. nil restores always-active. Call
// before StartHeartbeats.
func (r *Registry) SetMinWarmWindow(active func(time.Time) bool) {
	r.minWarmActive = active
}

// minWarmQuotas splits the fleet-wide floor into per-worker shares: the
// desiredMinWarm most-recently-touched processes fleet-wide, counted per
// worker. A worker's local reaper protects ITS most-recent quota-many
// processes, and local recency order agrees with fleet order restricted to
// that worker, so the pushed count protects exactly the fleet's top-N.
// Returns ok=false when the floor is unset (-1): workers then keep their
// boot-flag values, matching the other settings' unset semantics. Workers
// absent from the map get 0.
func (r *Registry) minWarmQuotas(ctx context.Context) (map[string]int, bool) {
	want := int(r.desiredMinWarm.Load())
	if want < 0 {
		return nil, false
	}
	quotas := map[string]int{}
	if want == 0 || (r.minWarmActive != nil && !r.minWarmActive(time.Now())) {
		return quotas, true
	}
	type cand struct {
		touch    time.Time
		workerID string
	}
	var cands []cand
	for _, e := range r.report(ctx) {
		// Only live processes hold a floor slot; a crashed record protects
		// nothing. LastTouch is zero from workers that predate the field,
		// which ranks them least-recent — the conservative end.
		if e.rep.Status != supervise.StatusRunning && e.rep.Status != supervise.StatusStarting {
			continue
		}
		cands = append(cands, cand{touch: e.rep.LastTouch, workerID: e.worker.id})
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].touch.After(cands[j].touch) })
	for i := 0; i < len(cands) && i < want; i++ {
		quotas[cands[i].workerID]++
	}
	return quotas, true
}

// SetIdleOverride sets the fleet-wide idle-timeout override (0 = restore
// per-manifest values), reconciled onto workers like SetMaxWarm.
func (r *Registry) SetIdleOverride(d time.Duration) {
	r.desiredIdleSec.Store(int64(max(d, 0) / time.Second))
}

// SetEventSink wires where worker-shipped process events land (the control
// node's process_events trail). Optional: without it the batches are dropped
// and fleet startup statistics stay empty.
func (r *Registry) SetEventSink(sink func([]supervise.ProcEventRecord)) {
	r.eventSink = sink
}

// Add registers a worker by id, reporting whether it was new. instanceID is
// the worker's cloud instance-id (empty if it has none), used for scale-in
// protection.
//
// Idempotent for a known worker, deliberately: workers re-announce every ~20s
// (so a restarted control node re-learns them), and replacing the state on
// each announcement zeroed heartbeat freshness — making a healthy worker
// unplaceable until the next heartbeat poll, ~a quarter of wall-clock. A
// small fleet could transiently read as empty, serving real requests the
// waking page and registering phantom unserved demand. The one case that DOES
// replace: the same endpoint returning with a different instance-id — that is
// a different machine (an ASG replacement reusing the IP), and fresh state,
// blind until its first heartbeat, is exactly right for it.
func (r *Registry) Add(id, instanceID string, be Backend) (isNew bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if w, ok := r.workers[id]; ok && w.instanceID == instanceID {
		return false
	}
	r.workers[id] = &workerState{id: id, instanceID: instanceID, be: be}
	return true
}

// Remove deregisters a worker.
func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.workers, id)
}

func (r *Registry) list() []*workerState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*workerState, 0, len(r.workers))
	for _, w := range r.workers {
		out = append(out, w)
	}
	return out
}

// recordHeartbeat updates a worker's cached capacity. Exposed for the poll loop
// and for tests.
func (r *Registry) recordHeartbeat(id string, hb workerapi.Heartbeat, at time.Time, reachable bool) {
	r.mu.RLock()
	w := r.workers[id]
	r.mu.RUnlock()
	if w == nil {
		return
	}
	w.mu.Lock()
	if reachable {
		// A counter running backwards means the worker restarted (since-boot
		// counters); bank what the previous boot accumulated so fleet totals
		// stay monotonic across the restart.
		if hb.WarmHits < w.hb.WarmHits || hb.ColdStarts < w.hb.ColdStarts {
			w.warmBase += w.hb.WarmHits
			w.coldBase += w.hb.ColdStarts
		}
		w.hb = hb
	}
	w.lastBeat = at
	w.reachable = reachable
	w.mu.Unlock()
}

// StartHeartbeats polls every worker's Heartbeat immediately and then on
// interval, until ctx is cancelled. Returns after launching the loop.
func (r *Registry) StartHeartbeats(ctx context.Context, interval time.Duration) {
	poll := func() {
		// One fleet-wide ranking per sweep: every worker's push below reads
		// the same recency snapshot, so the quotas sum to the floor.
		minQuota, haveMinQuota := r.minWarmQuotas(ctx)
		var wg sync.WaitGroup
		for _, w := range r.list() {
			wg.Add(1)
			go func(w *workerState) {
				defer wg.Done()
				hctx, cancel := context.WithTimeout(ctx, interval)
				defer cancel()
				hb, err := w.be.Heartbeat(hctx)
				r.recordHeartbeat(w.id, hb, time.Now(), err == nil)
				// Process events ride the heartbeat: the worker drained its
				// buffer into this response, and they only exist there — a
				// worker's database is ephemeral, so the control node's
				// process_events trail is the durable copy.
				if err == nil && len(hb.Events) > 0 && r.eventSink != nil {
					r.eventSink(hb.Events)
				}
				// Reconcile the dashboard-configured settings: a worker
				// reporting different values (fresh boot, missed push) gets
				// them re-applied here, so the settings survive the whole
				// fleet's churn without any worker-side persistence.
				if err == nil {
					var cfg workerapi.WorkerConfig
					if want := r.desiredMaxWarm.Load(); want >= 0 && hb.MaxWarm != int(want) {
						n := int(want)
						cfg.MaxWarm = &n
					}
					if haveMinQuota {
						if want := minQuota[w.id]; hb.MinWarm != want {
							n := want
							cfg.MinWarm = &n
						}
					}
					if want := r.desiredIdleSec.Load(); want >= 0 && hb.IdleTimeoutSeconds != int(want) {
						n := int(want)
						cfg.IdleTimeoutSeconds = &n
					}
					if cfg.MaxWarm != nil || cfg.MinWarm != nil || cfg.IdleTimeoutSeconds != nil {
						w.be.Configure(hctx, cfg) //nolint:errcheck // next poll retries
					}
				}
			}(w)
		}
		wg.Wait()
	}
	go func() {
		poll()
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				poll()
			}
		}
	}()
}

// EnsureRunning places k on a worker and starts (or reuses) the process there,
// returning the host:port the proxy dials. Implements proxy.Backends.
func (r *Registry) EnsureRunning(ctx context.Context, k supervise.Key, repoName string) (string, error) {
	w := r.place(k)
	if w == nil {
		r.demandMu.Lock()
		r.unservedDemand[placementKey(k)] = struct{}{}
		r.demandMu.Unlock()
		return "", ErrNoWorker
	}
	return w.be.EnsureRunning(ctx, k, repoName)
}

// Capacity totals committed warm slots (running) and the *bounded* fleet
// capacity (the sum of positive max_warm caps) across fresh workers. A worker
// with max_warm 0 (unlimited) contributes no finite capacity figure — see
// LoadRatio for how unlimited headroom is scored — so it never inflates the
// denominator into implying the fleet is full.
func (r *Registry) Capacity() (running, capacity int) {
	now := time.Now()
	for _, w := range r.list() {
		hb, last, reachable := w.snapshot()
		if !r.fresh(last, reachable, now) {
			continue
		}
		running += hb.Running
		if hb.MaxWarm > 0 {
			capacity += hb.MaxWarm
		}
	}
	return running, capacity
}

// FleetStat is the fleet-wide runtime summary the dashboard's statistics view
// renders: how many workers are fresh, their committed and bounded capacity,
// and the cumulative warm/cold EnsureRunning counts summed across them.
type FleetStat struct {
	Workers    int   `json:"workers"`
	Running    int   `json:"running"`
	Capacity   int   `json:"capacity"` // 0 = unbounded (some fresh worker is unlimited)
	WarmHits   int64 `json:"warm_hits"`
	ColdStarts int64 `json:"cold_starts"`
}

// Stat summarizes the fresh workers for the dashboard. Capacity is 0 when any
// fresh worker is unlimited (max_warm 0), matching Capacity's convention that
// an unlimited worker contributes no finite bound.
func (r *Registry) Stat() FleetStat {
	now := time.Now()
	var st FleetStat
	var unbounded bool
	for _, w := range r.list() {
		hb, last, reachable := w.snapshot()
		if !r.fresh(last, reachable, now) {
			continue
		}
		st.Workers++
		st.Running += hb.Running
		if hb.MaxWarm > 0 {
			st.Capacity += hb.MaxWarm
		} else {
			unbounded = true
		}
		warm, cold := w.hitTotals()
		st.WarmHits += warm
		st.ColdStarts += cold
	}
	if unbounded {
		st.Capacity = 0
	}
	return st
}

// LoadRatio is committed warm slots ÷ fleet capacity in [0,1] — the target a
// scale-out policy tracks. Any fresh worker with unlimited capacity (max_warm
// 0) means the fleet has spare room, so the ratio is 0 (never a scale-out
// trigger) rather than running/running = 1, which would read as "scale out
// forever". With no fresh worker, or all bounded workers at zero capacity, it
// is 1.
func (r *Registry) LoadRatio() float64 {
	now := time.Now()
	var running, capacity int
	var anyFresh, unbounded bool
	for _, w := range r.list() {
		hb, last, reachable := w.snapshot()
		if !r.fresh(last, reachable, now) {
			continue
		}
		anyFresh = true
		running += hb.Running
		if hb.MaxWarm > 0 {
			capacity += hb.MaxWarm
		} else {
			unbounded = true
		}
	}
	if unbounded {
		return 0
	}
	if !anyFresh || capacity <= 0 {
		return 1
	}
	return float64(running) / float64(capacity)
}

// DrainDemand returns the number of distinct placement keys that went unserved
// (ErrNoWorker) since the last call and resets the set. The autoscaler
// publishes it once per interval as the scale-from-zero metric: it reads as
// "previews currently waiting for capacity", however many times each was
// retried within the interval.
func (r *Registry) DrainDemand() int64 {
	r.demandMu.Lock()
	defer r.demandMu.Unlock()
	n := int64(len(r.unservedDemand))
	clear(r.unservedDemand)
	return n
}

// LoadSample returns fleet utilization as a percentage (committed warm slots ÷
// bounded capacity × 100) alongside the count of fresh workers. It exists so
// the autoscaler can tell an empty fleet (workers == 0) — which LoadRatio
// scores as a saturated 1, and which is driven off zero by demand not load —
// apart from a fleet that is genuinely busy or idle. Semantics otherwise match
// LoadRatio: an unbounded fresh worker means spare room, so load 0.
func (r *Registry) LoadSample() (loadPct float64, workers int) {
	now := time.Now()
	var running, capacity int
	var unbounded bool
	for _, w := range r.list() {
		hb, last, reachable := w.snapshot()
		if !r.fresh(last, reachable, now) {
			continue
		}
		workers++
		running += hb.Running
		if hb.MaxWarm > 0 {
			capacity += hb.MaxWarm
		} else {
			unbounded = true
		}
	}
	switch {
	case workers == 0:
		return 0, 0
	case unbounded:
		return 0, workers
	case capacity <= 0:
		return 100, workers
	default:
		return float64(running) / float64(capacity) * 100, workers
	}
}

// BusyByInstance maps each fresh worker's cloud instance-id to whether it is
// serving at least one preview (Running > 0). Workers with no instance-id are
// omitted — there is nothing to protect. The autoscaler scale-in-protects the
// busy ones so an ASG scale-in drains idle nodes instead of killing a preview.
func (r *Registry) BusyByInstance() map[string]bool {
	now := time.Now()
	out := map[string]bool{}
	for _, w := range r.list() {
		if w.instanceID == "" {
			continue
		}
		hb, last, reachable := w.snapshot()
		if !r.fresh(last, reachable, now) {
			continue
		}
		out[w.instanceID] = hb.Running > 0
	}
	return out
}

// IdleWorkers counts fresh, autoscaled (instance-id'd) workers serving
// nothing. It backs the IdleWorkers metric, whose alarm reclaims exactly such
// a node — a load-ratio threshold can't express "an empty worker exists"
// while the min-warm floor props the numerator up (12 floor-protected
// processes on a 32-slot fleet is 37.5% load forever). Draining workers are
// excluded: one mid-termination via the lifecycle hook is already being
// reclaimed, and counting it would decrement the group twice.
func (r *Registry) IdleWorkers() int {
	now := time.Now()
	idle := 0
	for _, w := range r.list() {
		if w.instanceID == "" {
			continue
		}
		hb, last, reachable := w.snapshot()
		if !r.fresh(last, reachable, now) || hb.Draining {
			continue
		}
		if hb.Running == 0 {
			idle++
		}
	}
	return idle
}

func (r *Registry) fresh(last time.Time, reachable bool, now time.Time) bool {
	return reachable && !last.IsZero() && now.Sub(last) < r.staleAfter
}

// place chooses a worker for k: the highest-weight rendezvous worker among
// those with free capacity; failing that (all full), the least-loaded fresh,
// non-draining worker; nil if none qualifies.
func (r *Registry) place(k supervise.Key) *workerState {
	key := placementKey(k)
	now := time.Now()

	var eligible, fallback []*workerState
	for _, w := range r.list() {
		hb, last, reachable := w.snapshot()
		if !r.fresh(last, reachable, now) || hb.Draining {
			continue
		}
		fallback = append(fallback, w)
		if hb.MaxWarm <= 0 || hb.Running < hb.MaxWarm {
			eligible = append(eligible, w)
		}
	}

	if len(eligible) > 0 {
		return topWeight(eligible, key)
	}
	// All fresh workers are full: place on the least-loaded so the preview still
	// serves (a per-worker cap is a soft target, not a hard refusal), ties broken
	// by rendezvous weight for stability.
	return leastLoaded(fallback, key)
}

// placementKey is what the hash is taken over. A process-mode frontend hashes
// on its backend's hash so it co-locates with the backend it shares a deploy
// network with; everything else hashes on its own artifact hash. Namespaced by
// repo so two repos' identical hashes (unlikely, but content-addresses can
// collide across trivial artifacts) don't force co-placement.
func placementKey(k supervise.Key) string {
	h := k.Hash
	if k.Side == supervise.SideFrontend && k.Peer != "" {
		h = k.Peer
	}
	return strconv.FormatInt(k.RepoID, 10) + "/" + h
}

// topWeight returns the worker with the highest rendezvous weight for key.
func topWeight(workers []*workerState, key string) *workerState {
	var best *workerState
	var bestScore uint64
	for _, w := range workers {
		s := hrw(w.id, key)
		if best == nil || s > bestScore {
			best, bestScore = w, s
		}
	}
	return best
}

// leastLoaded returns the worker with the smallest running/max_warm ratio;
// rendezvous weight breaks ties so placement stays stable.
func leastLoaded(workers []*workerState, key string) *workerState {
	var best *workerState
	var bestLoad float64
	var bestScore uint64
	for _, w := range workers {
		hb, _, _ := w.snapshot()
		load := 0.0
		if hb.MaxWarm > 0 {
			load = float64(hb.Running) / float64(hb.MaxWarm)
		}
		s := hrw(w.id, key)
		if best == nil || load < bestLoad || (load == bestLoad && s > bestScore) {
			best, bestLoad, bestScore = w, load, s
		}
	}
	return best
}

// hrw is the rendezvous weight of (workerID, key): a deterministic hash, so a
// key's ranking of workers is stable and independent of registration order.
func hrw(workerID, key string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(workerID))
	h.Write([]byte{0})
	h.Write([]byte(key))
	return h.Sum64()
}
