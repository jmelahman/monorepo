//go:build unix

package cli

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A source named by the PKGBUILD that is a FIFO would block os.Open forever;
// --fix must refuse it without opening it.
func TestLocalDigestRefusesFIFOWithoutBlocking(t *testing.T) {
	dir := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(dir, "demo.tar.gz"), 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := localDigest(dir, "demo.tar.gz")
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Errorf("localDigest of a FIFO = %v, want a not-a-regular-file refusal", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("localDigest blocked on a FIFO source")
	}
}

// A FIFO in the package directory would block the copy into a container
// forever if it were opened; it is left out of the tree instead.
func TestPackTreeSkipsFIFOWithoutBlocking(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte("pkgname=x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(dir, "pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	done := make(chan error, 1)
	go func() { done <- packTree(dir, &buf) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("packTree blocked on a FIFO")
	}
	entries := readTar(t, buf.Bytes())
	if entries["build/pipe"] != nil {
		t.Error("the FIFO was copied in")
	}
	if entries["build/PKGBUILD"] == nil {
		t.Error("the PKGBUILD beside the FIFO was not copied in")
	}
}

// A file the walk saw as regular and that is a FIFO by the time it is opened
// is refused, not opened: the open goes through pkgbuild.OpenRegular.
func TestPackFileRefusesFIFOSwap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "PKGBUILD")
	if err := os.WriteFile(path, []byte("pkgname=x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- packFile(tar.NewWriter(io.Discard), &tar.Header{Name: "build/PKGBUILD"}, path, fi) }()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Errorf("packFile of a swapped-in FIFO = %v, want a not-a-regular-file refusal", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("packFile blocked on a swapped-in FIFO")
	}
}
