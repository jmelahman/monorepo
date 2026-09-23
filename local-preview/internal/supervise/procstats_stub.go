//go:build !linux

package supervise

// Host-process sampling reads /proc, so other platforms report no stats for
// host processes. Container stats still work everywhere — the daemon does
// the measuring.

import "errors"

const clockTicksPerSec = 100

func sampleProcessGroup(pgid int) (uint64, uint64, error) {
	return 0, 0, errors.ErrUnsupported
}

func hostMemTotal() uint64 { return 0 }
