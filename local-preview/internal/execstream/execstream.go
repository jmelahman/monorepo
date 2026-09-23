// Package execstream is the wire protocol of `preview exec`: a byte stream of
// typed frames multiplexing an interactive exec session — stdin, stdout,
// stderr, terminal resizes, the final exit code, and transport-level errors —
// over a single bidirectional connection. The same frames ride every hop
// (CLI ↔ control API ↔ worker API ↔ supervisor), so a control node forwarding
// a session to a worker copies bytes blindly, without re-framing.
//
// Each frame is a 1-byte type, a 4-byte big-endian payload length, then the
// payload. The carrier is a WebSocket (see Accept/Dial) treated as a plain
// byte stream: WebSocket is the one upgrade ALBs and proxies reliably pass
// through, and the CSRF story (browser-enforced Origin on ws handshakes) comes
// for free.
package execstream

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Frame types. Stdin, StdinEOF, and Resize flow client→server; Stdout,
// Stderr, Exit, and Error flow server→client.
const (
	FrameStdin byte = iota + 1
	FrameStdout
	FrameStderr
	// FrameExit carries the command's exit code as one byte (0–255). It is
	// the last frame of a successful session.
	FrameExit
	// FrameResize carries cols then rows, each big-endian uint16.
	FrameResize
	// FrameStdinEOF signals the client closed stdin (payload empty); the
	// executed command sees EOF but the session keeps streaming output.
	FrameStdinEOF
	// FrameError carries human-readable text for a session that failed for
	// transport or orchestration reasons (as opposed to a command exiting
	// non-zero, which is FrameExit).
	FrameError
	// FramePing is a client keep-alive (payload empty) — traffic that stops a
	// load balancer's idle timeout from severing a quiet session. Receivers
	// ignore it, as they do any unknown frame type.
	FramePing
)

// MaxPayload bounds one frame's payload, and so the allocation a peer can
// force per frame. Writers chunk larger payloads across frames.
const MaxPayload = 1 << 20

// Frame is one decoded frame.
type Frame struct {
	Type    byte
	Payload []byte
}

// ReadFrame reads the next frame, rejecting payloads beyond MaxPayload.
func ReadFrame(r io.Reader) (Frame, error) {
	var hdr [5]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return Frame{}, err
	}
	n := binary.BigEndian.Uint32(hdr[1:])
	if n > MaxPayload {
		return Frame{}, fmt.Errorf("frame payload %d exceeds cap %d", n, MaxPayload)
	}
	f := Frame{Type: hdr[0]}
	if n > 0 {
		f.Payload = make([]byte, n)
		if _, err := io.ReadFull(r, f.Payload); err != nil {
			return Frame{}, err
		}
	}
	return f, nil
}

// Writer frames payloads onto an underlying stream. Safe for concurrent use —
// an exec session writes output and the exit frame from different goroutines.
type Writer struct {
	mu sync.Mutex
	w  io.Writer
}

// NewWriter wraps w.
func NewWriter(w io.Writer) *Writer { return &Writer{w: w} }

// WriteFrame writes one frame, chunking payloads larger than MaxPayload.
func (fw *Writer) WriteFrame(t byte, payload []byte) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	for {
		chunk := payload
		if len(chunk) > MaxPayload {
			chunk = chunk[:MaxPayload]
		}
		var hdr [5]byte
		hdr[0] = t
		binary.BigEndian.PutUint32(hdr[1:], uint32(len(chunk)))
		if _, err := fw.w.Write(hdr[:]); err != nil {
			return err
		}
		if len(chunk) > 0 {
			if _, err := fw.w.Write(chunk); err != nil {
				return err
			}
		}
		payload = payload[len(chunk):]
		if len(payload) == 0 {
			return nil
		}
	}
}

// StreamWriter adapts one frame type to io.Writer — the output pumps copy
// into these.
type StreamWriter struct {
	FW   *Writer
	Type byte
}

func (s StreamWriter) Write(p []byte) (int, error) {
	if err := s.FW.WriteFrame(s.Type, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// ResizePayload encodes a terminal size for FrameResize.
func ResizePayload(cols, rows uint16) []byte {
	var p [4]byte
	binary.BigEndian.PutUint16(p[:2], cols)
	binary.BigEndian.PutUint16(p[2:], rows)
	return p[:]
}

// DecodeResize decodes a FrameResize payload.
func DecodeResize(p []byte) (cols, rows uint16, err error) {
	if len(p) != 4 {
		return 0, 0, fmt.Errorf("resize payload must be 4 bytes, got %d", len(p))
	}
	return binary.BigEndian.Uint16(p[:2]), binary.BigEndian.Uint16(p[2:]), nil
}

// ExitError is a session that ended with the command exiting non-zero —
// distinct from transport failures so callers can propagate the code.
type ExitError struct{ Code int }

func (e ExitError) Error() string { return fmt.Sprintf("command exited with status %d", e.Code) }

// RemoteError is a FrameError received from the peer.
type RemoteError struct{ Msg string }

func (e RemoteError) Error() string { return e.Msg }

// Accept upgrades an HTTP request to the exec byte stream. It clears the
// server's per-request deadlines first — an interactive session outlives any
// ReadTimeout — and returns a net.Conn carrying the frame protocol. The
// WebSocket handshake enforces same-origin for browser callers; non-browser
// clients (the CLI, a control node) send no Origin and pass.
func Accept(w http.ResponseWriter, r *http.Request) (net.Conn, error) {
	rc := http.NewResponseController(w)
	if err := rc.SetReadDeadline(noDeadline); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return nil, err
	}
	if err := rc.SetWriteDeadline(noDeadline); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return nil, err
	}
	ws, err := websocket.Accept(w, r, nil)
	if err != nil {
		return nil, err
	}
	ws.SetReadLimit(MaxPayload + 16)
	return websocket.NetConn(context.Background(), ws, websocket.MessageBinary), nil
}

// Dial opens the exec byte stream against url (http/https, or ws/wss),
// sending header on the handshake. A failed handshake surfaces the HTTP
// response body when there is one — that's where the API's JSON error lands.
func Dial(ctx context.Context, url string, header http.Header) (net.Conn, error) {
	ws, resp, err := websocket.Dial(ctx, url, &websocket.DialOptions{ //nolint:bodyclose // library closes resp.Body
		HTTPHeader: header,
	})
	if err != nil {
		if resp != nil && resp.Body != nil {
			if msg, rerr := io.ReadAll(io.LimitReader(resp.Body, 4096)); rerr == nil && len(msg) > 0 {
				return nil, fmt.Errorf("exec handshake: %s: %s", resp.Status, msg)
			}
			return nil, fmt.Errorf("exec handshake: %s", resp.Status)
		}
		return nil, err
	}
	ws.SetReadLimit(MaxPayload + 16)
	return websocket.NetConn(context.Background(), ws, websocket.MessageBinary), nil
}

// noDeadline is the zero time SetReadDeadline/SetWriteDeadline take to mean
// "none".
var noDeadline = time.Time{}
