package hashkey

import (
	"testing"

	"github.com/jmelahman/local-preview/internal/gitrepo"
	"github.com/jmelahman/local-preview/internal/manifest"
)

func entry(oid, p string) gitrepo.TreeEntry {
	return gitrepo.TreeEntry{Mode: "100644", Type: "blob", OID: oid, Path: p}
}

var (
	fe = manifest.Frontend{Path: "web", Build: [][]string{{"npm", "run", "build"}}, Dist: "dist"}
	be = manifest.Backend{
		Path:       ".",
		Exclude:    []string{"docs/", "*.md"},
		Build:      [][]string{{"go", "build", "."}},
		Run:        []string{"./server", "{port}"},
		HealthPath: "/api/health",
	}
	tree = []gitrepo.TreeEntry{
		entry("aaa", "README.md"),
		entry("bbb", "docs/guide.md"),
		entry("ccc", "main.go"),
		entry("ddd", "web/index.html"),
		entry("eee", "web/src/app.tsx"),
	}
)

func TestDeterminism(t *testing.T) {
	h1, err := Frontend(fe, nil, tree)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := Frontend(fe, nil, tree)
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 || len(h1) != 64 {
		t.Fatalf("unstable or malformed hash: %q vs %q", h1, h2)
	}
}

func TestPartitionSensitivity(t *testing.T) {
	feBase, err := Frontend(fe, nil, tree)
	if err != nil {
		t.Fatal(err)
	}
	beBase, err := Backend(be, fe.Path, nil, tree)
	if err != nil {
		t.Fatal(err)
	}

	// Changing a frontend blob changes fe_hash but not be_hash.
	mod := append([]gitrepo.TreeEntry(nil), tree...)
	mod[3] = entry("zzz", "web/index.html")
	feMod, _ := Frontend(fe, nil, mod)
	beMod, _ := Backend(be, fe.Path, nil, mod)
	if feMod == feBase {
		t.Fatal("frontend change did not change fe_hash")
	}
	if beMod != beBase {
		t.Fatal("frontend change leaked into be_hash")
	}

	// Changing an excluded file (docs/, *.md) changes neither hash.
	mod = append([]gitrepo.TreeEntry(nil), tree...)
	mod[0] = entry("zzz", "README.md")
	mod[1] = entry("yyy", "docs/guide.md")
	feMod, _ = Frontend(fe, nil, mod)
	beMod, _ = Backend(be, fe.Path, nil, mod)
	if feMod != feBase || beMod != beBase {
		t.Fatal("excluded-file change affected a hash")
	}

	// Changing a backend blob changes be_hash but not fe_hash.
	mod = append([]gitrepo.TreeEntry(nil), tree...)
	mod[2] = entry("zzz", "main.go")
	feMod, _ = Frontend(fe, nil, mod)
	beMod, _ = Backend(be, fe.Path, nil, mod)
	if beMod == beBase {
		t.Fatal("backend change did not change be_hash")
	}
	if feMod != feBase {
		t.Fatal("backend change leaked into fe_hash")
	}
}

func TestArtifactPartition(t *testing.T) {
	art := manifest.Artifact{
		Path:  ".",
		Build: [][]string{{"go", "build", "-o", "bin/cli", "./cmd/cli"}},
		Files: []string{"bin/cli"},
	}
	base, err := Artifact(art, fe.Path, nil, tree)
	if err != nil {
		t.Fatal(err)
	}

	// The frontend subtree is subtracted, like the backend partition.
	mod := append([]gitrepo.TreeEntry(nil), tree...)
	mod[3] = entry("zzz", "web/index.html")
	if got, _ := Artifact(art, fe.Path, nil, mod); got != base {
		t.Fatal("frontend change leaked into the artifact hash")
	}

	// A change inside the partition rebuilds.
	mod = append([]gitrepo.TreeEntry(nil), tree...)
	mod[2] = entry("zzz", "main.go")
	if got, _ := Artifact(art, fe.Path, nil, mod); got == base {
		t.Fatal("partition change did not change the artifact hash")
	}

	// The manifest section feeds the hash.
	changed := art
	changed.Files = []string{"bin/other"}
	if got, _ := Artifact(changed, fe.Path, nil, tree); got == base {
		t.Fatal("section change did not change the artifact hash")
	}

	// An empty partition is an error.
	empty := art
	empty.Path = "missing"
	if _, err := Artifact(empty, fe.Path, nil, tree); err == nil {
		t.Fatal("empty artifact partition should be an error")
	}
}

func TestManifestSectionInHash(t *testing.T) {
	base, err := Frontend(fe, nil, tree)
	if err != nil {
		t.Fatal(err)
	}
	changed := fe
	changed.Build = [][]string{{"npm", "run", "build:prod"}}
	got, err := Frontend(changed, nil, tree)
	if err != nil {
		t.Fatal(err)
	}
	if got == base {
		t.Fatal("build-command change did not change the hash")
	}
}

// TestExtraEnvironmentInHash: the resolved-environment blob (devcontainer)
// feeds the digest when present; nil and empty leave it untouched, so repos
// without a devcontainer keep their pre-existing hashes.
func TestExtraEnvironmentInHash(t *testing.T) {
	base, err := Frontend(fe, nil, tree)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := Frontend(fe, []byte{}, tree); got != base {
		t.Fatal("empty extra changed the hash")
	}
	devc, _ := Frontend(fe, []byte(`{"image":"dev:1"}`), tree)
	if devc == base {
		t.Fatal("extra blob did not change the hash")
	}
	if bumped, _ := Frontend(fe, []byte(`{"image":"dev:2"}`), tree); bumped == devc {
		t.Fatal("extra blob change did not change the hash")
	}
}

func TestEmptyPartitionIsError(t *testing.T) {
	feBad := fe
	feBad.Path = "missing"
	if _, err := Frontend(feBad, nil, tree); err == nil {
		t.Fatal("empty frontend partition should be an error")
	}
	beBad := be
	beBad.Path = "web" // frontend subtracts everything
	if _, err := Backend(beBad, "web", nil, tree); err == nil {
		t.Fatal("empty backend partition should be an error")
	}
}
