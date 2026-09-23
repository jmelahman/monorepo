//go:build windows

package docker

// detectBuiltinUserFromClaude on Windows always falls back to the dev
// account. The bundled image's UID-based ownership probe relies on
// syscall.Stat_t which doesn't exist on Windows, and Windows hosts
// don't have a meaningful POSIX UID to compare against the image's
// dev (1000) / root (0) accounts in the first place. Users who need
// the root account must set $DEVCONTAINER_REMOTE_USER explicitly.
func detectBuiltinUserFromClaude() string {
	return "dev"
}
