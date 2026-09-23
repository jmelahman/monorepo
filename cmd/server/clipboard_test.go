package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeClipboardHelper puts a script named after the first entry of
// clipboardCommands at the front of PATH, so nativeCopy finds it before any
// real helper. It returns the file the script records stdin into.
func fakeClipboardHelper(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	sink := filepath.Join(dir, "clipboard.txt")
	name := clipboardCommands[0][0]
	script := "#!/bin/sh\n" + strings.ReplaceAll(body, "$SINK", sink) + "\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return sink
}

func TestNativeCopy(t *testing.T) {
	t.Run("helper gets the text on stdin", func(t *testing.T) {
		sink := fakeClipboardHelper(t, `cat > "$SINK"`)
		if err := nativeCopy("ticket #42"); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(sink)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "ticket #42" {
			t.Errorf("helper received %q", got)
		}
	})

	t.Run("a failing helper is reported", func(t *testing.T) {
		fakeClipboardHelper(t, "echo 'no display' >&2\necho noise >&2\nexit 1")
		err := nativeCopy("x")
		if err == nil {
			t.Fatal("want an error")
		}
		if !strings.Contains(err.Error(), clipboardCommands[0][0]) || !strings.Contains(err.Error(), "no display") {
			t.Errorf("err = %v", err)
		}
		if strings.Contains(err.Error(), "noise") {
			t.Errorf("err carries more than the first stderr line: %v", err)
		}
	})

	// No helper installed is the ordinary case over ssh and in containers:
	// OSC 52 is then the only path, and there's nothing to report.
	t.Run("no helper is not an error", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		if err := nativeCopy("x"); err != nil {
			t.Errorf("err = %v", err)
		}
	})
}

// TestSetClipboardWritesBothPaths pins the property the two mechanisms
// exist for: the same text goes to the terminal (OSC 52) and to the local
// helper, and only the helper can report a failure.
func TestSetClipboardWritesBothPaths(t *testing.T) {
	screen := newFormScreen(t, 40, 10)
	var native string
	restore := nativeCopy
	nativeCopy = func(text string) error {
		native = text
		return nil
	}
	t.Cleanup(func() { nativeCopy = restore })

	if err := setClipboard(screen, "branch-name"); err != nil {
		t.Fatal(err)
	}
	if got := string(screen.GetClipboardData()); got != "branch-name" {
		t.Errorf("OSC 52 wrote %q", got)
	}
	if native != "branch-name" {
		t.Errorf("helper got %q", native)
	}
}

func TestFirstLine(t *testing.T) {
	for _, tc := range []struct {
		in, fallback, want string
	}{
		{"boom\nmore", "fb", "boom"},
		{"\n\n  real error  \nrest", "fb", "real error"},
		{"", "fb", "fb"},
		{"\n \n", "fb", "fb"},
	} {
		if got := firstLine(tc.in, tc.fallback); got != tc.want {
			t.Errorf("firstLine(%q, %q) = %q, want %q", tc.in, tc.fallback, got, tc.want)
		}
	}
}
