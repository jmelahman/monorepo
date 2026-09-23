package alpmdb

import (
	"os"
	"testing"
)

// TestRealLocalDB exercises the reader against the host's actual pacman
// database when present; elsewhere it skips, keeping CI hermetic.
func TestRealLocalDB(t *testing.T) {
	if _, err := os.Stat(DefaultRoot); err != nil {
		t.Skip("no pacman local database on this host")
	}
	db, err := Load(DefaultRoot)
	if err != nil {
		t.Fatal(err)
	}
	if db == nil {
		t.Fatal("nil DB despite existing root")
	}
	// glibc is on every Arch system and ships libc.so.6; bash ships /usr/bin/bash.
	if got := db.LibraryOwner("libc.so.6", false); got != "glibc" {
		t.Errorf("LibraryOwner(libc.so.6) = %q, want glibc", got)
	}
	if got := db.CommandOwner("bash"); got != "bash" {
		t.Errorf("CommandOwner(bash) = %q, want bash", got)
	}
	closure := db.Closure([]string{"bash"})
	if !closure["glibc"] {
		t.Errorf("bash's dependency closure should include glibc (got %d packages)", len(closure))
	}
}
