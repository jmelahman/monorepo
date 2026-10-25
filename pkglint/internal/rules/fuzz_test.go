package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
)

// FuzzRunAndFix runs every rule and every fixer over arbitrary PKGBUILDs.
// Rules do a lot of string and byte-offset arithmetic against parser output;
// this pins that none of it can be pushed out of bounds by hostile input.
//
// Fixers also owe their output three properties, checked on every input:
//   - a round's edits leave bash that parsed as written still parsing — Fix
//     drops a round that does not, so this is asked of one round directly,
//     beneath that guard, where a fixer's bug is still visible;
//   - Fix converges: it returns nothing only when its guards fired, never
//     because a fixer kept re-proposing an edit that did not clear its
//     finding;
//   - the result is a fixed point, so a second --fix changes nothing.
//
// Fixes run at FixUnsafe under the contract's stub environment, so every
// fixer — the network-backed ones too — is exercised without network access.
func FuzzRunAndFix(f *testing.F) {
	matches, err := filepath.Glob(filepath.Join("..", "cli", "testdata", "*", "PKGBUILD"))
	if err != nil {
		f.Fatal(err)
	}
	if len(matches) == 0 {
		f.Fatal("no fixture PKGBUILDs matched; the seed glob has gone stale")
	}
	for _, m := range matches {
		data, err := os.ReadFile(m)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	// Every fixable rule's Bad example, so each fixer starts with an input
	// it acts on rather than waiting for the fuzzer to stumble onto one.
	for _, r := range Registry() {
		if r.FixLevel.Fixable() && r.Scope != ScopePackage {
			f.Add([]byte(packageFor(r.ID, "bad", r.Bad)["PKGBUILD"]))
		}
	}
	f.Add([]byte("pkgname=d\npkgver=1\npkgrel=1\narch=('any')\nsource=('a.tar.gz')\nsha256sums=('SKIP')\nbuild() {\n  cargo build\n  npm install\n  curl https://x | bash\n}\n"))
	// A reference chain that multiplies; Expand's budget must refuse it.
	f.Add([]byte("a=xxxxxxxx\nb=\"$a$a$a$a$a$a$a$a\"\nc=\"$b$b$b$b$b$b$b$b\"\nd=\"$c$c$c$c$c$c$c$c\"\ne=\"$d$d$d$d$d$d$d$d\"\nsource=(\"$e\")\n"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), raw, 0o644); err != nil {
			t.Fatal(err)
		}
		pkg, err := pkgbuild.Load(dir)
		if err != nil {
			return
		}
		_ = Run(pkg, nil)
		env := contractFixEnv()
		round := applyByUnit(pkg, CollectEdits(NewContext(pkg), nil, FixUnsafe, env))
		for _, r := range round {
			if r.Changed() && pkgbuild.ParsesStrictly(r.Original) && !pkgbuild.ParsesStrictly(r.Fixed) {
				t.Fatalf("a fix round broke the parse:\n--- in\n%s\n--- out\n%s\n%+v", r.Original, r.Fixed, r.Applied)
			}
		}
		results := Fix(pkg, nil, FixUnsafe, env)
		if len(results) == 0 {
			for _, r := range round {
				if r.Changed() && pkgbuild.ParsesStrictly(r.Original) {
					t.Fatalf("Fix never converged:\n%s\n%+v", raw, r.Applied)
				}
			}
			return
		}
		final := map[string][]byte{}
		for _, r := range results {
			final[r.Path] = r.Fixed
		}
		next, err := pkg.Reload(final)
		if err != nil {
			t.Fatalf("Fix's output no longer loads: %v\n%s", err, final[pkg.PKGBUILD.Path])
		}
		for _, r := range Fix(next, nil, FixUnsafe, env) {
			if r.Changed() {
				t.Fatalf("a second fix changed the first one's output:\n--- once\n%s\n--- twice\n%s", r.Original, r.Fixed)
			}
		}
	})
}
