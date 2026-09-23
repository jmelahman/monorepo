package main

import (
	"solod.dev/so/mem"
	"solod.dev/so/os"

	"github.com/jmelahman/undot/cli"
)

// Path strings returned by MkdirTemp are views into this buffer. Tests run
// sequentially and chdir into the new directory immediately, so reuse is safe.
var tmpBuf [os.MaxPathLen]byte

// enterTempRepo creates a fresh fake repository (a temp dir with a .git
// entry) and makes it the working directory. Tests never chdir back: the
// next test enters its own absolute temp path.
func enterTempRepo() bool {
	dir, err := os.MkdirTemp(tmpBuf[:], "", "undot-test-")
	if err != nil {
		return false
	}
	if os.Chdir(dir) != nil {
		return false
	}
	// Point the user-level config lookup at the (empty) temp repo so a
	// developer's real ~/.config/undot/undot.conf can't leak into tests.
	if os.Setenv("XDG_CONFIG_HOME", dir) != nil {
		return false
	}
	return os.Mkdir(".git", 0o755) == nil
}

func runCLI(sub string) int {
	return cli.Run([]string{"undot", sub})
}

func runCLI2(sub, arg string) int {
	return cli.Run([]string{"undot", sub, arg})
}

func runCLI3(sub, arg1, arg2 string) int {
	return cli.Run([]string{"undot", sub, arg1, arg2})
}

// isLinkTo reports whether name is a symlink pointing exactly at want.
func isLinkTo(name, want string) bool {
	fi, err := os.Lstat(name)
	if err != nil {
		return false
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return false
	}
	var buf [os.MaxPathLen]byte
	dest, rerr := os.Readlink(buf[:], name)
	if rerr != nil {
		return false
	}
	return dest == want
}

func isRealDir(name string) bool {
	fi, err := os.Lstat(name)
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeSymlink == 0 && fi.IsDir()
}

func exists(name string) bool {
	_, err := os.Lstat(name)
	return err == nil
}

// readFile returns the file's content, or "" if it can't be read.
// Uses the system allocator rather than the leak-tracked t.Allocator():
// the returned string aliases the allocation, which lives until exit.
func readFile(name string) string {
	data, err := os.ReadFile(mem.System, name)
	if err != nil {
		return ""
	}
	return string(data)
}
