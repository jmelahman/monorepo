package docker

import (
	"context"
	"net"
	"os"
	"strings"

	"github.com/docker/docker/api/types/network"
	derrdefs "github.com/docker/docker/errdefs"
)

// KanbanNetworkName is the docker network shared by the kanban server and all
// session containers, so sessions can resolve the kanban server by container
// name and call back to its API.
const KanbanNetworkName = "kanban"

// EnsureNetwork creates the named bridge network if it does not already exist.
func (c *Client) EnsureNetwork(ctx context.Context, name string) error {
	if _, err := c.cli.NetworkInspect(ctx, name, network.InspectOptions{}); err == nil {
		return nil
	} else if !derrdefs.IsNotFound(err) {
		return err
	}
	if _, err := c.cli.NetworkCreate(ctx, name, network.CreateOptions{Driver: "bridge"}); err != nil {
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
	insp, err := c.cli.ContainerInspect(ctx, hostname)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(insp.Name, "/")
}

// NetworkGatewayIPv4 returns the IPv4 gateway of the named network, or "" if
// the network does not exist or has no IPv4 gateway configured. Used to point
// host.docker.internal at the host as seen from a specific bridge, instead of
// the daemon-wide host-gateway (docker0). Session containers are not attached
// to docker0, so its IP is both routed cross-bridge and typically blocked by
// devcontainer egress firewalls; the attached network's own gateway is on a
// subnet the firewall already allow-lists.
func (c *Client) NetworkGatewayIPv4(ctx context.Context, name string) string {
	insp, err := c.cli.NetworkInspect(ctx, name, network.InspectOptions{})
	if err != nil {
		return ""
	}
	for _, cfg := range insp.IPAM.Config {
		if cfg.Gateway == "" {
			continue
		}
		if ip := net.ParseIP(cfg.Gateway); ip != nil && ip.To4() != nil {
			return ip.To4().String()
		}
	}
	return ""
}

// ConnectContainer attaches a container to the given network. A no-op if the
// container is already attached.
func (c *Client) ConnectContainer(ctx context.Context, networkName, containerID string) error {
	err := c.cli.NetworkConnect(ctx, networkName, containerID, nil)
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "already exists") {
		return nil
	}
	return err
}
