package orchestrator

import (
	"github.com/jmelahman/local-preview/internal/retain"
	"github.com/jmelahman/local-preview/internal/usage"
)

// RetentionPolicy bounds how much an instance accumulates. Deploys are the
// only thing that grows without bound — every commit previewed leaves
// artifacts, backend state, and logs behind long past their usefulness.
// The zero value evicts nothing.
type RetentionPolicy struct {
	// MaxDeploysPerRepo keeps at most N non-evicted deploys per repo,
	// newest first. 0 = unlimited.
	MaxDeploysPerRepo int `json:"max_deploys_per_repo"`
	// MaxAgeDays evicts deploys created more than N days ago. 0 = unlimited.
	MaxAgeDays int `json:"max_age_days"`
}

// Evicted describes one deploy a sweep evicted. The deploy row survives as
// history — only the bytes are reclaimed, and a redeploy revives it.
type Evicted struct {
	ID       int64  `json:"id"`
	Repo     string `json:"repo"`
	ShortSHA string `json:"short_sha"`
	Branch   string `json:"branch,omitempty"`
}

// GCResult summarizes one sweep.
type GCResult struct {
	Policy  RetentionPolicy `json:"policy"`
	Evicted []Evicted       `json:"evicted"`
	// FreedBytes is how much disk the sweep gave back, measured across the
	// categories a sweep can shrink.
	FreedBytes int64 `json:"freed_bytes"`
}

// StorageReport is the instance's disk footprint by category and by repo.
type StorageReport = usage.Report

// RepoStorage is one repo's slice of a StorageReport.
type RepoStorage = usage.RepoUsage

// RetentionPolicy returns the instance's persisted retention policy.
func (o *Orchestrator) RetentionPolicy() (RetentionPolicy, error) {
	p, err := retain.LoadPolicy(o.database)
	if err != nil {
		return RetentionPolicy{}, err
	}
	return RetentionPolicy(p), nil
}

// SetRetentionPolicy persists a new retention policy, rejecting negative
// limits. It takes effect on the next sweep — the retention interval, or
// immediately via CollectGarbage — so tightening limits never
// surprise-evicts on save.
func (o *Orchestrator) SetRetentionPolicy(p RetentionPolicy) error {
	return retain.SavePolicy(o.database, retain.Policy(p))
}

// Storage reports how much disk the instance uses. Sizes are computed by
// walking the data dir on every call, so this is a live number, not a
// cached one.
func (o *Orchestrator) Storage() (StorageReport, error) {
	var durable usage.DurableTier
	if t := o.files.ArtifactTier(); t != nil {
		if dt, ok := t.(usage.DurableTier); ok {
			durable = dt
		}
	}
	return usage.Compute(o.database, o.usageDirs(), durable)
}

// CollectGarbage runs one retention sweep immediately and reports what it
// evicted. With retention disabled it still collects stale staging
// leftovers, so it is always worth calling.
func (o *Orchestrator) CollectGarbage() (GCResult, error) {
	dirs := o.usageDirs()
	before := usage.Reclaimable(dirs)
	res, err := o.sweeper.Sweep()
	if err != nil {
		return GCResult{}, err
	}
	out := GCResult{
		Policy:     RetentionPolicy(res.Policy),
		Evicted:    make([]Evicted, len(res.Evicted)),
		FreedBytes: max(0, before-usage.Reclaimable(dirs)),
	}
	for i, e := range res.Evicted {
		out.Evicted[i] = Evicted(e)
	}
	return out, nil
}

// usageDirs locates the trees storage reporting walks. They mirror the
// layout New lays down under DataDir.
func (o *Orchestrator) usageDirs() usage.Dirs {
	return usage.Dirs{
		Artifacts: o.opts.DataDir + "/artifacts",
		State:     o.opts.DataDir + "/state",
		Logs:      o.opts.DataDir + "/logs",
		Tmp:       o.opts.DataDir + "/tmp",
		DBPath:    o.dbPath,
	}
}
