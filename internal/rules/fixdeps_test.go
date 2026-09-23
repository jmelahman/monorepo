package rules

import (
	"strings"
	"testing"
)

// Tests for the dependency-array fixers in fixdeps.go. Every case runs one
// rule's fixer alone: these fixtures are minimal PKGBUILDs that trip half a
// dozen rules at once, and a test asserting that nothing was rewritten has to
// say which rule it means.

const (
	tarballSource = `source=("https://example.com/demo-$pkgver.tar.gz")
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')`

	gitSource = `source=("demo::git+https://example.com/demo.git")
sha256sums=('SKIP')`
)

// depsFixture assembles a PKGBUILD from the parts these tests vary: the
// package name, the metadata lines that follow license — where a created array
// lands, so what is written next to them is what the assertions read — the
// source array, and the body. An empty src takes the plain tarball.
func depsFixture(name, decls, src, body string) string {
	if src == "" {
		src = tarballSource
	}
	header := "pkgname=" + name + `
pkgver=1.0.0
pkgrel=1
arch=('x86_64')
url='https://example.com/demo'
license=('MIT')`
	if decls != "" {
		header += "\n" + strings.TrimSuffix(decls, "\n")
	}
	return pkgbuildWith(header+"\n"+src, body)
}

// fixDeps runs one rule's fixer over a fixture and returns the rewritten file,
// or "" when the fixer declined.
func fixDeps(t *testing.T, id, src string, level FixLevel) string {
	t.Helper()
	return fixOnly(t, map[string]string{"PKGBUILD": src}, level, nil, id)["PKGBUILD"]
}

func TestFixToolMakedepends(t *testing.T) {
	// The four rules share one fixer; what differs is the tool each watches
	// for and the package that closes the gap.
	tools := []struct{ id, body, want string }{
		{"PB944", "build() {\n  cargo build --release\n}", "makedepends=('rust')"},
		{"PB952", "build() {\n  cmake -B build -S .\n}", "makedepends=('cmake')"},
		{"PB954", "build() {\n  meson setup build\n}", "makedepends=('meson')"},
		{"PB979", "build() {\n  npm ci\n}", "makedepends=('npm')"},
	}
	for _, c := range tools {
		t.Run(c.id+" declares the tool it runs", func(t *testing.T) {
			got := fixDeps(t, c.id, depsFixture("demo", "", "", c.body), FixSafe)
			// Created under the last metadata anchor, not above the
			// Maintainer comment that offset zero would put it over.
			mustContain(t, got, "license=('MIT')\n"+c.want+"\n")
			mustClear(t, c.id, got)
		})
	}

	t.Run("appends to an existing array", func(t *testing.T) {
		got := fixDeps(t, "PB952", depsFixture("demo", "makedepends=('git')", "",
			"build() {\n  cmake -B build -S .\n}"), FixSafe)
		mustContain(t, got, "makedepends=('git' 'cmake')")
		mustClear(t, "PB952", got)
	})

	t.Run("matches the quoting the array already uses", func(t *testing.T) {
		got := fixDeps(t, "PB952", depsFixture("demo", `makedepends=("git")`, "",
			"build() {\n  cmake -B build -S .\n}"), FixSafe)
		mustContain(t, got, `makedepends=("git" "cmake")`)
	})

	t.Run("one element per line stays one element per line", func(t *testing.T) {
		got := fixDeps(t, "PB952", depsFixture("demo", "makedepends=(\n  'git'\n)", "",
			"build() {\n  cmake -B build -S .\n}"), FixSafe)
		mustContain(t, got, "  'git'\n  'cmake'\n)")
		mustClear(t, "PB952", got)
	})

	t.Run("fills an empty array", func(t *testing.T) {
		got := fixDeps(t, "PB952", depsFixture("demo", "makedepends=()", "",
			"build() {\n  cmake -B build -S .\n}"), FixSafe)
		mustContain(t, got, "makedepends=('cmake')")
		mustClear(t, "PB952", got)
	})

	t.Run("creates an array under an anchor on the last line", func(t *testing.T) {
		// No trailing newline, and the anchor is the final assignment: the
		// edit has no line ending to claim, so it claims the anchor's last
		// byte and writes it back.
		src := `pkgname=demo
pkgver=1.0.0
pkgrel=1
arch=('x86_64')
url='https://example.com/demo'
` + tarballSource + `
build() {
  cmake -B build -S .
}
license=('MIT')`
		got := fixDeps(t, "PB952", src, FixSafe)
		mustContain(t, got, "license=('MIT')\nmakedepends=('cmake')")
		mustClear(t, "PB952", got)
	})

	t.Run("declines an array a later += extends", func(t *testing.T) {
		// The literal is not the whole array, so appending inside it would
		// write the entry somewhere other than where bash ends up.
		src := depsFixture("demo", "makedepends=('ninja')\nmakedepends+=('pkgconf')", "",
			"build() {\n  cmake -B build -S .\n}")
		expectRule(t, "PB952", map[string]string{"PKGBUILD": src})
		if got := fixDeps(t, "PB952", src, FixSafe); got != "" {
			t.Errorf("expected no rewrite of a += merged array, got:\n%s", got)
		}
	})

	t.Run("declines an array assigned by control flow", func(t *testing.T) {
		src := depsFixture("demo",
			"if [[ \"$CARCH\" == 'x86_64' ]]; then\n  makedepends=('ninja')\nfi", "",
			"build() {\n  cmake -B build -S .\n}")
		expectRule(t, "PB952", map[string]string{"PKGBUILD": src})
		if got := fixDeps(t, "PB952", src, FixSafe); got != "" {
			t.Errorf("expected no rewrite where the array is assigned conditionally, got:\n%s", got)
		}
	})
}

func TestFixVCSMakedepends(t *testing.T) {
	t.Run("declares the client the source needs", func(t *testing.T) {
		got := fixDeps(t, "PB711", depsFixture("demo-git", "", gitSource, ""), FixSafe)
		mustContain(t, got, "makedepends=('git')")
		mustClear(t, "PB711", got)
	})

	t.Run("one edit covers every client", func(t *testing.T) {
		src := depsFixture("demo-git", "", `source=("demo::git+https://example.com/demo.git"
        "docs::hg+https://example.com/docs")
sha256sums=('SKIP' 'SKIP')`, "")
		got := fixDeps(t, "PB711", src, FixSafe)
		mustContain(t, got, "makedepends=('git' 'mercurial')")
		mustClear(t, "PB711", got)
	})
}

func TestFixPythonBuildBackend(t *testing.T) {
	got := fixDeps(t, "PB933", depsFixture("python-demo", "", "", `
build() {
  python -m build --wheel --no-isolation
}
package() {
  python -m installer --destdir="$pkgdir" dist/demo.whl
}`), FixSafe)
	mustContain(t, got, "makedepends=('python-build' 'python-installer')")
	mustClear(t, "PB933", got)
}

func TestFixDkmsDepends(t *testing.T) {
	t.Run("declares dkms", func(t *testing.T) {
		got := fixDeps(t, "PB973", depsFixture("demo-dkms", "", "", ""), FixUnsafe)
		mustContain(t, got, "depends=('dkms')")
		mustClear(t, "PB973", got)
	})

	t.Run("appends beside the dependencies already declared", func(t *testing.T) {
		got := fixDeps(t, "PB973", depsFixture("demo-dkms", "depends=('libfoo')", "", ""), FixUnsafe)
		mustContain(t, got, "depends=('libfoo' 'dkms')")
		mustClear(t, "PB973", got)
	})

	t.Run("is not a safe-level fix", func(t *testing.T) {
		// It adds to what gets installed on a user's system, which --fix
		// alone does not do.
		if got := fixDeps(t, "PB973", depsFixture("demo-dkms", "", "", ""), FixSafe); got != "" {
			t.Errorf("expected --fix to leave a runtime dependency alone, got:\n%s", got)
		}
	})
}

func TestFixJavaRuntimeDependency(t *testing.T) {
	got := fixDeps(t, "PB981", depsFixture("demo", "", "", `
package() {
  install -Dm644 demo.jar "$pkgdir/usr/share/java/demo/demo.jar"
}`), FixUnsafe)
	mustContain(t, got, "depends=('java-runtime')")
	mustClear(t, "PB981", got)
}

func TestFixVCSProvidesConflicts(t *testing.T) {
	t.Run("declares both halves in one assignment block", func(t *testing.T) {
		got := fixDeps(t, "PB961", depsFixture("demo-git", "", gitSource, ""), FixUnsafe)
		mustContain(t, got, "provides=('demo')\nconflicts=('demo')")
		if n := countSubstring(got, "provides="); n != 1 {
			t.Errorf("want exactly one provides assignment, got %d:\n%s", n, got)
		}
		mustClear(t, "PB961", got)
	})

	t.Run("appends to the half that exists and creates the other", func(t *testing.T) {
		got := fixDeps(t, "PB961", depsFixture("demo-git", "provides=('demo-bin')", gitSource, ""), FixUnsafe)
		mustContain(t, got, "provides=('demo-bin' 'demo')")
		mustContain(t, got, "conflicts=('demo')")
		mustClear(t, "PB961", got)
	})

	t.Run("writes neither half when one cannot be written", func(t *testing.T) {
		// Half the pair is worse than the finding: provides without conflicts
		// lets both packages install at once.
		src := depsFixture("demo-git", "provides=('demo-bin')\nprovides+=('demo-alt')", gitSource, "")
		expectRule(t, "PB961", map[string]string{"PKGBUILD": src})
		if got := fixDeps(t, "PB961", src, FixUnsafe); got != "" {
			t.Errorf("expected no rewrite when provides cannot be appended to, got:\n%s", got)
		}
	})
}

func TestFixDeadDepEntries(t *testing.T) {
	// The safe removals: each entry the rule calls dead is deleted, and the
	// rest of the array is left spaced as it was written.
	cases := []struct {
		id, name, decls, want, absent string
	}{
		{
			id: "PB904", name: "demo",
			decls: "depends=('libfoo')\nmakedepends=('libfoo' 'cmake')",
			want:  "makedepends=('cmake')",
		},
		{
			id: "PB912", name: "demo",
			decls: "depends=('libfoo')\noptdepends=('libfoo: extras' 'libbar: more')",
			want:  "optdepends=('libbar: more')",
		},
		{
			id: "PB918", name: "demo",
			decls: "provides=('demo' 'demo-extra')",
			want:  "provides=('demo-extra')",
		},
		{
			id: "PB919", name: "demo",
			decls: "conflicts=('demo' 'demo-old')",
			want:  "conflicts=('demo-old')",
		},
	}
	for _, c := range cases {
		t.Run(c.id+" drops the entry", func(t *testing.T) {
			got := fixDeps(t, c.id, depsFixture(c.name, c.decls, "", ""), FixSafe)
			mustContain(t, got, c.want)
			if c.absent != "" {
				mustNotContain(t, got, c.absent)
			}
			mustClear(t, c.id, got)
		})
	}

	t.Run("an emptied array goes with its last entry", func(t *testing.T) {
		// `conflicts=()` is valid bash, but a rule that called the entry dead
		// metadata did not ask for an empty array in its place.
		got := fixDeps(t, "PB919", depsFixture("demo", "conflicts=('demo')", "", ""), FixSafe)
		mustNotContain(t, got, "conflicts")
		mustContain(t, got, "license=('MIT')\nsource=")
		mustClear(t, "PB919", got)
	})

	t.Run("an entry on its own line takes the line", func(t *testing.T) {
		got := fixDeps(t, "PB918", depsFixture("demo",
			"provides=(\n  'demo'\n  'demo-extra'\n)", "", ""), FixSafe)
		mustContain(t, got, "provides=(\n  'demo-extra'\n)")
		mustClear(t, "PB918", got)
	})

	t.Run("declines a brace group standing for more names", func(t *testing.T) {
		// One word, two entries: deleting it would drop demo-extra too.
		src := depsFixture("demo", "provides=({demo,demo-extra})", "", "")
		expectRule(t, "PB918", map[string]string{"PKGBUILD": src})
		if got := fixDeps(t, "PB918", src, FixSafe); got != "" {
			t.Errorf("expected no rewrite of a brace group, got:\n%s", got)
		}
	})

	t.Run("declines an array a later += extends", func(t *testing.T) {
		src := depsFixture("demo", "provides=('demo')\nprovides+=('demo-extra')", "", "")
		expectRule(t, "PB918", map[string]string{"PKGBUILD": src})
		if got := fixDeps(t, "PB918", src, FixSafe); got != "" {
			t.Errorf("expected no rewrite of a += merged array, got:\n%s", got)
		}
	})
}

func TestFixFontDepends(t *testing.T) {
	got := fixDeps(t, "PB971", depsFixture("ttf-demo", "depends=('fontconfig' 'freetype2')", "", ""), FixUnsafe)
	mustNotContain(t, got, "depends")
	mustClear(t, "PB971", got)
}

func TestFixDkmsKernelHeaders(t *testing.T) {
	t.Run("drops the pinned headers", func(t *testing.T) {
		got := fixDeps(t, "PB974", depsFixture("demo-dkms", "depends=('dkms' 'linux-headers')", "", ""), FixUnsafe)
		mustContain(t, got, "depends=('dkms')")
		mustClear(t, "PB974", got)
	})

	t.Run("leaves glibc's userspace headers alone", func(t *testing.T) {
		// linux-api-headers is not a kernel headers package, so there is
		// nothing to report and nothing to delete.
		src := depsFixture("demo-dkms", "depends=('dkms' 'linux-api-headers')", "", "")
		expectNoRule(t, "PB974", map[string]string{"PKGBUILD": src})
		if got := fixDeps(t, "PB974", src, FixUnsafe); got != "" {
			t.Errorf("expected linux-api-headers to be left in place, got:\n%s", got)
		}
	})
}

func TestFixPythonLintCheckdepends(t *testing.T) {
	t.Run("drops the plugin", func(t *testing.T) {
		got := fixDeps(t, "PB931", depsFixture("python-demo",
			"checkdepends=('python-pytest' 'python-pytest-cov')", "", ""), FixUnsafe)
		mustContain(t, got, "checkdepends=('python-pytest')")
		mustClear(t, "PB931", got)
	})

	t.Run("keeps a plugin a check command still asks for", func(t *testing.T) {
		// PB930's trap one rule over: dropping the dependency while check()
		// still passes --cov leaves a check phase that cannot run.
		src := depsFixture("python-demo", "checkdepends=('python-pytest-cov')", "", `
check() {
  pytest --cov=demo
}`)
		expectRule(t, "PB931", map[string]string{"PKGBUILD": src})
		if got := fixDeps(t, "PB931", src, FixUnsafe); got != "" {
			t.Errorf("expected the fix to stand down while the flag is passed, got:\n%s", got)
		}
	})

	t.Run("drops a plugin with no flag to pass", func(t *testing.T) {
		// python-pytest-runner is a setup.py plugin: no command line can be
		// depending on it.
		got := fixDeps(t, "PB931", depsFixture("python-demo",
			"checkdepends=('python-pytest-runner')", "", ""), FixUnsafe)
		mustNotContain(t, got, "checkdepends")
		mustClear(t, "PB931", got)
	})
}

// These fixers write their edit somewhere other than where their finding
// points — the fix for a cargo invocation is a makedepends line — so honoring
// an ignore directive takes more than the central per-edit line check: the
// fixer answers to the directive at the finding's own site.
func TestFixDepsSuppression(t *testing.T) {
	t.Run("a directive at the command blocks the makedepends add", func(t *testing.T) {
		src := depsFixture("demo", "", "", `
build() {
  # pkglint: ignore=PB944
  cargo build --release
}`)
		expectNoRule(t, "PB944", map[string]string{"PKGBUILD": src})
		if got := fixDeps(t, "PB944", src, FixSafe); got != "" {
			t.Errorf("a suppressed finding must not be fixed, got:\n%s", got)
		}
	})

	t.Run("a directive at one command leaves the other's package", func(t *testing.T) {
		// One edit speaks for several findings, and they can be waived one at
		// a time: only the backend whose finding stands is declared.
		src := depsFixture("python-demo", "", "", `
build() {
  # pkglint: ignore=PB933
  python -m build --wheel --no-isolation
}
package() {
  python -m installer --destdir="$pkgdir" dist/demo.whl
}`)
		got := fixDeps(t, "PB933", src, FixSafe)
		mustContain(t, got, "makedepends=('python-installer')")
		mustNotContain(t, got, "python-build")
	})

	t.Run("a directive on one entry keeps that entry", func(t *testing.T) {
		// The other entry still goes, and the array with it may not: a waived
		// entry must survive even when every entry of the array was flagged.
		src := depsFixture("ttf-demo", "depends=(\n  'fontconfig'\n  # pkglint: ignore=PB971\n  'freetype2'\n)", "", "")
		got := fixDeps(t, "PB971", src, FixUnsafe)
		mustNotContain(t, got, "'fontconfig'")
		mustContain(t, got, "'freetype2'")
	})

	t.Run("a directive at the pkgname finding blocks the pair", func(t *testing.T) {
		src := "# pkglint: ignore=PB961\n" + depsFixture("demo-git", "", gitSource, "")
		expectNoRule(t, "PB961", map[string]string{"PKGBUILD": src})
		if got := fixDeps(t, "PB961", src, FixUnsafe); got != "" {
			t.Errorf("a suppressed finding must not be fixed, got:\n%s", got)
		}
	})
}

func TestMergedArrayCreations(t *testing.T) {
	t.Run("two rules creating one array write one assignment", func(t *testing.T) {
		src := depsFixture("demo", "", "", `
build() {
  cargo build --release
  npm ci
}`)
		got := fixOnly(t, map[string]string{"PKGBUILD": src}, FixSafe, nil, "PB944", "PB979")["PKGBUILD"]
		if n := countSubstring(got, "makedepends="); n != 1 {
			t.Errorf("want exactly one makedepends assignment, got %d:\n%s", n, got)
		}
		mustContain(t, got, "makedepends=('rust' 'npm')")
		mustClear(t, "PB944", got)
		mustClear(t, "PB979", got)
	})

	t.Run("both rules are named on the merged edit", func(t *testing.T) {
		edits := []Edit{
			{Path: "PKGBUILD", Start: 10, End: 11, RuleID: "PB944", Desc: "add rust",
				creates: []arrayCreate{{Field: "makedepends", Entries: []string{"git", "rust"}}}, createTail: "\n"},
			{Path: "PKGBUILD", Start: 10, End: 11, RuleID: "PB711", Desc: "add git",
				creates: []arrayCreate{{Field: "makedepends", Entries: []string{"git"}}}, createTail: "\n"},
		}
		got := mergeCreations(edits)
		if len(got) != 1 {
			t.Fatalf("want one merged edit, got %d: %+v", len(got), got)
		}
		// A package both rules ask for is declared once.
		if want := "\nmakedepends=('git' 'rust')\n"; got[0].New != want {
			t.Errorf("merged text = %q, want %q", got[0].New, want)
		}
		if got[0].RuleID != "PB944,PB711" {
			t.Errorf("merged RuleID = %q, want both rules named", got[0].RuleID)
		}
		if got[0].Desc != "add rust; add git" {
			t.Errorf("merged Desc = %q, want both descriptions", got[0].Desc)
		}
	})

	t.Run("fields keep the order they were first asked for", func(t *testing.T) {
		edits := []Edit{
			{Path: "PKGBUILD", Start: 10, End: 11, RuleID: "PB961", Desc: "pair",
				creates: []arrayCreate{
					{Field: "provides", Entries: []string{"demo"}},
					{Field: "conflicts", Entries: []string{"demo"}},
				}, createTail: "\n"},
			{Path: "PKGBUILD", Start: 10, End: 11, RuleID: "PB973", Desc: "dkms",
				creates: []arrayCreate{{Field: "depends", Entries: []string{"dkms"}}}, createTail: "\n"},
		}
		got := mergeCreations(edits)
		if len(got) != 1 {
			t.Fatalf("want one merged edit, got %d", len(got))
		}
		want := "\nprovides=('demo')\nconflicts=('demo')\ndepends=('dkms')\n"
		if got[0].New != want {
			t.Errorf("merged text = %q, want %q", got[0].New, want)
		}
	})

	t.Run("a grouped edit is left out of the merge", func(t *testing.T) {
		// A merged edit cannot carry one contributor's all-or-nothing group
		// without imposing it on the rest, so grouped creations keep
		// colliding — which costs a run, not correctness.
		edits := []Edit{
			{Path: "PKGBUILD", Start: 10, End: 11, RuleID: "PB961", Group: "PB961",
				creates: []arrayCreate{{Field: "provides", Entries: []string{"demo"}}}, createTail: "\n"},
			{Path: "PKGBUILD", Start: 10, End: 11, RuleID: "PB973",
				creates: []arrayCreate{{Field: "depends", Entries: []string{"dkms"}}}, createTail: "\n"},
		}
		if got := mergeCreations(edits); len(got) != 2 {
			t.Errorf("want both edits kept apart, got %d: %+v", len(got), got)
		}
	})
}
