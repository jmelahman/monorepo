package supervise

// Point-in-time resource sampling — "docker stats" for previews. Works for
// both process kinds: containers are sampled through the daemon's stats
// endpoint, host processes by summing their process group in /proc
// (Linux-only; elsewhere host processes report no stats). CPU percentages
// need a delta, so each process keeps its previous sample and the first
// query after a start reports memory only.

import (
	"context"
	"runtime"
	"time"
)

// ProcessStats is one resource sample of a running supervised process. JSON
// tags because it crosses the worker wire verbatim (workerapi report).
type ProcessStats struct {
	Runtime     string    `json:"runtime,omitempty"` // "host" or "container"
	StartedAt   time.Time `json:"started_at,omitempty"`
	CPUPercent  *float64  `json:"cpu_percent,omitempty"` // % of one core, docker-stats convention; nil until a second sample exists
	MemoryBytes uint64    `json:"memory_bytes,omitempty"`
	// MemoryLimitBytes is the cgroup limit for containers (host total when
	// unlimited) and total system memory for host processes.
	MemoryLimitBytes uint64 `json:"memory_limit_bytes,omitempty"`
}

// cpuSample is the prior observation a CPU percentage is computed against.
type cpuSample struct {
	at     time.Time
	proc   uint64 // host: process-group CPU ticks; container: CPU nanoseconds
	system uint64 // container only: host CPU nanoseconds across all cores
	valid  bool
}

// Stats samples resource usage of the artifact's process, or nil when it
// isn't running or can't be sampled. Best-effort by design: errors read as
// "no stats", never as a failed request. Sampling is not a touch — watching
// a preview's stats doesn't keep it warm.
func (m *Manager) Stats(ctx context.Context, k Key) *ProcessStats {
	m.mu.Lock()
	p := m.procs[k]
	m.mu.Unlock()
	// Only ready processes are sampled: cmd/containerID are immutable once
	// ready closes, so no further locking is needed to read them.
	if p == nil || !isReady(p) || isExited(p) {
		return nil
	}
	p.statsMu.Lock()
	defer p.statsMu.Unlock()
	switch {
	case p.containerID != "":
		return m.containerStats(ctx, p)
	case p.cmd != nil && p.cmd.Process != nil:
		return hostStats(p)
	}
	return nil
}

func (m *Manager) containerStats(ctx context.Context, p *process) *ProcessStats {
	cli, err := m.docker(ctx)
	if err != nil {
		return nil
	}
	s, err := cli.SampleStats(ctx, p.containerID)
	if err != nil {
		return nil
	}
	out := &ProcessStats{
		Runtime:          "container",
		StartedAt:        p.startedAt,
		MemoryBytes:      s.MemoryBytes,
		MemoryLimitBytes: s.MemoryLimitBytes,
	}
	if prev := p.prevCPU; prev.valid && s.CPUTotal >= prev.proc && s.SystemCPUTotal > prev.system {
		cpus := s.OnlineCPUs
		if cpus <= 0 {
			cpus = runtime.NumCPU()
		}
		pct := float64(s.CPUTotal-prev.proc) / float64(s.SystemCPUTotal-prev.system) * float64(cpus) * 100
		out.CPUPercent = &pct
	}
	p.prevCPU = cpuSample{at: time.Now(), proc: s.CPUTotal, system: s.SystemCPUTotal, valid: true}
	return out
}

func hostStats(p *process) *ProcessStats {
	// Setpgid at start makes the child its own group leader, so its pid is
	// the pgid — the same identity stops use for signalling.
	now := time.Now()
	ticks, rss, err := sampleProcessGroup(p.cmd.Process.Pid)
	if err != nil {
		return nil
	}
	out := &ProcessStats{
		Runtime:          "host",
		StartedAt:        p.startedAt,
		MemoryBytes:      rss,
		MemoryLimitBytes: hostMemTotal(),
	}
	// A group member exiting takes its ticks with it and the sum can go
	// backwards; skip the percentage that round and rebase.
	if prev := p.prevCPU; prev.valid && ticks >= prev.proc && now.After(prev.at) {
		pct := float64(ticks-prev.proc) / clockTicksPerSec / now.Sub(prev.at).Seconds() * 100
		out.CPUPercent = &pct
	}
	p.prevCPU = cpuSample{at: now, proc: ticks, valid: true}
	return out
}
