package pkgbuild

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"mvdan.cc/sh/v3/syntax"
)

// loadPKGBUILD writes content as a PKGBUILD in a fresh temp dir and loads it,
// mirroring the lint helper in internal/rules/rules_test.go.
func loadPKGBUILD(t *testing.T, content string) *Package {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	pkg, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return pkg
}

// firstWord parses `echo <expr>` with the package's own parser and returns the
// word for <expr>. Going through a real command keeps every word part
// (quoting, expansions, substitutions) exactly as the linter sees it.
func firstWord(t *testing.T, expr string) *syntax.Word {
	t.Helper()
	f, err := newParser().Parse(strings.NewReader("echo "+expr), "test")
	if err != nil {
		t.Fatalf("parse %q: %v", expr, err)
	}
	if len(f.Stmts) != 1 {
		t.Fatalf("parse %q: got %d statements, want 1", expr, len(f.Stmts))
	}
	call, ok := f.Stmts[0].Cmd.(*syntax.CallExpr)
	if !ok {
		t.Fatalf("parse %q: got %T, want *syntax.CallExpr", expr, f.Stmts[0].Cmd)
	}
	if len(call.Args) != 2 {
		t.Fatalf("parse %q: got %d args, want 2", expr, len(call.Args))
	}
	return call.Args[1]
}

// TestRenderWord pins the static-vs-dynamic contract documented on RenderWord.
// It is the linter's core anti-obfuscation guarantee: anything whose value
// cannot be determined statically must render a NUL byte and report dynamic.
func TestRenderWord(t *testing.T) {
	for _, tc := range []struct {
		name    string
		expr    string
		vars    map[string]string
		want    string
		dynamic bool
	}{
		{name: "literal", expr: `hello`, want: "hello"},
		{name: "single quoted", expr: `'single quoted'`, want: "single quoted"},
		{name: "double quoted with unknown ref", expr: `"double $pkgname"`, want: "double $pkgname"},
		{name: "unknown variable", expr: `$unknownvar`, want: "$unknownvar"},
		{name: "braced variable", expr: `${pkgname}`, want: "$pkgname"},
		{name: "concatenated parts", expr: `"$pkgdir/usr/bin"`, want: "$pkgdir/usr/bin"},
		{name: "known variable resolves", expr: `$pkgname`, vars: map[string]string{"pkgname": "demo"}, want: "demo"},
		{name: "known variable braced", expr: `${pkgname}`, vars: map[string]string{"pkgname": "demo"}, want: "demo"},
		{name: "unknown variable with vars map", expr: `$other`, vars: map[string]string{"pkgname": "demo"}, want: "$other"},

		{name: "command substitution", expr: `$(id)`, want: "\x00", dynamic: true},
		{name: "quoted command substitution", expr: `"$(curl x)"`, want: "\x00", dynamic: true},
		{name: "backtick substitution", expr: "`id`", want: "\x00", dynamic: true},
		{name: "process substitution", expr: `<(id)`, want: "\x00", dynamic: true},
		{name: "parameter default", expr: `${x:-default}`, want: "\x00", dynamic: true},
		{name: "parameter length", expr: `${#pkgname}`, want: "\x00", dynamic: true},
		{name: "indirection", expr: `${!pkgname}`, want: "\x00", dynamic: true},
		{name: "slice", expr: `${pkgname:1:2}`, want: "\x00", dynamic: true},
		{name: "replacement", expr: `${pkgname/a/b}`, want: "\x00", dynamic: true},
		{name: "index", expr: `${arr[0]}`, want: "\x00", dynamic: true},
		{name: "arithmetic", expr: `$((1+2))`, want: "\x00", dynamic: true},
		{name: "dynamic wins over static prefix", expr: `"/usr/bin/$(id -u)"`, want: "/usr/bin/\x00", dynamic: true},
		{name: "parameter op is dynamic even with vars", expr: `${pkgname:-x}`, vars: map[string]string{"pkgname": "demo"}, want: "\x00", dynamic: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, dynamic := RenderWord(firstWord(t, tc.expr), tc.vars)
			if got != tc.want || dynamic != tc.dynamic {
				t.Errorf("RenderWord(%s) = (%q, %v), want (%q, %v)", tc.expr, got, dynamic, tc.want, tc.dynamic)
			}
		})
	}

	t.Run("nil word", func(t *testing.T) {
		got, dynamic := RenderWord(nil, nil)
		if got != "" || dynamic {
			t.Errorf("RenderWord(nil, nil) = (%q, %v), want (%q, false)", got, dynamic, "")
		}
	})
}

// TestExpandScalar pins variable substitution across top-level scalars.
func TestExpandScalar(t *testing.T) {
	t.Run("plain scalar", func(t *testing.T) {
		pkg := loadPKGBUILD(t, "pkgver=1.0\n")
		got, ok := pkg.Scalar("pkgver")
		if !ok || got != "1.0" {
			t.Errorf("Scalar(pkgver) = (%q, %v), want (%q, true)", got, ok, "1.0")
		}
	})

	t.Run("scalar composed from another scalar", func(t *testing.T) {
		pkg := loadPKGBUILD(t, "_base=foo\npkgname=$_base-bar\n")
		got, ok := pkg.Scalar("pkgname")
		if !ok || got != "foo-bar" {
			t.Errorf("Scalar(pkgname) = (%q, %v), want (%q, true)", got, ok, "foo-bar")
		}
	})

	t.Run("braced reference", func(t *testing.T) {
		pkg := loadPKGBUILD(t, "_base=foo\npkgname=${_base}-bar\n")
		if got := pkg.Expand("${_base}/x"); got != "foo/x" {
			t.Errorf("Expand(${_base}/x) = %q, want %q", got, "foo/x")
		}
	})

	t.Run("unknown reference is left as-is", func(t *testing.T) {
		pkg := loadPKGBUILD(t, "pkgver=1.0\n")
		if got := pkg.Expand("$nope"); got != "$nope" {
			t.Errorf("Expand($nope) = %q, want %q", got, "$nope")
		}
	})

	t.Run("no reference is returned unchanged", func(t *testing.T) {
		pkg := loadPKGBUILD(t, "pkgver=1.0\n")
		if got := pkg.Expand("plain"); got != "plain" {
			t.Errorf("Expand(plain) = %q, want %q", got, "plain")
		}
	})

	t.Run("array is not a scalar", func(t *testing.T) {
		pkg := loadPKGBUILD(t, "arch=('x86_64')\n")
		if got, ok := pkg.Scalar("arch"); ok {
			t.Errorf("Scalar(arch) = (%q, true), want ok=false for an array", got)
		}
	})

	// $arr means ${arr[0]} in bash; split PKGBUILDs like cyrus-imapd rely on
	// this for source entries such as ${pkgname}.service.
	t.Run("array reference expands to first element", func(t *testing.T) {
		pkg := loadPKGBUILD(t, "pkgname=(cyrus-imapd cyrus-imapd-docs)\n")
		if got := pkg.Expand("$pkgname.service"); got != "cyrus-imapd.service" {
			t.Errorf("Expand($pkgname.service) = %q, want %q", got, "cyrus-imapd.service")
		}
	})

	t.Run("empty array expands to empty string", func(t *testing.T) {
		pkg := loadPKGBUILD(t, "groups=()\n")
		if got := pkg.Expand("x${groups}y"); got != "xy" {
			t.Errorf("Expand(x${groups}y) = %q, want %q", got, "xy")
		}
	})

	t.Run("missing variable", func(t *testing.T) {
		pkg := loadPKGBUILD(t, "pkgver=1.0\n")
		if got, ok := pkg.Scalar("nope"); ok {
			t.Errorf("Scalar(nope) = (%q, true), want ok=false", got)
		}
	})

	// Mutually recursive scalars must terminate at the fixpoint cap rather than
	// looping forever; the exact residual value is not part of the contract.
	t.Run("self-referential chain terminates", func(t *testing.T) {
		pkg := loadPKGBUILD(t, "a=$b\nb=$a\n")
		got := pkg.Expand("$a")
		if got != "$a" && got != "$b" {
			t.Errorf("Expand($a) = %q, want an unresolved %q or %q", got, "$a", "$b")
		}
	})
}

// TestAppendAssignments pins bash `+=` semantics for top-level assignments.
// Before append support, a later `source+=(...)` overwrote the base
// `source=(...)`, so the base entries vanished from every rule that reads
// Sources() — and vice versa depending on which array makepkg actually used.
func TestAppendAssignments(t *testing.T) {
	// Array append is order-preserving: base elements first, appended after.
	t.Run("array append preserves order", func(t *testing.T) {
		pkg := loadPKGBUILD(t, "source=(\"a\")\nsource+=(\"b\" \"c\")\n")
		var got []string
		for _, e := range pkg.Sources() {
			if e.Arch != "" {
				t.Errorf("source %q has Arch %q, want \"\"", e.URL, e.Arch)
			}
			got = append(got, e.URL)
		}
		if want := []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
			t.Errorf("Sources() URLs = %v, want %v", got, want)
		}
		if v := pkg.Vars["source"]; !v.Array {
			t.Errorf("merged source Var has Array=false, want true")
		}
	})

	// Appending to an unset name is a plain assignment in bash.
	t.Run("append with no base assignment", func(t *testing.T) {
		pkg := loadPKGBUILD(t, "source+=(\"only\")\n")
		srcs := pkg.Sources()
		if len(srcs) != 1 || srcs[0].URL != "only" {
			t.Fatalf("Sources() = %+v, want a single entry %q", srcs, "only")
		}
	})

	// Checksums must stay index-aligned with sources across the merge, or
	// makepkg's source/sum pairing and SumsFor disagree.
	t.Run("checksum append stays index aligned", func(t *testing.T) {
		pkg := loadPKGBUILD(t, "source=(\"a\" \"b\")\nsha256sums=('AAAA')\nsha256sums+=('BBBB')\n")
		srcs := pkg.Sources()
		if len(srcs) != 2 {
			t.Fatalf("Sources() returned %d entries, want 2", len(srcs))
		}
		for i, want := range []string{"AAAA", "BBBB"} {
			if got := pkg.SumsFor(srcs[i]); !reflect.DeepEqual(got, []string{want}) {
				t.Errorf("SumsFor(index %d) = %v, want [%q]", i, got, want)
			}
		}
	})

	// Scalar `+=` concatenates rather than replacing.
	t.Run("scalar append concatenates", func(t *testing.T) {
		pkg := loadPKGBUILD(t, "_x=foo\n_x+=bar\n")
		got, ok := pkg.Scalar("_x")
		if !ok || got != "foobar" {
			t.Errorf("Scalar(_x) = (%q, %v), want (%q, true)", got, ok, "foobar")
		}
	})

	// Degenerate scalar-then-array mismatch: keep both sets of values as an
	// array. Dropping either half would hide values from the rules.
	t.Run("scalar then array append keeps both", func(t *testing.T) {
		pkg := loadPKGBUILD(t, "source=one\nsource+=(\"two\")\n")
		v := pkg.Vars["source"]
		if !v.Array {
			t.Errorf("merged Var has Array=false, want true")
		}
		if want := []string{"one", "two"}; !reflect.DeepEqual(v.Values, want) {
			t.Errorf("merged Values = %v, want %v", v.Values, want)
		}
	})

	// The merged Var keeps the first assignment's identity so byte-offset
	// fixes stay anchored to a real, editable AST node.
	t.Run("merged var keeps the first assignment identity", func(t *testing.T) {
		pkg := loadPKGBUILD(t, "pkgname=demo\nsource=(\"a\")\nsource+=(\"b\")\n")
		v := pkg.Vars["source"]
		if got := int(v.Pos.Line()); got != 2 {
			t.Errorf("merged Var Pos line = %d, want 2 (the first assignment)", got)
		}
		if v.Assign == nil || v.Assign.Append {
			t.Errorf("merged Var Assign = %+v, want the non-append first assignment", v.Assign)
		}
	})

	// Plain `=` reassignment must still overwrite, exactly as before.
	t.Run("plain reassignment still overwrites", func(t *testing.T) {
		pkg := loadPKGBUILD(t, "source=(\"a\")\nsource=(\"b\")\n")
		srcs := pkg.Sources()
		if len(srcs) != 1 || srcs[0].URL != "b" {
			t.Fatalf("Sources() = %+v, want a single entry %q", srcs, "b")
		}
	})
}

// Indexed element writes (`name[i]=val`) are array updates in bash: the
// variable becomes an array and only the addressed element changes. Treating
// them as scalar reassignments hid every other checksum from the rules and
// made PB708 flag — and "fix" into invalid bash — perfectly valid PKGBUILDs.
func TestIndexedAssignments(t *testing.T) {
	t.Run("override updates one element and keeps the rest", func(t *testing.T) {
		pkg := loadPKGBUILD(t, "sha256sums=('AAAA' 'BBBB' 'CCCC')\nsha256sums[1]='SKIP'\n")
		v := pkg.Vars["sha256sums"]
		if !v.Array {
			t.Errorf("merged Var has Array=false, want true")
		}
		if want := []string{"AAAA", "SKIP", "CCCC"}; !reflect.DeepEqual(v.Values, want) {
			t.Errorf("merged Values = %v, want %v", v.Values, want)
		}
	})

	t.Run("write past the end extends with empty elements", func(t *testing.T) {
		pkg := loadPKGBUILD(t, "sha256sums=('AAAA')\nsha256sums[2]='CCCC'\n")
		if got, want := pkg.Vars["sha256sums"].Values, []string{"AAAA", "", "CCCC"}; !reflect.DeepEqual(got, want) {
			t.Errorf("merged Values = %v, want %v", got, want)
		}
	})

	t.Run("indexed write with no prior assignment makes an array", func(t *testing.T) {
		pkg := loadPKGBUILD(t, "sha256sums[1]='BBBB'\n")
		v := pkg.Vars["sha256sums"]
		if !v.Array {
			t.Errorf("Var has Array=false, want true")
		}
		if want := []string{"", "BBBB"}; !reflect.DeepEqual(v.Values, want) {
			t.Errorf("Values = %v, want %v", v.Values, want)
		}
	})

	t.Run("scalar then indexed write converts to array", func(t *testing.T) {
		pkg := loadPKGBUILD(t, "_x=foo\n_x[1]=bar\n")
		v := pkg.Vars["_x"]
		if !v.Array {
			t.Errorf("Var has Array=false, want true")
		}
		if want := []string{"foo", "bar"}; !reflect.DeepEqual(v.Values, want) {
			t.Errorf("Values = %v, want %v", v.Values, want)
		}
	})

	t.Run("non-literal subscript marks the array type but places no value", func(t *testing.T) {
		pkg := loadPKGBUILD(t, "_i=1\nsha256sums=('AAAA')\nsha256sums[$_i]='BBBB'\n")
		v := pkg.Vars["sha256sums"]
		if !v.Array {
			t.Errorf("Var has Array=false, want true")
		}
		if want := []string{"AAAA"}; !reflect.DeepEqual(v.Values, want) {
			t.Errorf("Values = %v, want %v", v.Values, want)
		}
	})

	t.Run("element append concatenates", func(t *testing.T) {
		pkg := loadPKGBUILD(t, "_x=(foo)\n_x[0]+=bar\n")
		if got, want := pkg.Vars["_x"].Values, []string{"foobar"}; !reflect.DeepEqual(got, want) {
			t.Errorf("Values = %v, want %v", got, want)
		}
	})

	// Same anchoring contract as mergeAppend: edits address the first
	// assignment's bytes.
	t.Run("merged var keeps the first assignment identity", func(t *testing.T) {
		pkg := loadPKGBUILD(t, "pkgname=demo\nsha256sums=('AAAA')\nsha256sums[0]='BBBB'\n")
		v := pkg.Vars["sha256sums"]
		if got := int(v.Pos.Line()); got != 2 {
			t.Errorf("merged Var Pos line = %d, want 2 (the first assignment)", got)
		}
		if v.Assign == nil || v.Assign.Array == nil {
			t.Errorf("merged Var Assign = %+v, want the original array assignment", v.Assign)
		}
	})

	// PKGBUILDs are untrusted input: a huge literal subscript must not make
	// the parser allocate a huge Values slice.
	t.Run("oversized literal subscript is not allocated", func(t *testing.T) {
		pkg := loadPKGBUILD(t, "sha256sums[999999999]='AAAA'\n")
		v := pkg.Vars["sha256sums"]
		if !v.Array {
			t.Errorf("Var has Array=false, want true")
		}
		if len(v.Values) != 0 {
			t.Errorf("Values has %d elements, want 0", len(v.Values))
		}
	})

	// The override must flow through to source/checksum pairing.
	t.Run("checksum override stays index aligned", func(t *testing.T) {
		pkg := loadPKGBUILD(t, "source=(\"a\" \"b\")\nsha256sums=('AAAA' 'BBBB')\nsha256sums[1]='SKIP'\n")
		srcs := pkg.Sources()
		if len(srcs) != 2 {
			t.Fatalf("Sources() returned %d entries, want 2", len(srcs))
		}
		for i, want := range []string{"AAAA", "SKIP"} {
			if got := pkg.SumsFor(srcs[i]); !reflect.DeepEqual(got, []string{want}) {
				t.Errorf("SumsFor(index %d) = %v, want [%q]", i, got, want)
			}
		}
	})
}

// TestParseSuppressions pins the inline "# pkglint: ignore=" directive parser.
// Cross-file line collisions are deliberately not covered here.
func TestParseSuppressions(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want map[int]map[string]bool
	}{
		{
			name: "single id on line 3",
			raw:  "pkgname=demo\npkgver=1.0\n# pkglint: ignore=PB204\n",
			want: map[int]map[string]bool{3: {"PB204": true}},
		},
		{
			name: "two ids",
			raw:  "# pkglint: ignore=PB204,PB206\n",
			want: map[int]map[string]bool{1: {"PB204": true, "PB206": true}},
		},
		{
			name: "two ids with spaces",
			raw:  "# pkglint: ignore=PB204, PB206\n",
			want: map[int]map[string]bool{1: {"PB204": true, "PB206": true}},
		},
		{
			name: "trailing directive on a code line",
			raw:  "pkgname=demo # pkglint: ignore=PB101\n",
			want: map[int]map[string]bool{1: {"PB101": true}},
		},
		{
			name: "no directive",
			raw:  "pkgname=demo\n# just a comment\n",
			want: map[int]map[string]bool{},
		},
		{
			name: "lowercase ids are not recognized",
			raw:  "# pkglint: ignore=pb204\n",
			want: map[int]map[string]bool{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseSuppressions([]byte(tc.raw)); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("parseSuppressions(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

// TestSuppressed pins the "same line or the line above" window, scoped to the
// file the directive lives in.
func TestSuppressed(t *testing.T) {
	pkg := loadPKGBUILD(t, "pkgname=demo\n# pkglint: ignore=PB204\npkgver=1.0\n")
	for _, tc := range []struct {
		name string
		id   string
		path string
		line int
		want bool
	}{
		{name: "directive line", id: "PB204", path: pkg.PKGBUILD.Path, line: 2, want: true},
		{name: "line below directive", id: "PB204", path: pkg.PKGBUILD.Path, line: 3, want: true},
		{name: "line above directive", id: "PB204", path: pkg.PKGBUILD.Path, line: 1},
		{name: "two lines below directive", id: "PB204", path: pkg.PKGBUILD.Path, line: 4},
		{name: "other rule id", id: "PB206", path: pkg.PKGBUILD.Path, line: 2},
		{name: "other file, same line", id: "PB204", path: filepath.Join(pkg.Dir, "demo.install"), line: 2},
		{name: "unknown file", id: "PB204", path: "nowhere", line: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := pkg.Suppressed(tc.id, tc.path, tc.line); got != tc.want {
				t.Errorf("Suppressed(%q, %q, %d) = %v, want %v", tc.id, tc.path, tc.line, got, tc.want)
			}
		})
	}
}

// TestSuppressionsKeyedByFile pins that each file's directives are stored under
// that file's own path, including a scriptlet that fails to parse (its PB503
// finding reports at that path, so it must be suppressible from within it).
func TestSuppressionsKeyedByFile(t *testing.T) {
	// A scriptlet whose body cannot be parsed (an `if` with no `then`/`fi`),
	// carrying a directive on line 1 — the line PB503 reports.
	const brokenScriptlet = "# pkglint: ignore=PB503\npost_install() {\n  if [ -z ]\n}\n"

	dir := t.TempDir()
	for name, content := range map[string]string{
		"PKGBUILD":     "pkgname=demo\n# pkglint: ignore=PB204\npkgver=1.0\ninstall=demo.install\n",
		"demo.install": "# pkglint: ignore=PB501\npost_install() {\n  echo hi\n}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pkg, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	want := map[string]map[int]map[string]bool{
		filepath.Join(dir, "PKGBUILD"):     {2: {"PB204": true}},
		filepath.Join(dir, "demo.install"): {1: {"PB501": true}},
	}
	if !reflect.DeepEqual(pkg.Suppressions, want) {
		t.Errorf("Suppressions = %v, want %v", pkg.Suppressions, want)
	}

	brokenDir := t.TempDir()
	for name, content := range map[string]string{
		"PKGBUILD":       "pkgname=demo\npkgver=1.0\ninstall=broken.install\n",
		"broken.install": brokenScriptlet,
	} {
		if err := os.WriteFile(filepath.Join(brokenDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	bpkg, err := Load(brokenDir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(bpkg.ScriptletErrors) != 1 {
		t.Fatalf("ScriptletErrors = %+v, want one entry", bpkg.ScriptletErrors)
	}
	brokenPath := filepath.Join(brokenDir, "broken.install")
	if !bpkg.Suppressed("PB503", brokenPath, 1) {
		t.Errorf("Suppressed(PB503, %q, 1) = false, want true: an unparseable scriptlet must still contribute its own directives", brokenPath)
	}
}

// TestUnits pins that scriptlets referenced by install= are loaded alongside
// the PKGBUILD.
func TestUnits(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"PKGBUILD":     "pkgname=demo\npkgver=1.0\ninstall=demo.install\n",
		"demo.install": "post_install() {\n  echo hi\n}\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pkg, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	units := pkg.Units()
	if len(units) != 2 {
		t.Fatalf("Units() returned %d units, want 2 (PKGBUILD + scriptlet)", len(units))
	}
	if units[0].Scriptlet {
		t.Errorf("first unit should be the PKGBUILD, got %+v", units[0].Path)
	}
	if !units[1].Scriptlet || filepath.Base(units[1].Path) != "demo.install" {
		t.Errorf("second unit should be demo.install, got %q (scriptlet=%v)", units[1].Path, units[1].Scriptlet)
	}
	if _, ok := units[1].Functions["post_install"]; !ok {
		t.Errorf("scriptlet functions = %v, want post_install", units[1].Functions)
	}
}

// TestInstallFilesContainment pins the security contract that an install=
// value can only ever name a plain file inside the package directory. A
// hostile PKGBUILD must not be able to steer Load into reading a file outside
// the package (traversal) or an unbounded device such as /dev/zero (DoS).
//
// Every case builds a real file at the location the hostile name points to, so
// a regression would actually load it and the assertion would fail.
func TestInstallFilesContainment(t *testing.T) {
	const scriptlet = "post_install() {\n  echo pwned\n}\n"

	for _, tc := range []struct {
		name string
		// install is the install= value; %s is replaced with the parent temp
		// dir so absolute-path cases stay inside the test sandbox.
		install string
		// bait is the path, relative to the parent temp dir, of the file the
		// hostile name is trying to reach. Empty means no bait file.
		bait string
	}{
		{name: "parent traversal", install: "../evil.install", bait: "evil.install"},
		{name: "deep parent traversal", install: "../../evil.install"},
		{name: "separator", install: "sub/dir.install", bait: "pkg/sub/dir.install"},
		{name: "backslash separator", install: `sub\dir.install`},
		{name: "dot", install: "."},
		{name: "dotdot", install: ".."},
		{name: "trailing separator", install: "demo.install/", bait: "pkg/demo.install"},
		{name: "absolute path", install: "%s/evil.install", bait: "evil.install"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "pkg")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if tc.bait != "" {
				bait := filepath.Join(root, tc.bait)
				if err := os.MkdirAll(filepath.Dir(bait), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(bait, []byte(scriptlet), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			install := strings.ReplaceAll(tc.install, "%s", root)
			content := "pkgname=demo\npkgver=1.0\ninstall=" + install + "\n"
			if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			pkg, err := Load(dir)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if got := pkg.installFiles(); len(got) != 0 {
				t.Errorf("installFiles() = %v, want none for install=%q", got, install)
			}
			if len(pkg.Scriptlets) != 0 {
				t.Errorf("loaded %d scriptlets for install=%q, want 0", len(pkg.Scriptlets), install)
			}
			if len(pkg.Units()) != 1 {
				t.Errorf("Units() = %d, want 1 (PKGBUILD only) for install=%q", len(pkg.Units()), install)
			}
		})
	}

	// The containment check must not break the ordinary case.
	t.Run("plain name still loads", func(t *testing.T) {
		dir := t.TempDir()
		files := map[string]string{
			"PKGBUILD":     "pkgname=demo\npkgver=1.0\ninstall=demo.install\n",
			"demo.install": scriptlet,
		}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		pkg, err := Load(dir)
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		if got := pkg.installFiles(); !reflect.DeepEqual(got, []string{"demo.install"}) {
			t.Errorf("installFiles() = %v, want [demo.install]", got)
		}
		if len(pkg.Scriptlets) != 1 {
			t.Fatalf("loaded %d scriptlets, want 1", len(pkg.Scriptlets))
		}
		if _, ok := pkg.Scriptlets[0].Functions["post_install"]; !ok {
			t.Errorf("scriptlet functions = %v, want post_install", pkg.Scriptlets[0].Functions)
		}
	})
}

// TestParseDirective pins the inline-ignore syntax both the suppression parser
// and the stale-directive rule (which edits by span) build on.
func TestParseDirective(t *testing.T) {
	for _, tc := range []struct {
		name string
		line string
		ids  []string
		list string // line[IDsStart:IDsEnd]
	}{
		{"plain", "# pkglint: ignore=PB101", []string{"PB101"}, "PB101"},
		{"multiple ids", "# pkglint: ignore=PB101,PB102", []string{"PB101", "PB102"}, "PB101,PB102"},
		{"comma-space and duplicates", "# pkglint: ignore=PB101, PB102, PB101", []string{"PB101", "PB102"}, "PB101, PB102, PB101"},
		{"trailing separators trimmed", "# pkglint: ignore=PB101, ", []string{"PB101"}, "PB101"},
		{"trailing comment prose stops the list", "# pkglint: ignore=PB101 -- reviewed", []string{"PB101"}, "PB101"},
		{"after code", `go build . # pkglint: ignore=PB204`, []string{"PB204"}, "PB204"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, ok := ParseDirective(tc.line)
			if !ok {
				t.Fatalf("ParseDirective(%q) found nothing", tc.line)
			}
			if !reflect.DeepEqual(d.IDs, tc.ids) {
				t.Errorf("IDs = %v, want %v", d.IDs, tc.ids)
			}
			if got := tc.line[d.IDsStart:d.IDsEnd]; got != tc.list {
				t.Errorf("ID-list span = %q, want %q", got, tc.list)
			}
			if tc.line[d.Start] != '#' {
				t.Errorf("Start points at %q, want '#'", tc.line[d.Start])
			}
		})
	}
	for _, line := range []string{
		"echo hello",
		"# pkglint: ignore=",
		"# pkglint: ignore=, ,",
		"# just a comment mentioning pkglint",
	} {
		if d, ok := ParseDirective(line); ok {
			t.Errorf("ParseDirective(%q) = %+v, want no directive", line, d)
		}
	}
}

func restoreMaxSourceFile(t *testing.T, limit int64) func() {
	t.Helper()
	prev := maxSourceFile
	maxSourceFile = limit
	return func() { maxSourceFile = prev }
}

// TestLoadRefusesNonRegularSources pins that a source file which resolves to
// something other than a regular file is never read: a bare name in the
// package directory can be a symlink to a device, and reading /dev/zero would
// hang the linter. /dev/null stands in for the device here so a regression
// fails an assertion instead of hanging the suite.
func TestLoadRefusesNonRegularSources(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks are not portable on windows")
	}
	dir := t.TempDir()
	content := "pkgname=demo\npkgver=1\npkgrel=1\ninstall=demo.install\n"
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".SRCINFO", "demo.install"} {
		if err := os.Symlink("/dev/null", filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	pkg, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if pkg.SrcInfo != nil {
		t.Errorf("SrcInfo = %+v, want nil (non-regular .SRCINFO must be skipped)", pkg.SrcInfo)
	}
	if len(pkg.Scriptlets) != 0 {
		t.Errorf("Scriptlets = %+v, want none (non-regular scriptlet must be skipped)", pkg.Scriptlets)
	}

	// The PKGBUILD itself: a non-regular file is a load error, not a hang.
	other := t.TempDir()
	if err := os.Symlink("/dev/null", filepath.Join(other, "PKGBUILD")); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(other); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Errorf("Load(non-regular PKGBUILD) error = %v, want one mentioning \"not a regular file\"", err)
	}
}

// TestLoadRefusesOversizedSources pins the size ceiling on the package's own
// files, so a huge (or unbounded) file cannot exhaust memory.
func TestLoadRefusesOversizedSources(t *testing.T) {
	defer restoreMaxSourceFile(t, 64)()

	big := t.TempDir()
	oversized := "pkgname=demo\npkgver=1\npkgrel=1\n# " + strings.Repeat("x", 200) + "\n"
	if err := os.WriteFile(filepath.Join(big, "PKGBUILD"), []byte(oversized), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(big); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Errorf("Load(oversized PKGBUILD) error = %v, want one mentioning \"larger than\"", err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte("pkgname=d\npkgver=1\npkgrel=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srcinfo := "pkgbase = d\n\tpkgver = 1\n" + strings.Repeat("\t# pad\n", 20)
	if err := os.WriteFile(filepath.Join(dir, ".SRCINFO"), []byte(srcinfo), 0o644); err != nil {
		t.Fatal(err)
	}
	pkg, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if pkg.SrcInfo != nil {
		t.Errorf("SrcInfo = %+v, want nil (oversized .SRCINFO must be skipped)", pkg.SrcInfo)
	}
}
