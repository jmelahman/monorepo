package supervise

// Container-mode process start: a manifest run_image (or the repo's
// devcontainer, the default runtime when no run_image is pinned) runs the
// side's run command inside a container image with the artifact (and state
// dir) bind-mounted at their host paths — the same-path discipline the
// composed server already relies on for build containers. The published
// port equals the probed host port, so templating, health polling, and
// proxying are identical to host processes; the process must bind
// 0.0.0.0:{port} inside the container.

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jmelahman/local-preview/internal/dockerapi"
)

// imagePullTimeout bounds the runtime-image pull on first use.
const imagePullTimeout = 10 * time.Minute

func (m *Manager) startContainer(k Key, p *process, spec runSpec, rt runtimeEnv, argv, env []string, port int, logFile *os.File) error {
	ctx, cancel := context.WithTimeout(context.Background(), imagePullTimeout)
	defer cancel()
	cli, err := m.docker(ctx)
	if err != nil {
		return fmt.Errorf("run_image %q requires a docker daemon: %w", rt.image, err)
	}
	if rt.devc {
		fmt.Fprintf(logFile, "[devcontainer runtime: %s]\n", rt.image)
	}
	if err := cli.EnsureImage(ctx, rt.image, logFile); err != nil {
		return err
	}

	// The per-backend deploy network pairs a process-mode frontend with its
	// backend by alias. Get-or-create on both sides keeps start order
	// irrelevant; the last side out removes it (see the reaper below).
	networkID := ""
	alias := ""
	labels := map[string]string{
		dockerapi.ManagedLabel: "1",
		"local-preview.repo":   p.repoName,
		"local-preview.side":   string(k.Side),
		"local-preview.hash":   k.Hash,
	}
	deployNet := ""
	switch {
	case k.Side == SideBackend:
		deployNet = networkName(p.repoName, k.Hash)
		alias = "backend"
	case k.Peer != "":
		deployNet = networkName(p.repoName, k.Peer)
		alias = "frontend"
	}
	// Membership is settled under netMu, held from the lookup through
	// StartContainer (when the endpoint actually attaches), so a peer's
	// reaper can't release a just-resolved network before this container is
	// on it. The image pull above stays outside the lock.
	m.netMu.Lock()
	defer m.netMu.Unlock()
	if deployNet != "" {
		networkID, err = cli.EnsureNetwork(ctx, deployNet, map[string]string{
			dockerapi.ManagedLabel: "1",
			"local-preview.repo":   p.repoName,
		})
		if err != nil {
			return err
		}
	}
	// abort tears down a container that never started and gives the deploy
	// network back (refused, harmlessly, while the peer still holds it).
	abort := func(id string, err error) error {
		cli.RemoveContainer(ctx, id, true) //nolint:errcheck
		if networkID != "" {
			cli.RemoveNetwork(ctx, networkID) //nolint:errcheck
		}
		return err
	}

	user := fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
	if cli.Rootless() {
		// Container root maps to the daemon's host user — the identity that
		// owns the artifact and state dirs.
		user = "0:0"
	}
	binds := runtimeBinds(spec, rt)

	name := fmt.Sprintf("local-preview-%s-%s-%s", p.repoName, k.Side, k.Hash[:12])
	if k.Side == SideFrontend && k.Peer != "" {
		name += "-" + k.Peer[:6]
	}
	// A stale namesake from an unclean exit blocks creation; clear it. A
	// live tracked container can't collide — Key uniqueness guarantees one
	// start per identity.
	cli.RemoveContainerByName(ctx, name)

	id, err := cli.CreateContainer(ctx, dockerapi.ContainerSpec{
		Image:        rt.image,
		Cmd:          argv,
		Env:          append(env, "HOME="+rt.homeOrTmp()),
		User:         user,
		WorkDir:      spec.dir,
		Binds:        binds,
		Labels:       labels,
		Name:         name,
		Port:         port,
		PublishIP:    m.publishIP,
		NetworkID:    networkID,
		NetworkAlias: alias,
	})
	if err != nil {
		return err
	}
	// External dependency networks (manifest `networks`) must already
	// exist — they belong to the user-run deps stack.
	for _, netName := range spec.networks {
		netID, ok, err := cli.FindNetworkByName(ctx, netName)
		if err == nil && !ok {
			err = fmt.Errorf("network %q not found — is the dependency stack running?", netName)
		}
		if err != nil {
			return abort(id, fmt.Errorf("join network %q: %w", netName, err))
		}
		if err := cli.ConnectNetwork(ctx, netID, id, nil); err != nil {
			return abort(id, fmt.Errorf("join network %q: %w", netName, err))
		}
	}
	if err := cli.StartContainer(ctx, id); err != nil {
		return abort(id, fmt.Errorf("start container: %w", err))
	}
	p.containerID = id
	p.startedAt = time.Now()
	m.db.AddProcessEvent(k.RepoID, k.Hash, "start_attempt",
		fmt.Sprintf("container %.12s port %d", id, port))

	logsDone := make(chan struct{})
	go func() {
		defer close(logsDone)
		cli.StreamLogs(context.Background(), id, logFile) //nolint:errcheck
	}()

	// Reaper: the container analogue of cmd.Wait.
	go func() {
		code, waitErr := cli.WaitContainer(context.Background(), id)
		select {
		case <-logsDone:
		case <-time.After(5 * time.Second):
		}
		logFile.Close()
		rmCtx, rmCancel := context.WithTimeout(context.Background(), 30*time.Second)
		cli.RemoveContainer(rmCtx, id, true) //nolint:errcheck
		// Last one out: the peer side's endpoint, if still up, makes the
		// daemon refuse and the network outlives this container instead.
		m.releaseNetwork(rmCtx, cli, networkID)
		rmCancel()
		p.exit, p.exitOK = fmt.Sprintf("exit %d", code), waitErr == nil && code == 0
		if waitErr != nil {
			p.exit = waitErr.Error()
		}
		close(p.done)
		m.forget(k, p)
		if !p.intentional {
			m.db.AddProcessEvent(k.RepoID, k.Hash, "exited", p.exit)
			m.noteExit(k, p)
		}
	}()
	return nil
}

// runInitContainer runs one init step to completion inside the backend's
// container runtime — the same mounts, user, and external dependency
// networks as the server container, so migrations reach whatever the server
// will. The image pull gets its own timeout; ctx carries the init budget.
func (m *Manager) runInitContainer(ctx context.Context, spec runSpec, rt runtimeEnv, argv, env []string, logFile *os.File) error {
	pullCtx, cancel := context.WithTimeout(context.Background(), imagePullTimeout)
	defer cancel()
	cli, err := m.docker(pullCtx)
	if err != nil {
		return fmt.Errorf("run_image %q requires a docker daemon: %w", rt.image, err)
	}
	if err := cli.EnsureImage(pullCtx, rt.image, logFile); err != nil {
		return err
	}
	user := fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
	if cli.Rootless() {
		user = "0:0"
	}
	return cli.RunStep(ctx, dockerapi.Step{
		Image:    rt.image,
		Cmd:      argv,
		WorkDir:  spec.dir,
		Env:      append(env, "HOME="+rt.homeOrTmp()),
		User:     user,
		Binds:    runtimeBinds(spec, rt),
		Networks: spec.networks,
	}, logFile)
}

// runtimeBinds are a process container's mounts: the artifact (and state
// dir) at their host paths, plus the devcontainer's cache volumes when that
// runtime applies.
func runtimeBinds(spec runSpec, rt runtimeEnv) []string {
	binds := []string{spec.dir + ":" + spec.dir}
	if spec.stateDir != "" {
		binds = append(binds, spec.stateDir+":"+spec.stateDir)
	}
	for _, mt := range rt.mounts {
		binds = append(binds, mt.Source+":"+mt.Target)
	}
	return binds
}

// homeOrTmp is the HOME inside a process container: the devcontainer's
// remote-user home when known (so toolchain caches resolve onto its
// volumes), /tmp otherwise — a guaranteed-writable default.
func (rt runtimeEnv) homeOrTmp() string {
	if rt.home != "" {
		return rt.home
	}
	return "/tmp"
}

// networkName is the per-backend deploy network shared by a backend and
// its process-mode frontend.
func networkName(repo, beHash string) string {
	return "local-preview-" + repo + "-" + beHash[:12]
}
