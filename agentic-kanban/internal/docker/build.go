package docker

import (
	"context"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/pkg/stdcopy"
)

// Preview builds: one-shot containers that run a single build step against a
// bind-mounted scratch directory (an extracted commit tree). Unlike session
// containers these are short-lived, unnamed, off the kanban network, and
// always removed.

// buildMountTarget is where the scratch dir appears inside the container.
const buildMountTarget = "/preview-build"

// BuildStep is one command of a preview build. HostDir is bind-mounted
// read-write; Argv runs with the working directory Dir inside it.
type BuildStep struct {
	Image   string
	HostDir string
	Dir     string
	Argv    []string
	Env     []string
	// User is "uid:gid" so files created in the mount stay owned by the
	// kanban host user (a root-owned scratch dir breaks artifact publishing).
	User string
	// Mounts are extra volume mounts alongside HostDir — the devcontainer's
	// named cache volumes, so a repeat build finds a warm toolchain cache.
	Mounts []BuildMount
}

// BuildMount is one extra mount of a build step. Source is a named volume
// (or a host path) and Target its path inside the container.
type BuildMount struct {
	Source string
	Target string
}

// EnsureBuildImage resolves a devcontainer config (discovered at root, an
// extracted commit tree) to a runnable image. Dockerfile builds are tagged
// by content hash under tagBase, so unchanged devcontainer configs reuse the
// cached image across commits and scratch dirs.
func (c *Client) EnsureBuildImage(ctx context.Context, cfg *DevcontainerConfig, root, tagBase string) (string, error) {
	return c.ensureImage(ctx, cfg, root, tagBase, nil)
}

// RunBuildStep runs one build step to completion, streaming combined
// stdout/stderr to out. A non-zero exit is an error. The container is
// always removed.
func (c *Client) RunBuildStep(ctx context.Context, step BuildStep, out io.Writer) error {
	if len(step.Argv) == 0 {
		return fmt.Errorf("build step has empty argv")
	}
	// When kanban itself runs in a container the daemon still interprets
	// bind sources as host paths.
	hostDir := TranslateToHost(step.HostDir)
	binds := []string{hostDir + ":" + buildMountTarget}
	for _, m := range step.Mounts {
		binds = append(binds, m.Source+":"+m.Target)
	}

	created, err := c.cli.ContainerCreate(ctx, &container.Config{
		Image: step.Image,
		// Entrypoint is set explicitly so image entrypoints can't wrap the
		// step's argv.
		Entrypoint: strslice.StrSlice(step.Argv[:1]),
		Cmd:        strslice.StrSlice(step.Argv[1:]),
		WorkingDir: path.Join(buildMountTarget, filepath.ToSlash(step.Dir)),
		Env:        step.Env,
		User:       step.User,
	}, &container.HostConfig{
		Binds: binds,
	}, nil, nil, "")
	if err != nil {
		return fmt.Errorf("create build container: %w", err)
	}
	defer func() {
		rmCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		c.cli.ContainerRemove(rmCtx, created.ID, container.RemoveOptions{Force: true}) //nolint:errcheck
	}()

	attach, err := c.cli.ContainerAttach(ctx, created.ID, container.AttachOptions{
		Stream: true, Stdout: true, Stderr: true,
	})
	if err != nil {
		return fmt.Errorf("attach build container: %w", err)
	}
	defer attach.Close()

	if err := c.cli.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("start build container: %w", err)
	}

	copied := make(chan struct{})
	go func() {
		defer close(copied)
		_, _ = stdcopy.StdCopy(out, out, attach.Reader)
	}()

	waitCh, errCh := c.cli.ContainerWait(ctx, created.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		return fmt.Errorf("wait for build container: %w", err)
	case res := <-waitCh:
		<-copied
		if res.StatusCode != 0 {
			return fmt.Errorf("exited with status %d", res.StatusCode)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
