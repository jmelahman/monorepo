package dockerapi

// Long-running container support for supervised preview processes — the
// runtime counterpart to RunStep's one-shot build containers. Containers
// are labeled so crash recovery and repo deletion can find them by label
// instead of DB bookkeeping.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"time"
)

// ManagedLabel marks every container and network this server creates.
const ManagedLabel = "local-preview.managed"

// ContainerSpec describes one supervised preview process container. Port is
// published to 127.0.0.1:<Port> on the daemon host — the process must bind
// 0.0.0.0:<Port> inside the container.
type ContainerSpec struct {
	Image   string
	Cmd     []string // full argv; overrides the image entrypoint
	Env     []string
	User    string
	WorkDir string
	Binds   []string
	Labels  map[string]string
	Name    string
	Port    int
	// PublishIP, when set, additionally publishes Port on that host address —
	// how a worker exposes its processes to the control node's proxy. The
	// loopback binding always exists (local health checks poll it); firewalls
	// own who can reach the extra one.
	PublishIP string
	// NetworkID, when set, attaches the container at create time with
	// NetworkAlias as its DNS name on that network.
	NetworkID    string
	NetworkAlias string
}

// CreateContainer creates (but does not start) a container.
func (c *Client) CreateContainer(ctx context.Context, spec ContainerSpec) (string, error) {
	portKey := fmt.Sprintf("%d/tcp", spec.Port)
	bindings := []map[string]string{{"HostIp": "127.0.0.1", "HostPort": strconv.Itoa(spec.Port)}}
	switch spec.PublishIP {
	case "", "127.0.0.1":
		// Loopback only — the single-node default.
	case "0.0.0.0":
		// All interfaces subsumes loopback; listing both would collide.
		bindings = []map[string]string{{"HostIp": "0.0.0.0", "HostPort": strconv.Itoa(spec.Port)}}
	default:
		bindings = append(bindings, map[string]string{"HostIp": spec.PublishIP, "HostPort": strconv.Itoa(spec.Port)})
	}
	body := map[string]any{
		"Image":        spec.Image,
		"Env":          spec.Env,
		"User":         spec.User,
		"WorkingDir":   spec.WorkDir,
		"Labels":       spec.Labels,
		"ExposedPorts": map[string]any{portKey: struct{}{}},
		"HostConfig": map[string]any{
			"Binds": spec.Binds,
			"PortBindings": map[string]any{
				portKey: bindings,
			},
		},
	}
	if len(spec.Cmd) > 0 {
		body["Entrypoint"] = spec.Cmd[:1]
		body["Cmd"] = spec.Cmd[1:]
	}
	if spec.NetworkID != "" {
		body["NetworkingConfig"] = map[string]any{
			"EndpointsConfig": map[string]any{
				spec.NetworkID: map[string]any{"Aliases": []string{spec.NetworkAlias}},
			},
		}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	path := "/containers/create"
	if spec.Name != "" {
		path += "?name=" + url.QueryEscape(spec.Name)
	}
	created, err := c.raw(ctx, "POST", path, raw)
	if err != nil {
		return "", fmt.Errorf("create container: %w", err)
	}
	var container struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(created, &container); err != nil {
		return "", err
	}
	return container.ID, nil
}

// StartContainer starts a created container.
func (c *Client) StartContainer(ctx context.Context, id string) error {
	_, err := c.raw(ctx, "POST", "/containers/"+id+"/start", nil)
	return err
}

// StopContainer gracefully stops a container: the daemon sends SIGTERM,
// waits timeoutSeconds, then SIGKILLs.
func (c *Client) StopContainer(ctx context.Context, id string, timeoutSeconds int) error {
	_, err := c.raw(ctx, "POST", fmt.Sprintf("/containers/%s/stop?t=%d", id, timeoutSeconds), nil)
	return err
}

// RemoveContainer deletes a container.
func (c *Client) RemoveContainer(ctx context.Context, id string, force bool) error {
	path := "/containers/" + id
	if force {
		path += "?force=1"
	}
	_, err := c.raw(ctx, "DELETE", path, nil)
	return err
}

// RemoveContainerByName force-removes a container by name, ignoring
// not-found. Used to clear a stale namesake before re-creating.
func (c *Client) RemoveContainerByName(ctx context.Context, name string) {
	c.raw(ctx, "DELETE", "/containers/"+url.PathEscape(name)+"?force=1", nil) //nolint:errcheck
}

// WaitContainer blocks until the container exits and returns its status
// code — the container analogue of exec.Cmd.Wait.
func (c *Client) WaitContainer(ctx context.Context, id string) (int, error) {
	waited, err := c.raw(ctx, "POST", "/containers/"+id+"/wait", nil)
	if err != nil {
		return 0, err
	}
	var res struct {
		StatusCode int `json:"StatusCode"`
	}
	if err := json.Unmarshal(waited, &res); err != nil {
		return 0, err
	}
	return res.StatusCode, nil
}

// StreamLogs follows the container's combined output into out until the
// container exits or ctx is cancelled.
func (c *Client) StreamLogs(ctx context.Context, id string, out io.Writer) error {
	resp, err := c.do(ctx, "GET", "/containers/"+id+"/logs?follow=1&stdout=1&stderr=1", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return demux(resp.Body, out)
}

// ContainerSummary is one row of a container listing.
type ContainerSummary struct {
	ID     string            `json:"Id"`
	Labels map[string]string `json:"Labels"`
}

// ListContainersByLabel returns containers (including stopped ones)
// carrying the label, optionally filtered to an exact value.
func (c *Client) ListContainersByLabel(ctx context.Context, label, value string) ([]ContainerSummary, error) {
	f := label
	if value != "" {
		f = label + "=" + value
	}
	filters, _ := json.Marshal(map[string][]string{"label": {f}})
	raw, err := c.raw(ctx, "GET", "/containers/json?all=1&filters="+url.QueryEscape(string(filters)), nil)
	if err != nil {
		return nil, err
	}
	var out []ContainerSummary
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// EnsureNetwork returns the ID of the named bridge network, creating it
// (labeled) if absent.
func (c *Client) EnsureNetwork(ctx context.Context, name string, labels map[string]string) (string, error) {
	if id, ok, err := c.FindNetworkByName(ctx, name); err != nil {
		return "", err
	} else if ok {
		return id, nil
	}
	body, _ := json.Marshal(map[string]any{"Name": name, "Driver": "bridge", "Labels": labels})
	raw, err := c.raw(ctx, "POST", "/networks/create", body)
	if err != nil {
		return "", fmt.Errorf("create network %s: %w", name, err)
	}
	var res struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", err
	}
	return res.ID, nil
}

// FindNetworkByName resolves a network name to its ID.
func (c *Client) FindNetworkByName(ctx context.Context, name string) (string, bool, error) {
	filters, _ := json.Marshal(map[string][]string{"name": {name}})
	raw, err := c.raw(ctx, "GET", "/networks?filters="+url.QueryEscape(string(filters)), nil)
	if err != nil {
		return "", false, err
	}
	var nets []struct {
		ID   string `json:"Id"`
		Name string `json:"Name"`
	}
	if err := json.Unmarshal(raw, &nets); err != nil {
		return "", false, err
	}
	// The name filter matches substrings; require an exact hit.
	for _, n := range nets {
		if n.Name == name {
			return n.ID, true, nil
		}
	}
	return "", false, nil
}

// ConnectNetwork attaches a created container to an additional network.
func (c *Client) ConnectNetwork(ctx context.Context, networkID, containerID string, aliases []string) error {
	body, _ := json.Marshal(map[string]any{
		"Container":      containerID,
		"EndpointConfig": map[string]any{"Aliases": aliases},
	})
	_, err := c.raw(ctx, "POST", "/networks/"+networkID+"/connect", body)
	return err
}

// NetworkSummary is one row of a network listing.
type NetworkSummary struct {
	ID   string `json:"Id"`
	Name string `json:"Name"`
}

// ListNetworksByLabel returns networks carrying the label, optionally
// filtered to an exact value.
func (c *Client) ListNetworksByLabel(ctx context.Context, label, value string) ([]NetworkSummary, error) {
	f := label
	if value != "" {
		f = label + "=" + value
	}
	filters, _ := json.Marshal(map[string][]string{"label": {f}})
	raw, err := c.raw(ctx, "GET", "/networks?filters="+url.QueryEscape(string(filters)), nil)
	if err != nil {
		return nil, err
	}
	var out []NetworkSummary
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RemoveNetwork deletes a network (its containers must be gone first).
func (c *Client) RemoveNetwork(ctx context.Context, id string) error {
	_, err := c.raw(ctx, "DELETE", "/networks/"+id, nil)
	return err
}

// StopTimeout is the graceful-stop grace period handed to the daemon.
const StopTimeout = 5 * time.Second
