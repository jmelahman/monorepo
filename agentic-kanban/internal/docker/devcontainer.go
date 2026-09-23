package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"

	"github.com/jmelahman/kanban/internal/jsonc"
)

// DevcontainerConfig is a minimal subset of the devcontainer.json spec.
type DevcontainerConfig struct {
	Name             string            `json:"name"`
	Build            BuildConfig       `json:"build"`
	Image            string            `json:"image"`
	RunArgs          []string          `json:"runArgs"`
	Mounts           MountList         `json:"mounts"`
	WorkspaceMount   string            `json:"workspaceMount"`
	WorkspaceFolder  string            `json:"workspaceFolder"`
	RemoteUser       string            `json:"remoteUser"`
	ContainerUser    string            `json:"containerUser"`
	ContainerEnv     map[string]string `json:"containerEnv"`
	PostStartCommand string            `json:"postStartCommand"`
	WaitFor          string            `json:"waitFor"`

	// ConfigDir is the directory containing the loaded devcontainer.json. Build
	// context and Dockerfile paths resolve relative to it so user-level fallback
	// configs can ship their own Dockerfile alongside the json.
	ConfigDir string `json:"-"`

	// BuiltIn is true when this config was synthesized by BuiltinDevcontainer
	// rather than parsed from disk. The session manager keys docker-socket
	// injection off this flag so hand-written devcontainer.json files retain
	// full control over their mounts.
	BuiltIn bool `json:"-"`
}

type BuildConfig struct {
	Dockerfile string            `json:"dockerfile"`
	Context    string            `json:"context"`
	Args       map[string]string `json:"args"`
}

// MountList is a list of docker --mount strings. Its unmarshaler also accepts
// the object form the devcontainer spec allows ({"source": …, "target": …,
// "type": …}), normalizing objects to string form so variable substitution
// and parseMountString handle one shape downstream.
type MountList []string

func (m *MountList) UnmarshalJSON(data []byte) error {
	var entries []json.RawMessage
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		var s string
		if err := json.Unmarshal(e, &s); err == nil {
			out = append(out, s)
			continue
		}
		var obj struct {
			Source string `json:"source"`
			Target string `json:"target"`
			Type   string `json:"type"`
		}
		if err := json.Unmarshal(e, &obj); err != nil {
			return fmt.Errorf("mount entry %s: neither string nor object form", string(e))
		}
		var parts []string
		if obj.Type != "" {
			parts = append(parts, "type="+obj.Type)
		}
		if obj.Source != "" {
			parts = append(parts, "source="+obj.Source)
		}
		if obj.Target != "" {
			parts = append(parts, "target="+obj.Target)
		}
		out = append(out, strings.Join(parts, ","))
	}
	*m = out
	return nil
}

// PortMapping is a port to publish from container to host.
type PortMapping struct {
	HostPort      int
	ContainerPort int
}

// PullProgress is an aggregate snapshot of an in-flight image pull, summed
// across all layers the daemon has reported so far. Total is 0 until the
// first byte counter arrives, and Current >= Total never holds true mid-pull
// (Total grows as new layers are discovered).
type PullProgress struct {
	Image   string `json:"image"`
	Current int64  `json:"current"`
	Total   int64  `json:"total"`
	Layers  int    `json:"layers"`
	Status  string `json:"status"`
	Done    bool   `json:"done"`
}

// PullProgressFunc receives throttled progress updates while an image pulls.
// It is called from the goroutine driving the pull; implementations must not
// block.
type PullProgressFunc func(PullProgress)

// SpawnOptions configures a devcontainer spawn.
type SpawnOptions struct {
	// WorktreePath is the host path used for variable substitution in the
	// devcontainer config (${localWorkspaceFolder}). The actual workspace
	// bind mount uses MountPath, which defaults to WorktreePath when unset.
	WorktreePath string
	// MountPath is the host directory bind-mounted into the container at the
	// devcontainer's workspaceFolder. May be a parent directory of multiple
	// repos. Defaults to WorktreePath if empty.
	MountPath string
	// RepoWorktreePath, when non-empty, is bind-mounted at /repository inside
	// the container so the session can find its branch-isolated worktree at a
	// stable path even when MountPath points at a parent directory.
	RepoWorktreePath string
	// SourceRepoPath is the host path of the parent git repo that owns the
	// worktree. When set, the parent repo's .git directory is bind-mounted
	// at the same absolute host path so the worktree's gitdir pointer
	// resolves and `git` works inside the container.
	SourceRepoPath string
	ContainerName  string
	Ports          []PortMapping
	// ExtraEnv is appended to the container's environment after devcontainer
	// containerEnv values, so callers can override.
	ExtraEnv map[string]string
	// AttachNetwork, if non-empty, names a docker network the container is
	// attached to after start so it can reach kanban (or other peers) by name.
	AttachNetwork string
	// OnPullProgress, if non-nil, receives throttled progress snapshots while
	// the image is being pulled. No-op when the image is already cached.
	OnPullProgress PullProgressFunc
}

// RepoMountTarget is where the per-session git worktree is bind-mounted inside
// the container when SpawnOptions.RepoWorktreePath is set. It is independent
// of the devcontainer workspaceFolder so multi-repo containers can mount a
// parent directory at /workspace and still expose the active branch worktree
// at a stable, predictable path.
const RepoMountTarget = "/repository"

// SpawnResult is what we return after starting a devcontainer.
type SpawnResult struct {
	ContainerID   string
	ContainerName string
}

var varRE = regexp.MustCompile(`\$\{([^}]+)\}`)

// SubstitutionContext provides the inputs that devcontainer.json variable
// references resolve against. See https://containers.dev/implementors/spec/#variable-substitution.
type SubstitutionContext struct {
	LocalWorkspaceFolder     string
	ContainerWorkspaceFolder string
	DevcontainerID           string
}

// NewSubstitutionContext builds a context for the given worktree and the
// container-side workspace folder (defaults to /workspace if empty).
func NewSubstitutionContext(worktreePath, containerWorkspaceFolder string) SubstitutionContext {
	if containerWorkspaceFolder == "" {
		containerWorkspaceFolder = "/workspace"
	}
	sum := sha256.Sum256([]byte(worktreePath))
	return SubstitutionContext{
		LocalWorkspaceFolder:     worktreePath,
		ContainerWorkspaceFolder: containerWorkspaceFolder,
		DevcontainerID:           hex.EncodeToString(sum[:]),
	}
}

// Substitute resolves devcontainer.json variable references in s per the
// containers.dev spec. Supports ${localEnv:VAR[:default]}, ${localWorkspaceFolder},
// ${localWorkspaceFolderBasename}, ${containerWorkspaceFolder},
// ${containerWorkspaceFolderBasename}, and ${devcontainerId}.
// ${containerEnv:VAR[:default]} references are left literal: they are intended
// to be resolved at runtime inside the container, after it has started.
func Substitute(s string, ctx SubstitutionContext) string {
	return varRE.ReplaceAllStringFunc(s, func(match string) string {
		inner := match[2 : len(match)-1]
		switch inner {
		case "localWorkspaceFolder":
			return ctx.LocalWorkspaceFolder
		case "localWorkspaceFolderBasename":
			return filepath.Base(ctx.LocalWorkspaceFolder)
		case "containerWorkspaceFolder":
			return ctx.ContainerWorkspaceFolder
		case "containerWorkspaceFolderBasename":
			return filepath.Base(ctx.ContainerWorkspaceFolder)
		case "devcontainerId":
			return ctx.DevcontainerID
		}
		if rest, ok := strings.CutPrefix(inner, "localEnv:"); ok {
			name, def := splitDefault(rest)
			if v, ok := os.LookupEnv(name); ok {
				return v
			}
			return def
		}
		// containerEnv:* and any unknown variable are left literal.
		return match
	})
}

// splitDefault splits "VAR:default" into ("VAR", "default"). With no colon,
// returns (s, "").
func splitDefault(s string) (string, string) {
	name, def, _ := strings.Cut(s, ":")
	return name, def
}

// Substitute resolves variable references in every string field of the config
// in place, per the containers.dev spec.
func (c *DevcontainerConfig) Substitute(ctx SubstitutionContext) {
	c.Name = Substitute(c.Name, ctx)
	c.Image = Substitute(c.Image, ctx)
	c.Build.Dockerfile = Substitute(c.Build.Dockerfile, ctx)
	c.Build.Context = Substitute(c.Build.Context, ctx)
	for k, v := range c.Build.Args {
		c.Build.Args[k] = Substitute(v, ctx)
	}
	for i, a := range c.RunArgs {
		c.RunArgs[i] = Substitute(a, ctx)
	}
	for i, m := range c.Mounts {
		c.Mounts[i] = Substitute(m, ctx)
	}
	c.WorkspaceMount = Substitute(c.WorkspaceMount, ctx)
	c.WorkspaceFolder = Substitute(c.WorkspaceFolder, ctx)
	c.RemoteUser = Substitute(c.RemoteUser, ctx)
	c.ContainerUser = Substitute(c.ContainerUser, ctx)
	for k, v := range c.ContainerEnv {
		c.ContainerEnv[k] = Substitute(v, ctx)
	}
	c.PostStartCommand = Substitute(c.PostStartCommand, ctx)
	c.WaitFor = Substitute(c.WaitFor, ctx)
}

// LoadDevcontainer parses devcontainer.json for a worktree. It probes, in
// order:
//  1. <worktree>/.devcontainer/devcontainer.json
//  2. <worktree>/.devcontainer.json
//  3. <userConfigDir>/kanban/.devcontainer/devcontainer.json
//  4. <userConfigDir>/kanban/devcontainer.json
//  5. the built-in default (BuiltinDevcontainer), pinned to the kanban
//     release's published image.
//
// The user-level fallbacks let users without a repo-defined devcontainer ship
// a sibling Dockerfile; the built-in fallback ensures every session gets a
// container even with no kanban configuration on disk.
func LoadDevcontainer(worktreePath string) (*DevcontainerConfig, error) {
	candidates := []string{
		filepath.Join(worktreePath, ".devcontainer", "devcontainer.json"),
		filepath.Join(worktreePath, ".devcontainer.json"),
	}
	if userDir, err := userKanbanConfigDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(userDir, ".devcontainer", "devcontainer.json"),
			filepath.Join(userDir, "devcontainer.json"),
		)
	}

	var data []byte
	var loaded string
	for _, path := range candidates {
		b, err := os.ReadFile(path)
		if err == nil {
			data = b
			loaded = path
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
	}
	if data == nil {
		log.Printf("devcontainer: %s has no devcontainer.json; using built-in default %s", worktreePath, BuiltinImage)
		return BuiltinDevcontainer(), nil
	}
	if !strings.HasPrefix(loaded, worktreePath) {
		log.Printf("devcontainer: %s has no devcontainer.json; using user fallback at %s", worktreePath, loaded)
	}

	stripped := jsonc.Strip(data)
	var cfg DevcontainerConfig
	if err := json.Unmarshal(stripped, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", loaded, err)
	}
	cfg.ConfigDir = filepath.Dir(loaded)
	return &cfg, nil
}

// userKanbanConfigDir returns the user-level kanban config directory,
// $XDG_CONFIG_HOME/kanban or ~/.config/kanban.
func userKanbanConfigDir() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "kanban"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "kanban"), nil
}

// Spawn builds and runs the devcontainer for a given worktree.
func (c *Client) Spawn(ctx context.Context, cfg *DevcontainerConfig, opts SpawnOptions) (*SpawnResult, error) {
	if cfg.WorkspaceFolder == "" {
		cfg.WorkspaceFolder = "/workspace"
	}
	substSource := opts.MountPath
	if substSource == "" {
		substSource = opts.WorktreePath
	}
	cfg.Substitute(NewSubstitutionContext(substSource, cfg.WorkspaceFolder))

	imageRef, err := c.ensureImage(ctx, cfg, opts.WorktreePath, filepath.Base(opts.WorktreePath), opts.OnPullProgress)
	if err != nil {
		return nil, fmt.Errorf("ensure image: %w", err)
	}

	// Resolve host.docker.internal to the gateway of the network the container
	// will actually be attached to, rather than the daemon-wide host-gateway
	// (docker0). The session is not on docker0, so that IP is unreachable
	// cross-bridge and typically rejected by devcontainer egress firewalls;
	// the attached network's gateway is on a subnet such firewalls allow.
	hostGatewayIP := ""
	if opts.AttachNetwork != "" {
		hostGatewayIP = c.NetworkGatewayIPv4(ctx, opts.AttachNetwork)
	}

	hostCfg, netCfg, containerCfg, err := buildContainerConfig(cfg, opts, imageRef, hostGatewayIP)
	if err != nil {
		return nil, err
	}

	created, err := c.cli.ContainerCreate(ctx, containerCfg, hostCfg, netCfg, nil, opts.ContainerName)
	if err != nil {
		return nil, fmt.Errorf("create container: %w", err)
	}
	if opts.AttachNetwork != "" {
		if err := c.ConnectContainer(ctx, opts.AttachNetwork, created.ID); err != nil {
			return nil, fmt.Errorf("attach network %s: %w", opts.AttachNetwork, err)
		}
	}
	if err := c.cli.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("start container: %w", err)
	}

	if cfg.PostStartCommand != "" {
		if _, err := c.ExecRun(ctx, created.ID, []string{"sh", "-lc", cfg.PostStartCommand}); err != nil {
			return nil, fmt.Errorf("postStartCommand: %w", err)
		}
	}

	return &SpawnResult{ContainerID: created.ID, ContainerName: opts.ContainerName}, nil
}

func buildContainerConfig(cfg *DevcontainerConfig, opts SpawnOptions, imageRef string, hostGatewayIP string) (*container.HostConfig, *network.NetworkingConfig, *container.Config, error) {
	wsFolder := cfg.WorkspaceFolder

	mountSource := opts.MountPath
	if mountSource == "" {
		mountSource = opts.WorktreePath
	}
	mounts := []mount.Mount{
		{Type: mount.TypeBind, Source: TranslateToHost(mountSource), Target: wsFolder, Consistency: mount.ConsistencyDelegated},
	}
	if opts.RepoWorktreePath != "" && opts.RepoWorktreePath != mountSource {
		mounts = append(mounts, mount.Mount{
			Type:        mount.TypeBind,
			Source:      TranslateToHost(opts.RepoWorktreePath),
			Target:      RepoMountTarget,
			Consistency: mount.ConsistencyDelegated,
		})
	}
	if gitDir, ok := resolveSourceGitDir(opts.SourceRepoPath); ok {
		// Target stays as the in-container path so the worktree's gitdir
		// pointer (an absolute path written by the host kanban container)
		// resolves inside the session container.
		mounts = append(mounts, mount.Mount{
			Type:        mount.TypeBind,
			Source:      TranslateToHost(gitDir),
			Target:      gitDir,
			Consistency: mount.ConsistencyDelegated,
		})
	}
	for _, raw := range cfg.Mounts {
		m, err := parseMountString(raw)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("parse mount %q: %w", raw, err)
		}
		if m.Type == mount.TypeBind {
			m.Source = TranslateToHost(m.Source)
		}
		mounts = append(mounts, m)
	}

	// Resolve host.docker.internal so status hooks can reach a host-mode kanban
	// server on bare-metal Linux (Docker Desktop adds this mapping automatically).
	// Prefer the explicit IP of the network the container is attached to, since
	// docker's "host-gateway" magic resolves to docker0 — a network the session
	// isn't on, and one whose subnet the devcontainer egress firewall doesn't
	// allow-list.
	hostAlias := "host.docker.internal:host-gateway"
	if hostGatewayIP != "" {
		hostAlias = "host.docker.internal:" + hostGatewayIP
	}
	hostCfg := &container.HostConfig{
		Mounts:        mounts,
		AutoRemove:    false,
		RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyDisabled},
		ExtraHosts:    []string{hostAlias},
	}

	// Note: we deliberately do not publish session container ports to the host.
	// The kanban server's PortProxy bridges traffic via `docker exec ... socat`,
	// so the host-side binding lives on the kanban container itself (which
	// reserves the entire configured port range).
	exposed := nat.PortSet{}
	for _, p := range opts.Ports {
		port, err := nat.NewPort("tcp", fmt.Sprintf("%d", p.ContainerPort))
		if err != nil {
			return nil, nil, nil, err
		}
		exposed[port] = struct{}{}
	}

	applyRunArgs(cfg.RunArgs, hostCfg)

	// remoteUser is what tools should run as and is what kanban execs use;
	// containerUser (the user the container itself runs as) is the closest
	// substitute when a config sets only that.
	user := cfg.RemoteUser
	if user == "" {
		user = cfg.ContainerUser
	}
	containerCfg := &container.Config{
		Image:        imageRef,
		WorkingDir:   wsFolder,
		Tty:          false,
		AttachStdout: false,
		AttachStderr: false,
		ExposedPorts: exposed,
		User:         user,
		Cmd:          []string{"sh", "-c", "tail -f /dev/null"},
	}
	if len(cfg.ContainerEnv) > 0 || len(opts.ExtraEnv) > 0 {
		env := make([]string, 0, len(cfg.ContainerEnv)+len(opts.ExtraEnv))
		for k, v := range cfg.ContainerEnv {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		for k, v := range opts.ExtraEnv {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		containerCfg.Env = env
	}

	return hostCfg, &network.NetworkingConfig{}, containerCfg, nil
}

// applyRunArgs translates a small subset of `docker run` flags from runArgs.
func applyRunArgs(args []string, hostCfg *container.HostConfig) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--cap-add" && i+1 < len(args):
			hostCfg.CapAdd = append(hostCfg.CapAdd, args[i+1])
			i++
		case strings.HasPrefix(a, "--cap-add="):
			hostCfg.CapAdd = append(hostCfg.CapAdd, strings.TrimPrefix(a, "--cap-add="))
		case a == "--cap-drop" && i+1 < len(args):
			hostCfg.CapDrop = append(hostCfg.CapDrop, args[i+1])
			i++
		case strings.HasPrefix(a, "--cap-drop="):
			hostCfg.CapDrop = append(hostCfg.CapDrop, strings.TrimPrefix(a, "--cap-drop="))
		case a == "--privileged":
			hostCfg.Privileged = true
		case a == "--network" && i+1 < len(args):
			hostCfg.NetworkMode = container.NetworkMode(args[i+1])
			i++
		case strings.HasPrefix(a, "--network="):
			hostCfg.NetworkMode = container.NetworkMode(strings.TrimPrefix(a, "--network="))
		case a == "--init":
			t := true
			hostCfg.Init = &t
		}
	}
}

// resolveSourceGitDir returns the absolute path of the parent repo's gitdir
// when it can be bind-mounted as-is. Returns ok=false when SourceRepoPath is
// empty, missing, or .git is not a plain directory (e.g., the source itself
// is a linked worktree — handle that case manually).
func resolveSourceGitDir(repoPath string) (string, bool) {
	if repoPath == "" {
		return "", false
	}
	candidate := filepath.Join(repoPath, ".git")
	info, err := os.Stat(candidate)
	if err != nil || !info.IsDir() {
		return "", false
	}
	return candidate, true
}

func parseMountString(s string) (mount.Mount, error) {
	m := mount.Mount{Type: mount.TypeBind}
	for _, part := range strings.Split(s, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		switch key {
		case "source", "src":
			m.Source = val
		case "target", "destination", "dst":
			m.Target = val
		case "type":
			m.Type = mount.Type(val)
		case "consistency":
			m.Consistency = mount.Consistency(val)
		case "readonly", "ro":
			if val == "true" || val == "1" {
				m.ReadOnly = true
			}
		}
	}
	if m.Target == "" {
		return m, fmt.Errorf("mount missing target")
	}
	if m.Type == mount.TypeBind && m.Source == "" {
		return m, fmt.Errorf("bind mount has empty source (an unset ${localEnv:VAR} likely substituted to empty)")
	}
	return m, nil
}

func (c *Client) ensureImage(ctx context.Context, cfg *DevcontainerConfig, worktreePath, tagBase string, onProgress PullProgressFunc) (string, error) {
	if cfg.Image != "" {
		if _, _, err := c.cli.ImageInspectWithRaw(ctx, cfg.Image); err == nil {
			return cfg.Image, nil
		}
		rc, err := c.cli.ImagePull(ctx, cfg.Image, image.PullOptions{})
		if err != nil {
			return "", fmt.Errorf("pull image %q: %w", cfg.Image, err)
		}
		defer rc.Close()
		if onProgress != nil {
			if err := streamPullProgress(rc, cfg.Image, onProgress); err != nil {
				return "", fmt.Errorf("pull image %q: %w", cfg.Image, err)
			}
		} else if err := jsonmessage.DisplayJSONMessagesStream(rc, io.Discard, 0, false, nil); err != nil {
			return "", fmt.Errorf("pull image %q: %w", cfg.Image, err)
		}
		return cfg.Image, nil
	}
	contextDir, dockerfilePath := resolveBuildPaths(cfg, worktreePath)

	tag, err := imageTag(tagBase, dockerfilePath, cfg.Build.Args)
	if err != nil {
		return "", err
	}
	if _, _, err := c.cli.ImageInspectWithRaw(ctx, tag); err == nil {
		return tag, nil
	}

	tarball, err := buildBuildContext(contextDir, dockerfilePath)
	if err != nil {
		return "", fmt.Errorf("build context: %w", err)
	}

	buildArgs := map[string]*string{}
	for k, v := range cfg.Build.Args {
		val := v
		buildArgs[k] = &val
	}

	resp, err := c.cli.ImageBuild(ctx, bytes.NewReader(tarball), types.ImageBuildOptions{
		Tags:       []string{tag},
		Dockerfile: filepath.Base(dockerfilePath),
		BuildArgs:  buildArgs,
		Remove:     true,
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if err := jsonmessage.DisplayJSONMessagesStream(resp.Body, io.Discard, 0, false, nil); err != nil {
		return "", fmt.Errorf("image build: %w", err)
	}
	return tag, nil
}

// resolveBuildPaths picks the build context dir and Dockerfile path for cfg.
// It prefers the directory the loaded devcontainer.json lives in (handling
// repo .devcontainer/, repo-root .devcontainer.json, and user-level fallback
// uniformly), then falls back to the worktree root so an in-.devcontainer/
// json can reference a Dockerfile at the repo root without a `..` prefix.
func resolveBuildPaths(cfg *DevcontainerConfig, worktreePath string) (contextDir, dockerfilePath string) {
	dockerfileRel := cfg.Build.Dockerfile
	if dockerfileRel == "" {
		dockerfileRel = "Dockerfile"
	}
	contextRel := cfg.Build.Context
	if contextRel == "" {
		contextRel = "."
	}
	configDir := cfg.ConfigDir
	if configDir == "" {
		configDir = filepath.Join(worktreePath, ".devcontainer")
	}
	contextDir = filepath.Join(configDir, contextRel)
	if _, err := os.Stat(contextDir); err != nil {
		contextDir = filepath.Join(worktreePath, contextRel)
	}
	dockerfilePath = filepath.Join(configDir, dockerfileRel)
	if _, err := os.Stat(dockerfilePath); err != nil {
		dockerfilePath = filepath.Join(worktreePath, dockerfileRel)
	}
	return contextDir, dockerfilePath
}

// imageTag content-addresses a Dockerfile build: same Dockerfile + args
// under the same base name reuse the cached image. Callers pass a stable
// base (worktree basename for sessions, repo name for preview builds) so
// the cache survives across checkouts and scratch dirs.
func imageTag(base, dockerfilePath string, args map[string]string) (string, error) {
	data, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write(data)
	for k, v := range args {
		h.Write([]byte(k))
		h.Write([]byte("="))
		h.Write([]byte(v))
	}
	digest := hex.EncodeToString(h.Sum(nil))[:12]
	return fmt.Sprintf("kanban-%s:%s", sanitizeTag(base), digest), nil
}

var tagSafe = regexp.MustCompile(`[^a-zA-Z0-9_.-]`)

func sanitizeTag(s string) string {
	return strings.ToLower(tagSafe.ReplaceAllString(s, "-"))
}

// buildBuildContext tars a directory plus a Dockerfile and returns the bytes.
func buildBuildContext(contextDir, dockerfilePath string) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	dockerfileName := filepath.Base(dockerfilePath)

	err := filepath.Walk(contextDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(contextDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
	if err != nil {
		return nil, err
	}

	// Ensure Dockerfile is in the tarball even if it sits outside the context dir.
	if _, statErr := os.Stat(filepath.Join(contextDir, dockerfileName)); statErr != nil {
		data, err := os.ReadFile(dockerfilePath)
		if err != nil {
			return nil, err
		}
		hdr := &tar.Header{Name: dockerfileName, Mode: 0o644, Size: int64(len(data)), ModTime: time.Now()}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write(data); err != nil {
			return nil, err
		}
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Exec creates and starts an exec; non-attached, returns exec ID.
func (c *Client) Exec(ctx context.Context, containerID string, cmd []string, workDir string, env []string) (string, error) {
	resp, err := c.cli.ContainerExecCreate(ctx, containerID, container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
		WorkingDir:   workDir,
		Env:          env,
	})
	if err != nil {
		return "", err
	}
	if err := c.cli.ContainerExecStart(ctx, resp.ID, container.ExecStartOptions{}); err != nil {
		return "", err
	}
	return resp.ID, nil
}

// ExecRun runs a command synchronously, returning the combined output.
// Returns an error if the exec exits with a non-zero status.
func (c *Client) ExecRun(ctx context.Context, containerID string, cmd []string) (string, error) {
	resp, err := c.cli.ContainerExecCreate(ctx, containerID, container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return "", err
	}
	att, err := c.cli.ContainerExecAttach(ctx, resp.ID, container.ExecStartOptions{})
	if err != nil {
		return "", err
	}
	defer att.Close()
	var stdout, stderr bytes.Buffer
	_, _ = stdcopy.StdCopy(&stdout, &stderr, att.Reader)
	out := stdout.String() + stderr.String()
	inspect, err := c.cli.ContainerExecInspect(ctx, resp.ID)
	if err != nil {
		return out, err
	}
	if inspect.ExitCode != 0 {
		return out, fmt.Errorf("exec exited %d: %s", inspect.ExitCode, strings.TrimSpace(out))
	}
	return out, nil
}

// ExecAttachTTY creates a TTY exec and attaches stdio; the caller pipes IO.
type AttachedExec struct {
	ID   string
	Conn types.HijackedResponse
}

func (c *Client) ExecAttachTTY(ctx context.Context, containerID string, cmd []string, workDir string, env []string) (*AttachedExec, error) {
	resp, err := c.cli.ContainerExecCreate(ctx, containerID, container.ExecOptions{
		Cmd:          cmd,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		WorkingDir:   workDir,
		Env:          env,
	})
	if err != nil {
		return nil, err
	}
	att, err := c.cli.ContainerExecAttach(ctx, resp.ID, container.ExecStartOptions{Tty: true})
	if err != nil {
		return nil, err
	}
	return &AttachedExec{ID: resp.ID, Conn: att}, nil
}

func (c *Client) ResizeExec(ctx context.Context, execID string, cols, rows uint) error {
	return c.cli.ContainerExecResize(ctx, execID, container.ResizeOptions{Width: cols, Height: rows})
}

func (c *Client) StopContainer(ctx context.Context, id string, timeout time.Duration) error {
	t := int(timeout.Seconds())
	return c.cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &t})
}

func (c *Client) RemoveContainer(ctx context.Context, id string) error {
	return c.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
}

// isLayerStatus reports whether a Docker pull status string identifies a
// per-layer lifecycle event. Filters out manifest-level lines like
// "Pulling from library/alpine" whose ID is the tag, not a layer.
func isLayerStatus(s string) bool {
	switch s {
	case "Pulling fs layer", "Waiting", "Downloading",
		"Verifying Checksum", "Download complete",
		"Extracting", "Pull complete", "Already exists":
		return true
	}
	return false
}

// streamPullProgress decodes a Docker pull JSON stream and emits aggregated
// snapshots to onProgress at a fixed cadence. Each layer's last-known current
// and total bytes are summed; once the daemon reports a layer as complete
// ("Pull complete", "Already exists", "Download complete", "Verifying
// Checksum") we snap its current to its total so the bar fills smoothly. The
// final emission has Done=true so consumers can clear transient UI state.
func streamPullProgress(rc io.Reader, imageRef string, onProgress PullProgressFunc) error {
	type layer struct{ current, total int64 }
	layers := map[string]*layer{}
	dec := json.NewDecoder(rc)
	var lastEmit time.Time
	var lastStatus string
	emit := func(done bool) {
		var cur, tot int64
		for _, l := range layers {
			cur += l.current
			tot += l.total
		}
		onProgress(PullProgress{
			Image:   imageRef,
			Current: cur,
			Total:   tot,
			Layers:  len(layers),
			Status:  lastStatus,
			Done:    done,
		})
	}
	for {
		var msg jsonmessage.JSONMessage
		if err := dec.Decode(&msg); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if msg.Error != nil {
			return msg.Error
		}
		if msg.Status != "" {
			lastStatus = msg.Status
		}
		if msg.ID != "" && isLayerStatus(msg.Status) {
			l, ok := layers[msg.ID]
			if !ok {
				l = &layer{}
				layers[msg.ID] = l
			}
			if msg.Progress != nil {
				if msg.Progress.Total > l.total {
					l.total = msg.Progress.Total
				}
				if msg.Progress.Current > l.current {
					l.current = msg.Progress.Current
				}
			}
			switch msg.Status {
			case "Pull complete", "Already exists", "Download complete", "Verifying Checksum":
				if l.total > 0 {
					l.current = l.total
				}
			}
		}
		if time.Since(lastEmit) > 200*time.Millisecond {
			lastEmit = time.Now()
			emit(false)
		}
	}
	emit(true)
	return nil
}
