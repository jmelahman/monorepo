package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jmelahman/git-orchard/git"
)

func TestApply(t *testing.T) {
	b := newBuilder()
	manifest := "orchard.squash\nfalse\x00" +
		"subtree.templates/PKGBUILDs-template.remote\ngit@github.com:o/PKGBUILDs-template.git\x00" +
		"subtree.tag.remote\ngit@github.com:o/tag.git\x00" +
		"subtree.tag.branch\nmain\x00"
	if err := b.apply(manifest); err != nil {
		t.Fatal(err)
	}
	// A local override replaces one key and leaves the rest.
	if err := b.apply("subtree.tag.remote\n/srv/tag.git\x00"); err != nil {
		t.Fatal(err)
	}
	c, err := b.build()
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		Squash:    false,
		SharedDir: DefaultSharedDir,
		Subtrees: []Subtree{
			{Prefix: "tag", Remote: "/srv/tag.git", Branch: "main"},
			{Prefix: "templates/PKGBUILDs-template", Remote: "git@github.com:o/PKGBUILDs-template.git", Branch: DefaultBranch},
		},
	}
	if !reflect.DeepEqual(c, want) {
		t.Errorf("got %+v, want %+v", c, want)
	}
}

func TestApplyShared(t *testing.T) {
	b := newBuilder()
	manifest := "orchard.shareddir\nconfigs/shared/\x00" +
		"subtree.tag.remote\ngit@github.com:o/tag.git\x00" +
		"subtree.tag.shared\nbase\x00" +
		"subtree.tag.shared\ngo\x00"
	if err := b.apply(manifest); err != nil {
		t.Fatal(err)
	}
	// A later layer adds profiles, skipping ones already listed.
	if err := b.apply("subtree.tag.shared\ngo\x00subtree.tag.shared\ngo-cli\x00"); err != nil {
		t.Fatal(err)
	}
	c, err := b.build()
	if err != nil {
		t.Fatal(err)
	}
	if c.SharedDir != "configs/shared" {
		t.Errorf("shared dir is %q", c.SharedDir)
	}
	if s, _ := c.Lookup("tag"); !reflect.DeepEqual(s.Shared, []string{"base", "go", "go-cli"}) {
		t.Errorf("shared is %q", s.Shared)
	}
	if err := newBuilder().apply("subtree.tag.shared\x00"); err == nil {
		t.Error("expected an error for a shared key without a profile")
	}
}

func TestApplyBareBoolean(t *testing.T) {
	b := newBuilder()
	b.squash = false
	if err := b.apply("orchard.squash\x00"); err != nil {
		t.Fatal(err)
	}
	if !b.squash {
		t.Error("a bare orchard.squash should be true")
	}
}

func TestBuildRequiresRemote(t *testing.T) {
	b := newBuilder()
	if err := b.apply("subtree.tag.branch\nmaster\x00"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.build(); err == nil {
		t.Error("expected an error for a subtree without a remote")
	}
}

func TestSelect(t *testing.T) {
	c := Config{Subtrees: []Subtree{{Prefix: "a"}, {Prefix: "b/c"}}}
	got, err := c.Select([]string{"./b/c/"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Prefix != "b/c" {
		t.Errorf("got %+v", got)
	}
	if _, err := c.Select([]string{"d"}); err == nil {
		t.Error("expected an error for an unlisted prefix")
	}
	if got, _ := c.Select(nil); len(got) != 2 {
		t.Errorf("no prefixes should select all, got %+v", got)
	}
}

func TestLoadAndAdd(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	repo := git.Repo{Dir: dir}
	if _, err := repo.Output("init", "--quiet"); err != nil {
		t.Fatal(err)
	}

	c, err := Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Squash || len(c.Subtrees) != 0 || c.Manifest != ".gitsubtrees" {
		t.Errorf("a missing manifest should be empty with squash on, got %+v", c)
	}

	if err := Add(repo, c.Manifest, Subtree{Prefix: "tools/foo", Remote: "git@example.com:foo.git", Branch: "main"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, c.Manifest))
	if err != nil {
		t.Fatal(err)
	}
	want := "[subtree \"tools/foo\"]\n\tremote = git@example.com:foo.git\n\tbranch = main\n"
	if string(data) != want {
		t.Errorf("manifest:\n%s\nwant:\n%s", data, want)
	}

	if _, err := repo.Output("config", "subtree.tools/foo.remote", "/srv/foo.git"); err != nil {
		t.Fatal(err)
	}
	c, err = Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	if s, _ := c.Lookup("tools/foo"); s.Remote != "/srv/foo.git" || s.Branch != "main" {
		t.Errorf(".git/config should override the manifest's remote only, got %+v", s)
	}
}

func TestLoadConfigDirManifest(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	repo := git.Repo{Dir: dir}
	if _, err := repo.Output("init", "--quiet"); err != nil {
		t.Fatal(err)
	}

	manifest := ".config/git-orchard/subtrees"
	if err := Add(repo, manifest, Subtree{Prefix: "tag", Remote: "git@example.com:tag.git"}); err != nil {
		t.Fatal(err)
	}
	c, err := Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	if c.Manifest != manifest {
		t.Errorf("manifest is %q, want %q", c.Manifest, manifest)
	}
	if _, ok := c.Lookup("tag"); !ok {
		t.Errorf("tag not loaded from %s: %+v", manifest, c)
	}

	if err := os.WriteFile(filepath.Join(dir, ".gitsubtrees"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(repo); err == nil {
		t.Error("expected an error with both manifests present")
	}
}
