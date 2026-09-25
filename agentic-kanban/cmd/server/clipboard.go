package server

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
)

// clipboardCommands are the native "read stdin, set the clipboard" helpers,
// tried in order. The first one on PATH wins.
var clipboardCommands = [][]string{
	{"pbcopy"},                           // macOS
	{"wl-copy"},                          // Wayland
	{"xclip", "-selection", "clipboard"}, // X11
	{"xsel", "--clipboard", "--input"},   // X11 alternative
	{"clip.exe"},                         // WSL -> Windows
}

// setClipboard puts text on the user's clipboard from inside a tcell view.
// A package variable so tests can capture copies instead of touching the
// machine's real clipboard.
//
// Two mechanisms, both attempted: OSC 52 through the screen, and a native
// helper if one is installed. Neither covers every case on its own — a
// native helper writes the clipboard of the machine kanban runs on, which is
// the wrong one over ssh or from inside a container, while OSC 52 reaches
// the terminal the user is actually sitting at but is silently dropped by
// terminals that don't implement it (and by tmux/screen without the right
// setting). Doing both means the text lands wherever it can, and the two
// paths always carry identical content.
//
// With no helper installed the OSC 52 write is all there is, and there's no
// way to ask whether the terminal took it, so that counts as success: an
// error comes back only when a helper was found and it failed.
var setClipboard = func(s tcell.Screen, text string) error {
	s.SetClipboard([]byte(text))
	return nativeCopy(text)
}

// nativeCopyTimeout bounds a clipboard helper. Copies run on the view's
// event loop, so a wedged helper would otherwise freeze the terminal.
var nativeCopyTimeout = 3 * time.Second

// nativeCopy pipes text into the first installed clipboard helper. Finding
// none is not an error — see setClipboard. A package variable so tests can
// exercise setClipboard without writing the machine's real clipboard.
//
// xclip, xsel and wl-copy fork a child that stays alive to serve the
// selection until something else takes it, and that child inherits the
// helper's stdio. Stderr therefore goes to a file, never a pipe: with a
// pipe, Wait blocks until every holder of the write end exits — i.e.
// until the user next copies something elsewhere — and the view hangs.
var nativeCopy = func(text string) error {
	for _, argv := range clipboardCommands {
		path, err := exec.LookPath(argv[0])
		if err != nil {
			continue
		}
		stderr, err := os.CreateTemp("", "kanban-clipboard-*")
		if err != nil {
			return err
		}
		defer func() {
			stderr.Close()
			os.Remove(stderr.Name())
		}()
		ctx, cancel := context.WithTimeout(context.Background(), nativeCopyTimeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, path, argv[1:]...)
		cmd.Stdin = strings.NewReader(text)
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			if ctx.Err() != nil {
				return errors.New(argv[0] + ": timed out")
			}
			msg, _ := io.ReadAll(io.NewSectionReader(stderr, 0, 4096))
			return errors.New(argv[0] + ": " + firstLine(string(msg), err.Error()))
		}
		return nil
	}
	return nil
}

// firstLine returns the first non-blank line of s, or fallback when s has
// none — command stderr is usually one useful line followed by noise.
func firstLine(s, fallback string) string {
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return fallback
}
