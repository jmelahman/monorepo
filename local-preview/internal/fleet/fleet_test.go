package fleet

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/jmelahman/local-preview/internal/supervise"
	"github.com/jmelahman/local-preview/internal/workerapi"
)

type fakeBackend struct {
	id      string
	hb      workerapi.Heartbeat // returned by Heartbeat (zero by default)
	ensured []supervise.Key
	report  []supervise.ProcReport
	logs    map[string]supervise.RunLog // keyed side+"-"+hash
	stopped []supervise.Key
	execed  []supervise.Key

	// Configure records, mutex-guarded: the heartbeat loop pushes from its
	// own goroutines.
	cfgMu         sync.Mutex
	configured    []int
	configuredMin []int
}

// lastConfigured returns the most recent MaxWarm and MinWarm pushes (-1 =
// never pushed).
func (f *fakeBackend) lastConfigured() (maxWarm, minWarm int) {
	f.cfgMu.Lock()
	defer f.cfgMu.Unlock()
	maxWarm, minWarm = -1, -1
	if n := len(f.configured); n > 0 {
		maxWarm = f.configured[n-1]
	}
	if n := len(f.configuredMin); n > 0 {
		minWarm = f.configuredMin[n-1]
	}
	return maxWarm, minWarm
}

func (f *fakeBackend) EnsureRunning(_ context.Context, k supervise.Key, _ string) (string, error) {
	f.ensured = append(f.ensured, k)
	return f.id + ":8000", nil
}
func (f *fakeBackend) Heartbeat(context.Context) (workerapi.Heartbeat, error) {
	return f.hb, nil
}
func (f *fakeBackend) Report(context.Context) ([]supervise.ProcReport, error) {
	return f.report, nil
}
func (f *fakeBackend) RunLog(_ context.Context, _, side, hash string, _ int, _ int64) (supervise.RunLog, error) {
	return f.logs[side+"-"+hash], nil
}
func (f *fakeBackend) Exec(_ context.Context, k supervise.Key, _ supervise.ExecOptions, _ io.ReadWriter) error {
	f.execed = append(f.execed, k)
	return nil
}
func (f *fakeBackend) Stop(_ context.Context, k supervise.Key, _ string) error {
	f.stopped = append(f.stopped, k)
	return nil
}
func (f *fakeBackend) Configure(_ context.Context, cfg workerapi.WorkerConfig) error {
	f.cfgMu.Lock()
	defer f.cfgMu.Unlock()
	if cfg.MaxWarm != nil {
		f.configured = append(f.configured, *cfg.MaxWarm)
	}
	if cfg.MinWarm != nil {
		f.configuredMin = append(f.configuredMin, *cfg.MinWarm)
	}
	return nil
}

// regFresh registers workers with the given heartbeats, all beating "now".
func regFresh(t *testing.T, hbs map[string]workerapi.Heartbeat) (*Registry, map[string]*fakeBackend) {
	t.Helper()
	r := New(time.Minute)
	bes := map[string]*fakeBackend{}
	for id, hb := range hbs {
		be := &fakeBackend{id: id}
		bes[id] = be
		r.Add(id, "", be)
		r.recordHeartbeat(id, hb, time.Now(), true)
	}
	return r, bes
}

func TestCoPlacesFrontendWithBackend(t *testing.T) {
	r, _ := regFresh(t, map[string]workerapi.Heartbeat{
		"w1": {Running: 0, MaxWarm: 10},
		"w2": {Running: 0, MaxWarm: 10},
		"w3": {Running: 0, MaxWarm: 10},
	})
	be := r.place(supervise.BackendKey(42, "beHASH"))
	fe := r.place(supervise.FrontendKey(42, "feHASH", "beHASH"))
	if be == nil || fe == nil {
		t.Fatal("expected placements")
	}
	if be.id != fe.id {
		t.Fatalf("frontend placed on %s but its backend on %s — must co-locate", fe.id, be.id)
	}
	// And placement is deterministic.
	if again := r.place(supervise.BackendKey(42, "beHASH")); again.id != be.id {
		t.Fatalf("non-deterministic placement: %s then %s", be.id, again.id)
	}
}

func TestDrainingExcluded(t *testing.T) {
	r, _ := regFresh(t, map[string]workerapi.Heartbeat{
		"only": {Running: 0, MaxWarm: 10, Draining: true},
	})
	if w := r.place(supervise.BackendKey(1, "h")); w != nil {
		t.Fatalf("placed on a draining worker %s", w.id)
	}
	if _, err := r.EnsureRunning(context.Background(), supervise.BackendKey(1, "h"), "demo"); !errors.Is(err, ErrNoWorker) {
		t.Fatalf("err = %v, want ErrNoWorker", err)
	}
}

func TestFullFallsBackToLeastLoaded(t *testing.T) {
	// Both workers are at capacity; placement must still pick one (soft cap).
	r, _ := regFresh(t, map[string]workerapi.Heartbeat{
		"busy":   {Running: 10, MaxWarm: 10},
		"busier": {Running: 20, MaxWarm: 10},
	})
	w := r.place(supervise.BackendKey(1, "h"))
	if w == nil {
		t.Fatal("expected a fallback placement when all are full")
	}
	if w.id != "busy" {
		t.Fatalf("fallback chose %s, want the least-loaded 'busy'", w.id)
	}
}

func TestPrefersWorkerWithFreeCapacity(t *testing.T) {
	// "free" has a slot; "full" does not — regardless of hash, the eligible one wins.
	r, _ := regFresh(t, map[string]workerapi.Heartbeat{
		"free": {Running: 1, MaxWarm: 10},
		"full": {Running: 10, MaxWarm: 10},
	})
	for i := 0; i < 5; i++ {
		if w := r.place(supervise.BackendKey(int64(i), "h")); w.id != "free" {
			t.Fatalf("placed on %s, want the only worker with capacity", w.id)
		}
	}
}

func TestNoWorkersOrAllStale(t *testing.T) {
	r := New(time.Minute)
	if _, err := r.EnsureRunning(context.Background(), supervise.BackendKey(1, "h"), "demo"); !errors.Is(err, ErrNoWorker) {
		t.Fatalf("empty registry err = %v, want ErrNoWorker", err)
	}
	// A stale heartbeat is as good as gone.
	r.Add("w", "", &fakeBackend{id: "w"})
	r.recordHeartbeat("w", workerapi.Heartbeat{MaxWarm: 10}, time.Now().Add(-time.Hour), true)
	if w := r.place(supervise.BackendKey(1, "h")); w != nil {
		t.Fatalf("placed on stale worker %s", w.id)
	}
}

// Unserved demand counts previews waiting for capacity, not raw request
// retries: N retries of one preview must read as demand 1 or the scale-out
// policy launches nodes for phantom load, while a frontend co-placed with its
// backend shares the backend's placement key (one preview, one demand).
func TestDrainDemandDedupes(t *testing.T) {
	r := New(time.Minute)
	ctx := context.Background()
	for range 5 {
		r.EnsureRunning(ctx, supervise.BackendKey(1, "beA"), "demo") //nolint:errcheck // ErrNoWorker is the point
	}
	r.EnsureRunning(ctx, supervise.FrontendKey(1, "feA", "beA"), "demo") //nolint:errcheck // co-placed: same key as beA
	r.EnsureRunning(ctx, supervise.BackendKey(1, "beB"), "demo")         //nolint:errcheck // a second waiting preview
	if got := r.DrainDemand(); got != 2 {
		t.Fatalf("DrainDemand = %d, want 2 (distinct previews, retries and co-placed sides deduped)", got)
	}
	// Drain resets: quiet interval reads 0, and a fresh miss counts anew.
	if got := r.DrainDemand(); got != 0 {
		t.Fatalf("DrainDemand after drain = %d, want 0", got)
	}
	r.EnsureRunning(ctx, supervise.BackendKey(1, "beA"), "demo") //nolint:errcheck // still unserved next interval
	if got := r.DrainDemand(); got != 1 {
		t.Fatalf("DrainDemand re-registered = %d, want 1", got)
	}
}

func TestCapacityAndLoad(t *testing.T) {
	r, _ := regFresh(t, map[string]workerapi.Heartbeat{
		"a": {Running: 3, MaxWarm: 10},
		"b": {Running: 5, MaxWarm: 10},
	})
	running, capacity := r.Capacity()
	if running != 8 || capacity != 20 {
		t.Fatalf("capacity = %d/%d, want 8/20", running, capacity)
	}
	if got := r.LoadRatio(); got != 0.4 {
		t.Fatalf("load ratio = %v, want 0.4", got)
	}
}

func TestStat(t *testing.T) {
	r, _ := regFresh(t, map[string]workerapi.Heartbeat{
		"a": {Running: 3, MaxWarm: 10, WarmHits: 40, ColdStarts: 5},
		"b": {Running: 5, MaxWarm: 10, WarmHits: 50, ColdStarts: 5},
	})
	st := r.Stat()
	if st.Workers != 2 || st.Running != 8 || st.Capacity != 20 {
		t.Fatalf("stat = %+v, want workers=2 running=8 capacity=20", st)
	}
	if st.WarmHits != 90 || st.ColdStarts != 10 {
		t.Fatalf("stat hits = %d/%d, want 90/10", st.WarmHits, st.ColdStarts)
	}

	// An unbounded worker zeroes the finite capacity figure (matches Capacity).
	unb, _ := regFresh(t, map[string]workerapi.Heartbeat{
		"full":      {Running: 10, MaxWarm: 10},
		"unbounded": {Running: 3, MaxWarm: 0},
	})
	if got := unb.Stat(); got.Capacity != 0 || got.Workers != 2 {
		t.Fatalf("unbounded stat = %+v, want capacity=0 workers=2", got)
	}
}

func TestLoadRatioUnboundedIsHeadroom(t *testing.T) {
	// An unlimited worker (max_warm 0) means the fleet has spare room, so the
	// scale-out signal must read 0, not running/running = 1 ("scale forever").
	r, _ := regFresh(t, map[string]workerapi.Heartbeat{
		"unbounded": {Running: 100, MaxWarm: 0},
	})
	if got := r.LoadRatio(); got != 0 {
		t.Fatalf("all-unbounded load = %v, want 0", got)
	}
	// Even mixed with a full bounded worker, the unbounded one is headroom.
	mixed, _ := regFresh(t, map[string]workerapi.Heartbeat{
		"full":      {Running: 10, MaxWarm: 10},
		"unbounded": {Running: 3, MaxWarm: 0},
	})
	if got := mixed.LoadRatio(); got != 0 {
		t.Fatalf("mixed-with-unbounded load = %v, want 0", got)
	}
	// No workers at all: fully loaded (need capacity).
	if got := New(time.Minute).LoadRatio(); got != 1 {
		t.Fatalf("empty fleet load = %v, want 1", got)
	}
}

func TestEnsureRoutesToPlacedWorker(t *testing.T) {
	r, bes := regFresh(t, map[string]workerapi.Heartbeat{
		"w1": {Running: 0, MaxWarm: 10},
		"w2": {Running: 0, MaxWarm: 10},
	})
	k := supervise.BackendKey(9, "hh")
	placed := r.place(k)
	addr, err := r.EnsureRunning(context.Background(), k, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if addr != placed.id+":8000" {
		t.Fatalf("routed to %q, expected worker %s", addr, placed.id)
	}
	if len(bes[placed.id].ensured) != 1 {
		t.Fatalf("placed worker %s was not asked to ensure", placed.id)
	}
}

// TestStatSurvivesWorkerRestart: a worker's since-boot counters running
// backwards means it restarted (routine on spot); the registry banks the
// previous boot's totals so the fleet-wide sums stay monotonic.
func TestStatSurvivesWorkerRestart(t *testing.T) {
	r, _ := regFresh(t, map[string]workerapi.Heartbeat{
		"w": {MaxWarm: 10, WarmHits: 40, ColdStarts: 5},
	})
	// The worker reboots and reports fresh, smaller counters.
	r.recordHeartbeat("w", workerapi.Heartbeat{MaxWarm: 10, WarmHits: 2, ColdStarts: 1}, time.Now(), true)
	st := r.Stat()
	if st.WarmHits != 42 || st.ColdStarts != 6 {
		t.Fatalf("hits after restart = %d/%d, want 42/6 (banked 40/5 + fresh 2/1)", st.WarmHits, st.ColdStarts)
	}
}

// Workers re-announce every ~20s; replacing the state on each announcement
// zeroed heartbeat freshness, making a healthy worker unplaceable until the
// next heartbeat poll (~a quarter of wall-clock) — and letting a small fleet
// transiently read as empty, which registered phantom unserved demand.
func TestReRegistrationPreservesFreshness(t *testing.T) {
	r := New(time.Minute)
	if isNew := r.Add("w", "i-1", &fakeBackend{id: "w"}); !isNew {
		t.Fatal("first Add should report new")
	}
	r.recordHeartbeat("w", workerapi.Heartbeat{MaxWarm: 10, Running: 1}, time.Now(), true)

	// Same endpoint + same instance: a re-announcement. Live state must hold.
	if isNew := r.Add("w", "i-1", &fakeBackend{id: "w"}); isNew {
		t.Fatal("re-announcement should not report new")
	}
	if w := r.place(supervise.BackendKey(1, "h")); w == nil {
		t.Fatal("worker lost placement freshness on re-announcement")
	}
	if _, workers := r.LoadSample(); workers != 1 {
		t.Fatalf("LoadSample workers = %d, want 1 (freshness lost)", workers)
	}

	// Same endpoint, different instance-id: an ASG replacement reusing the
	// IP — a different machine, so fresh (heartbeat-blind) state is correct.
	if isNew := r.Add("w", "i-2", &fakeBackend{id: "w"}); !isNew {
		t.Fatal("new instance-id should replace")
	}
	if w := r.place(supervise.BackendKey(1, "h")); w != nil {
		t.Fatal("replaced worker must be blind until its first heartbeat")
	}
}

// Process events ride the worker heartbeat to the control node's event sink —
// a worker's own database is ephemeral, so the sink is the durable trail.
func TestHeartbeatShipsEventsToSink(t *testing.T) {
	r := New(time.Minute)
	be := &fakeBackend{id: "w", hb: workerapi.Heartbeat{
		MaxWarm: 10,
		Events: []supervise.ProcEventRecord{
			{RepoID: 1, BeHash: "h", Event: "start_attempt"},
			{RepoID: 1, BeHash: "h", Event: "healthy", Detail: "port 1"},
		},
	}}
	r.Add("w", "", be)
	got := make(chan []supervise.ProcEventRecord, 1)
	r.SetEventSink(func(evs []supervise.ProcEventRecord) {
		select {
		case got <- evs:
		default:
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.StartHeartbeats(ctx, 10*time.Millisecond)
	select {
	case evs := <-got:
		if len(evs) != 2 || evs[1].Event != "healthy" {
			t.Fatalf("sink received %+v", evs)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("event sink never received the heartbeat batch")
	}
}

// TestMinWarmQuotasAreFleetWide asserts the floor protects the N most-recent
// processes ACROSS workers, not N per worker: quotas follow where the fleet's
// freshest processes live and sum to the floor.
func TestMinWarmQuotasAreFleetWide(t *testing.T) {
	base := time.Now()
	r, bes := regFresh(t, map[string]workerapi.Heartbeat{
		"w1": {}, "w2": {},
	})
	bes["w1"].report = []supervise.ProcReport{
		{Key: supervise.BackendKey(1, "a"), Status: supervise.StatusRunning, LastTouch: base.Add(-1 * time.Hour)},
		{Key: supervise.BackendKey(1, "b"), Status: supervise.StatusRunning, LastTouch: base.Add(-3 * time.Hour)},
		// Crashed holds no floor slot, however fresh.
		{Key: supervise.BackendKey(1, "x"), Status: supervise.StatusCrashed, LastTouch: base},
	}
	bes["w2"].report = []supervise.ProcReport{
		{Key: supervise.BackendKey(2, "c"), Status: supervise.StatusRunning, LastTouch: base},
		{Key: supervise.BackendKey(2, "d"), Status: supervise.StatusStarting, LastTouch: base.Add(-2 * time.Hour)},
		{Key: supervise.BackendKey(2, "e"), Status: supervise.StatusRunning, LastTouch: base.Add(-4 * time.Hour)},
	}

	r.SetMinWarm(3)
	quotas, ok := r.minWarmQuotas(context.Background())
	if !ok {
		t.Fatal("floor set but quotas reported unset")
	}
	// Top 3 by recency: c (base, w2), a (-1h, w1), d (-2h, w2).
	if quotas["w1"] != 1 || quotas["w2"] != 2 {
		t.Fatalf("quotas = %v, want w1:1 w2:2", quotas)
	}

	// A floor larger than the fleet's processes protects them all.
	r.SetMinWarm(50)
	r.reportAt = time.Time{} // bust the report cache
	if quotas, _ = r.minWarmQuotas(context.Background()); quotas["w1"]+quotas["w2"] != 5 {
		t.Fatalf("oversized floor quotas = %v, want 5 total", quotas)
	}
}

// TestMinWarmQuotasUnsetAndWindow asserts the two off states: an unset floor
// leaves workers alone entirely, and a set floor outside its active window
// pushes 0 everywhere (so the fleet drains).
func TestMinWarmQuotasUnsetAndWindow(t *testing.T) {
	r, bes := regFresh(t, map[string]workerapi.Heartbeat{"w1": {}})
	bes["w1"].report = []supervise.ProcReport{
		{Key: supervise.BackendKey(1, "a"), Status: supervise.StatusRunning, LastTouch: time.Now()},
	}

	if _, ok := r.minWarmQuotas(context.Background()); ok {
		t.Fatal("unset floor must report ok=false (workers keep boot values)")
	}

	r.SetMinWarm(12)
	r.SetMinWarmWindow(func(time.Time) bool { return false })
	quotas, ok := r.minWarmQuotas(context.Background())
	if !ok || quotas["w1"] != 0 {
		t.Fatalf("outside the window: quotas = %v, ok = %v, want 0/true", quotas, ok)
	}
}

// TestHeartbeatLoopPushesMinWarmQuotas asserts the plumbing: the poll pushes
// each worker its own share, and outside the window it pushes 0 over a
// worker's nonzero boot value.
func TestHeartbeatLoopPushesMinWarmQuotas(t *testing.T) {
	base := time.Now()
	r, bes := regFresh(t, map[string]workerapi.Heartbeat{
		"w1": {MinWarm: 5}, "w2": {MinWarm: 5},
	})
	bes["w1"].hb = workerapi.Heartbeat{MinWarm: 5}
	bes["w2"].hb = workerapi.Heartbeat{MinWarm: 5}
	bes["w1"].report = []supervise.ProcReport{
		{Key: supervise.BackendKey(1, "a"), Status: supervise.StatusRunning, LastTouch: base},
	}
	bes["w2"].report = []supervise.ProcReport{
		{Key: supervise.BackendKey(2, "b"), Status: supervise.StatusRunning, LastTouch: base.Add(-time.Hour)},
		{Key: supervise.BackendKey(2, "c"), Status: supervise.StatusRunning, LastTouch: base.Add(-2 * time.Hour)},
	}
	r.SetMinWarm(2)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.StartHeartbeats(ctx, 10*time.Millisecond)

	deadline := time.After(2 * time.Second)
	for {
		_, w1Min := bes["w1"].lastConfigured()
		_, w2Min := bes["w2"].lastConfigured()
		if w1Min == 1 && w2Min == 1 { // top-2: a (w1), b (w2)
			return
		}
		select {
		case <-deadline:
			t.Fatalf("quotas never pushed: w1=%d w2=%d, want 1/1", w1Min, w2Min)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// TestIdleWorkers counts only reclaimable nodes: fresh, autoscaled, serving
// nothing — draining workers (already mid-termination) and hand-wired ones
// (no instance to reclaim) are excluded.
func TestIdleWorkers(t *testing.T) {
	r := New(time.Minute)
	add := func(id, instanceID string, hb workerapi.Heartbeat, at time.Time) {
		r.Add(id, instanceID, &fakeBackend{id: id})
		r.recordHeartbeat(id, hb, at, true)
	}
	now := time.Now()
	add("idle", "i-idle", workerapi.Heartbeat{Running: 0}, now)
	add("busy", "i-busy", workerapi.Heartbeat{Running: 3}, now)
	add("draining", "i-drain", workerapi.Heartbeat{Running: 0, Draining: true}, now)
	add("handwired", "", workerapi.Heartbeat{Running: 0}, now)
	add("stale", "i-stale", workerapi.Heartbeat{Running: 0}, now.Add(-time.Hour))

	if got := r.IdleWorkers(); got != 1 {
		t.Fatalf("IdleWorkers = %d, want 1 (only the fresh, non-draining, autoscaled idle one)", got)
	}
}
