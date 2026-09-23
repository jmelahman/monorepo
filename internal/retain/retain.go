// Package retain enforces the instance's artifact retention policy: a
// periodic (or on-demand) sweep that evicts deploys exceeding a per-repo
// count or age limit, reclaiming their artifacts, state, and logs via
// supervise.Manager.GCDeploy. Eviction keeps the deploy row as history —
// an evicted deploy can be revived with a redeploy.
package retain

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/store"
	"github.com/jmelahman/local-preview/internal/supervise"
)

// SettingKey is the settings-table key the policy is persisted under.
const SettingKey = "retention_policy"

// DefaultInterval is how often the background sweep runs.
const DefaultInterval = time.Hour

// tmpMaxAge matches the startup SweepTmp horizon: staging entries older
// than this are crash leftovers.
const tmpMaxAge = 24 * time.Hour

// Policy is the instance's retention policy. The zero value disables
// automatic eviction entirely.
type Policy struct {
	// MaxDeploysPerRepo keeps at most N non-evicted deploys per repo,
	// newest first. 0 = unlimited.
	MaxDeploysPerRepo int `json:"max_deploys_per_repo"`
	// MaxAgeDays evicts deploys created more than N days ago. 0 = unlimited.
	MaxAgeDays int `json:"max_age_days"`
}

// Enabled reports whether the policy evicts anything.
func (p Policy) Enabled() bool { return p.MaxDeploysPerRepo > 0 || p.MaxAgeDays > 0 }

// Validate rejects nonsensical limits.
func (p Policy) Validate() error {
	if p.MaxDeploysPerRepo < 0 {
		return fmt.Errorf("max_deploys_per_repo must be >= 0 (0 = unlimited)")
	}
	if p.MaxAgeDays < 0 {
		return fmt.Errorf("max_age_days must be >= 0 (0 = unlimited)")
	}
	return nil
}

// LoadPolicy reads the persisted policy; an unset key is the zero
// (disabled) policy.
func LoadPolicy(database *db.Store) (Policy, error) {
	raw, err := database.GetSetting(SettingKey)
	if err == db.ErrNotFound {
		return Policy{}, nil
	}
	if err != nil {
		return Policy{}, err
	}
	var p Policy
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return Policy{}, fmt.Errorf("parse %s setting: %w", SettingKey, err)
	}
	return p, nil
}

// SavePolicy validates and persists the policy.
func SavePolicy(database *db.Store, p Policy) error {
	if err := p.Validate(); err != nil {
		return err
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return database.SetSetting(SettingKey, string(raw))
}

// Evicted describes one deploy a sweep evicted.
type Evicted struct {
	ID       int64  `json:"id"`
	Repo     string `json:"repo"`
	ShortSHA string `json:"short_sha"`
	Branch   string `json:"branch,omitempty"`
}

// Result summarizes one sweep.
type Result struct {
	Policy  Policy    `json:"policy"`
	Evicted []Evicted `json:"evicted"`
}

// Sweeper runs retention sweeps against the instance's deploys.
type Sweeper struct {
	db    *db.Store
	super *supervise.Manager
	files *store.Store

	interval time.Duration
	now      func() time.Time

	mu sync.Mutex // one sweep at a time (ticker vs. POST /api/gc)
}

// New returns a Sweeper. Call Start for the background loop; Sweep runs one
// pass on demand.
func New(database *db.Store, super *supervise.Manager, files *store.Store) *Sweeper {
	return &Sweeper{
		db:       database,
		super:    super,
		files:    files,
		interval: DefaultInterval,
		now:      time.Now,
	}
}

// SetInterval overrides how often Start's loop sweeps. Non-positive
// durations are ignored — a caller disabling the sweep just doesn't Start
// it. Call before Start.
func (s *Sweeper) SetInterval(d time.Duration) {
	if d > 0 {
		s.interval = d
	}
}

// Start launches the periodic sweep: once at startup (an instance that was
// down past its limits catches up immediately), then every interval.
func (s *Sweeper) Start(ctx context.Context) {
	go func() {
		s.logged()
		tick := time.NewTicker(s.interval)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				s.logged()
			}
		}
	}()
}

func (s *Sweeper) logged() {
	res, err := s.Sweep()
	if err != nil {
		log.Printf("retention sweep: %v", err)
		return
	}
	if len(res.Evicted) > 0 {
		log.Printf("retention sweep: evicted %d deploys", len(res.Evicted))
	}
}

// Sweep runs one retention pass and reports what it evicted. Deploys are
// considered per repo, newest first; a deploy is evicted when it exceeds
// the policy's count or age limit. Never evicted: queued/building deploys,
// each repo's newest ready deploy (a repo always keeps one working
// preview), and branch-alias targets (their branch URL must keep working).
// Stale tmp/ staging entries are collected on every sweep, policy or not.
func (s *Sweeper) Sweep() (Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.files.SweepTmp(tmpMaxAge); err != nil {
		log.Printf("retention sweep: sweep tmp: %v", err)
	}

	policy, err := LoadPolicy(s.db)
	if err != nil {
		return Result{}, err
	}
	res := Result{Policy: policy, Evicted: []Evicted{}}
	if !policy.Enabled() {
		return res, nil
	}

	aliased, err := s.db.BranchAliasDeployIDs()
	if err != nil {
		return res, err
	}
	rows, err := s.db.ListDeploys(db.DeployFilter{}) // newest first
	if err != nil {
		return res, err
	}
	var cutoff time.Time
	if policy.MaxAgeDays > 0 {
		cutoff = s.now().AddDate(0, 0, -policy.MaxAgeDays)
	}

	kept := map[int64]int{}         // repo id → deploys retained so far
	newestReady := map[int64]bool{} // repo id → newest ready deploy seen
	for _, row := range rows {
		if row.Status == db.DeployEvicted {
			continue // holds nothing; doesn't count toward the limit
		}
		protected := row.Status == db.DeployQueued || row.Status == db.DeployBuilding ||
			aliased[row.ID]
		if row.Status == db.DeployReady && !newestReady[row.RepoID] {
			newestReady[row.RepoID] = true
			protected = true
		}
		if !protected {
			overCount := policy.MaxDeploysPerRepo > 0 && kept[row.RepoID] >= policy.MaxDeploysPerRepo
			tooOld := policy.MaxAgeDays > 0 && createdBefore(row.CreatedAt, cutoff)
			if overCount || tooOld {
				if err := s.db.SetDeployEvicted(row.ID); err != nil {
					log.Printf("retention sweep: evict deploy %d: %v", row.ID, err)
					continue
				}
				s.super.GCDeploy(row)
				res.Evicted = append(res.Evicted, Evicted{
					ID: row.ID, Repo: row.RepoName, ShortSHA: row.ShortSHA, Branch: row.Branch,
				})
				continue
			}
		}
		kept[row.RepoID]++
	}
	return res, nil
}

// createdBefore reports whether the deploy timestamp (RFC 3339, as written
// by SQLite's strftime) is before cutoff. Unparseable timestamps are never
// "too old" — better to keep bytes than evict on bad data.
func createdBefore(createdAt string, cutoff time.Time) bool {
	t, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return false
	}
	return t.Before(cutoff)
}
