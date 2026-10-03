package build

import (
	"testing"

	"github.com/jmelahman/local-preview/internal/manifest"
)

// A project-scoped manifest with only [artifacts] must stay artifacts-only:
// scoping once allocated an empty backend.exclude, which made [backend]
// non-zero and sent the build off to hash a frontend with no path.
func TestScopeManifestKeepsArtifactsOnly(t *testing.T) {
	m, err := manifest.ParseAt([]byte(`
[previews.artifacts.apk]
path    = "."
exclude = ["desktop/", "*.md"]
build   = [["true"]]
files   = ["app.apk"]
`), "previews")
	if err != nil {
		t.Fatal(err)
	}
	scoped := scopeManifest(m, "5-wild")
	if !scoped.ArtifactsOnly() {
		t.Fatalf("scoped manifest is no longer artifacts-only: frontend=%+v backend=%+v", scoped.Frontend, scoped.Backend)
	}
	a := scoped.Artifacts["apk"]
	if a.Path != "5-wild" {
		t.Errorf("artifact path = %q, want %q", a.Path, "5-wild")
	}
	if want := []string{"5-wild/desktop/", "*.md"}; len(a.Exclude) != 2 || a.Exclude[0] != want[0] || a.Exclude[1] != want[1] {
		t.Errorf("artifact exclude = %q, want %q", a.Exclude, want)
	}
}
