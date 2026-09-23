package dockerapi

// Exec support: run a command inside an already-running container with the
// caller attached — the engine-API half of `preview exec`. Create/resize/
// inspect ride the normal HTTP client; the attach itself needs a hijacked
// connection (the engine upgrades the exec-start request to a raw
// bidirectional byte stream), which the buffered http.Client cannot do, so
// ExecStart dials the daemon socket directly and speaks HTTP/1.1 by hand.

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
)

// ExecSpec configures one exec in a running container.
type ExecSpec struct {
	Cmd []string
	Env []string
	// TTY allocates a pseudo-terminal: output arrives as one raw stream and
	// stderr is folded into it. Without it, output is multiplexed and
	// DemuxStream splits the two.
	TTY bool
	// Stdin attaches the caller's input stream.
	Stdin bool
	// WorkDir and User default to the container's own when empty.
	WorkDir string
	User    string
}

// ExecCreate registers an exec instance in the container and returns its id.
func (c *Client) ExecCreate(ctx context.Context, containerID string, spec ExecSpec) (string, error) {
	if len(spec.Cmd) == 0 {
		return "", fmt.Errorf("empty command")
	}
	body, err := json.Marshal(map[string]any{
		"AttachStdin":  spec.Stdin,
		"AttachStdout": true,
		"AttachStderr": true,
		"Tty":          spec.TTY,
		"Cmd":          spec.Cmd,
		"Env":          spec.Env,
		"WorkingDir":   spec.WorkDir,
		"User":         spec.User,
	})
	if err != nil {
		return "", err
	}
	raw, err := c.raw(ctx, "POST", "/containers/"+containerID+"/exec", body)
	if err != nil {
		return "", err
	}
	var res struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", err
	}
	return res.ID, nil
}

// ExecStart starts the exec instance attached, returning the hijacked
// bidirectional stream: write to conn for stdin, read output from br (which
// may already hold buffered stream bytes — always read through it, never
// conn directly). tty must match the ExecCreate spec. The caller owns conn.
func (c *Client) ExecStart(ctx context.Context, execID string, tty bool) (net.Conn, *bufio.Reader, error) {
	body, err := json.Marshal(map[string]any{"Detach": false, "Tty": tty})
	if err != nil {
		return nil, nil, err
	}
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", c.sock)
	if err != nil {
		return nil, nil, err
	}
	req := fmt.Sprintf("POST /exec/%s/start HTTP/1.1\r\nHost: docker\r\nContent-Type: application/json\r\nConnection: Upgrade\r\nUpgrade: tcp\r\nContent-Length: %d\r\n\r\n%s",
		execID, len(body), body)
	if _, err := conn.Write([]byte(req)); err != nil {
		conn.Close()
		return nil, nil, err
	}
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("exec start: %w", err)
	}
	// 101 is the upgrade; older engines answer a plain 200 and stream anyway.
	if resp.StatusCode != http.StatusSwitchingProtocols && resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		conn.Close()
		var e struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(msg, &e) == nil && e.Message != "" {
			return nil, nil, fmt.Errorf("exec start: %s", e.Message)
		}
		return nil, nil, fmt.Errorf("exec start: status %d", resp.StatusCode)
	}
	return conn, br, nil
}

// ExecResize resizes the exec's TTY.
func (c *Client) ExecResize(ctx context.Context, execID string, cols, rows uint16) error {
	q := url.Values{"w": {strconv.Itoa(int(cols))}, "h": {strconv.Itoa(int(rows))}}
	_, err := c.raw(ctx, "POST", "/exec/"+execID+"/resize?"+q.Encode(), nil)
	return err
}

// ExecInspect reports whether the exec is still running and, once it isn't,
// its exit code.
func (c *Client) ExecInspect(ctx context.Context, execID string) (exitCode int, running bool, err error) {
	raw, err := c.raw(ctx, "GET", "/exec/"+execID+"/json", nil)
	if err != nil {
		return 0, false, err
	}
	var res struct {
		Running  bool `json:"Running"`
		ExitCode int  `json:"ExitCode"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return 0, false, err
	}
	return res.ExitCode, res.Running, nil
}

// DemuxStream unpacks the engine's multiplexed attach stream (the non-TTY
// exec output format; same framing as container logs) into separate stdout
// and stderr writers, until EOF.
func DemuxStream(r io.Reader, stdout, stderr io.Writer) error {
	var header [8]byte
	buf := make([]byte, 32*1024)
	for {
		if _, err := io.ReadFull(r, header[:]); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil
			}
			return err
		}
		out := stdout
		if header[0] == 2 {
			out = stderr
		}
		n := int64(binary.BigEndian.Uint32(header[4:]))
		if _, err := io.CopyBuffer(out, io.LimitReader(r, n), buf); err != nil {
			return err
		}
	}
}
