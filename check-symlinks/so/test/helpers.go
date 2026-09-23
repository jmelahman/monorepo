package main

import (
	"solod.dev/so/os"

	"github.com/jmelahman/check-symlinks/cli"
)

// tmpBuf backs the paths returned by MkdirTemp. Tests run sequentially and
// chdir into the new directory immediately, so reuse is safe.
var tmpBuf [os.MaxPathLen]byte

// enterRepo creates a fresh fake repository (a temp dir holding a .git
// entry) and makes it the working directory. Tests never chdir back: the
// next test enters its own absolute temp path.
func enterRepo() bool {
	dir, err := os.MkdirTemp(tmpBuf[:], "", "check-symlinks-test-")
	if err != nil {
		return false
	}
	if os.Chdir(dir) != nil {
		return false
	}
	return os.Mkdir(".git", 0o755) == nil
}

// makeDir creates a directory. The helpers avoid the obvious names
// (mkdir, write, link): package-level functions in package main become C
// globals, where those collide with the POSIX declarations in <unistd.h>.
func makeDir(name string) bool {
	return os.Mkdir(name, 0o755) == nil
}

// makeFile creates an empty regular file.
func makeFile(name string) bool {
	var empty [1]byte
	return os.WriteFile(name, empty[:0], 0o644) == nil
}

// writeText creates a file holding s.
func writeText(name string, s string) bool {
	var buf [4096]byte
	n := copy(buf[:], s)
	return os.WriteFile(name, buf[:n], 0o644) == nil
}

// makeLink creates a symlink at name pointing at target.
func makeLink(target, name string) bool {
	return os.Symlink(target, name) == nil
}

// The runN helpers invoke the CLI the way a shell would, argv[0] included.

func run1(a string) int {
	args := [2]string{"check-symlinks", a}
	return cli.Run(args[:])
}

func run2(a, b string) int {
	args := [3]string{"check-symlinks", a, b}
	return cli.Run(args[:])
}

func run3(a, b, c string) int {
	args := [4]string{"check-symlinks", a, b, c}
	return cli.Run(args[:])
}
