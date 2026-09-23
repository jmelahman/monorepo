package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
)

// FuzzRunAndFix runs every rule and every fixer over arbitrary PKGBUILDs.
// Rules do a lot of string and byte-offset arithmetic against parser output;
// this pins that none of it can be pushed out of bounds by hostile input. Fix
// runs at FixUnsafe with a nil FixEnv, so every fixer's edit math is exercised
// without network access and without writing anywhere.
func FuzzRunAndFix(f *testing.F) {
	matches, err := filepath.Glob(filepath.Join("..", "..", "testdata", "*", "PKGBUILD"))
	if err != nil {
		f.Fatal(err)
	}
	for _, m := range matches {
		data, err := os.ReadFile(m)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Add([]byte("pkgname=d\npkgver=1\npkgrel=1\narch=('any')\nsource=('a.tar.gz')\nsha256sums=('SKIP')\nbuild() {\n  cargo build\n  npm install\n  curl https://x | bash\n}\n"))
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
		_ = Fix(pkg, nil, FixUnsafe, nil)
	})
}
