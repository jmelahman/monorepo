package server

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// warmFromMemory derives a per-worker warm-process cap from the machine's total
// memory: (totalGiB - reserveGiB) * perGB, floored at 1. It exists so a single
// launch template can drive a mixed-instances fleet of differently sized nodes
// — each worker sizes its own cap from the RAM it actually landed on, instead
// of every node inheriting one static --max-warm that only fits one size.
//
// The result is clamped to at least 1: a worker that advertised a 0 cap would
// read as "unlimited" to the fleet (see fleet.Capacity), the opposite of a
// small node's intent, so calibration never yields 0.
func warmFromMemory(memTotalBytes int64, perGB, reserveGB float64) int {
	usableGiB := float64(memTotalBytes)/(1<<30) - reserveGB
	if usableGiB <= 0 {
		return 1
	}
	n := int(usableGiB * perGB)
	if n < 1 {
		return 1
	}
	return n
}

// readMemTotalBytes returns the machine's MemTotal from /proc/meminfo. The
// worker runs --network host with no container memory limit, so /proc/meminfo
// reflects the instance's full RAM — the figure we want to calibrate against.
func readMemTotalBytes() (int64, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "MemTotal:") {
			continue
		}
		// "MemTotal:       16069616 kB"
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0, fmt.Errorf("meminfo: malformed MemTotal line %q", line)
		}
		kb, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("meminfo: parse MemTotal %q: %w", fields[1], err)
		}
		return kb * 1024, nil
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	return 0, fmt.Errorf("meminfo: no MemTotal line")
}
