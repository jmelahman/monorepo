package docker

import (
	"os"
	"path/filepath"
	"runtime"
)

// BuiltinImage is the Docker image reference for the bundled session
// devcontainer. Release builds override this via -ldflags so each kanban
// version pins to the exact image digest published alongside the binary:
//
//	-ldflags "-X github.com/jmelahman/kanban/internal/docker.BuiltinImage=lahmanja/kanban-devcontainer@sha256:..."
//
// The default points at the floating :latest tag for unpinned development
// builds; tag releases must always set the digest form.
var BuiltinImage = "lahmanja/kanban-devcontainer:latest"

// BuiltinDevcontainer returns the bundled DevcontainerConfig used when a
// session has no repo-level or user-level devcontainer.json. It is
// deliberately permissive: SSH agent and gh credentials are forwarded
// from the host so that git, ssh, and gh "just work" on a fresh install.
// Host Claude Code config is layered on later by the session manager
// (see ClaudeConfigMounts) so it can be toggled via .kanban.toml.
//
// The bundled image ships with both a `dev` (UID 1000) and a `root`
// account. The active one is normally inferred from the owner of the
// host's ~/.claude so the bind-mounted credentials are readable and
// writable inside the session — without that, every new session gets a
// permission-denied on the credentials file and re-prompts /login.
// $DEVCONTAINER_REMOTE_USER / $DEVCONTAINER_REMOTE_HOME override the
// auto-pick when set; the two are derived together so a session can't
// end up as `root` with `/home/dev` (or vice versa).
func BuiltinDevcontainer() *DevcontainerConfig {
	user, _ := builtinIdentity()
	cfg := &DevcontainerConfig{
		Name:            "kanban-default",
		Image:           BuiltinImage,
		WorkspaceFolder: "/workspace",
		RemoteUser:      user,
		BuiltIn:         true,
		ContainerEnv:    map[string]string{},
	}
	if sock := sshAgentSocketPath(); sock != "" {
		cfg.Mounts = append(cfg.Mounts,
			"type=bind,source="+sock+",target=/ssh-agent.sock")
		cfg.ContainerEnv["SSH_AUTH_SOCK"] = "/ssh-agent.sock"
	}
	if tok := os.Getenv("GH_TOKEN"); tok != "" {
		cfg.ContainerEnv["GH_TOKEN"] = tok
	}
	return cfg
}

// ClaudeConfigMounts returns bind-mount strings that forward the host's
// Claude Code config (~/.claude and ~/.claude.json) into the built-in
// container's home directory so credentials and session history persist
// across sessions. Sources that don't exist on the host are skipped so
// users without Claude Code installed aren't broken. The session manager
// applies these to built-in configs unless the .kanban.toml
// [devcontainer].claude_config flag is set to false.
//
// The in-container target follows the home dir picked by builtinIdentity
// (auto-derived from the host file owner, overridable via
// $DEVCONTAINER_REMOTE_HOME) so the bound files land where the chosen
// remote user expects them.
func ClaudeConfigMounts() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	_, target := builtinIdentity()
	var mounts []string
	for _, name := range []string{".claude", ".claude.json"} {
		src := filepath.Join(home, name)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		mounts = append(mounts, "type=bind,source="+src+",target="+target+"/"+name)
	}
	return mounts
}

// builtinIdentity returns the (user, home) pair the built-in devcontainer
// should run as. Resolution order, per-half:
//
//  1. $DEVCONTAINER_REMOTE_USER / $DEVCONTAINER_REMOTE_HOME (escape hatch).
//  2. Otherwise, stat the host's ~/.claude (then ~/.claude.json) and pick
//     the built-in account whose UID matches: 0 → root, 1000 → dev.
//     Aligning the session user with the bind source's owner is what
//     makes the credentials readable/writable from inside the container
//     — without this, dev (1000) can't even list a root-owned ~/.claude
//     and each new session re-prompts /login.
//  3. Fall back to dev.
//
// The two halves are resolved together: an explicit user without a home
// derives the home from a known-user table, and vice versa, so callers
// can never observe a (root, /home/dev) or (dev, /root) pair.
func builtinIdentity() (user, home string) {
	user = os.Getenv("DEVCONTAINER_REMOTE_USER")
	home = os.Getenv("DEVCONTAINER_REMOTE_HOME")
	if user == "" {
		user = detectBuiltinUserFromClaude()
	}
	if home == "" {
		home = builtinHomeForUser(user)
	}
	return user, home
}

// builtinHomeForUser maps a built-in account name to its in-container
// home. Unknown users fall through to /home/dev rather than guessing
// /home/<user> — the image only provisions homes for the two shipped
// accounts and a wrong path would silently lose bind-mounted state.
func builtinHomeForUser(user string) string {
	if user == "root" {
		return "/root"
	}
	return "/home/dev"
}

// DockerSocketMount returns the host docker socket bind mount string applied
// to built-in configs when DevcontainerSection.DockerSocket is true.
// It prefers $KANBAN_HOST_DOCKER_SOCK when set (for kanban-in-a-container
// installs where the local probe would resolve a path that's invalid on the
// host), then probes /var/run/docker.sock, then $XDG_RUNTIME_DIR/docker.sock
// for rootless installs. Returns "" when nothing is set and neither candidate
// exists. Hand-written devcontainer.json files manage their own socket mount
// and are unaffected by the flag.
func DockerSocketMount() string {
	if src := os.Getenv(envHostDockerSock); src != "" {
		return "type=bind,source=" + src + ",target=/var/run/docker.sock"
	}
	for _, src := range dockerSocketCandidates() {
		if _, err := os.Stat(src); err == nil {
			return "type=bind,source=" + src + ",target=/var/run/docker.sock"
		}
	}
	return ""
}

func dockerSocketCandidates() []string {
	paths := []string{"/var/run/docker.sock"}
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		paths = append(paths, filepath.Join(xdg, "docker.sock"))
	}
	return paths
}

// sshAgentSocketPath returns a host path to bind-mount as the container's
// ssh-agent socket. Returns SSH_AUTH_SOCK when set, falling back to the
// Docker Desktop magic socket on macOS so users with launchd-managed agents
// get forwarding without manual env wiring.
func sshAgentSocketPath() string {
	if s := os.Getenv("SSH_AUTH_SOCK"); s != "" {
		return s
	}
	if runtime.GOOS == "darwin" {
		return "/run/host-services/ssh-auth.sock"
	}
	return ""
}
