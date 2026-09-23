package main

import (
	"solod.dev/so/testing"
)

// The exit codes are the contract: 0 = every symlink resolves, 1 = at least
// one is broken, 2 = usage error.

func TestValidLinks(t *testing.T) {
	if !enterRepo() {
		t.Fatal("cannot create test repository")
		return
	}
	if !makeFile("some_file") || !makeLink("some_file", "valid_link") {
		t.Fatal("cannot create fixture")
		return
	}
	if rc := run1("."); rc != 0 {
		t.Errorf("run(.) = %d, want 0", rc)
	}
	if rc := run1("valid_link"); rc != 0 {
		t.Errorf("run(valid_link) = %d, want 0", rc)
	}
}

func TestBrokenLink(t *testing.T) {
	if !enterRepo() {
		t.Fatal("cannot create test repository")
		return
	}
	if !makeLink("not_a_file", "broken_link") {
		t.Fatal("cannot create fixture")
		return
	}
	if rc := run1("."); rc != 1 {
		t.Errorf("run(.) = %d, want 1", rc)
	}
	if rc := run1("broken_link"); rc != 1 {
		t.Errorf("run(broken_link) = %d, want 1", rc)
	}
	if rc := run2("-q", "."); rc != 1 {
		t.Errorf("run(-q .) = %d, want 1", rc)
	}
}

// A link to a broken link is broken too.
func TestRecursiveBrokenLink(t *testing.T) {
	if !enterRepo() {
		t.Fatal("cannot create test repository")
		return
	}
	if !makeLink("not_a_file", "broken_link") || !makeLink("broken_link", "recursive_broken_link") {
		t.Fatal("cannot create fixture")
		return
	}
	if rc := run1("recursive_broken_link"); rc != 1 {
		t.Errorf("run(recursive_broken_link) = %d, want 1", rc)
	}
}

// A directory symlink is not followed, so a cycle terminates.
func TestDirectoryLink(t *testing.T) {
	if !enterRepo() {
		t.Fatal("cannot create test repository")
		return
	}
	if !makeFile("some_file") || !makeLink(".", "self_link") {
		t.Fatal("cannot create fixture")
		return
	}
	if rc := run1("."); rc != 0 {
		t.Errorf("run(.) = %d, want 0", rc)
	}
}

// Paths that do not exist, and paths that are not symlinks, are not errors.
func TestNonSymlinkArgs(t *testing.T) {
	if !enterRepo() {
		t.Fatal("cannot create test repository")
		return
	}
	if !makeFile("some_file") {
		t.Fatal("cannot create fixture")
		return
	}
	if rc := run1("doesnt_exist"); rc != 0 {
		t.Errorf("run(doesnt_exist) = %d, want 0", rc)
	}
	if rc := run1("some_file"); rc != 0 {
		t.Errorf("run(some_file) = %d, want 0", rc)
	}
	if rc := run1(""); rc != 0 {
		t.Errorf("run(\"\") = %d, want 0", rc)
	}
}

// Hidden entries are skipped while walking, but a hidden path named on the
// command line is always checked.
func TestHidden(t *testing.T) {
	if !enterRepo() {
		t.Fatal("cannot create test repository")
		return
	}
	if !makeLink("not_a_file", ".hidden_broken_link") {
		t.Fatal("cannot create fixture")
		return
	}
	if rc := run1("."); rc != 0 {
		t.Errorf("run(.) = %d, want 0", rc)
	}
	if rc := run2("--hidden", "."); rc != 1 {
		t.Errorf("run(--hidden .) = %d, want 1", rc)
	}
	if rc := run1(".hidden_broken_link"); rc != 1 {
		t.Errorf("run(.hidden_broken_link) = %d, want 1", rc)
	}
}

func TestHiddenDirectory(t *testing.T) {
	if !enterRepo() {
		t.Fatal("cannot create test repository")
		return
	}
	if !makeDir(".hidden_dir") || !makeFile(".hidden_dir/hidden_file") ||
		!makeLink("not_a_file", ".hidden_dir/hidden_broken_link") {
		t.Fatal("cannot create fixture")
		return
	}
	if rc := run1("."); rc != 0 {
		t.Errorf("run(.) = %d, want 0", rc)
	}
	// A hidden directory named as a root is filtered by the same rule, so
	// nothing under it is checked.
	if rc := run1(".hidden_dir"); rc != 0 {
		t.Errorf("run(.hidden_dir) = %d, want 0", rc)
	}
	if rc := run1(".hidden_dir/hidden_file"); rc != 0 {
		t.Errorf("run(.hidden_dir/hidden_file) = %d, want 0", rc)
	}
	if rc := run1(".hidden_dir/hidden_broken_link"); rc != 1 {
		t.Errorf("run(.hidden_dir/hidden_broken_link) = %d, want 1", rc)
	}
	if rc := run2("--hidden", "."); rc != 1 {
		t.Errorf("run(--hidden .) = %d, want 1", rc)
	}
}

func TestMultipleRoots(t *testing.T) {
	if !enterRepo() {
		t.Fatal("cannot create test repository")
		return
	}
	if !makeFile("some_file") || !makeLink("some_file", "valid_link") ||
		!makeLink("not_a_file", "broken_link") {
		t.Fatal("cannot create fixture")
		return
	}
	if rc := run2("some_file", "valid_link"); rc != 0 {
		t.Errorf("run(some_file valid_link) = %d, want 0", rc)
	}
	if rc := run3("some_file", "valid_link", "broken_link"); rc != 1 {
		t.Errorf("run(some_file valid_link broken_link) = %d, want 1", rc)
	}
}

// .symlinkignore, and .config/symlinkignore as a fallback, exclude paths
// from the walk; --no-ignore turns them off.
func TestIgnoreFile(t *testing.T) {
	if !enterRepo() {
		t.Fatal("cannot create test repository")
		return
	}
	if !makeLink("not_a_file", "broken_link") || !writeText(".symlinkignore", "# comment\nbroken_link\n") {
		t.Fatal("cannot create fixture")
		return
	}
	if rc := run1("."); rc != 0 {
		t.Errorf("run(.) = %d, want 0", rc)
	}
	if rc := run2("--no-ignore", "."); rc != 1 {
		t.Errorf("run(--no-ignore .) = %d, want 1", rc)
	}
}

func TestIgnoreFileInConfigDir(t *testing.T) {
	if !enterRepo() {
		t.Fatal("cannot create test repository")
		return
	}
	if !makeDir(".config") || !makeLink("not_a_file", "broken_link") ||
		!writeText(".config/symlinkignore", "broken_link\n") {
		t.Fatal("cannot create fixture")
		return
	}
	if rc := run1("."); rc != 0 {
		t.Errorf("run(.) = %d, want 0", rc)
	}
}

// The walk is the same whatever the thread count.
func TestSingleThreaded(t *testing.T) {
	if !enterRepo() {
		t.Fatal("cannot create test repository")
		return
	}
	if !makeDir("sub") || !makeLink("not_a_file", "sub/broken_link") {
		t.Fatal("cannot create fixture")
		return
	}
	if rc := run3("-threads", "1", "."); rc != 1 {
		t.Errorf("run(-threads 1 .) = %d, want 1", rc)
	}
	if rc := run3("-threads", "4", "."); rc != 1 {
		t.Errorf("run(-threads 4 .) = %d, want 1", rc)
	}
}

func TestUnknownFlag(t *testing.T) {
	if !enterRepo() {
		t.Fatal("cannot create test repository")
		return
	}
	if rc := run1("--foo"); rc != 2 {
		t.Errorf("run(--foo) = %d, want 2", rc)
	}
}

func TestVersion(t *testing.T) {
	if !enterRepo() {
		t.Fatal("cannot create test repository")
		return
	}
	if rc := run1("--version"); rc != 0 {
		t.Errorf("run(--version) = %d, want 0", rc)
	}
}
