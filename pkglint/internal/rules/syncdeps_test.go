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

const syncHeader = "pkgname=foo\npkgver=1\npkgrel=1\narch=(x86_64)\n"

func TestSyncDeps(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		run        RunCheck
		want       []string
	}{
		{
			name: "fields in makepkg's order, constraints cut, duplicates dropped",
			body: `depends=('glibc>=2.38' 'libfoo.so=1-64')
makedepends=(go 'jq<2' glibc)
checkdepends=('python-pytest')
check() { :; }`,
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
		{
			name: "a helper called in a subshell cannot change the lists",
			body: `_v() { makedepends=(evil); echo 1; }
_x=$(_v)
makedepends=(go)`,
			want: []string{"go"},
		},
		{name: "nothing to sync", body: `depends=()`, want: nil},
		{name: "checkdepends without check() are not needed", body: `checkdepends=(pytest)`, want: nil},
		{name: "options=(!check) skips them", body: "checkdepends=(pytest)\noptions=('!check')\ncheck() { :; }", want: nil},
		{name: "--check overrides !check", body: "checkdepends=(pytest)\noptions=('!check')\ncheck() { :; }", run: CheckForced, want: []string{"pytest"}},
		{name: "--nocheck skips them", body: "checkdepends=(pytest)\ncheck() { :; }", run: CheckSkipped, want: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pkg := loadPKGBUILD(t, syncHeader+tc.body+"\npackage() { :; }\n")
			got, err := SyncDeps(pkg, "x86_64", tc.run)
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
// error naming the problem, never a partial or guessed list.
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
		{"conditional referenced variable", "_py=python\n[[ -e /x ]] && _py=python2\nmakedepends=(\"$_py\")", "_py is assigned under top-level control flow"},
		{"referenced variable assigned later", "_py=python\nmakedepends=(\"$_py\")\n_py=ruby", "reads _py, which is assigned again after it"},
		{"transitively", "_a=x\n_b=\"$_a\"\nmakedepends=(\"$_b\")\n_a=y", "reads _a"},
		{"indirect", "_n=_py\n_py=python\nmakedepends=(\"${!_n}\")", "indirect reference"},
		{"eval", "makedepends=(go)\neval 'makedepends+=(jq)'", "runs eval"},
		{"source", "makedepends=(go)\nsource ./deps.sh", "runs source"},
		{"dot", "makedepends=(go)\n. ./deps.sh", "runs ."},
		{"mapfile", "mapfile -t makedepends < deps.txt", "runs mapfile"},
		{"readarray", "readarray makedepends < deps.txt", "runs readarray"},
		{"unset", "makedepends=(go jq)\nunset 'makedepends[1]'", "runs unset"},
		{"trap", "trap 'makedepends=(x)' DEBUG\nmakedepends=(go)", "runs trap"},
		{"builtin wrapper", "makedepends=(go)\nbuiltin eval 'makedepends=(x)'", "runs eval"},
		{"command wrapper", "makedepends=(go)\ncommand -p source x", "runs source"},
		{"dynamic command name", "_c=eval\nmakedepends=(go)\n$_c 'makedepends=(x)'", "not a literal"},
		{"function call", "_deps() { makedepends=(x); }\nmakedepends=(go)\n_deps", "own function _deps"},
		{"declare nameref", "declare -n _r=makedepends\nmakedepends=(go)\n_r=(x)", "nameref"},
		{"declare", "declare -a makedepends=(go)", "set by declare"},
		{"typeset", "makedepends=(go)\ntypeset -a depends=(x)", "set by typeset"},
		{"read", "makedepends=(go)\nread -ra makedepends <<< x", "handed the name makedepends"},
		{"printf -v", "depends=(go)\nprintf -vdepends x", "handed the name depends"},
		{"indexed write", "makedepends=(go)\nmakedepends[0]=x", "by index"},
		{"temporary assignment", "makedepends=(go)\nmakedepends=x true", "for a single command"},
		{"assigning expansion", "makedepends=(go)\n: \"${_x:=y}\"", "${…=…}"},
		{"arithmetic", "makedepends=(go)\n(( _i += 1 ))", "arithmetic assignment"},
		{"increment", "makedepends=(go)\n(( _i++ ))", "arithmetic assignment"},
		{"for loop variable", "for depends in a; do :; done", "for-loop variable"},
		{"subshell assignment", "makedepends=(go)\n( makedepends=(x) )", "in a subshell"},
		{"conditional check()", "checkdepends=(x)\nif true; then check() { :; }; fi", "check() is defined under"},
		{"dynamic options", "checkdepends=(x)\ncheck() { :; }\noptions=(\"$(echo '!check')\")", "options entry"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pkg := loadPKGBUILD(t, syncHeader+tc.body+"\npackage() { :; }\n")
			got, err := SyncDeps(pkg, "x86_64", CheckDefault)
			if err == nil {
				t.Fatalf("SyncDeps = %q, want an error", got)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q lacks %q", err, tc.want)
			}
		})
	}
}

func TestNameProblem(t *testing.T) {
	for _, name := range []string{"go", "lib32-glibc", "gtk+", "foo@bar", "python-foo_bar"} {
		if p := NameProblem(name); p != "" {
			t.Errorf("NameProblem(%q) = %q, want none", name, p)
		}
	}
	for _, name := range []string{"", "-oops", ".x", "a b", "ñ", "x;y"} {
		if NameProblem(name) == "" {
			t.Errorf("NameProblem(%q) found nothing", name)
		}
	}
}
