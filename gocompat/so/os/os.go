// Package os mirrors the subset of solod.dev/so/os that check-symlinks
// uses, backed by the Go standard library. Solod's buffer- and
// allocator-taking signatures are kept; the buffers and allocators are
// ignored because Go manages the memory.
package os

import (
	"errors"
	"io/fs"
	goos "os"

	"solod.dev/so/mem"
)

const MaxNameLen = 256
const MaxPathLen = 4096

var Args = goos.Args

// Solod reports errors as sentinel values rather than wrapped error types,
// and callers compare them with ==. mapErr translates the Go errors back
// into these sentinels so that `err == os.ErrNotExist` behaves the same
// under both toolchains.
var (
	ErrClosed     = errors.New("os: file already closed")
	ErrExist      = errors.New("os: file already exists")
	ErrIsDir      = errors.New("os: is a directory")
	ErrNotDir     = errors.New("os: not a directory")
	ErrNotExist   = errors.New("os: no such file or directory")
	ErrPermission = errors.New("os: permission denied")
	ErrIO         = errors.New("os: i/o error")
)

func mapErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, fs.ErrNotExist):
		return ErrNotExist
	case errors.Is(err, fs.ErrExist):
		return ErrExist
	case errors.Is(err, fs.ErrPermission):
		return ErrPermission
	case errors.Is(err, fs.ErrClosed):
		return ErrClosed
	}
	return err
}

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

func Stat(name string) (FileInfo, error) {
	fi, err := goos.Stat(name)
	if err != nil {
		return FileInfo{}, mapErr(err)
	}
	return FileInfo{fi: fi}, nil
}

func Lstat(name string) (FileInfo, error) {
	fi, err := goos.Lstat(name)
	if err != nil {
		return FileInfo{}, mapErr(err)
	}
	return FileInfo{fi: fi}, nil
}

// DirEntry mirrors solod's plain-struct DirEntry (fields, not methods).
// Type comes straight from the dirent's d_type, which is what lets the walk
// identify symlinks without an Lstat per entry.
type DirEntry struct {
	Name  string
	IsDir bool
	Type  FileMode
}

func ReadDir(a mem.Allocator, name string) ([]DirEntry, error) {
	_ = a
	entries, err := goos.ReadDir(name)
	if err != nil {
		return nil, mapErr(err)
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

// File mirrors solod's File, which is returned and passed by value.
type File struct {
	f *goos.File
}

var (
	Stdin  = &File{f: goos.Stdin}
	Stdout = &File{f: goos.Stdout}
	Stderr = &File{f: goos.Stderr}
)

func Open(name string) (File, error) {
	f, err := goos.Open(name)
	if err != nil {
		return File{}, mapErr(err)
	}
	return File{f: f}, nil
}

func Create(name string) (File, error) {
	f, err := goos.Create(name)
	if err != nil {
		return File{}, mapErr(err)
	}
	return File{f: f}, nil
}

func (f *File) Name() string { return f.f.Name() }

func (f *File) Read(b []byte) (int, error) {
	n, err := f.f.Read(b)
	if err != nil {
		// io.EOF must stay itself: readers compare against it.
		return n, err
	}
	return n, nil
}

func (f *File) Write(b []byte) (int, error) {
	n, err := f.f.Write(b)
	return n, mapErr(err)
}

func (f *File) WriteString(s string) (int, error) {
	n, err := f.f.WriteString(s)
	return n, mapErr(err)
}

func (f *File) Close() error {
	if f.f == nil {
		return ErrClosed
	}
	return mapErr(f.f.Close())
}

func Chdir(dir string) error                 { return mapErr(goos.Chdir(dir)) }
func Exit(code int)                          { goos.Exit(code) }
func Mkdir(name string, perm FileMode) error { return mapErr(goos.Mkdir(name, perm)) }
func Remove(name string) error               { return mapErr(goos.Remove(name)) }
func Rename(old, new string) error           { return mapErr(goos.Rename(old, new)) }
func Symlink(old, new string) error          { return mapErr(goos.Symlink(old, new)) }
func TempDir() string                        { return goos.TempDir() }

func Getenv(key string) string            { return goos.Getenv(key) }
func LookupEnv(key string) (string, bool) { return goos.LookupEnv(key) }
func Setenv(key, value string) error      { return mapErr(goos.Setenv(key, value)) }
func Unsetenv(key string) error           { return mapErr(goos.Unsetenv(key)) }

func ReadFile(a mem.Allocator, name string) ([]byte, error) {
	_ = a
	b, err := goos.ReadFile(name)
	return b, mapErr(err)
}

func WriteFile(name string, data []byte, perm FileMode) error {
	return mapErr(goos.WriteFile(name, data, perm))
}

func Getwd(buf []byte) (string, error) {
	_ = buf
	dir, err := goos.Getwd()
	return dir, mapErr(err)
}

func Readlink(buf []byte, name string) (string, error) {
	_ = buf
	dest, err := goos.Readlink(name)
	return dest, mapErr(err)
}

func MkdirTemp(buf []byte, dir, pattern string) (string, error) {
	_ = buf
	name, err := goos.MkdirTemp(dir, pattern)
	return name, mapErr(err)
}
