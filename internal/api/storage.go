package api

import (
	"net/http"

	"github.com/jmelahman/local-preview/internal/retain"
	"github.com/jmelahman/local-preview/internal/usage"
)

// usageDirs locates the trees storage reporting walks.
func (d Deps) usageDirs() usage.Dirs {
	return usage.Dirs{
		Artifacts: d.Config.ArtifactsDir(),
		State:     d.Config.StateDir(),
		Logs:      d.Config.LogsDir(),
		Tmp:       d.Config.TmpDir(),
		DBPath:    d.DBPath,
	}
}

// handleStorage reports the instance's disk usage, by category and by repo.
func (d Deps) handleStorage(w http.ResponseWriter, r *http.Request) {
	rep, err := usage.Compute(d.Store, d.usageDirs(), d.durableTier())
	if err != nil {
		internalError(w, "compute storage usage", err)
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

// durableTier returns the durable artifact tier as a usage reporter, or nil
// when none is configured. The tier is owned by the store; storage reporting
// only needs its size accessor.
func (d Deps) durableTier() usage.DurableTier {
	t := d.Files.ArtifactTier()
	if t == nil {
		return nil
	}
	if dt, ok := t.(usage.DurableTier); ok {
		return dt
	}
	return nil
}

func (d Deps) handleGetRetention(w http.ResponseWriter, r *http.Request) {
	policy, err := retain.LoadPolicy(d.Store)
	if err != nil {
		internalError(w, "load retention policy", err)
		return
	}
	writeJSON(w, http.StatusOK, policy)
}

// handlePutRetention replaces the retention policy. The new policy takes
// effect on the next sweep — hourly, or immediately via POST /api/gc — so
// tightening limits never surprise-evicts on save.
func (d Deps) handlePutRetention(w http.ResponseWriter, r *http.Request) {
	var p retain.Policy
	if !decodeJSON(w, r, &p) {
		return
	}
	if err := p.Validate(); err != nil {
		httpError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := retain.SavePolicy(d.Store, p); err != nil {
		internalError(w, "save retention policy", err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// gcJSON is the POST /api/gc response: what the sweep evicted and how much
// disk it gave back.
type gcJSON struct {
	retain.Result
	FreedBytes int64 `json:"freed_bytes"`
}

// handleRunGC runs one retention sweep immediately. With retention disabled
// it still collects stale tmp/ staging leftovers.
func (d Deps) handleRunGC(w http.ResponseWriter, r *http.Request) {
	dirs := d.usageDirs()
	before := usage.Reclaimable(dirs)
	res, err := d.Sweeper.Sweep()
	if err != nil {
		internalError(w, "retention sweep", err)
		return
	}
	writeJSON(w, http.StatusOK, gcJSON{
		Result:     res,
		FreedBytes: max(0, before-usage.Reclaimable(dirs)),
	})
}

func (d Deps) handleGetWarm(w http.ResponseWriter, r *http.Request) {
	if d.WarmPolicy == nil {
		httpError(w, http.StatusNotFound, "warm policy not configurable on this server")
		return
	}
	writeJSON(w, http.StatusOK, d.WarmPolicy())
}

// handlePutWarm persists and applies a new warm policy. Takes effect without
// a restart: the local reaper enforces both values on its next tick (the idle
// override governs already-running processes too), and the fleet's heartbeat
// loop pushes them to every worker (re-pushing after worker reboots, so the
// dashboard's values outlive the flags they override).
func (d Deps) handlePutWarm(w http.ResponseWriter, r *http.Request) {
	if d.SetWarmPolicy == nil {
		httpError(w, http.StatusNotFound, "warm policy not configurable on this server")
		return
	}
	var p WarmPolicy
	if !decodeJSON(w, r, &p) {
		return
	}
	if p.MaxWarm < 0 {
		httpError(w, http.StatusBadRequest, "max_warm must be >= 0 (0 = unlimited)")
		return
	}
	if p.IdleTimeoutSeconds < 0 {
		httpError(w, http.StatusBadRequest, "idle_timeout_seconds must be >= 0 (0 = per-manifest values)")
		return
	}
	if p.MinWarm < 0 {
		httpError(w, http.StatusBadRequest, "min_warm must be >= 0")
		return
	}
	if p.MaxWarm > 0 && p.MinWarm > p.MaxWarm {
		httpError(w, http.StatusBadRequest, "min_warm must not exceed max_warm (the floor can't sit above the target)")
		return
	}
	if err := d.SetWarmPolicy(p); err != nil {
		internalError(w, "apply warm policy", err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}
