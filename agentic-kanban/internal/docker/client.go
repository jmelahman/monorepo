package docker

import (
	"context"
	"strings"
	"sync"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/client"
)

type Client struct {
	cli *client.Client

	rootlessOnce sync.Once
	rootless     bool
}

func NewClient() (*Client, error) {
	c, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Client{cli: c}, nil
}

func (c *Client) Close() error { return c.cli.Close() }

func (c *Client) Raw() *client.Client { return c.cli }

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.cli.Ping(ctx)
	return err
}

// ContainerRunning reports whether the container exists and is running. A
// container the daemon no longer knows about is simply not running; any
// other inspect failure (daemon unreachable, permission denied) is returned
// so callers can decide whether to trust their last known state instead.
func (c *Client) ContainerRunning(ctx context.Context, id string) (bool, error) {
	insp, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return insp.State != nil && insp.State.Running, nil
}

// ExecState reports whether a docker exec is still running and, once it
// has finished, its exit code. An exec the daemon no longer knows about
// (its container was removed or restarted), or one that was created but
// never started (no pid), is simply not running, with no exit code; any
// other inspect failure is returned, as for ContainerRunning.
func (c *Client) ExecState(ctx context.Context, execID string) (running bool, exitCode *int, err error) {
	insp, err := c.cli.ContainerExecInspect(ctx, execID)
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return false, nil, nil
		}
		return false, nil, err
	}
	if insp.Running {
		return true, nil, nil
	}
	if insp.Pid == 0 {
		return false, nil, nil // never started: its zero ExitCode is not a result
	}
	code := insp.ExitCode
	return false, &code, nil
}

// Rootless reports whether the daemon runs in rootless mode (cached after
// the first call). Under rootless docker, root inside a container maps to
// the daemon's host user while other uids map to subordinate ids — callers
// that care about bind-mount file ownership need to know.
func (c *Client) Rootless(ctx context.Context) bool {
	c.rootlessOnce.Do(func() {
		info, err := c.cli.Info(ctx)
		if err != nil {
			return
		}
		for _, opt := range info.SecurityOptions {
			if strings.Contains(opt, "rootless") {
				c.rootless = true
				return
			}
		}
	})
	return c.rootless
}
