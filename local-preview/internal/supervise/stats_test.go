package supervise

import (
	"context"
	"runtime"
	"testing"
	"time"
)

func TestHostStats(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("host stats sample /proc")
	}
	f := newFixture(t)
	f.provision(t, "be-stats", serverArgv(t))
	k := BackendKey(f.repoID, "be-stats")
	ctx := context.Background()

	// Not running yet: no stats.
	if s := f.m.Stats(ctx, k); s != nil {
		t.Fatalf("stats before start = %+v, want nil", s)
	}

	if _, err := f.m.EnsureRunning(ctx, k, "demo"); err != nil {
		t.Fatal(err)
	}
	first := f.m.Stats(ctx, k)
	if first == nil {
		t.Fatal("no stats for a running process")
	}
	if first.Runtime != "host" || first.MemoryBytes == 0 || first.MemoryLimitBytes == 0 {
		t.Fatalf("first sample = %+v", first)
	}
	if first.CPUPercent != nil {
		t.Fatalf("first sample has cpu_percent %v, want nil (no delta yet)", *first.CPUPercent)
	}
	if first.StartedAt.IsZero() || time.Since(first.StartedAt) > time.Minute {
		t.Fatalf("started_at = %v", first.StartedAt)
	}

	time.Sleep(50 * time.Millisecond)
	second := f.m.Stats(ctx, k)
	if second == nil || second.CPUPercent == nil {
		t.Fatalf("second sample = %+v, want cpu_percent set", second)
	}
	if *second.CPUPercent < 0 {
		t.Fatalf("cpu_percent = %v", *second.CPUPercent)
	}

	f.m.Stop(k, "test")
	if s := f.m.Stats(ctx, k); s != nil {
		t.Fatalf("stats after stop = %+v, want nil", s)
	}
}
