package server

// `preview exec [ref] [-- command ...]`: run a command inside the container
// backing a preview, docker-exec style, with the same ref resolution as
// `preview open`/`preview logs`. The session rides a WebSocket of execstream
// frames end to end (CLI → apex API → worker, in fleet mode), so it works
// against a remote server exactly as against a local one.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/jmelahman/local-preview/internal/client"
	"github.com/jmelahman/local-preview/internal/execstream"
)

func execCmd() *cobra.Command {
	var serverURL, repoFlag, side string
	var interactive, tty bool

	cmd := &cobra.Command{
		Use:   "exec [ref] [-- command ...]",
		Short: "Run a command inside a preview's container",
		Long: "Exec into the container backing the deploy matching a ref (default:\n" +
			"HEAD of the current repo), like docker exec. The ref may be a branch\n" +
			"(its newest deploy wins), a tag, or a full or abbreviated commit sha;\n" +
			"the repo is auto-detected from the working directory unless --repo is\n" +
			"given. The command goes after \"--\"; without one you get /bin/sh, and\n" +
			"when the terminal allows it the session is interactive (-it) by\n" +
			"default. The preview's process must be running (open the preview to\n" +
			"start it) and containered — host-process previews have nothing to exec\n" +
			"into. The command's exit status becomes this command's exit status.",
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, argv, err := splitExecArgs(cmd, args)
			if err != nil {
				return err
			}
			stdinTerm := term.IsTerminal(int(os.Stdin.Fd()))
			if len(argv) == 0 {
				argv = []string{"/bin/sh"}
				// A bare `preview exec` from a terminal means "give me a
				// shell" — attach and allocate a TTY unless flags said
				// otherwise.
				if !cmd.Flags().Changed("interactive") && !cmd.Flags().Changed("tty") && stdinTerm {
					interactive, tty = true, true
				}
			}
			if tty && !stdinTerm {
				return fmt.Errorf("--tty requires stdin to be a terminal")
			}
			url, err := resolveURL(cmd, serverURL)
			if err != nil {
				return err
			}
			c, err := newClient(url)
			if err != nil {
				return err
			}
			d, err := runtimeDeploy(cmd.Context(), c, cmd.OutOrStdout(), repoFlag, ref)
			if err != nil {
				return err
			}
			hctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			conn, err := c.Exec(hctx, d.ID, client.ExecOptions{
				Side:  side,
				Cmd:   argv,
				TTY:   tty,
				Stdin: interactive,
				Term:  os.Getenv("TERM"),
			})
			if err != nil {
				return err
			}
			err = runExecSession(conn, interactive, tty, cmd.OutOrStdout(), cmd.ErrOrStderr())
			conn.Close()
			var ee execstream.ExitError
			if errors.As(err, &ee) {
				// Propagate the command's own exit status, silently — a
				// non-zero exit is the command's result, not a CLI failure.
				os.Exit(ee.Code)
			}
			return err
		},
	}
	addServerFlag(cmd, &serverURL)
	cmd.Flags().StringVar(&repoFlag, "repo", "", "Registered repo name (default: auto-detect from cwd)")
	cmd.Flags().StringVar(&side, "side", "be", "Which process: be (backend) or fe (process-mode frontend)")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Attach this terminal's stdin to the command")
	cmd.Flags().BoolVarP(&tty, "tty", "t", false, "Allocate a pseudo-terminal (for interactive shells and TUIs)")
	return cmd
}

// splitExecArgs separates the optional ref (before "--") from the command
// (after it). Multiple pre-dash args mean the user forgot the dash — say so
// rather than guessing which words were the command.
func splitExecArgs(cmd *cobra.Command, args []string) (ref string, argv []string, err error) {
	pre := args
	if dash := cmd.ArgsLenAtDash(); dash >= 0 {
		pre, argv = args[:dash], args[dash:]
	}
	switch len(pre) {
	case 0:
	case 1:
		ref = pre[0]
	default:
		return "", nil, fmt.Errorf("at most one ref before the command; put the command after \"--\" (e.g. preview exec %s -- %s)",
			pre[0], pre[1])
	}
	return ref, argv, nil
}

// execPingInterval keeps a quiet session alive through load-balancer idle
// timeouts (ALB defaults to 60s).
const execPingInterval = 30 * time.Second

// runExecSession drives one established exec connection: local stdin/resizes
// out as frames, output frames onto stdout/stderr, until FrameExit (returned
// as ExitError when non-zero) or FrameError. With tty set, the local
// terminal goes raw for the duration.
func runExecSession(conn net.Conn, interactive, tty bool, stdout, stderr io.Writer) error {
	fw := execstream.NewWriter(conn)
	done := make(chan struct{})
	defer close(done)

	if tty {
		oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			return fmt.Errorf("raw terminal: %w", err)
		}
		defer term.Restore(int(os.Stdin.Fd()), oldState) //nolint:errcheck
		// Track terminal size by polling — portable where SIGWINCH isn't, and
		// a 1s lag on a resize is imperceptible next to the redraw itself.
		go func() {
			var lastCols, lastRows int
			for {
				if cols, rows, err := term.GetSize(int(os.Stdin.Fd())); err == nil &&
					(cols != lastCols || rows != lastRows) {
					lastCols, lastRows = cols, rows
					fw.WriteFrame(execstream.FrameResize, execstream.ResizePayload(uint16(cols), uint16(rows))) //nolint:errcheck
				}
				select {
				case <-done:
					return
				case <-time.After(time.Second):
				}
			}
		}()
	}
	if interactive {
		go func() {
			buf := make([]byte, 32*1024)
			for {
				n, rerr := os.Stdin.Read(buf)
				if n > 0 {
					if fw.WriteFrame(execstream.FrameStdin, buf[:n]) != nil {
						return
					}
				}
				if rerr != nil {
					fw.WriteFrame(execstream.FrameStdinEOF, nil) //nolint:errcheck
					return
				}
			}
		}()
	}
	go func() {
		for {
			select {
			case <-done:
				return
			case <-time.After(execPingInterval):
				fw.WriteFrame(execstream.FramePing, nil) //nolint:errcheck
			}
		}
	}()

	for {
		f, err := execstream.ReadFrame(conn)
		if err != nil {
			return fmt.Errorf("exec session ended unexpectedly: %w", err)
		}
		switch f.Type {
		case execstream.FrameStdout:
			stdout.Write(f.Payload) //nolint:errcheck
		case execstream.FrameStderr:
			stderr.Write(f.Payload) //nolint:errcheck
		case execstream.FrameExit:
			code := 0
			if len(f.Payload) == 1 {
				code = int(f.Payload[0])
			}
			if code != 0 {
				return execstream.ExitError{Code: code}
			}
			return nil
		case execstream.FrameError:
			return execstream.RemoteError{Msg: string(f.Payload)}
		}
	}
}
