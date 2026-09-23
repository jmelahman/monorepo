package session

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/docker"
)

// replayBufferSize bounds the recent-output buffer that gets replayed to
// reconnecting clients. 64 KiB comfortably covers a screenful or two of
// output for a typical terminal.
const replayBufferSize = 64 * 1024

// clientWriteTimeout caps how long a single broadcast write to one client
// may block. Keeps a stalled device (e.g. a mobile in a tunnel) from
// backing up the read loop and starving the other attached clients.
const clientWriteTimeout = 5 * time.Second

// ringBuffer retains the most-recent bytes written to it, up to a fixed
// capacity. Not safe for concurrent use; the caller serializes access.
type ringBuffer struct {
	buf  []byte
	n    int // number of valid bytes (clamped at len(buf))
	head int // next write index
}

func newRingBuffer(size int) *ringBuffer {
	return &ringBuffer{buf: make([]byte, size)}
}

func (r *ringBuffer) Write(p []byte) {
	if len(r.buf) == 0 {
		return
	}
	if len(p) >= len(r.buf) {
		copy(r.buf, p[len(p)-len(r.buf):])
		r.head = 0
		r.n = len(r.buf)
		return
	}
	for len(p) > 0 {
		space := len(r.buf) - r.head
		n := len(p)
		if n > space {
			n = space
		}
		copy(r.buf[r.head:], p[:n])
		r.head = (r.head + n) % len(r.buf)
		if r.n < len(r.buf) {
			r.n += n
			if r.n > len(r.buf) {
				r.n = len(r.buf)
			}
		}
		p = p[n:]
	}
}

// Snapshot returns the buffered bytes in oldest-first order.
func (r *ringBuffer) Snapshot() []byte {
	if r.n < len(r.buf) {
		out := make([]byte, r.n)
		copy(out, r.buf[:r.n])
		return out
	}
	out := make([]byte, len(r.buf))
	n := copy(out, r.buf[r.head:])
	copy(out[n:], r.buf[:r.head])
	return out
}

// clientView is the per-attached-client state held by a sessionPTY.
// cols/rows are zero until the client sends its first resize control frame.
type clientView struct {
	cols uint
	rows uint
}

// sessionPTY brokers a single docker exec PTY for a session. It owns the
// hijacked exec connection and survives WebSocket attach/detach so that
// clients can reconnect (e.g. after a page refresh) without killing the
// underlying agent process.
//
// Multiple clients may attach concurrently; output is broadcast to every
// attached client and stdin is accepted from any of them — shared-session
// semantics, similar to `tmux attach`. The single reader goroutine is the
// only writer of binary output frames; WS handlers call register / unregister
// / write / resize.
type sessionPTY struct {
	key      brokerKey
	attached *docker.AttachedExec
	docker   *docker.Client
	set      *brokerSet

	// containerID is the container the exec runs in; harness is the harness
	// id the agent was launched with ("" for other kinds). marker is this
	// exec's unique $KANBAN_PTY_ID, which is how its process is found again
	// inside the container: closing the hijacked connection doesn't end a
	// TTY exec, so tearing one down on purpose means signalling it.
	containerID string
	harness     string
	marker      string

	mu      sync.Mutex
	buf     *ringBuffer
	cols    uint // currently-applied PTY size (aggregated across clients)
	rows    uint
	clients map[*websocket.Conn]*clientView
	closed  bool
}

// brokerKey identifies a broker by session id and kind ("agent", "shell"),
// so a single session can host multiple independent PTYs.
type brokerKey struct {
	sessionID int64
	kind      string
}

// brokerSet manages the set of active brokers, one per (session id, kind).
type brokerSet struct {
	docker *docker.Client

	mu      sync.Mutex
	perSess map[brokerKey]*sessionPTY
}

func newBrokerSet(dc *docker.Client) *brokerSet {
	return &brokerSet{
		docker:  dc,
		perSess: map[brokerKey]*sessionPTY{},
	}
}

// ptyMarkerEnv names the env var that tags each brokered exec with its
// broker's marker.
const ptyMarkerEnv = "KANBAN_PTY_ID"

// attach returns the broker for this session and kind, creating it (and
// starting the underlying docker exec) on first use. Subsequent calls with
// the same key return the existing broker — the harnessID, cmd and workDir
// arguments are only honored at creation time.
func (s *brokerSet) attach(ctx context.Context, sess *db.Session, kind, harnessID string, cmd []string, workDir string) (*sessionPTY, error) {
	key := brokerKey{sessionID: sess.ID, kind: kind}
	s.mu.Lock()
	if b, ok := s.perSess[key]; ok {
		s.mu.Unlock()
		return b, nil
	}
	if sess.ContainerID == nil || *sess.ContainerID == "" {
		s.mu.Unlock()
		return nil, errors.New("session not running")
	}
	marker := kind + "-" + rand.Text()
	att, err := s.docker.ExecAttachTTY(ctx, *sess.ContainerID, cmd, workDir, []string{ptyMarkerEnv + "=" + marker})
	if err != nil {
		s.mu.Unlock()
		return nil, err
	}
	b := &sessionPTY{
		key:         key,
		attached:    att,
		docker:      s.docker,
		set:         s,
		containerID: *sess.ContainerID,
		harness:     harnessID,
		marker:      marker,
		buf:         newRingBuffer(replayBufferSize),
		clients:     map[*websocket.Conn]*clientView{},
	}
	s.perSess[key] = b
	s.mu.Unlock()
	go b.readLoop()
	return b, nil
}

// closeAgentUnless tears down the session's agent broker unless it was
// launched with harnessID, leaving the other kinds (the shell) running, and
// returns the broker it closed (nil if none). The broker leaves the set
// before its lock is released, so a concurrent attach starts a fresh agent
// rather than rejoining the one being closed.
func (s *brokerSet) closeAgentUnless(sessionID int64, harnessID string) *sessionPTY {
	key := brokerKey{sessionID: sessionID, kind: "agent"}
	s.mu.Lock()
	b, ok := s.perSess[key]
	if !ok || b.harness == harnessID {
		s.mu.Unlock()
		return nil
	}
	delete(s.perSess, key)
	s.mu.Unlock()
	b.shutdown()
	return b
}

// closeFor tears down all brokers for a session. Idempotent and safe when no
// broker exists.
func (s *brokerSet) closeFor(sessionID int64) {
	s.mu.Lock()
	var victims []*sessionPTY
	for k, b := range s.perSess {
		if k.sessionID == sessionID {
			victims = append(victims, b)
		}
	}
	s.mu.Unlock()
	for _, b := range victims {
		b.shutdown()
	}
}

// readLoop pumps bytes from the docker exec into the ring buffer and fans
// them out to every attached client. On read error / EOF it triggers a full
// shutdown.
func (b *sessionPTY) readLoop() {
	defer b.shutdown()
	buf := make([]byte, 4096)
	for {
		n, err := b.attached.Conn.Reader.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			b.mu.Lock()
			if b.closed {
				b.mu.Unlock()
				return
			}
			b.buf.Write(chunk)
			b.broadcastBinaryLocked(chunk)
			b.mu.Unlock()
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				log.Printf("session %d %s pty read: %v", b.key.sessionID, b.key.kind, err)
			}
			return
		}
	}
}

// broadcastBinaryLocked writes a chunk to every attached client, dropping
// any client whose write fails or times out. Caller must hold b.mu.
func (b *sessionPTY) broadcastBinaryLocked(p []byte) {
	var dead []*websocket.Conn
	for ws := range b.clients {
		_ = ws.SetWriteDeadline(time.Now().Add(clientWriteTimeout))
		if err := ws.WriteMessage(websocket.BinaryMessage, p); err != nil {
			dead = append(dead, ws)
		}
	}
	for _, ws := range dead {
		delete(b.clients, ws)
		_ = ws.Close()
	}
}

// shutdown notifies every attached client, closes the hijacked exec
// connection, and removes the broker from the set. Idempotent.
func (b *sessionPTY) shutdown() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	clients := b.clients
	b.clients = map[*websocket.Conn]*clientView{}
	b.mu.Unlock()

	for ws := range clients {
		_ = ws.SetWriteDeadline(time.Now().Add(clientWriteTimeout))
		_ = ws.WriteMessage(websocket.TextMessage, []byte("\r\n[session ended]\r\n"))
		_ = ws.Close()
	}
	_ = b.attached.Conn.Conn.Close()

	b.set.mu.Lock()
	if cur, ok := b.set.perSess[b.key]; ok && cur == b {
		delete(b.set.perSess, b.key)
	}
	b.set.mu.Unlock()
}

// register adds ws to the set of attached clients and replays the recent
// output buffer to it. Other already-attached clients are left alone — the
// session is shared across all attached devices.
func (b *sessionPTY) register(ws *websocket.Conn) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return errors.New("session ended")
	}
	_ = ws.SetWriteDeadline(time.Now().Add(clientWriteTimeout))
	if err := ws.WriteMessage(websocket.BinaryMessage, replayPayload(b.buf.Snapshot())); err != nil {
		return err
	}
	b.clients[ws] = &clientView{}
	return nil
}

// replayPayload is the first frame sent to a newly-attached client: a
// terminal Reset to Initial State (RIS, ESC c) followed by the raw PTY
// snapshot. The reset gives the receiving terminal a known parser/mode/cursor
// state before replay; without it, a snapshot that begins mid-escape-sequence
// or carries over alt-screen state can leave visible artifacts on screen
// (the kind users would otherwise clear with Ctrl+L).
func replayPayload(snap []byte) []byte {
	out := make([]byte, 0, len(snap)+2)
	out = append(out, 0x1b, 'c')
	return append(out, snap...)
}

// unregister removes ws from the attached set and reapplies the aggregated
// PTY size now that this client's reported dimensions no longer constrain
// the others.
func (b *sessionPTY) unregister(ctx context.Context, ws *websocket.Conn) {
	b.mu.Lock()
	if _, ok := b.clients[ws]; !ok {
		b.mu.Unlock()
		return
	}
	delete(b.clients, ws)
	cols, rows := b.aggregateSizeLocked()
	apply := cols != 0 && rows != 0 && (cols != b.cols || rows != b.rows)
	if apply {
		b.cols = cols
		b.rows = rows
	}
	execID := b.attached.ID
	b.mu.Unlock()
	if apply {
		_ = b.docker.ResizeExec(ctx, execID, cols, rows)
	}
}

// write forwards stdin from an attached client to the docker exec. Any
// currently-attached client may write — multiple devices share the session.
// Writes from a connection that is no longer attached are silently dropped.
func (b *sessionPTY) write(from *websocket.Conn, p []byte) error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	if _, ok := b.clients[from]; !ok {
		b.mu.Unlock()
		return nil
	}
	conn := b.attached.Conn.Conn
	b.mu.Unlock()
	_, err := conn.Write(p)
	return err
}

// resize records this client's reported dimensions and applies the
// aggregated PTY size (min cols, min rows across all attached clients) to
// the docker exec. Aggregating to the minimum matches tmux's default
// non-aggressive resize: every viewer sees complete output, the larger
// device gets blank padding rather than the smaller one seeing wraparound.
func (b *sessionPTY) resize(ctx context.Context, from *websocket.Conn, cols, rows uint) error {
	if cols == 0 || rows == 0 {
		return nil
	}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	view, ok := b.clients[from]
	if !ok {
		b.mu.Unlock()
		return nil
	}
	view.cols = cols
	view.rows = rows
	aggCols, aggRows := b.aggregateSizeLocked()
	apply := aggCols != 0 && aggRows != 0 && (aggCols != b.cols || aggRows != b.rows)
	if apply {
		b.cols = aggCols
		b.rows = aggRows
	}
	execID := b.attached.ID
	b.mu.Unlock()
	if !apply {
		return nil
	}
	return b.docker.ResizeExec(ctx, execID, aggCols, aggRows)
}

// aggregateSizeLocked returns the smallest non-zero cols and rows reported
// across all attached clients. Returns (0, 0) when no client has reported a
// size yet. Caller must hold b.mu.
func (b *sessionPTY) aggregateSizeLocked() (uint, uint) {
	var cols, rows uint
	for _, v := range b.clients {
		if v.cols != 0 && (cols == 0 || v.cols < cols) {
			cols = v.cols
		}
		if v.rows != 0 && (rows == 0 || v.rows < rows) {
			rows = v.rows
		}
	}
	return cols, rows
}
