package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmelahman/git-orchard/cmd"
	"github.com/jmelahman/git-orchard/git"
	"github.com/jmelahman/git-orchard/internal/gittest"
)

// repo is a monorepo, in the process's working directory, beside a bare
// upstream it can add as a subtree.
type repo struct {
	t        *testing.T
	mono     git.Repo
	upstream git.Repo // a clone of the bare upstream, for making commits there
	bare     string
}

// newRepo builds the repositories and chdirs into the monorepo, since the
// commands open the repository in the working directory.
func newRepo(t *testing.T) *repo {
	t.Helper()
	gittest.Isolate(t)
	dir := t.TempDir()
	r := &repo{
		t:        t,
		mono:     git.Repo{Dir: filepath.Join(dir, "mono")},
		upstream: git.Repo{Dir: filepath.Join(dir, "foo")},
		bare:     filepath.Join(dir, "foo.git"),
	}

	r.git(git.Repo{Dir: dir}, "init", "--quiet", "--bare", "--initial-branch=master", r.bare)
	r.git(git.Repo{Dir: dir}, "clone", "--quiet", r.bare, r.upstream.Dir)
	r.commit(r.upstream, "README", "foo\n", "Start foo")
	r.git(r.upstream, "push", "--quiet", "origin", "HEAD:master")

	r.git(git.Repo{Dir: dir}, "init", "--quiet", "--initial-branch=master", r.mono.Dir)
	r.commit(r.mono, "README", "mono\n", "Start mono")

	t.Chdir(r.mono.Dir)
	return r
}

// withSubtree is newRepo with the upstream added at tools/foo, through the
// command line, and committed.
func withSubtree(t *testing.T) *repo {
	t.Helper()
	r := newRepo(t)
	if code, _, stderr := run(t, "add", r.bare, "tools/foo"); code != 0 {
		t.Fatalf("add exited %d: %s", code, stderr)
	}
	r.git(r.mono, "add", "-A")
	r.git(r.mono, "commit", "--quiet", "-m", "List tools/foo")
	return r
}

// withProfile is withSubtree with tools/foo sharing the profile "base",
// whose one file, hello.txt, the subtree doesn't have yet.
func withProfile(t *testing.T) *repo {
	t.Helper()
	r := withSubtree(t)
	// tools/foo's section is the manifest's last, so this extends it.
	manifest := filepath.Join(r.mono.Dir, r.manifest())
	f, err := os.OpenFile(manifest, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("\tshared = base\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	r.write(r.mono, ".config/git-orchard/shared/base/hello.txt", "hi\n")
	return r
}

// run runs the command line and returns its exit status and both streams.
func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = cmd.Main(args, &out, &errb)
	return code, out.String(), errb.String()
}

func (r *repo) git(repo git.Repo, args ...string) string {
	r.t.Helper()
	out, err := repo.Output(args...)
	if err != nil {
		r.t.Fatal(err)
	}
	return out
}

func (r *repo) write(repo git.Repo, path, content string) {
	r.t.Helper()
	full := filepath.Join(repo.Dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func (r *repo) commit(repo git.Repo, path, content, message string) {
	r.t.Helper()
	r.write(repo, path, content)
	r.git(repo, "add", path)
	r.git(repo, "commit", "--quiet", "-m", message)
}

// read returns the content of a file in the monorepo, failing the test if
// there is none.
func (r *repo) read(path string) string {
	r.t.Helper()
	content, err := os.ReadFile(filepath.Join(r.mono.Dir, path))
	if err != nil {
		r.t.Fatal(err)
	}
	return string(content)
}

func (r *repo) exists(path string) bool {
	r.t.Helper()
	_, err := os.Stat(filepath.Join(r.mono.Dir, path))
	if err != nil && !os.IsNotExist(err) {
		r.t.Fatal(err)
	}
	return err == nil
}

// manifest returns the manifest's path, relative to the monorepo, which is
// whichever of the allowed ones exists.
func (r *repo) manifest() string {
	r.t.Helper()
	for _, m := range []string{".gitsubtrees", ".config/git-orchard/subtrees"} {
		if r.exists(m) {
			return m
		}
	}
	r.t.Fatal("the monorepo has no manifest")
	return ""
}

func TestAdd(t *testing.T) {
	r := newRepo(t)
	code, _, stderr := run(t, "add", r.bare, "tools/foo")
	if code != 0 {
		t.Fatalf("exited %d: %s", code, stderr)
	}
	_, after, ok := strings.Cut(stderr, "Added tools/foo to ")
	if !ok {
		t.Fatalf("stderr doesn't say what was added: %q", stderr)
	}
	manifest, _, ok := strings.Cut(after, ";")
	if !ok {
		t.Fatalf("stderr doesn't name the manifest: %q", stderr)
	}
	if got := r.read("tools/foo/README"); got != "foo\n" {
		t.Errorf("tools/foo/README is %q", got)
	}
	if got := r.read(manifest); !strings.Contains(got, `[subtree "tools/foo"]`) {
		t.Errorf("%s doesn't list tools/foo:\n%s", manifest, got)
	}
}

func TestAddDefaultsPrefixToRepoName(t *testing.T) {
	r := newRepo(t)
	if code, _, stderr := run(t, "add", r.bare); code != 0 {
		t.Fatalf("exited %d: %s", code, stderr)
	}
	// The upstream is foo.git.
	if got := r.read("foo/README"); got != "foo\n" {
		t.Errorf("foo/README is %q", got)
	}
}

func TestList(t *testing.T) {
	r := withSubtree(t)
	code, stdout, stderr := run(t, "list")
	if code != 0 || stderr != "" {
		t.Fatalf("exited %d, stderr %q", code, stderr)
	}
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1:\n%s", len(lines), stdout)
	}
	want := []string{"tools/foo", r.bare, "master"}
	if got := strings.Fields(lines[0]); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestListUseHistory(t *testing.T) {
	withSubtree(t)
	code, stdout, stderr := run(t, "list", "--use-history")
	if code != 0 {
		t.Fatalf("exited %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Prefix: tools/foo\n") {
		t.Errorf("tools/foo wasn't found in history:\n%s", stdout)
	}
}

func TestStatus(t *testing.T) {
	r := withSubtree(t)
	code, before, stderr := run(t, "status")
	if code != 0 {
		t.Fatalf("exited %d: %s", code, stderr)
	}
	if want := "tools/foo  up to date\n"; before != want {
		t.Errorf("got %q, want %q", before, want)
	}

	r.commit(r.upstream, "main.go", "package main\n", "Add main")
	r.git(r.upstream, "push", "--quiet", "origin", "HEAD:master")
	code, after, stderr := run(t, "status")
	if code != 0 {
		t.Fatalf("exited %d: %s", code, stderr)
	}
	if want := "tools/foo  1 behind\n"; after != want {
		t.Errorf("got %q once the upstream moved, want %q", after, want)
	}
}

func TestStatusUnknownPrefix(t *testing.T) {
	withSubtree(t)
	code, stdout, stderr := run(t, "status", "no/such")
	if code != 2 {
		t.Errorf("exited %d, want 2", code)
	}
	if !strings.HasPrefix(stderr, "Error: ") {
		t.Errorf("stderr is %q", stderr)
	}
	if stdout != "" {
		t.Errorf("stdout is %q", stdout)
	}
}

func TestSyncCheckReportsWithoutWriting(t *testing.T) {
	r := withProfile(t)
	code, stdout, stderr := run(t, "sync", "--check")
	if code != 1 {
		t.Errorf("exited %d, want 1: %s", code, stderr)
	}
	if want := "1 shared file(s) out of date; run git orchard sync\n"; !strings.Contains(stderr, want) {
		t.Errorf("stderr is %q, want %q in it", stderr, want)
	}
	if !strings.Contains(stdout, "tools/foo/hello.txt") || !strings.Contains(stdout, "+hi\n") {
		t.Errorf("stdout isn't the diff of hello.txt:\n%s", stdout)
	}
	if r.exists("tools/foo/hello.txt") {
		t.Error("--check wrote tools/foo/hello.txt")
	}
}

func TestSyncWritesAndExitsOne(t *testing.T) {
	r := withProfile(t)
	code, stdout, stderr := run(t, "sync")
	if code != 1 {
		t.Errorf("exited %d, want 1: %s", code, stderr)
	}
	if want := "Updated tools/foo/hello.txt\n"; stderr != want {
		t.Errorf("stderr is %q, want %q", stderr, want)
	}
	if stdout != "" {
		t.Errorf("stdout is %q", stdout)
	}
	if got := r.read("tools/foo/hello.txt"); got != "hi\n" {
		t.Errorf("tools/foo/hello.txt is %q", got)
	}

	// With nothing left to do, it succeeds, silently.
	for _, args := range [][]string{{"sync"}, {"sync", "--check"}} {
		code, stdout, stderr := run(t, args...)
		if code != 0 || stdout != "" || stderr != "" {
			t.Errorf("%v again exited %d, stdout %q, stderr %q", args, code, stdout, stderr)
		}
	}
}

func TestOutsideARepository(t *testing.T) {
	gittest.Isolate(t)
	t.Chdir(t.TempDir())
	code, stdout, stderr := run(t, "list")
	if code != 2 {
		t.Errorf("exited %d, want 2", code)
	}
	if !strings.HasPrefix(stderr, "Error: ") {
		t.Errorf("stderr is %q", stderr)
	}
	if stdout != "" {
		t.Errorf("stdout is %q", stdout)
	}
}
