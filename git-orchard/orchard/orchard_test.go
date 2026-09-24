package orchard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmelahman/git-orchard/config"
	"github.com/jmelahman/git-orchard/git"
)

func TestSplitTag(t *testing.T) {
	for _, tc := range []struct {
		tag, prefix, name string
		ok                bool
	}{
		{"connections/v1.2.3", "connections", "v1.2.3", true},
		{"templates/golang-template/v1.0.0", "templates/golang-template", "v1.0.0", true},
		{"v1.2.3", "", "", false},
		{"connections/", "", "", false},
		{"/v1", "", "", false},
	} {
		prefix, name, ok := SplitTag(tc.tag)
		if prefix != tc.prefix || name != tc.name || ok != tc.ok {
			t.Errorf("SplitTag(%q) = %q, %q, %v", tc.tag, prefix, name, ok)
		}
	}
}

func TestStatusString(t *testing.T) {
	for st, want := range map[Status]string{
		{}:                    "up to date",
		{Ahead: 2}:            "2 ahead",
		{Behind: 1}:           "1 behind",
		{Ahead: 1, Behind: 3}: "diverged: 1 ahead, 3 behind",
	} {
		if got := st.String(); got != want {
			t.Errorf("%+v: got %q, want %q", st, got, want)
		}
	}
}

// fixture is a monorepo with one squashed subtree, "tools/foo", whose
// upstream is a bare repository beside it.
type fixture struct {
	t        *testing.T
	mono     git.Repo
	upstream git.Repo // a clone of the bare upstream, for making commits there
	bare     string
	orchard  *Orchard
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	isolateGit(t)
	dir := t.TempDir()
	f := &fixture{
		t:        t,
		mono:     git.Repo{Dir: filepath.Join(dir, "mono")},
		upstream: git.Repo{Dir: filepath.Join(dir, "foo")},
		bare:     filepath.Join(dir, "foo.git"),
	}

	f.git(git.Repo{Dir: dir}, "init", "--quiet", "--bare", "--initial-branch=master", f.bare)
	f.git(git.Repo{Dir: dir}, "clone", "--quiet", f.bare, f.upstream.Dir)
	f.commit(f.upstream, "README", "foo\n", "Start foo")
	f.git(f.upstream, "push", "--quiet", "origin", "HEAD:master")

	f.git(git.Repo{Dir: dir}, "init", "--quiet", "--initial-branch=master", f.mono.Dir)
	f.commit(f.mono, "README", "mono\n", "Start mono")

	o, err := Open(f.mono.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := o.Add(config.Subtree{Prefix: "tools/foo", Remote: f.bare, Branch: "master"}, ""); err != nil {
		t.Fatal(err)
	}
	f.git(f.mono, "add", o.Config.Manifest)
	f.git(f.mono, "commit", "--quiet", "-m", "List tools/foo")
	f.orchard = f.reopen()
	return f
}

// isolateGit keeps the user's git configuration (signing, hooks, URL
// rewrites) out of the tests.
func isolateGit(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, k := range []string{"GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME"} {
		t.Setenv(k, "Test")
	}
	for _, k := range []string{"GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(k, "test@example.com")
	}
}

func (f *fixture) reopen() *Orchard {
	o, err := Open(f.mono.Dir)
	if err != nil {
		f.t.Fatal(err)
	}
	return o
}

func (f *fixture) git(repo git.Repo, args ...string) string {
	f.t.Helper()
	out, err := repo.Output(args...)
	if err != nil {
		f.t.Fatal(err)
	}
	return out
}

func (f *fixture) commit(repo git.Repo, path, content, message string) {
	f.t.Helper()
	full := filepath.Join(repo.Dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
	f.git(repo, "add", path)
	f.git(repo, "commit", "--quiet", "-m", message)
}

func (f *fixture) subtree() config.Subtree {
	s, ok := f.orchard.Config.Lookup("tools/foo")
	if !ok {
		f.t.Fatal("tools/foo is not in the manifest")
	}
	return s
}

func (f *fixture) status() Status {
	f.t.Helper()
	st, err := f.orchard.Status(f.subtree(), "HEAD")
	if err != nil {
		f.t.Fatal(err)
	}
	return st
}

func TestAddRecordsManifest(t *testing.T) {
	f := newFixture(t)
	s := f.subtree()
	if s.Remote != f.bare || s.Branch != "master" {
		t.Errorf("got %+v", s)
	}
	if st := f.status(); st != (Status{}) {
		t.Errorf("a fresh subtree should be up to date, got %s", st)
	}
}

func TestPushFastForwards(t *testing.T) {
	f := newFixture(t)
	f.commit(f.mono, "tools/foo/main.go", "package main\n", "Add main")
	if st := f.status(); st != (Status{Ahead: 1}) {
		t.Errorf("got %s, want 1 ahead", st)
	}

	if err := f.orchard.Push(f.subtree(), "HEAD", false); err != nil {
		t.Fatal(err)
	}
	f.git(f.upstream, "pull", "--quiet", "--ff-only")
	if _, err := os.Stat(filepath.Join(f.upstream.Dir, "main.go")); err != nil {
		t.Errorf("main.go wasn't published: %v", err)
	}
	if got := f.git(f.upstream, "log", "-1", "--format=%s"); got != "Add main" {
		t.Errorf("upstream head is %q", got)
	}
	if st := f.status(); st != (Status{}) {
		t.Errorf("got %s after pushing, want up to date", st)
	}
}

func TestPushRefusesDivergedUpstream(t *testing.T) {
	f := newFixture(t)
	f.commit(f.upstream, "UPSTREAM", "x\n", "Upstream only")
	f.git(f.upstream, "push", "--quiet", "origin", "HEAD:master")
	f.commit(f.mono, "tools/foo/main.go", "package main\n", "Add main")

	if st := f.status(); st != (Status{Ahead: 1, Behind: 1}) {
		t.Errorf("got %s, want diverged", st)
	}
	if err := f.orchard.Push(f.subtree(), "HEAD", false); err == nil {
		t.Fatal("push over a diverged upstream should fail")
	}

	// Pulling the upstream in makes the next push a fast-forward.
	if err := f.orchard.Pull(f.subtree(), "Update tools/foo"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(f.mono.Dir, "tools/foo/UPSTREAM")); err != nil {
		t.Errorf("pull didn't bring in UPSTREAM: %v", err)
	}
	if err := f.orchard.Push(f.subtree(), "HEAD", false); err != nil {
		t.Fatal(err)
	}
	if st := f.status(); st != (Status{}) {
		t.Errorf("got %s, want up to date", st)
	}
}

func TestPushTag(t *testing.T) {
	f := newFixture(t)
	f.commit(f.mono, "tools/foo/main.go", "package main\n", "Add main")
	f.git(f.mono, "tag", "-a", "-m", "Release", "tools/foo/v1.0.0")
	f.commit(f.mono, "tools/foo/later.go", "package main\n", "After the release")

	if err := f.orchard.PushTag("refs/tags/tools/foo/v1.0.0", false); err != nil {
		t.Fatal(err)
	}
	got := f.git(git.Repo{Dir: f.bare}, "log", "-1", "--format=%s", "v1.0.0")
	if got != "Add main" {
		t.Errorf("v1.0.0 is at %q, want the tagged commit", got)
	}
	if err := f.orchard.PushTag("tools/foo/v2.0.0", false); err == nil || !strings.Contains(err.Error(), "doesn't exist") {
		t.Errorf("a missing tag should say so, got %v", err)
	}
	if err := f.orchard.PushTag("bar/v1.0.0", false); err == nil {
		t.Error("a tag for an unlisted prefix should fail")
	}
}

func TestChangedSince(t *testing.T) {
	f := newFixture(t)
	s := f.subtree()
	all := []config.Subtree{s}
	base := f.git(f.mono, "rev-parse", "HEAD")

	f.commit(f.mono, "README", "changed\n", "Outside the subtree")
	if got, err := f.orchard.ChangedSince(all, base, "HEAD"); err != nil || len(got) != 0 {
		t.Errorf("got %+v, %v; want nothing changed", got, err)
	}

	f.commit(f.mono, "tools/foo/main.go", "package main\n", "Inside the subtree")
	if got, err := f.orchard.ChangedSince(all, base, "HEAD"); err != nil || len(got) != 1 {
		t.Errorf("got %+v, %v; want tools/foo", got, err)
	}

	for _, since := range []string{"0000000000000000000000000000000000000000", "deadbeef"} {
		if got, err := f.orchard.ChangedSince(all, since, "HEAD"); err != nil || len(got) != 1 {
			t.Errorf("since %s: got %+v, %v; want everything", since, got, err)
		}
	}
}
