package dockerapi

// Point-in-time container resource sampling — the container half of the
// dashboard's docker-stats-like view of supervised preview processes.

import (
	"context"
	"encoding/json"
	"fmt"
)

// ContainerStats is one resource sample of a running container, reduced to
// what the observability endpoints report. CPU fields are cumulative
// counters: a percentage needs two samples —
// Δ(CPUTotal)/Δ(SystemCPUTotal) × OnlineCPUs × 100.
type ContainerStats struct {
	CPUTotal       uint64 // container CPU consumed, nanoseconds
	SystemCPUTotal uint64 // host CPU across all cores, nanoseconds
	OnlineCPUs     int    // 0 when the daemon doesn't report it
	// MemoryBytes is usage minus reclaimable file cache — the figure
	// `docker stats` shows.
	MemoryBytes      uint64
	MemoryLimitBytes uint64 // cgroup limit; the host total when unlimited
}

// SampleStats takes one stats sample of a running container. one-shot mode
// skips the daemon's ~1s priming read, so calls return immediately.
func (c *Client) SampleStats(ctx context.Context, id string) (ContainerStats, error) {
	raw, err := c.raw(ctx, "GET", "/containers/"+id+"/stats?stream=false&one-shot=true", nil)
	if err != nil {
		return ContainerStats{}, err
	}
	var s struct {
		CPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemCPUUsage uint64 `json:"system_cpu_usage"`
			OnlineCPUs     int    `json:"online_cpus"`
		} `json:"cpu_stats"`
		MemoryStats struct {
			Usage uint64            `json:"usage"`
			Stats map[string]uint64 `json:"stats"`
			Limit uint64            `json:"limit"`
		} `json:"memory_stats"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return ContainerStats{}, fmt.Errorf("decode container stats: %w", err)
	}
	mem := s.MemoryStats.Usage
	// Discount reclaimable file cache like `docker stats` does:
	// inactive_file under cgroup v2, total_inactive_file under v1.
	inactive, ok := s.MemoryStats.Stats["inactive_file"]
	if !ok {
		inactive = s.MemoryStats.Stats["total_inactive_file"]
	}
	if inactive < mem {
		mem -= inactive
	}
	return ContainerStats{
		CPUTotal:         s.CPUStats.CPUUsage.TotalUsage,
		SystemCPUTotal:   s.CPUStats.SystemCPUUsage,
		OnlineCPUs:       s.CPUStats.OnlineCPUs,
		MemoryBytes:      mem,
		MemoryLimitBytes: s.MemoryStats.Limit,
	}, nil
}
