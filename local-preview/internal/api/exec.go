package api

// GET /api/deploys/{id}/exec — the apex end of `preview exec`. The request is
// a WebSocket handshake (the one upgrade load balancers reliably pass
// through); after the upgrade the connection carries execstream frames, which
// the runtime view bridges into the preview's container — local on a single
// node, forwarded to the owning worker on a control node. Session options
// travel as query parameters because an upgrade request has no usable body.

import (
	"context"
	"log"
	"net/http"

	"github.com/jmelahman/local-preview/internal/execstream"
	"github.com/jmelahman/local-preview/internal/supervise"
)

// handleDeployExec upgrades the request and runs one exec session against the
// deploy's supervised process. Query params: side (be/fe, default be), cmd
// (repeated, the argv), tty/stdin (1 to enable), term (client's TERM).
// Everything checkable is rejected as a plain HTTP error before the upgrade;
// failures after it arrive as FrameError on the stream.
func (d Deps) handleDeployExec(w http.ResponseWriter, r *http.Request) {
	row, ok := d.deployFromPath(w, r)
	if !ok {
		return
	}
	_, hash, key, ok := d.sideKey(w, r, row)
	if !ok {
		return
	}
	if hash == "" {
		httpError(w, http.StatusNotFound, "deploy has no such process (not built yet)")
		return
	}
	q := r.URL.Query()
	opts := supervise.ExecOptions{
		Cmd:   q["cmd"],
		TTY:   q.Get("tty") == "1",
		Stdin: q.Get("stdin") == "1",
		Term:  q.Get("term"),
	}
	if len(opts.Cmd) == 0 {
		httpError(w, http.StatusBadRequest, "missing cmd parameter (repeat it per argv element)")
		return
	}
	if st := d.runtime().Status(key); st != supervise.StatusRunning {
		httpError(w, http.StatusConflict,
			"preview process is not running (status \""+st+"\") — open the preview to start it, then retry")
		return
	}

	conn, err := execstream.Accept(w, r)
	if err != nil {
		// Accept already wrote the handshake failure response.
		log.Printf("exec upgrade (deploy %d): %v", row.ID, err)
		return
	}
	defer conn.Close()
	// The request context dies with the hijacked HTTP request; the session's
	// lifetime is the connection itself.
	if err := d.runtime().Exec(context.Background(), key, opts, conn); err != nil {
		execstream.NewWriter(conn).WriteFrame(execstream.FrameError, []byte(err.Error())) //nolint:errcheck // conn may already be gone
	}
}
