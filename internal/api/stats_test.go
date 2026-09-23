package api

import (
	"encoding/json"
	"math"
	"testing"
)

func TestPercentile(t *testing.T) {
	sorted := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	cases := []struct {
		p    float64
		want float64
	}{
		{0.50, 6}, // nearest-rank: index int(0.5*10)=5 → 6
		{0.90, 10},
		{0.99, 10},
	}
	for _, c := range cases {
		if got := percentile(sorted, c.p); math.Abs(got-c.want) > 0.001 {
			t.Errorf("percentile(p=%v) = %v, want %v", c.p, got, c.want)
		}
	}
	if got := percentile([]float64{42}, 0.5); got != 42 {
		t.Errorf("single-element percentile = %v, want 42", got)
	}
}

func TestStartupPercentilesEmpty(t *testing.T) {
	st := startupPercentiles(nil)
	if st.Count != 0 || st.P50Seconds != nil || st.P90Seconds != nil {
		t.Fatalf("empty durations should yield count 0 and nil percentiles, got %+v", st)
	}
}

func TestHandleStatsSingleNode(t *testing.T) {
	deps, _ := newTestDeps(t)
	mux := NewMux(deps)

	r, err := deps.Store.CreateRepo("demo", "/src/demo", "/data/repos/demo.git", "ready")
	if err != nil {
		t.Fatal(err)
	}
	// A start_attempt paired with a healthy yields one measurable duration.
	if err := deps.Store.AddProcessEvent(r.ID, "be1", "start_attempt", ""); err != nil {
		t.Fatal(err)
	}
	if err := deps.Store.AddProcessEvent(r.ID, "be1", "healthy", ""); err != nil {
		t.Fatal(err)
	}

	rec := doJSON(t, mux, "GET", "/api/stats", "")
	if rec.Code != 200 {
		t.Fatalf("GET /api/stats = %d: %s", rec.Code, rec.Body.String())
	}
	var got statsJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Startup.Count != 1 {
		t.Errorf("startup.count = %d, want 1", got.Startup.Count)
	}
	if got.Startup.P50Seconds == nil {
		t.Error("startup.p50_seconds should be present when count > 0")
	}
	// No fleet configured → single-node view: one worker, no requests yet.
	if got.Runtime.Workers != 1 {
		t.Errorf("runtime.workers = %d, want 1 (single node)", got.Runtime.Workers)
	}
	if got.Hits.Warm != 0 || got.Hits.Cold != 0 || got.Hits.WarmRatio != nil {
		t.Errorf("expected no hits and omitted ratio, got %+v", got.Hits)
	}
}

func TestHandleStatsFleet(t *testing.T) {
	deps, _ := newTestDeps(t)
	deps.FleetStats = func() FleetSummary {
		return FleetSummary{Workers: 3, Running: 7, Capacity: 30, WarmHits: 90, ColdStarts: 10}
	}
	mux := NewMux(deps)

	rec := doJSON(t, mux, "GET", "/api/stats", "")
	if rec.Code != 200 {
		t.Fatalf("GET /api/stats = %d: %s", rec.Code, rec.Body.String())
	}
	var got statsJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Runtime.Workers != 3 || got.Runtime.Running != 7 || got.Runtime.Capacity != 30 {
		t.Errorf("runtime = %+v, want workers=3 running=7 capacity=30", got.Runtime)
	}
	if got.Hits.Warm != 90 || got.Hits.Cold != 10 {
		t.Errorf("hits = %+v, want warm=90 cold=10", got.Hits)
	}
	if got.Hits.WarmRatio == nil || math.Abs(*got.Hits.WarmRatio-0.9) > 0.001 {
		t.Errorf("warm_ratio = %v, want 0.9", got.Hits.WarmRatio)
	}
}
