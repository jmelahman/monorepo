// Package dockerapi is a deliberately tiny Docker Engine API client — plain
// HTTP over the unix socket, no SDK dependency — sufficient to run one-shot
// build-step containers: ensure an image, create/start a container, stream
// its output, wait, remove. Used for manifest-declared build images so
// standalone local-preview gets reproducible builds even when its own host
// (possibly a container itself) has no toolchains.
package dockerapi

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Client talks to one Docker daemon.
type Client struct {
	http     *http.Client
	sock     string // daemon socket path, for hijacked (exec-attach) connections
	rootless bool
}

// SocketPath resolves the daemon socket: $DOCKER_HOST (unix:// only), then
// /var/run/docker.sock, then the rootless per-user socket.
func SocketPath() (string, error) {
	if h := os.Getenv("DOCKER_HOST"); h != "" {
		if p, ok := strings.CutPrefix(h, "unix://"); ok {
			return p, nil
		}
		return "", fmt.Errorf("unsupported DOCKER_HOST %q (only unix:// sockets)", h)
	}
	candidates := []string{"/var/run/docker.sock"}
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		candidates = append(candidates, dir+"/docker.sock")
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("no docker socket found")
}

// Connect dials the daemon, verifies it responds, and probes rootless mode.
func Connect(ctx context.Context) (*Client, error) {
	sock, err := SocketPath()
	if err != nil {
		return nil, err
	}
	c := &Client{sock: sock, http: &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", sock)
			},
		},
	}}
	if _, err := c.raw(ctx, "GET", "/_ping", nil); err != nil {
		return nil, fmt.Errorf("docker daemon at %s: %w", sock, err)
	}
	var info struct {
		SecurityOptions []string
	}
	if body, err := c.raw(ctx, "GET", "/info", nil); err == nil {
		if json.Unmarshal(body, &info) == nil {
			for _, opt := range info.SecurityOptions {
				if strings.Contains(opt, "rootless") {
					c.rootless = true
				}
			}
		}
	}
	return c, nil
}

// Rootless reports whether the daemon runs rootless. Under rootless docker,
// container root maps to the daemon's host user — the right identity for
// writing bind-mounted build dirs.
func (c *Client) Rootless() bool { return c.rootless }

// EnsureImage makes the image locally available, pulling if needed.
func (c *Client) EnsureImage(ctx context.Context, image string, out io.Writer) error {
	if _, err := c.raw(ctx, "GET", "/images/"+image+"/json", nil); err == nil {
		return nil
	}
	fmt.Fprintf(out, "pulling image %s…\n", image)
	resp, err := c.do(ctx, "POST", "/images/create?fromImage="+url.QueryEscape(image), nil)
	if err != nil {
		return fmt.Errorf("pull %s: %w", image, err)
	}
	defer resp.Body.Close()
	if err := streamPull(resp.Body, out); err != nil {
		return fmt.Errorf("pull %s: %w", image, err)
	}
	return nil
}

// pullProgressInterval is how often streamPull writes an aggregate progress
// line. A var so tests can shrink it.
var pullProgressInterval = 5 * time.Second

// streamPull consumes a docker pull JSON stream, surfacing in-stream failures
// and logging aggregate progress at a fixed cadence — first pulls of large
// images take minutes, and a log that goes silent after "pulling image" reads
// as a hang. Per-layer byte counters keep their maxima (layer totals arrive
// out of order and grow as layers are discovered), and layers the daemon
// reports finished snap to their total, so the printed numbers only climb.
func streamPull(body io.Reader, out io.Writer) error {
	type layer struct{ current, total int64 }
	layers := map[string]*layer{}
	dec := json.NewDecoder(body)
	last := time.Now()
	for {
		var msg struct {
			Status string `json:"status"`
			ID     string `json:"id"`
			Error  string `json:"error"`
			Detail struct {
				Current int64 `json:"current"`
				Total   int64 `json:"total"`
			} `json:"progressDetail"`
		}
		if err := dec.Decode(&msg); err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}
		if msg.Error != "" {
			return errors.New(msg.Error)
		}
		if msg.ID != "" {
			l := layers[msg.ID]
			if l == nil {
				l = &layer{}
				layers[msg.ID] = l
			}
			if msg.Detail.Total > l.total {
				l.total = msg.Detail.Total
			}
			if msg.Detail.Current > l.current {
				l.current = msg.Detail.Current
			}
			switch msg.Status {
			case "Pull complete", "Already exists", "Download complete":
				if l.total > 0 {
					l.current = l.total
				}
			}
		}
		if time.Since(last) >= pullProgressInterval {
			last = time.Now()
			var cur, tot int64
			for _, l := range layers {
				cur += l.current
				tot += l.total
			}
			if tot > 0 {
				fmt.Fprintf(out, "pulled %d/%d MB\n", cur>>20, tot>>20)
			}
		}
	}
}

// Step is one command run in a fresh container to completion. Networks
// names existing external networks the container joins before starting.
type Step struct {
	Image    string
	Cmd      []string
	WorkDir  string
	Env      []string
	User     string
	Binds    []string
	Networks []string
}

// RunStep creates, runs, and removes a container for the step, streaming
// combined output to out. Non-zero exits are errors.
func (c *Client) RunStep(ctx context.Context, step Step, out io.Writer) error {
	if len(step.Cmd) == 0 {
		return fmt.Errorf("empty command")
	}
	if err := c.EnsureImage(ctx, step.Image, out); err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{
		"Image":      step.Image,
		"Entrypoint": step.Cmd[:1],
		"Cmd":        step.Cmd[1:],
		"WorkingDir": step.WorkDir,
		"Env":        step.Env,
		"User":       step.User,
		"HostConfig": map[string]any{"Binds": step.Binds},
	})
	if err != nil {
		return err
	}
	created, err := c.raw(ctx, "POST", "/containers/create", body)
	if err != nil {
		return fmt.Errorf("create build container: %w", err)
	}
	var container struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(created, &container); err != nil {
		return err
	}
	defer func() {
		rmCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		c.raw(rmCtx, "DELETE", "/containers/"+container.ID+"?force=1", nil) //nolint:errcheck
	}()

	for _, name := range step.Networks {
		netID, ok, err := c.FindNetworkByName(ctx, name)
		if err == nil && !ok {
			err = fmt.Errorf("network %q not found — is the dependency stack running?", name)
		}
		if err != nil {
			return fmt.Errorf("join network %q: %w", name, err)
		}
		if err := c.ConnectNetwork(ctx, netID, container.ID, nil); err != nil {
			return fmt.Errorf("join network %q: %w", name, err)
		}
	}

	if _, err := c.raw(ctx, "POST", "/containers/"+container.ID+"/start", nil); err != nil {
		return fmt.Errorf("start build container: %w", err)
	}

	logsDone := make(chan error, 1)
	go func() {
		resp, err := c.do(ctx, "GET",
			"/containers/"+container.ID+"/logs?follow=1&stdout=1&stderr=1", nil)
		if err != nil {
			logsDone <- err
			return
		}
		defer resp.Body.Close()
		logsDone <- demux(resp.Body, out)
	}()

	waited, err := c.raw(ctx, "POST", "/containers/"+container.ID+"/wait", nil)
	if err != nil {
		return fmt.Errorf("wait for build container: %w", err)
	}
	<-logsDone
	var res struct {
		StatusCode int `json:"StatusCode"`
	}
	if err := json.Unmarshal(waited, &res); err != nil {
		return err
	}
	if res.StatusCode != 0 {
		return fmt.Errorf("exited with status %d", res.StatusCode)
	}
	return nil
}

// demux unpacks the engine's multiplexed log stream (8-byte frame headers:
// stream type, three zero bytes, big-endian payload length).
func demux(r io.Reader, out io.Writer) error {
	header := make([]byte, 8)
	for {
		if _, err := io.ReadFull(r, header); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil
			}
			return err
		}
		n := binary.BigEndian.Uint32(header[4:])
		if _, err := io.CopyN(out, r, int64(n)); err != nil {
			return err
		}
	}
}

// raw performs a request and returns the full response body.
func (c *Client) raw(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	resp, err := c.do(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// do performs a request, converting non-2xx responses into errors using the
// engine's {"message": ...} body.
func (c *Client) do(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://docker"+path, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		var e struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(raw, &e) == nil && e.Message != "" {
			return nil, fmt.Errorf("%s %s: %s", method, path, e.Message)
		}
		return nil, fmt.Errorf("%s %s: status %d", method, path, resp.StatusCode)
	}
	return resp, nil
}
