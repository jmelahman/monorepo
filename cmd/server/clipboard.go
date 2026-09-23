package server

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"

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

// nativeCopy pipes text into the first installed clipboard helper. Finding
// none is not an error — see setClipboard. A package variable so tests can
// exercise setClipboard without writing the machine's real clipboard.
var nativeCopy = func(text string) error {
	for _, argv := range clipboardCommands {
		path, err := exec.LookPath(argv[0])
		if err != nil {
			continue
		}
		cmd := exec.Command(path, argv[1:]...)
		cmd.Stdin = strings.NewReader(text)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return errors.New(argv[0] + ": " + firstLine(stderr.String(), err.Error()))
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
