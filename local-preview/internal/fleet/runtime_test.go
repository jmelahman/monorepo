package fleet

import (
	"context"
	"testing"
	"time"

	"github.com/jmelahman/local-preview/internal/supervise"
	"github.com/jmelahman/local-preview/internal/workerapi"
)

// The runtime view merges worker reports: statuses, crash details, stats, and
// run logs come from whichever worker holds the process; unknown keys are idle.
func TestRuntimeViewMergesFleetReports(t *testing.T) {
	r, bes := regFresh(t, map[string]workerapi.Heartbeat{
		"w1": {MaxWarm: 10},
		"w2": {MaxWarm: 10},
	})
	running := supervise.BackendKey(1, "runhash")
	crashed := supervise.BackendKey(1, "crashhash")
	mem := uint64(64)
	bes["w1"].report = []supervise.ProcReport{{
		Key: running, Repo: "demo", Status: supervise.StatusRunning,
		Stats: &supervise.ProcessStats{MemoryBytes: mem},
	}}
	bes["w1"].logs = map[string]supervise.RunLog{"be-runhash": {Attempt: 2, Content: "log line"}}
	bes["w2"].report = []supervise.ProcReport{{
		Key: crashed, Status: supervise.StatusCrashed, Error: "exit status 3",
	}}

	if got := r.Status(running); got != supervise.StatusRunning {
		t.Fatalf("running status = %q", got)
	}
	if got := r.Status(supervise.BackendKey(9, "nowhere")); got != supervise.StatusIdle {
		t.Fatalf("unknown key status = %q, want idle", got)
	}
	if f, ok := r.LastFailure(crashed); !ok || f.Detail != "exit status 3" {
		t.Fatalf("failure = %+v, %v", f, ok)
	}
	if keys := r.CrashedKeys(); len(keys) != 1 || keys[0] != crashed {
		t.Fatalf("crashed keys = %v", keys)
	}
	if s := r.Stats(context.Background(), running); s == nil || s.MemoryBytes != mem {
		t.Fatalf("stats = %+v", s)
	}
	chunk, err := r.RunLog("demo", "be", "runhash", 0, 0)
	if err != nil || chunk.Attempt != 2 || chunk.Content != "log line" {
		t.Fatalf("runlog = %+v, %v", chunk, err)
	}
}

// A stop fans out to every fresh worker and is visible on the next read (the
// report cache is invalidated).
func TestRuntimeViewStopFansOutAndInvalidates(t *testing.T) {
	r, bes := regFresh(t, map[string]workerapi.Heartbeat{"w1": {}, "w2": {}})
	k := supervise.BackendKey(3, "h")
	bes["w1"].report = []supervise.ProcReport{{Key: k, Status: supervise.StatusRunning}}
	if got := r.Status(k); got != supervise.StatusRunning {
		t.Fatalf("status before stop = %q", got)
	}
	bes["w1"].report = nil // the worker will report it gone after the stop
	r.Stop(k, "test")
	for _, be := range bes {
		if len(be.stopped) != 1 || be.stopped[0] != k {
			t.Fatalf("worker %s stops = %v", be.id, be.stopped)
		}
	}
	if got := r.Status(k); got != supervise.StatusIdle {
		t.Fatalf("status after stop = %q, want idle (cache invalidated)", got)
	}
}

// Duplicate keys across workers (placement moved) resolve to the liveliest
// status.
func TestRuntimeViewPrefersLiveOverStale(t *testing.T) {
	r, bes := regFresh(t, map[string]workerapi.Heartbeat{"w1": {}, "w2": {}})
	k := supervise.BackendKey(5, "moved")
	bes["w1"].report = []supervise.ProcReport{{Key: k, Status: supervise.StatusCrashed, Error: "old crash"}}
	bes["w2"].report = []supervise.ProcReport{{Key: k, Status: supervise.StatusRunning}}
	if got := r.Status(k); got != supervise.StatusRunning {
		t.Fatalf("status = %q, want running to win over a stale crash", got)
	}
	_ = time.Now
}

// confBackend delivers Configure calls on a channel so the heartbeat-loop
// reconcile can be awaited without racing the poller goroutine.
type confBackend struct {
	fakeBackend
	got     chan int
	gotIdle chan int
}

func (c *confBackend) Configure(_ context.Context, cfg workerapi.WorkerConfig) error {
	if cfg.MaxWarm != nil {
		c.got <- *cfg.MaxWarm
	}
	if cfg.IdleTimeoutSeconds != nil {
		c.gotIdle <- *cfg.IdleTimeoutSeconds
	}
	return nil
}

// The heartbeat loop reconciles the dashboard-configured settings onto any
// worker whose reported values drift — how they survive worker reboots.
func TestHeartbeatReconcilesMaxWarm(t *testing.T) {
	r := New(time.Minute)
	be := &confBackend{got: make(chan int, 1), gotIdle: make(chan int, 1)}
	r.Add("w", "", be) // Heartbeat reports zero values — the "boot flag" state
	r.SetMaxWarm(5)
	r.SetIdleOverride(90 * time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.StartHeartbeats(ctx, 50*time.Millisecond)

	select {
	case n := <-be.got:
		if n != 5 {
			t.Fatalf("configured %d, want 5", n)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("heartbeat loop never pushed the warm cap")
	}
	select {
	case n := <-be.gotIdle:
		if n != 90 {
			t.Fatalf("configured idle %ds, want 90s", n)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("heartbeat loop never pushed the idle override")
	}
}
