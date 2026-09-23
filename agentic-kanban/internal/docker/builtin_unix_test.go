//go:build !windows

package docker

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// Smoke test that the syscall.Stat_t assertion in detectBuiltinUserFromClaude
// matches the type os.Stat actually returns on this platform — if a future
// build adds a non-Unix target the switch needs revisiting.
func TestDetectBuiltinUser_StatTypeAssertion(t *testing.T) {
	f := filepath.Join(t.TempDir(), "probe")
	if err := os.WriteFile(f, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(f)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := info.Sys().(*syscall.Stat_t); !ok {
		t.Fatalf("os.Stat().Sys() is not *syscall.Stat_t on %T", info.Sys())
	}
}
