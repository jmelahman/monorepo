package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/gorilla/websocket"
	"golang.org/x/term"

	"github.com/jmelahman/kanban/internal/client"
)

// defaultDetachKeys mirrors `docker attach`: the sequence is swallowed
// rather than forwarded, so it must be one the agent doesn't need.
const defaultDetachKeys = "ctrl-p,ctrl-q"

// stdinIsTerminal reports whether the process has an interactive terminal
// on both stdin and stdout. A package variable so tests can force either
// branch without a pty.
var stdinIsTerminal = func() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

// sessionInfo is the subset of the session JSON the attach flow needs.
type sessionInfo struct {
	ID          int64  `json:"id"`
	TicketID    int64  `json:"ticket_id"`
	Status      string `json:"status"`
	ContainerID string `json:"container_id"`
	Harness     string `json:"harness"`
}

// runTicketAttach ensures the ticket has a running session, then attaches
// the calling terminal to its agent PTY (kind "agent") or to an interactive
// shell in the container (kind "shell"). A non-empty harnessID switches the
// session to that harness first. It returns after the user detaches or the
// remote process exits; the session keeps running either way.
func runTicketAttach(ctx context.Context, url string, out io.Writer, ticketID int64, kind, detachKeys, harnessID string) error {
	seq, err := parseDetachKeys(detachKeys)
	if err != nil {
		return err
	}
	if !stdinIsTerminal() {
		return errors.New("attach needs an interactive terminal on stdin and stdout")
	}
	sess, err := ensureRunningSession(ctx, client.New(url, nil), out, ticketID, harnessID)
	if err != nil {
		return err
	}

	what := "the agent"
	if kind == "shell" {
		what = "a shell"
	}
	fmt.Fprintf(out, "attaching to %s for ticket #%d (session #%d); press %s to detach\n", what, ticketID, sess.ID, detachKeys)

	resize, stopResize := notifyResize()
	defer stopResize()
	fd := int(os.Stdin.Fd())
	size := func() (int, int, bool) {
		cols, rows, err := term.GetSize(fd)
		return cols, rows, err == nil
	}

	var detached bool
	err = withRawTerminal(fd, func() error {
		var aerr error
		detached, aerr = attachSession(ctx, url, sess.ID, kind, seq, os.Stdin, os.Stdout, size, resize)
		return aerr
	})
	// The PTY stream leaves the cursor wherever the remote left it; start
	// the wrap-up on a fresh line.
	fmt.Fprintln(out)
	if err != nil {
		return err
	}
	if detached {
		fmt.Fprintf(out, "detached from session #%d; it keeps running. Reattach with: kanban ticket attach %d\n", sess.ID, ticketID)
	} else {
		fmt.Fprintf(out, "session #%d %s ended\n", sess.ID, what)
	}
	return nil
}

// ensureRunningSession creates the ticket's session if missing, switches it
// to harnessID when that's set and differs from its current choice, and
// starts it when stopped. The switch happens before the start so a fresh
// session never launches the wrong agent; on a running session the server
// restarts the agent. Starting blocks while the devcontainer image is
// pulled or built, which on a first run can take minutes, so it says so up
// front.
func ensureRunningSession(ctx context.Context, c *client.Client, out io.Writer, ticketID int64, harnessID string) (sessionInfo, error) {
	var sess sessionInfo
	raw, err := c.EnsureSession(ctx, ticketID)
	if err != nil {
		return sess, err
	}
	if err := json.Unmarshal(raw, &sess); err != nil {
		return sess, fmt.Errorf("decode session: %w", err)
	}
	if harnessID != "" && harnessID != sess.Harness {
		fmt.Fprintf(out, "switching session #%d to the %s harness\n", sess.ID, harnessID)
		if raw, err = c.SetSessionHarness(ctx, sess.ID, harnessID); err != nil {
			return sess, err
		}
		if err := json.Unmarshal(raw, &sess); err != nil {
			return sess, fmt.Errorf("decode session: %w", err)
		}
	}
	if sess.ContainerID != "" && sess.Status != "stopped" && sess.Status != "error" {
		return sess, nil
	}
	fmt.Fprintf(out, "starting session #%d (a first start pulls or builds the devcontainer image, which can take a few minutes)...\n", sess.ID)
	raw, err = c.StartSession(ctx, sess.ID)
	if err != nil {
		return sess, err
	}
	if err := json.Unmarshal(raw, &sess); err != nil {
		return sess, fmt.Errorf("decode session: %w", err)
	}
	if sess.ContainerID == "" {
		return sess, fmt.Errorf("session #%d is not running (status %q)", sess.ID, sess.Status)
	}
	return sess, nil
}

// withRawTerminal runs fn with the terminal on fd in raw mode, restoring
// the previous mode afterwards (including on error).
func withRawTerminal(fd int, fn func() error) error {
	if !term.IsTerminal(fd) {
		return fn()
	}
	old, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("raw terminal: %w", err)
	}
	defer func() { _ = term.Restore(fd, old) }()
	return fn()
}

// sessionWSURL turns the server base URL into the WebSocket URL for a
// session's PTY (kind "agent") or shell (kind "shell") stream, plus the
// Origin the server's same-origin check expects.
func sessionWSURL(serverURL string, sessionID int64, kind string) (wsURL, origin string, err error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return "", "", fmt.Errorf("server url: %w", err)
	}
	httpScheme := u.Scheme
	switch u.Scheme {
	case "http", "ws":
		u.Scheme, httpScheme = "ws", "http"
	case "https", "wss":
		u.Scheme, httpScheme = "wss", "https"
	default:
		return "", "", fmt.Errorf("server url %q: unsupported scheme %q", serverURL, u.Scheme)
	}
	if u.Host == "" {
		return "", "", fmt.Errorf("server url %q: missing host", serverURL)
	}
	path := "pty"
	if kind == "shell" {
		path = "shell"
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/ws/sessions/" + strconv.FormatInt(sessionID, 10) + "/" + path
	u.RawQuery, u.Fragment = "", ""
	return u.String(), httpScheme + "://" + u.Host, nil
}

// attachSession runs the attach loop: stdin bytes go to the session as
// binary frames (minus the detach sequence), frames from the session are
// written to out verbatim, and a JSON resize control frame is sent on
// connect and whenever resize fires. Returns detached=true when the detach
// sequence ended it, false when the remote side closed the stream.
func attachSession(
	ctx context.Context, serverURL string, sessionID int64, kind string, detach []byte,
	in io.Reader, out io.Writer, size func() (cols, rows int, ok bool), resize <-chan struct{},
) (detached bool, err error) {
	wsURL, origin, err := sessionWSURL(serverURL, sessionID, kind)
	if err != nil {
		return false, err
	}
	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, wsURL, http.Header{"Origin": {origin}})
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			return false, fmt.Errorf("attach: %s: %s", resp.Status, strings.TrimSpace(string(body)))
		}
		return false, fmt.Errorf("attach: %w", err)
	}
	defer conn.Close()

	// gorilla allows one concurrent writer; stdin and resize share it.
	var wmu sync.Mutex
	send := func(msgType int, p []byte) error {
		wmu.Lock()
		defer wmu.Unlock()
		return conn.WriteMessage(msgType, p)
	}
	sendResize := func() {
		if cols, rows, ok := size(); ok && cols > 0 && rows > 0 {
			p, _ := json.Marshal(map[string]any{"type": "resize", "cols": cols, "rows": rows})
			_ = send(websocket.TextMessage, p)
		}
	}
	sendResize()

	remoteDone := make(chan error, 1)
	go func() {
		for {
			msgType, data, err := conn.ReadMessage()
			if err != nil {
				remoteDone <- err
				return
			}
			// The broker reports attach failures as a text frame before
			// closing; surface those as errors instead of terminal output.
			if msgType == websocket.TextMessage && bytes.HasPrefix(data, []byte("error: ")) {
				remoteDone <- errors.New(string(bytes.TrimPrefix(data, []byte("error: "))))
				return
			}
			if _, err := out.Write(data); err != nil {
				remoteDone <- err
				return
			}
		}
	}()

	detachCh := make(chan struct{})
	stdinDone := make(chan error, 1)
	go func() {
		det := detachDetector{seq: detach}
		buf := make([]byte, 4096)
		for {
			n, err := in.Read(buf)
			if n > 0 {
				fwd, hit := det.feed(buf[:n])
				if len(fwd) > 0 {
					if err := send(websocket.BinaryMessage, fwd); err != nil {
						stdinDone <- err
						return
					}
				}
				if hit {
					close(detachCh)
					return
				}
			}
			if err != nil {
				stdinDone <- err
				return
			}
		}
	}()

	closeQuietly := func() {
		_ = send(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	}
	for {
		select {
		case <-ctx.Done():
			closeQuietly()
			return false, ctx.Err()
		case <-resize:
			sendResize()
		case <-detachCh:
			closeQuietly()
			return true, nil
		case err := <-stdinDone:
			closeQuietly()
			if errors.Is(err, io.EOF) {
				return true, nil
			}
			return false, fmt.Errorf("read stdin: %w", err)
		case err := <-remoteDone:
			var ce *websocket.CloseError
			if errors.As(err, &ce) {
				// Any close — clean or the abrupt 1006 the broker produces
				// after "[session ended]" — means the remote process is gone.
				return false, nil
			}
			return false, fmt.Errorf("attach: %w", err)
		}
	}
}

// detachDetector scans the stdin stream for the detach sequence the way
// docker does: bytes matching a prefix of the sequence are held back, and
// released if the next byte breaks the match.
type detachDetector struct {
	seq     []byte
	matched int
}

// feed consumes p and returns the bytes to forward to the session. hit is
// true once the full sequence has been seen; anything after it is dropped.
func (d *detachDetector) feed(p []byte) (forward []byte, hit bool) {
	if len(d.seq) == 0 {
		return p, false
	}
	var out []byte
	for _, b := range p {
		if b == d.seq[d.matched] {
			d.matched++
			if d.matched == len(d.seq) {
				d.matched = 0
				return out, true
			}
			continue
		}
		// Mismatch: release what we held, then retry this byte from the
		// start of the sequence (it may itself begin a new match).
		out = append(out, d.seq[:d.matched]...)
		d.matched = 0
		if b == d.seq[0] {
			d.matched = 1
			continue
		}
		out = append(out, b)
	}
	return out, false
}

// parseDetachKeys parses docker's --detach-keys syntax: a comma-separated
// list where each item is either a single printable character or
// ctrl-<key> with key in a-z, @, [, \, ], ^, _.
func parseDetachKeys(spec string) ([]byte, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, errors.New("detach keys: empty sequence")
	}
	var seq []byte
	for _, item := range strings.Split(spec, ",") {
		item = strings.TrimSpace(item)
		switch {
		case strings.HasPrefix(strings.ToLower(item), "ctrl-"):
			key := []rune(item[len("ctrl-"):])
			if len(key) != 1 {
				return nil, fmt.Errorf("detach keys: bad item %q (want ctrl-<key>)", item)
			}
			k := unicode.ToUpper(key[0])
			if (k < 'A' || k > 'Z') && !strings.ContainsRune("@[\\]^_", k) {
				return nil, fmt.Errorf("detach keys: ctrl-%c is not a control character", key[0])
			}
			seq = append(seq, byte(k)&0x1f)
		case len([]rune(item)) == 1 && item[0] >= 0x20 && item[0] < 0x7f:
			seq = append(seq, item[0])
		default:
			return nil, fmt.Errorf("detach keys: bad item %q (want a single character or ctrl-<key>)", item)
		}
	}
	return seq, nil
}
