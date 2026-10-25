package docker

import (
	"context"
	"os"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/client"
)

// KanbanNetworkName is the docker network shared by the kanban server and all
// session containers, so sessions can resolve the kanban server by container
// name and call back to its API.
const KanbanNetworkName = "kanban"

// EnsureNetwork creates the named bridge network if it does not already exist.
func (c *Client) EnsureNetwork(ctx context.Context, name string) error {
	if _, err := c.cli.NetworkInspect(ctx, name, client.NetworkInspectOptions{}); err == nil {
		return nil
	} else if !cerrdefs.IsNotFound(err) {
		return err
	}
	if _, err := c.cli.NetworkCreate(ctx, name, client.NetworkCreateOptions{Driver: "bridge"}); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return nil
		}
		return err
	}
	return nil
}

// SelfContainerName returns the name of the container kanban itself is running
// in (without the leading slash), or "" if kanban is not running in a
// container or the container cannot be identified via the hostname.
func (c *Client) SelfContainerName(ctx context.Context) string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return ""
	}
	res, err := c.cli.ContainerInspect(ctx, hostname, client.ContainerInspectOptions{})
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(res.Container.Name, "/")
}

// NetworkGatewayIPv4 returns the IPv4 gateway of the named network, or "" if
// the network does not exist or has no IPv4 gateway configured. Used to point
// host.docker.internal at the host as seen from a specific bridge, instead of
// the daemon-wide host-gateway (docker0). Session containers are not attached
// to docker0, so its IP is both routed cross-bridge and typically blocked by
// devcontainer egress firewalls; the attached network's own gateway is on a
// subnet the firewall already allow-lists.
func (c *Client) NetworkGatewayIPv4(ctx context.Context, name string) string {
	res, err := c.cli.NetworkInspect(ctx, name, client.NetworkInspectOptions{})
	if err != nil {
		return ""
	}
	for _, cfg := range res.Network.IPAM.Config {
		if ip := cfg.Gateway.Unmap(); ip.Is4() {
			return ip.String()
		}
	}
	return ""
}

// ConnectContainer attaches a container to the given network. A no-op if the
// container is already attached.
func (c *Client) ConnectContainer(ctx context.Context, networkName, containerID string) error {
	_, err := c.cli.NetworkConnect(ctx, networkName, client.NetworkConnectOptions{Container: containerID})
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "already exists") {
		return nil
	}
	return err
}
