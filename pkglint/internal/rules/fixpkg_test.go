package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmelahman/pkglint/internal/alpmdb"
	"github.com/jmelahman/pkglint/internal/pkgbuild"
	"github.com/jmelahman/pkglint/internal/pkgfile"
	"github.com/jmelahman/pkglint/internal/pkgfile/pkgtest"
)

// Tests for the package-scope fixers: the ones that read a built archive and
// write the PKGBUILD that produced it. Everything here goes through
// FixPackage, because the pairing it takes is the thing under test as much as
// the edit is.

// fixBuilt runs the package-scope fixers over one PKGBUILD paired with one
// archive and returns the rewritten PKGBUILD, or "" when nothing was fixed.
func fixBuilt(t *testing.T, db *alpmdb.DB, pkgbuildSrc, info string, members ...pkgtest.Member) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(pkgbuildSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	pkg, err := pkgbuild.Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	built, err := pkgfile.Read(strings.NewReader(string(pkgtest.Tar(info, members...))), "demo-1.0-1.pkg.tar")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range FixPackage(pkg, built, db, nil, FixSafe, nil) {
		if r.Changed() {
			return string(r.Fixed)
		}
	}
	return ""
}

// pngBin links libpng16.so.16, which pkgDB's libpng owns.
func pngBin() []byte {
	return pkgtest.ELF(hardened(pkgtest.ELFOpts{
		Needed: []string{"libpng16.so.16"}, Undefined: []string{"png_x"},
	}))
}

// builtPKGBUILD is a single-package PKGBUILD whose depends line the tests vary.
func builtPKGBUILD(name, depends string) string {
	return "pkgname=" + name + `
pkgver=1.0.0
pkgrel=1
arch=('x86_64')
url='https://example.com/demo'
license=('MIT')
` + depends + `
source=("https://example.com/demo-$pkgver.tar.gz")
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')

package() {
  install -Dm755 demo "$pkgdir/usr/bin/demo"
}
`
}

func TestFixMissingLibraryDependency(t *testing.T) {
	db := pkgDB(t)
	bin := pkgtest.Member{Name: "usr/bin/demo", Data: pngBin(), Mode: 0o755}

	// The error case: glibc is declared (so the closure is complete and
	// checkable), libpng is not, and the binary links its library.
	got := fixBuilt(t, db, builtPKGBUILD("demo", "depends=('glibc')"),
		pkgtest.Info("demo", "x86_64", "depend = glibc"), bin)
	if !strings.Contains(got, "depends=('glibc' 'libpng')") {
		t.Errorf("missing dependency: want libpng added, got:\n%s", got)
	}

	// No depends array at all: one is written.
	got = fixBuilt(t, db, builtPKGBUILD("demo", ""), pkgtest.Info("demo", "x86_64"), bin)
	if !strings.Contains(got, "depends=('libpng')") {
		t.Errorf("absent array: want a depends array, got:\n%s", got)
	}
}

func TestFixMissingLibraryDependencyDeclines(t *testing.T) {
	db := pkgDB(t)
	bin := pkgtest.Member{Name: "usr/bin/demo", Data: pngBin(), Mode: 0o755}
	pkginfo := pkgtest.Info("demo", "x86_64", "depend = glibc")

	for _, tc := range []struct {
		name, pkgbuild, info string
		db                   *alpmdb.DB
		why                  string
	}{
		{
			name: "no database", pkgbuild: builtPKGBUILD("demo", "depends=('glibc')"), info: pkginfo, db: nil,
			why: "nothing can resolve a soname to an owner",
		},
		{
			name:     "split package",
			pkgbuild: builtPKGBUILD("(demo demo-extra)", "depends=('glibc')"), info: pkginfo, db: db,
			why: "per-package depends live in each package_<name>()",
		},
		{
			name:     "archive is not this package",
			pkgbuild: builtPKGBUILD("other", "depends=('glibc')"), info: pkginfo, db: db,
			why: "the pair does not match",
		},
		{
			name:     "debug split member",
			pkgbuild: builtPKGBUILD("demo", "depends=('glibc')"), db: db,
			info: pkgtest.Info("demo-debug", "x86_64", "depend = glibc", "pkgtype = debug"),
			why:  "the -debug member's depends are makepkg's, not the maintainer's",
		},
		{
			name: "incomplete closure",
			// not-installed-here is declared but absent from the database, so
			// the rule itself softens to informational and the fix stands down.
			pkgbuild: builtPKGBUILD("demo", "depends=('not-installed-here')"), db: db,
			info: pkgtest.Info("demo", "x86_64", "depend = not-installed-here"),
			why:  "the declared closure could not be walked",
		},
		{
			name: "pkgname set by control flow",
			// makepkg runs the `case` before it reads the metadata, so pkgname
			// is set as far as it is concerned and pkgnames() sees nothing.
			pkgbuild: "case \"$CARCH\" in x86_64) pkgname=demo ;; esac\n" +
				builtPKGBUILD("demo", "depends=('glibc')"),
			info: pkginfo, db: db,
			why: "which name the file builds is not knowable without running it",
		},
		{
			name: "package() assigns depends",
			pkgbuild: builtPKGBUILD("demo", "depends=('glibc')") +
				"\npackage() {\n  depends=('glibc')\n}\n",
			info: pkginfo, db: db,
			why: "a top-level array would be overwritten by the function's",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := fixBuilt(t, tc.db, tc.pkgbuild, tc.info, bin); got != "" {
				t.Errorf("want no fix (%s), got:\n%s", tc.why, got)
			}
		})
	}
}

// The transitive and optdepends gaps are real findings the fix deliberately
// leaves to the maintainer: which way to resolve them is a judgement.
func TestFixMissingLibraryDependencyOnlyFixesErrors(t *testing.T) {
	db := pkgDB(t)
	bin := pkgtest.Member{Name: "usr/bin/demo", Data: pngBin(), Mode: 0o755}
	for _, tc := range []struct{ name, depend string }{
		{"transitive", "depend = middle"},
		{"optdepends only", "optdepend = libpng: png export"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The PKGBUILD mirrors the archive's metadata so the rule sees the
			// same coverage the fixer does.
			decl := "depends=('middle')"
			if strings.HasPrefix(tc.depend, "optdepend") {
				decl = "optdepends=('libpng: png export')"
			}
			if got := fixBuilt(t, db, builtPKGBUILD("demo", decl),
				pkgtest.Info("demo", "x86_64", tc.depend), bin); got != "" {
				t.Errorf("want no fix for a %s gap, got:\n%s", tc.name, got)
			}
		})
	}
}

// A directive on the array the fix writes declines it, the way one on a
// finding's own line declines a PKGBUILD-scope fix. The finding itself is
// anchored inside the archive, where no PKGBUILD line could carry a directive.
func TestFixMissingLibraryDependencySuppressed(t *testing.T) {
	src := builtPKGBUILD("demo", "depends=('glibc')  # pkglint: ignore=PB809")
	got := fixBuilt(t, pkgDB(t), src, pkgtest.Info("demo", "x86_64", "depend = glibc"),
		pkgtest.Member{Name: "usr/bin/demo", Data: pngBin(), Mode: 0o755})
	if got != "" {
		t.Errorf("want the directive to decline the fix, got:\n%s", got)
	}
}

// FixPackage collects only package-scope fixers, and Fix only PKGBUILD-scope
// ones: the two commands that call them repair different things.
func TestFixScopesDoNotOverlap(t *testing.T) {
	for _, r := range registry() {
		if r.Fix == nil {
			continue
		}
		if r.Scope == ScopePackage && r.FixCommand() != "build --"+strings.TrimPrefix(r.FixLevel.Flag(), "--") {
			t.Errorf("%s: package-scope fix advertises %q", r.ID, r.FixCommand())
		}
		if r.Scope == ScopePKGBUILD && r.FixCommand() != r.FixLevel.Flag() {
			t.Errorf("%s: PKGBUILD-scope fix advertises %q", r.ID, r.FixCommand())
		}
	}
}
