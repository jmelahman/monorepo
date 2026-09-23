//go:build !windows

package docker

import (
	"os"
	"path/filepath"
	"syscall"
)

// detectBuiltinUserFromClaude returns the built-in account whose UID
// owns the host's Claude Code config. Other UIDs (e.g., a macOS user
// with UID 501) fall through to dev — the image doesn't ship an account
// matching them, so for those hosts the user has to set
// $DEVCONTAINER_REMOTE_USER explicitly or chown the bind source.
func detectBuiltinUserFromClaude() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "dev"
	}
	for _, name := range []string{".claude", ".claude.json"} {
		info, err := os.Stat(filepath.Join(home, name))
		if err != nil {
			continue
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return "dev"
		}
		switch stat.Uid {
		case 0:
			return "root"
		case 1000:
			return "dev"
		}
	}
	return "dev"
}
