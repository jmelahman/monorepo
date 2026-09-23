// Package os mirrors the subset of solod.dev/so/os used by undot,
// backed by the Go standard library. Solod's buffer- and allocator-taking
// signatures are kept; the buffers and allocators are ignored because Go
// manages the memory.
package os

import (
	"io/fs"
	goos "os"

	"solod.dev/so/mem"
)

const MaxNameLen = 256
const MaxPathLen = 4096

var Args = goos.Args

var (
	Stdin  = goos.Stdin
	Stdout = goos.Stdout
	Stderr = goos.Stderr
)

type FileMode = fs.FileMode

const (
	ModeDir     = fs.ModeDir
	ModeSymlink = fs.ModeSymlink
	ModePerm    = fs.ModePerm
)

// FileInfo mirrors solod's struct-with-methods shape.
type FileInfo struct {
	fi fs.FileInfo
}

func (f *FileInfo) IsDir() bool    { return f.fi.IsDir() }
func (f *FileInfo) Mode() FileMode { return f.fi.Mode() }
func (f *FileInfo) Name() string   { return f.fi.Name() }
func (f *FileInfo) Size() int64    { return f.fi.Size() }

func Lstat(name string) (FileInfo, error) {
	fi, err := goos.Lstat(name)
	if err != nil {
		return FileInfo{}, err
	}
	return FileInfo{fi: fi}, nil
}

func Stat(name string) (FileInfo, error) {
	fi, err := goos.Stat(name)
	if err != nil {
		return FileInfo{}, err
	}
	return FileInfo{fi: fi}, nil
}

// DirEntry mirrors solod's plain-struct DirEntry (fields, not methods).
type DirEntry struct {
	Name  string
	IsDir bool
	Type  FileMode
}

func ReadDir(a mem.Allocator, name string) ([]DirEntry, error) {
	_ = a
	entries, err := goos.ReadDir(name)
	if err != nil {
		return nil, err
	}
	out := make([]DirEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, DirEntry{Name: e.Name(), IsDir: e.IsDir(), Type: e.Type()})
	}
	return out, nil
}

func FreeDirEntry(a mem.Allocator, entries []DirEntry) {
	_, _ = a, entries
}

func ReadFile(a mem.Allocator, name string) ([]byte, error) {
	_ = a
	return goos.ReadFile(name)
}

func WriteFile(name string, data []byte, perm FileMode) error {
	return goos.WriteFile(name, data, perm)
}

func Getenv(key string) string          { return goos.Getenv(key) }
func Setenv(key, value string) error    { return goos.Setenv(key, value) }
func Unsetenv(key string) error         { return goos.Unsetenv(key) }
func LookupEnv(key string) (string, bool) { return goos.LookupEnv(key) }

func Chdir(dir string) error               { return goos.Chdir(dir) }
func Exit(code int)                        { goos.Exit(code) }
func Mkdir(name string, perm FileMode) error { return goos.Mkdir(name, perm) }
func Remove(name string) error             { return goos.Remove(name) }
func Rename(oldpath, newpath string) error { return goos.Rename(oldpath, newpath) }
func Symlink(oldname, newname string) error { return goos.Symlink(oldname, newname) }
func TempDir() string                      { return goos.TempDir() }

func Getwd(buf []byte) (string, error) {
	_ = buf
	return goos.Getwd()
}

func Readlink(buf []byte, name string) (string, error) {
	_ = buf
	return goos.Readlink(name)
}

func MkdirTemp(buf []byte, dir, pattern string) (string, error) {
	_ = buf
	return goos.MkdirTemp(dir, pattern)
}
