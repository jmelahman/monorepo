package fleet

// The control node's read side of the fleet: the dashboard renders process
// status, failure detail, resource samples, and run logs, all of which are
// node-local truth on whichever worker runs (or last ran) the process. The
// Registry merges every fresh worker's bulk report into one cached view, so a
// deploy listing costs one round trip per worker per cache window — never one
// per row per field.

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/supervise"
)

// reportTTL bounds staleness of the merged fleet report. Dashboard polling is
// what drives refreshes, so this is also the fan-out rate limit: at most one
// report request per worker per window, however many rows render.
const reportTTL = 2 * time.Second

// reportTimeout bounds one worker's report fetch; a hung worker must not
// stall the dashboard.
const reportTimeout = 5 * time.Second

// procEntry is one key's reported state plus the worker that reported it —
// the worker to ask for that key's run log.
type procEntry struct {
	rep    supervise.ProcReport
	worker *workerState
}

// report returns the merged fleet view, refreshing it when older than the
// TTL. Refresh holds the mutex, so concurrent dashboard requests coalesce on
// one fan-out.
func (r *Registry) report(ctx context.Context) map[supervise.Key]procEntry {
	r.reportMu.Lock()
	defer r.reportMu.Unlock()
	if time.Since(r.reportAt) < reportTTL && r.reportCache != nil {
		return r.reportCache
	}
	now := time.Now()
	merged := map[supervise.Key]procEntry{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, w := range r.list() {
		_, last, reachable := w.snapshot()
		if !r.fresh(last, reachable, now) {
			continue
		}
		wg.Add(1)
		go func(w *workerState) {
			defer wg.Done()
			rctx, cancel := context.WithTimeout(ctx, reportTimeout)
			defer cancel()
			procs, err := w.be.Report(rctx)
			if err != nil {
				return // absent worker reads as idle; heartbeats age it out
			}
			mu.Lock()
			defer mu.Unlock()
			for _, p := range procs {
				// A key can appear on two workers (placement moved after a
				// scale event); prefer the live process over a stale crash
				// record.
				if prev, ok := merged[p.Key]; ok && rank(prev.rep.Status) >= rank(p.Status) {
					continue
				}
				merged[p.Key] = procEntry{rep: p, worker: w}
			}
		}(w)
	}
	wg.Wait()
	r.reportCache = merged
	r.reportAt = time.Now()
	return merged
}

// rank orders statuses by how alive they are, for merging duplicate keys.
func rank(status string) int {
	switch status {
	case supervise.StatusRunning:
		return 3
	case supervise.StatusStarting:
		return 2
	case supervise.StatusCrashed:
		return 1
	}
	return 0
}

// Status implements the dashboard's RuntimeView against the fleet: a key
// absent from every worker's report is idle.
func (r *Registry) Status(k supervise.Key) string {
	if e, ok := r.report(context.Background())[k]; ok {
		return e.rep.Status
	}
	return supervise.StatusIdle
}

// LastFailure returns the failure detail behind a "crashed" status.
func (r *Registry) LastFailure(k supervise.Key) (supervise.Failure, bool) {
	if e, ok := r.report(context.Background())[k]; ok && e.rep.Error != "" {
		return supervise.Failure{Detail: e.rep.Error}, true
	}
	return supervise.Failure{}, false
}

// CrashedKeys returns every key the fleet reports as crashed.
func (r *Registry) CrashedKeys() []supervise.Key {
	var keys []supervise.Key
	for k, e := range r.report(context.Background()) {
		if e.rep.Status == supervise.StatusCrashed {
			keys = append(keys, k)
		}
	}
	return keys
}

// Stats returns the key's most recent resource sample from the fleet report.
// The sample is up to reportTTL old — the dashboard polls slower than that.
func (r *Registry) Stats(ctx context.Context, k supervise.Key) *supervise.ProcessStats {
	if e, ok := r.report(ctx)[k]; ok {
		return e.rep.Stats
	}
	return nil
}

// RunLog fetches an incremental run-log slice from the worker holding the
// key's process. Logs outlive processes, so a key nobody reports (stopped,
// reaped, or a crash older than the worker's process table) falls back to
// asking every fresh worker and keeping the answer with the newest attempt.
func (r *Registry) RunLog(repoName, side, hash string, attempt int, offset int64) (supervise.RunLog, error) {
	ctx, cancel := context.WithTimeout(context.Background(), reportTimeout)
	defer cancel()
	// Run logs are addressed by artifact identity (side, hash); RepoID/Peer
	// don't narrow which worker holds the file.
	for k, e := range r.report(ctx) {
		if k.Side == supervise.Side(side) && k.Hash == hash {
			return e.worker.be.RunLog(ctx, repoName, side, hash, attempt, offset)
		}
	}
	var best supervise.RunLog
	now := time.Now()
	for _, w := range r.list() {
		_, last, reachable := w.snapshot()
		if !r.fresh(last, reachable, now) {
			continue
		}
		chunk, err := w.be.RunLog(ctx, repoName, side, hash, attempt, offset)
		if err == nil && chunk.Attempt > best.Attempt {
			best = chunk
		}
	}
	return best, nil
}

// Exec forwards one exec session to the worker reporting the key's process.
// Unlike run logs there is no fall-back sweep: exec needs the live process,
// so a key nobody reports is simply not running.
func (r *Registry) Exec(ctx context.Context, k supervise.Key, opts supervise.ExecOptions, stream io.ReadWriter) error {
	e, ok := r.report(ctx)[k]
	if !ok {
		return fmt.Errorf("preview process is not running on any worker — open the preview to start it, then retry")
	}
	return e.worker.be.Exec(ctx, k, opts, stream)
}

// StartDeploy warm-starts a deploy's processes on the fleet — the same
// pre-warm a single node does after a build, routed through placement so the
// process lands where user traffic will find it (a control node's own
// Manager supervises nothing, so warming there would burn control-node
// memory on a process the proxy never routes to).
func (r *Registry) StartDeploy(ctx context.Context, row db.DeployRow) error {
	if row.BeHash != "" {
		if _, err := r.EnsureRunning(ctx, supervise.BackendKey(row.RepoID, row.BeHash), row.RepoName); err != nil {
			return fmt.Errorf("start backend: %w", err)
		}
	}
	if row.FeHash != "" && row.HasFeProcess {
		if _, err := r.EnsureRunning(ctx, supervise.FrontendKey(row.RepoID, row.FeHash, row.BeHash), row.RepoName); err != nil {
			return fmt.Errorf("start frontend: %w", err)
		}
	}
	return nil
}

// Stop fans a stop out to every fresh worker: cheap, and placement fallbacks
// mean the process may not be where rendezvous says.
func (r *Registry) Stop(k supervise.Key, reason string) {
	ctx, cancel := context.WithTimeout(context.Background(), reportTimeout)
	defer cancel()
	now := time.Now()
	for _, w := range r.list() {
		_, last, reachable := w.snapshot()
		if !r.fresh(last, reachable, now) {
			continue
		}
		w.be.Stop(ctx, k, reason) //nolint:errcheck // best-effort; idle reap is the backstop
	}
	r.invalidateReport()
}

// StopRepo stops every reported process of the repo across the fleet.
func (r *Registry) StopRepo(repoID int64, reason string) {
	for k := range r.report(context.Background()) {
		if k.RepoID == repoID {
			r.Stop(k, reason)
		}
	}
}

// invalidateReport forces the next read to re-fetch — a stop should be
// visible on the dashboard's next poll, not a TTL later.
func (r *Registry) invalidateReport() {
	r.reportMu.Lock()
	r.reportAt = time.Time{}
	r.reportMu.Unlock()
}
