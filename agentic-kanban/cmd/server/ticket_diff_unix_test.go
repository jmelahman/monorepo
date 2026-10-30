//go:build unix

package server

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeViewer installs an executable in dir that records its arguments, then
// its stdin, to log.
func writeViewer(t *testing.T, dir, name, log string) {
	t.Helper()
	script := "#!/bin/sh\necho \"args: $*\" > " + log + "\n/bin/cat >> " + log + "\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
}

// linkShell puts sh on a PATH that's otherwise limited to dir, since $PAGER runs
// through the shell.
func linkShell(t *testing.T, dir string) {
	t.Helper()
	if err := os.Symlink("/bin/sh", filepath.Join(dir, "sh")); err != nil {
		t.Fatal(err)
	}
}

func readLog(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestViewPatch(t *testing.T) {
	const patch = "@@ -1 +1 @@\n+added\n"
	const colored = "\x1b[36m@@ -1 +1 @@\x1b[0m\n\x1b[32m+added\x1b[0m\n"

	t.Run("prefers_hunk", func(t *testing.T) {
		dir := t.TempDir()
		log := filepath.Join(dir, "log")
		writeViewer(t, dir, "hunk", log)
		writeViewer(t, dir, "less", filepath.Join(dir, "less.log"))
		t.Setenv("PATH", dir)
		t.Setenv("PAGER", "")

		var out bytes.Buffer
		if err := viewPatch(t.Context(), patch, &out, &out); err != nil {
			t.Fatalf("%v\n%s", err, out.String())
		}
		if got, want := readLog(t, log), "args: patch -\n"+patch; got != want {
			t.Errorf("hunk got %q, want %q", got, want)
		}
	})

	t.Run("hunk_failure_is_reported", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "hunk"), []byte("#!/bin/sh\nexit 3\n"), 0o700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", dir)

		var out bytes.Buffer
		err := viewPatch(t.Context(), patch, &out, &out)
		if err == nil || !strings.Contains(err.Error(), "--plain") {
			t.Fatalf("err = %v, want a hint about --plain", err)
		}
	})

	t.Run("survives_ctrl_c_in_viewer", func(t *testing.T) {
		dir := t.TempDir()
		log := filepath.Join(dir, "log")
		script := "#!/bin/sh\nkill -INT $PPID\n/bin/cat > " + log + "\n"
		if err := os.WriteFile(filepath.Join(dir, "hunk"), []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", dir)

		var out bytes.Buffer
		if err := viewPatch(t.Context(), patch, &out, &out); err != nil {
			t.Fatalf("%v\n%s", err, out.String())
		}
		if got := readLog(t, log); got != patch {
			t.Errorf("hunk got %q, want %q", got, patch)
		}
	})

	t.Run("falls_back_to_pager_env", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "my tools")
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		log := filepath.Join(t.TempDir(), "log")
		writeViewer(t, dir, "mypager", log)
		linkShell(t, dir)
		t.Setenv("PATH", dir)
		t.Setenv("PAGER", `"`+filepath.Join(dir, "mypager")+`" --flag`)

		var out bytes.Buffer
		if err := viewPatch(t.Context(), patch, &out, &out); err != nil {
			t.Fatalf("%v\n%s", err, out.String())
		}
		if got, want := readLog(t, log), "args: --flag\n"+colored; got != want {
			t.Errorf("pager got %q, want %q", got, want)
		}
	})

	t.Run("missing_pager_is_reported", func(t *testing.T) {
		dir := t.TempDir()
		linkShell(t, dir)
		t.Setenv("PATH", dir)
		t.Setenv("PAGER", "no-such-pager")

		var out bytes.Buffer
		err := viewPatch(t.Context(), patch, &out, &out)
		if err == nil || !strings.Contains(err.Error(), "--plain") {
			t.Fatalf("err = %v, want a hint about --plain", err)
		}
	})

	t.Run("escapes_controls_for_pager", func(t *testing.T) {
		dir := t.TempDir()
		log := filepath.Join(dir, "log")
		writeViewer(t, dir, "less", log)
		t.Setenv("PATH", dir)
		t.Setenv("PAGER", "")

		var out bytes.Buffer
		if err := viewPatch(t.Context(), "+\x1b]0;title\x07\n", &out, &out); err != nil {
			t.Fatalf("%v\n%s", err, out.String())
		}
		if got, want := readLog(t, log), "args: \n\x1b[32m+^[]0;title^G\x1b[0m\n"; got != want {
			t.Errorf("less got %q, want %q", got, want)
		}
	})

	t.Run("falls_back_to_less", func(t *testing.T) {
		dir := t.TempDir()
		log := filepath.Join(dir, "log")
		writeViewer(t, dir, "less", log)
		t.Setenv("PATH", dir)
		t.Setenv("PAGER", "")

		var out bytes.Buffer
		if err := viewPatch(t.Context(), patch, &out, &out); err != nil {
			t.Fatalf("%v\n%s", err, out.String())
		}
		if got, want := readLog(t, log), "args: \n"+colored; got != want {
			t.Errorf("less got %q, want %q", got, want)
		}
	})

	t.Run("prints_without_a_viewer", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		t.Setenv("PAGER", "")

		var out bytes.Buffer
		if err := viewPatch(t.Context(), patch+"+\x1b[2Khidden\n", &out, &out); err != nil {
			t.Fatal(err)
		}
		want := colored + "\x1b[32m+^[[2Khidden\x1b[0m\n"
		if out.String() != want {
			t.Errorf("output = %q, want %q", out.String(), want)
		}
	})
}
