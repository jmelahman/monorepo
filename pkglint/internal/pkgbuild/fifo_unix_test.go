//go:build unix

package pkgbuild

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// Opening a FIFO with no writer blocks, so a type check made on the open file
// never runs. A .SRCINFO that is a FIFO must be skipped, not hang Load.
func TestLoadRefusesFIFOWithoutBlocking(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte("pkgname=demo\npkgver=1\npkgrel=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(dir, ".SRCINFO"), 0o600); err != nil {
		t.Fatal(err)
	}
	type result struct {
		pkg *Package
		err error
	}
	done := make(chan result, 1)
	go func() {
		pkg, err := Load(dir)
		done <- result{pkg, err}
	}()
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("load: %v", r.err)
		}
		if r.pkg.SrcInfo != nil {
			t.Errorf("SrcInfo = %+v, want nil (a FIFO .SRCINFO must be skipped)", r.pkg.SrcInfo)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Load blocked on a FIFO .SRCINFO")
	}
}
