//go:build !windows

package server

import (
	"fmt"
	"syscall"
)

// filesystemBytes returns the total size of the filesystem containing path.
// Inside the orchestrator container the data dir is bind-mounted from the
// host at the same path, so this measures the host volume it lives on.
func filesystemBytes(path string) (int64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, fmt.Errorf("statfs %s: %w", path, err)
	}
	return int64(st.Blocks) * int64(st.Bsize), nil //nolint:unconvert // Bsize is int64 on linux, uint32 on darwin
}
