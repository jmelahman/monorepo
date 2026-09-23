package supervise

// The dashboard's read-side view of supervised processes: incremental run-log
// chunks and a bulk per-key report. Both live on the Manager because they read
// node-local truth (the process table, the failure table, the run-log files on
// this node's disk) — on a control node whose previews run on workers, the
// same data is fetched over the worker API and merged by the fleet registry,
// which is why the shapes here carry JSON tags.

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"
	"strings"
)

// runLogTailBytes caps how much history a fresh run-log view loads; the
// client keeps the returned offset and receives only appended bytes after.
const runLogTailBytes = 256 * 1024

// runLogChunkBytes caps one response; a lagging client catches up across
// polls.
const runLogChunkBytes = 1 << 20

// RunLog is one incremental slice of a process run log.
type RunLog struct {
	Attempt int    `json:"attempt"` // Nth start of the artifact; 0 = never started
	Offset  int64  `json:"offset"`  // echo back to receive only new bytes
	Content string `json:"content"`
	// Truncated marks a fresh view that skipped history beyond the tail cap.
	Truncated bool `json:"truncated,omitempty"`
}

// RunLog returns an incremental slice of the artifact side's run log — the
// supervised process's combined stdout+stderr (init output included). Each
// call returns the latest start attempt's log from the requested offset;
// echoing attempt and offset back yields only new bytes, and a restart (new
// attempt) resets the view to a tail of the new file. Run logs outlive their
// process, so crash output stays readable after the exit. attempt/offset from
// an earlier call select the increment; pass (0, 0) for a fresh view.
func (m *Manager) RunLog(repoName, side, hash string, attempt int, offset int64) (RunLog, error) {
	var chunk RunLog
	// Run logs are per artifact (shared by every deploy with the same hash),
	// one numbered file per start attempt.
	dir := filepath.Join(m.logsDir, repoName, "run", side+"-"+hash)
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if base, found := strings.CutSuffix(e.Name(), ".log"); found {
			if n, err := strconv.Atoi(base); err == nil && n > chunk.Attempt {
				chunk.Attempt = n
			}
		}
	}
	if chunk.Attempt == 0 {
		return chunk, nil
	}

	f, err := os.Open(filepath.Join(dir, strconv.Itoa(chunk.Attempt)+".log"))
	if err != nil {
		return chunk, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return chunk, err
	}
	size := info.Size()

	start := int64(0)
	if attempt == chunk.Attempt && offset >= 0 && offset <= size {
		start = offset
	} else if size > runLogTailBytes {
		start = size - runLogTailBytes
		chunk.Truncated = true
	}

	buf := make([]byte, min(size-start, runLogChunkBytes))
	n, err := f.ReadAt(buf, start)
	if err != nil && err != io.EOF {
		return chunk, err
	}
	chunk.Content = string(buf[:n])
	chunk.Offset = start + int64(n)
	return chunk, nil
}

// ProcReport is one key's live state in a bulk report: everything the
// dashboard renders about a process, sampled together so a fleet's control
// node fetches one response per worker instead of one round trip per field
// per deploy row.
type ProcReport struct {
	Key    Key           `json:"-"`
	Repo   string        `json:"repo,omitempty"` // empty for a crashed key whose process is gone
	Status string        `json:"status"`
	Error  string        `json:"error,omitempty"` // failure detail behind a "crashed" status
	Stats  *ProcessStats `json:"stats,omitempty"`
	// LastTouch is the process's recency (when a request last hit it) — what
	// the control node ranks fleet-wide to hand each worker its share of the
	// min-warm floor. Zero for crashed records and from workers that predate
	// the field, which ranks them least-recent.
	LastTouch time.Time `json:"last_touch,omitempty"`
}

// Report returns every key with live state on this node: tracked processes
// (starting/running, with a resource sample once ready) and recorded failures
// ("crashed" with the detail). Keys that would read "idle" are absent — idle
// is the default the reader assumes for anything unlisted.
func (m *Manager) Report(ctx context.Context) []ProcReport {
	type live struct {
		k     Key
		p     *process
		touch time.Time // captured under the lock; lastTouch is mu-guarded
	}
	m.mu.Lock()
	procs := make([]live, 0, len(m.procs))
	for k, p := range m.procs {
		procs = append(procs, live{k, p, p.lastTouch})
	}
	crashed := make([]ProcReport, 0, len(m.failures))
	for k, f := range m.failures {
		if m.procs[k] == nil {
			crashed = append(crashed, ProcReport{Key: k, Status: StatusCrashed, Error: f.Detail})
		}
	}
	m.mu.Unlock()

	out := crashed
	for _, l := range procs {
		rep := ProcReport{Key: l.k, Repo: l.p.repoName, Status: m.Status(l.k), LastTouch: l.touch}
		if rep.Status == StatusIdle {
			continue // exited between the snapshot and now; idle is the default
		}
		if rep.Status == StatusRunning {
			rep.Stats = m.Stats(ctx, l.k) // best-effort; nil is fine
		}
		out = append(out, rep)
	}
	return out
}
