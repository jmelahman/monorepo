package api

import (
	"net/http"
	"sort"
)

// FleetSummary is the runtime rollup a control node's fleet registry provides:
// fresh worker count, committed and bounded capacity, and cumulative warm/cold
// EnsureRunning counts summed across workers. Deps.FleetStats returns it (nil
// on a single node, where the stats handler falls back to the local Manager).
type FleetSummary struct {
	Workers    int
	Running    int
	Capacity   int // 0 = unbounded
	WarmHits   int64
	ColdStarts int64
}

// startupStats are cold-start latency percentiles over the reporting window.
// Percentile fields are omitted when no starts were recorded (count 0).
type startupStats struct {
	Count      int      `json:"count"`
	P50Seconds *float64 `json:"p50_seconds,omitempty"`
	P90Seconds *float64 `json:"p90_seconds,omitempty"`
	P99Seconds *float64 `json:"p99_seconds,omitempty"`
}

// hitStats are cumulative warm-vs-cold EnsureRunning outcomes. WarmRatio is
// warm/(warm+cold), omitted when there have been no requests at all.
type hitStats struct {
	Warm      int64    `json:"warm"`
	Cold      int64    `json:"cold"`
	WarmRatio *float64 `json:"warm_ratio,omitempty"`
}

// runtimeStats is the live serving footprint: worker count, committed warm
// slots, and bounded capacity (0 = unlimited).
type runtimeStats struct {
	Workers  int `json:"workers"`
	Running  int `json:"running"`
	Capacity int `json:"capacity"`
}

type statsJSON struct {
	Startup startupStats `json:"startup"`
	Hits    hitStats     `json:"hits"`
	Runtime runtimeStats `json:"runtime"`
}

// handleStats reports instance-wide statistics: cold-start latency percentiles
// (from the process_events trail), the cumulative warm-hit ratio, and the live
// serving footprint. On a control node with workers the runtime and hit figures
// come from the fleet; otherwise they come from this node's local Manager.
func (d Deps) handleStats(w http.ResponseWriter, r *http.Request) {
	durs, err := d.Store.StartupDurations(30)
	if err != nil {
		internalError(w, "compute startup durations", err)
		return
	}
	out := statsJSON{Startup: startupPercentiles(durs)}

	var warm, cold int64
	switch {
	case d.FleetStats != nil:
		fs := d.FleetStats()
		out.Runtime = runtimeStats{Workers: fs.Workers, Running: fs.Running, Capacity: fs.Capacity}
		warm, cold = fs.WarmHits, fs.ColdStarts
	case d.Super != nil:
		warm, cold = d.Super.HitStats()
		rt := runtimeStats{Workers: 1, Running: d.Super.Running()}
		if d.WarmPolicy != nil {
			rt.Capacity = d.WarmPolicy().MaxWarm
		}
		out.Runtime = rt
	}
	out.Hits = hitStats{Warm: warm, Cold: cold}
	if total := warm + cold; total > 0 {
		ratio := float64(warm) / float64(total)
		out.Hits.WarmRatio = &ratio
	}

	writeJSON(w, http.StatusOK, out)
}

// startupPercentiles reduces raw cold-start durations to p50/p90/p99.
func startupPercentiles(durs []float64) startupStats {
	st := startupStats{Count: len(durs)}
	if len(durs) == 0 {
		return st
	}
	sort.Float64s(durs)
	st.P50Seconds = ptr(percentile(durs, 0.50))
	st.P90Seconds = ptr(percentile(durs, 0.90))
	st.P99Seconds = ptr(percentile(durs, 0.99))
	return st
}

// percentile returns the p-quantile (0..1) of a sorted slice using the
// nearest-rank method. sorted must be non-empty and ascending.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 1 {
		return sorted[0]
	}
	rank := int(p * float64(len(sorted)))
	if rank >= len(sorted) {
		rank = len(sorted) - 1
	}
	return sorted[rank]
}

func ptr(f float64) *float64 { return &f }
