package client

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"github.com/jmelahman/local-preview/internal/execstream"
)

// ExecOptions shapes one `preview exec` session.
type ExecOptions struct {
	// Side is "be" or "fe" (which of the deploy's processes to exec into).
	Side string
	// Cmd is the argv to run inside the container.
	Cmd []string
	// TTY allocates a pseudo-terminal; Stdin attaches this process's input.
	TTY   bool
	Stdin bool
	// Term is the local TERM, exported into the session when TTY is set.
	Term string
}

// Exec opens an exec session against the deploy's supervised process: a
// WebSocket upgrade whose connection carries execstream frames. The caller
// runs the frame loop and owns the returned conn. ctx bounds only the
// handshake.
func (c *Client) Exec(ctx context.Context, id int64, opts ExecOptions) (net.Conn, error) {
	q := url.Values{"side": {opts.Side}, "cmd": opts.Cmd}
	if opts.TTY {
		q.Set("tty", "1")
	}
	if opts.Stdin {
		q.Set("stdin", "1")
	}
	if opts.Term != "" {
		q.Set("term", opts.Term)
	}
	hdr := http.Header{}
	if c.token != "" {
		hdr.Set("Authorization", "Bearer "+c.token)
	}
	return execstream.Dial(ctx, fmt.Sprintf("%s/api/deploys/%d/exec?%s", c.base, id, q.Encode()), hdr)
}
