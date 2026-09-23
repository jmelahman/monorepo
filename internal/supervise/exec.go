package supervise

// Interactive exec into a supervised process's container — the supervisor
// half of `preview exec`. The session arrives as an execstream frame stream
// (whatever transport carried it — the apex API locally, the worker API on a
// fleet node) and is bridged onto a hijacked docker exec-attach connection.
// Host-process (non-container) previews have no exec surface and report a
// clear error instead.

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/jmelahman/local-preview/internal/dockerapi"
	"github.com/jmelahman/local-preview/internal/execstream"
)

// ExecOptions shapes one exec session.
type ExecOptions struct {
	Cmd []string `json:"cmd"`
	// TTY allocates a pseudo-terminal (raw combined output stream); Stdin
	// attaches the client's input.
	TTY   bool `json:"tty"`
	Stdin bool `json:"stdin"`
	// Term is the client's TERM, exported into the exec's environment when a
	// TTY is allocated.
	Term string `json:"term,omitempty"`
}

// execSetupTimeout bounds the non-streaming setup calls (create, start dial);
// the session itself is unbounded.
const execSetupTimeout = 30 * time.Second

// execExitPoll bounds how long the exit code is polled for after the output
// stream ends; the engine marks the exec stopped a beat after EOF.
const execExitPoll = 2 * time.Second

// Exec runs opts.Cmd inside the container backing k, bridging the session's
// execstream frames (on stream) to the engine's exec-attach connection. It
// returns when the command exits (after writing FrameExit) or the transport
// fails; orchestration errors are returned without any frames written, for
// the caller to report.
func (m *Manager) Exec(ctx context.Context, k Key, opts ExecOptions, stream io.ReadWriter) error {
	if len(opts.Cmd) == 0 {
		return fmt.Errorf("empty command")
	}
	m.mu.Lock()
	p := m.procs[k]
	m.mu.Unlock()
	if p == nil || !isReady(p) {
		return fmt.Errorf("preview process is not running (status %q) — open the preview to start it, then retry", m.Status(k))
	}
	if p.containerID == "" {
		return fmt.Errorf("this preview runs as a host process, not a container — exec needs a container runtime (a manifest run_image or a devcontainer)")
	}

	setupCtx, cancel := context.WithTimeout(ctx, execSetupTimeout)
	defer cancel()
	cli, err := m.docker(setupCtx)
	if err != nil {
		return fmt.Errorf("docker daemon: %w", err)
	}
	spec := dockerapi.ExecSpec{Cmd: opts.Cmd, TTY: opts.TTY, Stdin: opts.Stdin}
	if opts.TTY && opts.Term != "" {
		spec.Env = []string{"TERM=" + opts.Term}
	}
	execID, err := cli.ExecCreate(setupCtx, p.containerID, spec)
	if err != nil {
		return fmt.Errorf("create exec: %w", err)
	}
	conn, br, err := cli.ExecStart(setupCtx, execID, opts.TTY)
	if err != nil {
		return fmt.Errorf("start exec: %w", err)
	}
	defer conn.Close()
	m.touch(k)

	fw := execstream.NewWriter(stream)

	// Input pump: client frames → the exec's stdin (plus resizes). Ends when
	// the client goes away (read error) — closing conn then unblocks the
	// output side too.
	go func() {
		defer conn.Close()
		for {
			f, err := execstream.ReadFrame(stream)
			if err != nil {
				return
			}
			switch f.Type {
			case execstream.FrameStdin:
				if _, err := conn.Write(f.Payload); err != nil {
					return
				}
				m.touch(k)
			case execstream.FrameStdinEOF:
				if cw, ok := conn.(interface{ CloseWrite() error }); ok {
					cw.CloseWrite() //nolint:errcheck // best-effort EOF signal
				}
			case execstream.FrameResize:
				if cols, rows, err := execstream.DecodeResize(f.Payload); err == nil {
					rctx, rcancel := context.WithTimeout(context.Background(), 5*time.Second)
					cli.ExecResize(rctx, execID, cols, rows) //nolint:errcheck // cosmetic
					rcancel()
				}
			}
		}
	}()

	// Output pump: the exec's output → client frames, until the command exits
	// (EOF on the attach stream). An interactive session must count as
	// activity — output flowing keeps the process off the idle reaper.
	out := m.touchingReader(k, br)
	if opts.TTY {
		_, err = io.Copy(execstream.StreamWriter{FW: fw, Type: execstream.FrameStdout}, out)
	} else {
		err = dockerapi.DemuxStream(out,
			execstream.StreamWriter{FW: fw, Type: execstream.FrameStdout},
			execstream.StreamWriter{FW: fw, Type: execstream.FrameStderr})
	}
	if err != nil {
		return fmt.Errorf("exec stream: %w", err)
	}

	// The engine flips Running off a beat after the stream ends; poll briefly.
	deadline := time.Now().Add(execExitPoll)
	for {
		ictx, icancel := context.WithTimeout(context.Background(), execSetupTimeout)
		code, running, ierr := cli.ExecInspect(ictx, execID)
		icancel()
		if ierr != nil {
			return fmt.Errorf("exec exit code: %w", ierr)
		}
		if !running {
			return fw.WriteFrame(execstream.FrameExit, []byte{byte(min(max(code, 0), 255))})
		}
		if time.Now().After(deadline) {
			return fw.WriteFrame(execstream.FrameExit, []byte{0})
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// touch marks the key's process recently used, shielding it from the idle
// reaper while an exec session is active.
func (m *Manager) touch(k Key) {
	m.mu.Lock()
	if p := m.procs[k]; p != nil {
		p.lastTouch = time.Now()
	}
	m.mu.Unlock()
}

// touchingReader touches k on every read from r.
func (m *Manager) touchingReader(k Key, r io.Reader) io.Reader {
	return readerFunc(func(p []byte) (int, error) {
		n, err := r.Read(p)
		if n > 0 {
			m.touch(k)
		}
		return n, err
	})
}

type readerFunc func([]byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) { return f(p) }
