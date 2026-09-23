package docker

import (
	"os"
	"path/filepath"
	"strings"
)

// Env vars that translate container-internal paths to their host equivalents
// when kanban itself runs inside a container (e.g. a devcontainer). Each
// rewrites a single path prefix:
//
//   - KANBAN_HOST_WORKSPACE: host path of /workspace inside the kanban
//     container. Lets boards stored with paths under /workspace bind-mount
//     correctly when dockerd is running on the host.
//   - KANBAN_HOST_HOME: host path of $HOME inside the kanban container. Lets
//     ~/.claude (and similar) bind-mount correctly.
//   - KANBAN_HOST_DOCKER_SOCK: host path of the docker socket that the
//     built-in devcontainer auto-mount should use as the bind source.
//     Needed when kanban runs inside a container whose own
//     /var/run/docker.sock is a bind of the host's rootless socket (e.g.
//     /var/run/user/1000/docker.sock) — the in-container path can't be
//     handed to the host's dockerd as a mount source.
//
// All three are unset in the default (host) install — paths pass through
// unchanged.
const (
	envHostWorkspace  = "KANBAN_HOST_WORKSPACE"
	envHostHome       = "KANBAN_HOST_HOME"
	envHostDockerSock = "KANBAN_HOST_DOCKER_SOCK"

	// containerWorkspacePrefix is the conventional in-container workspace
	// folder for kanban itself. Boards created from inside the kanban
	// devcontainer typically have repo_path/worktree_root under this prefix.
	containerWorkspacePrefix = "/workspace"
)

// TranslateToHost rewrites a path that's valid inside the kanban container
// into the equivalent host path that dockerd can stat. When neither env var
// is set, or when path doesn't start with a known container prefix, the
// path is returned unchanged.
//
// Used to translate the Source side of bind mounts before handing them to
// dockerd. Targets are NOT translated — those are paths inside the new
// session container and should remain as-is.
func TranslateToHost(path string) string {
	if path == "" {
		return path
	}
	if mapped, ok := translatePrefix(path, containerWorkspacePrefix, os.Getenv(envHostWorkspace)); ok {
		return mapped
	}
	if home, err := os.UserHomeDir(); err == nil {
		if mapped, ok := translatePrefix(path, home, os.Getenv(envHostHome)); ok {
			return mapped
		}
	}
	return path
}

// translatePrefix swaps containerPrefix for hostPrefix at the start of path.
// Both prefixes must be non-empty for the swap to apply, and the match must
// be on a path-segment boundary (so /workspace doesn't match /workspaceful).
func translatePrefix(path, containerPrefix, hostPrefix string) (string, bool) {
	if containerPrefix == "" || hostPrefix == "" {
		return "", false
	}
	if path == containerPrefix {
		return hostPrefix, true
	}
	if strings.HasPrefix(path, containerPrefix+string(filepath.Separator)) {
		return hostPrefix + path[len(containerPrefix):], true
	}
	return "", false
}
