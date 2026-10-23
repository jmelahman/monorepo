//go:build unix

package cli

import (
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
