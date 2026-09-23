package rules_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
	"github.com/jmelahman/pkglint/internal/rules"
)

// benchCorpus returns the package directories the linting benchmarks run over.
// It defaults to the checked-in testdata packages so `go test -bench` works
// anywhere; point PKGLINT_BENCH_CORPUS at a directory of package directories
// (e.g. an AUR checkout) to measure against a realistic corpus instead.
func benchCorpus(b *testing.B) []string {
	root := os.Getenv("PKGLINT_BENCH_CORPUS")
	if root == "" {
		root = filepath.Join("..", "..", "testdata")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		b.Fatalf("reading corpus %s: %v", root, err)
	}
	var dirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if _, err := os.Stat(filepath.Join(dir, "PKGBUILD")); err != nil {
			continue
		}
		dirs = append(dirs, dir)
	}
	if len(dirs) == 0 {
		b.Fatalf("no package directories under %s", root)
	}
	return dirs
}

func loadCorpus(b *testing.B) []*pkgbuild.Package {
	dirs := benchCorpus(b)
	pkgs := make([]*pkgbuild.Package, 0, len(dirs))
	for _, dir := range dirs {
		pkg, err := pkgbuild.Load(dir)
		if err != nil {
			continue // an unparseable package is a report, not a benchmark input
		}
		pkgs = append(pkgs, pkg)
	}
	if len(pkgs) == 0 {
		b.Fatal("no loadable packages in corpus")
	}
	return pkgs
}

// BenchmarkLoad measures parsing alone: reading the PKGBUILD and scriptlets
// and building the AST.
func BenchmarkLoad(b *testing.B) {
	dirs := benchCorpus(b)
	b.ReportAllocs()
	for b.Loop() {
		for _, dir := range dirs {
			if _, err := pkgbuild.Load(dir); err != nil {
				b.Fatal(err)
			}
		}
	}
}

// BenchmarkRun measures rule evaluation over already-parsed packages, which is
// where the linter spends its time on a warm corpus.
func BenchmarkRun(b *testing.B) {
	pkgs := loadCorpus(b)
	ignore := map[string]bool{}
	b.ReportAllocs()
	for b.Loop() {
		for _, pkg := range pkgs {
			rules.Run(pkg, ignore)
		}
	}
}

// BenchmarkNewContext isolates the shared precomputation Run does before any
// rule sees the package.
func BenchmarkNewContext(b *testing.B) {
	pkgs := loadCorpus(b)
	b.ReportAllocs()
	for b.Loop() {
		for _, pkg := range pkgs {
			rules.NewContext(pkg)
		}
	}
}

// BenchmarkRegistry measures the public Registry accessor. The registry is
// built once and shared, so steady-state this is the cost of cloning it.
func BenchmarkRegistry(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		rules.Registry()
	}
}

// BenchmarkLoadAndRun is the end-to-end per-package cost: parse, then lint.
func BenchmarkLoadAndRun(b *testing.B) {
	dirs := benchCorpus(b)
	ignore := map[string]bool{}
	b.ReportAllocs()
	for b.Loop() {
		for _, dir := range dirs {
			pkg, err := pkgbuild.Load(dir)
			if err != nil {
				continue
			}
			rules.Run(pkg, ignore)
		}
	}
}
