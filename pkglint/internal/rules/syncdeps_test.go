package rules

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
)

func loadPKGBUILD(t *testing.T, src string) *pkgbuild.Package {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	pkg, err := pkgbuild.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return pkg
}

func TestSyncDeps(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       []string
	}{
		{
			name: "fields in makepkg's order, constraints cut, duplicates dropped",
			body: `depends=('glibc>=2.38' 'libfoo.so=1-64')
makedepends=(go 'jq<2' glibc)
checkdepends=('python-pytest')`,
			want: []string{"glibc", "libfoo.so", "go", "jq", "python-pytest"},
		},
		{
			name: "the carch variant is merged, other arches are not",
			body: `arch=(x86_64 aarch64)
makedepends=(cmake)
makedepends_x86_64=(nasm)
makedepends_aarch64=(armstuff)
depends_x86_64=('lib32-glibc')`,
			want: []string{"lib32-glibc", "cmake", "nasm"},
		},
		{
			name: "split packages' own depends are not synced",
			body: `pkgname=(foo-core foo-cli)
makedepends=(meson)
package_foo-core() { depends=(zlib); }
package_foo-cli() { depends=(foo-core); }`,
			want: []string{"meson"},
		},
		{
			name: "static variables and braces are expanded",
			body: `_py=python
makedepends=("${_py}-build" ${_py}-{wheel,installer})`,
			want: []string{"python-build", "python-wheel", "python-installer"},
		},
		{name: "nothing to sync", body: `depends=()`, want: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pkg := loadPKGBUILD(t, "pkgname=foo\npkgver=1\npkgrel=1\narch=(x86_64)\n"+tc.body+"\npackage() { :; }\n")
			got, err := SyncDeps(pkg, "x86_64")
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("SyncDeps = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestSyncDepsRefusesDynamic pins that a list the parse cannot know is an
// error naming the field, never a partial list.
func TestSyncDepsRefusesDynamic(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
	}{
		{"command substitution", `makedepends=("$(echo evil)")`, "makedepends entry"},
		{"unset variable", `depends=("$_unknown")`, "depends entry"},
		{"array reference", `makedepends=("${_unset[@]}" c)`, "makedepends holds"},
		{"word splitting", `_list="a b"; depends=($_list)`, "contains whitespace"},
		{"control flow", "if true; then\n  checkdepends=(x)\nfi", "checkdepends is assigned under top-level control flow"},
		{"carch variant", `makedepends_x86_64=("$(uname -m)")`, "makedepends_x86_64 entry"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pkg := loadPKGBUILD(t, "pkgname=foo\npkgver=1\npkgrel=1\narch=(x86_64)\n"+tc.body+"\npackage() { :; }\n")
			got, err := SyncDeps(pkg, "x86_64")
			if err == nil {
				t.Fatalf("SyncDeps = %q, want an error", got)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q lacks %q", err, tc.want)
			}
		})
	}
}
