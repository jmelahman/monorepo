package orchard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmelahman/git-orchard/config"
	"github.com/jmelahman/git-orchard/git"
	"github.com/jmelahman/git-orchard/semver"
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

	if err := f.orchard.Push(f.subtree(), "HEAD", PushOptions{}); err != nil {
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

// TestPushFromFreshClone pushes from a clone that, like CI's checkout, lacks
// the upstream commits the squashes were made from.
func TestPushFromFreshClone(t *testing.T) {
	f := newFixture(t)
	f.commit(f.mono, "tools/foo/main.go", "package main\n", "Add main")
	clone := git.Repo{Dir: filepath.Join(t.TempDir(), "clone")}
	f.git(f.mono, "clone", "--quiet", "--no-local", f.mono.Dir, clone.Dir)
	split := f.git(f.mono, "log", "-1", "--format=%(trailers:key=git-subtree-split,valueonly)", "--grep=^git-subtree-dir:")
	if _, err := clone.Output("cat-file", "-e", split); err == nil {
		t.Fatal("the clone already has the upstream commit")
	}

	o, err := Open(clone.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := o.Push(f.subtree(), "HEAD", PushOptions{}); err != nil {
		t.Fatal(err)
	}
	if got := f.git(git.Repo{Dir: f.bare}, "log", "-1", "--format=%s", "master"); got != "Add main" {
		t.Errorf("upstream head is %q", got)
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
	if err := f.orchard.Push(f.subtree(), "HEAD", PushOptions{}); err == nil {
		t.Fatal("push over a diverged upstream should fail")
	}

	// Pulling the upstream in makes the next push a fast-forward.
	if err := f.orchard.Pull(f.subtree(), "Update tools/foo"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(f.mono.Dir, "tools/foo/UPSTREAM")); err != nil {
		t.Errorf("pull didn't bring in UPSTREAM: %v", err)
	}
	if err := f.orchard.Push(f.subtree(), "HEAD", PushOptions{}); err != nil {
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

	if err := f.orchard.PushTag("refs/tags/tools/foo/v1.0.0", PushOptions{}); err != nil {
		t.Fatal(err)
	}
	got := f.git(git.Repo{Dir: f.bare}, "log", "-1", "--format=%s", "v1.0.0")
	if got != "Add main" {
		t.Errorf("v1.0.0 is at %q, want the tagged commit", got)
	}
	if err := f.orchard.PushTag("tools/foo/v2.0.0", PushOptions{}); err == nil || !strings.Contains(err.Error(), "doesn't exist") {
		t.Errorf("a missing tag should say so, got %v", err)
	}
	if err := f.orchard.PushTag("bar/v1.0.0", PushOptions{}); err == nil {
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

// installHook makes repo's hook fail.
func (f *fixture) installHook(repo git.Repo, hook string) {
	f.t.Helper()
	path := filepath.Join(f.git(repo, "rev-parse", "--absolute-git-dir"), "hooks", hook)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		f.t.Fatal(err)
	}
}

func TestNoVerify(t *testing.T) {
	f := newFixture(t)
	s := f.subtree()
	f.installHook(f.mono, "pre-push")
	f.installHook(f.mono, "pre-merge-commit")

	f.commit(f.mono, "tools/foo/main.go", "package main\n", "Add main")
	if err := f.orchard.Push(s, "HEAD", PushOptions{}); err == nil {
		t.Error("push should run the pre-push hook")
	}
	f.orchard.NoVerify = true
	if err := f.orchard.Push(s, "HEAD", PushOptions{}); err != nil {
		t.Fatal(err)
	}

	f.git(f.upstream, "pull", "--quiet", "origin", "master")
	f.commit(f.upstream, "lib.go", "package main\n", "Add lib")
	f.git(f.upstream, "push", "--quiet", "origin", "HEAD:master")
	f.orchard.NoVerify = false
	if err := f.orchard.Pull(s, ""); err == nil {
		t.Error("pull should run the pre-merge-commit hook")
	}
	f.git(f.mono, "merge", "--abort")
	f.orchard.NoVerify = true
	if err := f.orchard.Pull(s, ""); err != nil {
		t.Fatal(err)
	}
}

func TestRelease(t *testing.T) {
	f := newFixture(t)
	s := f.subtree()
	origin := filepath.Join(filepath.Dir(f.bare), "mono.git")
	f.git(f.mono, "init", "--quiet", "--bare", origin)
	opts := ReleaseOptions{Rev: "HEAD", Remote: origin}

	f.commit(f.mono, "tools/foo/main.go", "package main\n", "Add main")
	if _, err := f.orchard.Release(s, "v1.0.0", opts); err == nil || !strings.Contains(err.Error(), "lacks 1 commit") {
		t.Fatalf("releasing a commit the upstream lacks: got %v", err)
	}
	if err := f.orchard.Push(s, "HEAD", PushOptions{}); err != nil {
		t.Fatal(err)
	}
	tag, err := f.orchard.Release(s, "v1.0.0", opts)
	if err != nil {
		t.Fatal(err)
	}
	if tag != "tools/foo/v1.0.0" {
		t.Errorf("got tag %s", tag)
	}
	if got := f.git(git.Repo{Dir: origin}, "log", "-1", "--format=%s", tag); got != "Add main" {
		t.Errorf("origin's %s is at %q", tag, got)
	}
	// A release tagged but never published is finished by releasing it again.
	f.git(f.mono, "tag", "--annotate", "--message=Unpublished", "tools/foo/v1.0.1")
	if _, err := f.orchard.Release(s, "v1.0.1", opts); err != nil {
		t.Fatal(err)
	}
	if got := f.git(git.Repo{Dir: origin}, "tag", "--list", "--format=%(contents:subject)", "tools/foo/v1.0.1"); got != "Unpublished" {
		t.Errorf("origin's tools/foo/v1.0.1 is %q, want the existing tag", got)
	}
	f.commit(f.mono, "tools/foo/other.go", "package main\n", "Add other")
	if err := f.orchard.Push(s, "HEAD", PushOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.orchard.Release(s, "v1.0.0", opts); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Errorf("releasing a tag of another commit: got %v", err)
	}
	for _, bad := range []string{"", "a/b", "v1..0"} {
		if _, err := f.orchard.Release(s, bad, opts); err == nil {
			t.Errorf("version %q should be rejected", bad)
		}
	}

	f.commit(f.mono, "tools/foo/later.go", "package main\n", "Add later")
	opts.Upstream = true
	if _, err := f.orchard.Release(s, "v1.1.0", opts); err != nil {
		t.Fatal(err)
	}
	upstream := git.Repo{Dir: f.bare}
	if got := f.git(upstream, "log", "-1", "--format=%s", "v1.1.0"); got != "Add later" {
		t.Errorf("upstream v1.1.0 is at %q", got)
	}
	if got := f.git(upstream, "rev-parse", "master"); got != f.git(upstream, "rev-parse", "v1.1.0^{commit}") {
		t.Error("--upstream should publish the branch too")
	}

	// Releasing again changes nothing, even after the upstream moves on.
	f.commit(f.mono, "tools/foo/next.go", "package main\n", "Add next")
	if err := f.orchard.Push(s, "HEAD", PushOptions{}); err != nil {
		t.Fatal(err)
	}
	head := f.git(upstream, "rev-parse", "master")
	opts.Rev = "HEAD~1"
	for range 2 {
		if _, err := f.orchard.Release(s, "v1.1.0", opts); err != nil {
			t.Fatal(err)
		}
	}
	if got := f.git(upstream, "rev-parse", "master"); got != head {
		t.Error("releasing again rewound the upstream branch")
	}
}

func TestPushForce(t *testing.T) {
	f := newFixture(t)
	s := f.subtree()
	f.commit(f.upstream, "lib.go", "package main\n", "Upstream only")
	f.git(f.upstream, "push", "--quiet", "origin", "HEAD:master")
	f.commit(f.mono, "tools/foo/main.go", "package main\n", "Add main")

	if err := f.orchard.Push(s, "HEAD", PushOptions{}); err == nil {
		t.Fatal("a diverged upstream should reject an unforced push")
	}
	if err := f.orchard.Push(s, "HEAD", PushOptions{Force: true}); err != nil {
		t.Fatal(err)
	}
	if got := f.git(git.Repo{Dir: f.bare}, "log", "-1", "--format=%s", "master"); got != "Add main" {
		t.Errorf("upstream master is at %q", got)
	}
}

func TestLsRemote(t *testing.T) {
	f := newFixture(t)
	head := f.git(git.Repo{Dir: f.bare}, "rev-parse", "master")
	f.git(f.upstream, "tag", "-a", "-m", "annotated", "v1")
	f.git(f.upstream, "push", "--quiet", "origin", "v1")

	for ref, want := range map[string][2]string{
		"refs/heads/master": {head, head},
		"refs/tags/v1":      {f.git(f.upstream, "rev-parse", "v1"), head},
		"refs/tags/v2":      {"", ""},
	} {
		oid, commit, err := f.orchard.lsRemote(f.bare, ref)
		if err != nil || oid != want[0] || commit != want[1] {
			t.Errorf("%s: got %q, %q, %v; want %q", ref, oid, commit, err, want)
		}
	}
	if commit, err := f.orchard.UpstreamTag(f.subtree(), "v1"); err != nil || commit != head {
		t.Errorf("UpstreamTag(v1) = %q, %v; want %q", commit, err, head)
	}
}

func TestRemoteURL(t *testing.T) {
	f := newFixture(t)
	f.git(f.mono, "remote", "add", "origin", f.bare)
	if got := f.orchard.RemoteURL("origin"); got != f.bare {
		t.Errorf("RemoteURL(origin) = %q, want %q", got, f.bare)
	}
	if got := f.orchard.RemoteURL(f.bare); got != f.bare {
		t.Errorf("RemoteURL(%s) = %q, want it back", f.bare, got)
	}
}

func TestReleaseForce(t *testing.T) {
	f := newFixture(t)
	s := f.subtree()
	origin := filepath.Join(filepath.Dir(f.bare), "mono.git")
	f.git(f.mono, "init", "--quiet", "--bare", origin)
	opts := ReleaseOptions{Rev: "HEAD", Remote: origin}
	release := func(force bool) error {
		opts.Force = force
		_, err := f.orchard.Release(s, "v1.0.0", opts)
		return err
	}
	originTag := func() string {
		return f.git(git.Repo{Dir: origin}, "log", "-1", "--format=%s", "tools/foo/v1.0.0")
	}

	// Tagged on origin, but not yet published upstream: movable.
	f.commit(f.mono, "tools/foo/main.go", "package main\n", "Add main")
	if err := f.orchard.Push(s, "HEAD", PushOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := release(false); err != nil {
		t.Fatal(err)
	}
	f.commit(f.mono, "tools/foo/fix.go", "package main\n", "Fix main")
	if err := f.orchard.Push(s, "HEAD", PushOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := release(false); err == nil {
		t.Fatal("an existing tag should need --force")
	}
	if err := release(true); err != nil {
		t.Fatal(err)
	}
	if got := originTag(); got != "Fix main" {
		t.Errorf("origin's tag is at %q, want it moved", got)
	}

	// Published upstream: re-releasing the same commit is fine, moving isn't.
	if err := f.orchard.PushTag("tools/foo/v1.0.0", PushOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := release(true); err != nil {
		t.Errorf("re-releasing the published commit: %v", err)
	}
	f.commit(f.mono, "tools/foo/later.go", "package main\n", "Later")
	if err := f.orchard.Push(s, "HEAD", PushOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := release(true); err == nil || !strings.Contains(err.Error(), "already published") {
		t.Errorf("moving a published release: got %v", err)
	}
	if got := originTag(); got != "Fix main" {
		t.Errorf("a refused release moved origin's tag to %q", got)
	}
}

func TestNextVersion(t *testing.T) {
	f := newFixture(t)
	s := f.subtree()
	origin := filepath.Join(filepath.Dir(f.bare), "mono.git")
	f.git(f.mono, "init", "--quiet", "--bare", origin)
	// A want of "=v1.0.0" expects HEAD's existing release.
	next := func(inc semver.Increment, suffix, want string) {
		t.Helper()
		got, released, err := f.orchard.NextVersion(s, NextOptions{Rev: "HEAD", Remote: origin, Increment: inc, Suffix: suffix})
		if err != nil {
			t.Fatal(err)
		}
		if released {
			got = "=" + got
		}
		if got != want {
			t.Errorf("next version (increment %d, suffix %q): got %s, want %s", inc, suffix, got, want)
		}
	}
	release := func(version string) {
		t.Helper()
		f.commit(f.mono, "tools/foo/"+version, version+"\n", "Prepare "+version)
		if _, err := f.orchard.Release(s, version, ReleaseOptions{Rev: "HEAD", Remote: origin, Upstream: true}); err != nil {
			t.Fatal(err)
		}
	}

	next(semver.Auto, "", "v0.0.1")
	next(semver.Auto, "rc", "v0.0.1-rc")

	// Releases from before the subtree count.
	f.git(f.upstream, "tag", "v0.6.1")
	f.git(f.upstream, "tag", "not-a-version")
	f.git(f.upstream, "push", "--quiet", "origin", "--tags")
	next(semver.Auto, "", "v0.6.2")
	next(semver.Minor, "", "v0.7.0")
	next(semver.Major, "", "v1.0.0")

	release("v1.0.0")
	// An already released revision is released again, to finish publishing.
	next(semver.Auto, "", "=v1.0.0")
	f.commit(f.mono, "tools/foo/later", "later\n", "Later")
	next(semver.Auto, "", "v1.0.1")

	// Pre-releases follow the stable release and precede the next one.
	next(semver.Auto, "rc", "v1.0.1-rc")
	release("v1.0.1-rc")
	next(semver.Auto, "", "v1.0.1")
	next(semver.Auto, "rc", "=v1.0.1-rc")
	next(semver.Auto, "beta", "v1.0.1-beta")
	f.commit(f.mono, "tools/foo/fix", "fix\n", "Fix")
	next(semver.Auto, "rc", "v1.0.1-rc.1")
	next(semver.Auto, "beta", "v1.0.1-beta")
	next(semver.Auto, "", "v1.0.1")
	next(semver.Patch, "", "v1.0.2")

	// Tags only on the monorepo's remote count, but not other subtrees'.
	f.git(f.mono, "tag", "tools/foo/v1.4.0")
	f.git(f.mono, "tag", "tools/foo/bar/v9.0.0")
	f.git(f.mono, "tag", "tools/v9.0.0")
	f.git(f.mono, "push", "--quiet", origin, "tools/foo/v1.4.0")
	f.git(f.mono, "tag", "--delete", "tools/foo/v1.4.0")
	next(semver.Auto, "", "v1.4.1")
}
