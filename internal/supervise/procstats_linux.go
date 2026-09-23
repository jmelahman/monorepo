//go:build linux

package supervise

// /proc-based resource sampling for host processes. Supervised children run
// in their own process group (Setpgid), and run commands routinely fork —
// shell wrappers, node servers — so a sample sums every process whose pgrp
// matches, not just the leader.

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

// clockTicksPerSec is USER_HZ, the unit of /proc CPU-time fields — part of
// the kernel ABI, fixed at 100 regardless of the scheduler's CONFIG_HZ.
const clockTicksPerSec = 100

// sampleProcessGroup sums cumulative CPU ticks (utime+stime) and resident
// bytes across all live processes in the group. An empty group is an error:
// the process is gone. Shared pages are counted once per process — the same
// over-count `ps` makes, close enough for a debugging view.
func sampleProcessGroup(pgid int) (ticks, rssBytes uint64, err error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, 0, err
	}
	pageSize := uint64(os.Getpagesize())
	found := false
	for _, e := range entries {
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		raw, err := os.ReadFile("/proc/" + e.Name() + "/stat")
		if err != nil {
			continue // exited mid-scan
		}
		// Fields after comm (the only field that can contain spaces):
		// 0 state, 1 ppid, 2 pgrp, …, 11 utime, 12 stime, …, 21 rss (pages).
		f := statFields(string(raw))
		if len(f) < 22 {
			continue
		}
		if g, err := strconv.Atoi(f[2]); err != nil || g != pgid {
			continue
		}
		found = true
		utime, _ := strconv.ParseUint(f[11], 10, 64)
		stime, _ := strconv.ParseUint(f[12], 10, 64)
		rss, _ := strconv.ParseUint(f[21], 10, 64)
		ticks += utime + stime
		rssBytes += rss * pageSize
	}
	if !found {
		return 0, 0, fmt.Errorf("no processes in group %d", pgid)
	}
	return ticks, rssBytes, nil
}

// statFields splits /proc/<pid>/stat after the parenthesized comm field.
func statFields(s string) []string {
	i := strings.LastIndexByte(s, ')')
	if i < 0 {
		return nil
	}
	return strings.Fields(s[i+1:])
}

// hostMemTotal is the machine's total memory — the "limit" a host process
// runs under. Read once; it doesn't change.
var hostMemTotal = sync.OnceValue(func() uint64 {
	raw, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for line := range strings.Lines(string(raw)) {
		if rest, ok := strings.CutPrefix(line, "MemTotal:"); ok {
			f := strings.Fields(rest)
			if len(f) > 0 {
				kb, _ := strconv.ParseUint(f[0], 10, 64)
				return kb * 1024
			}
		}
	}
	return 0
})
