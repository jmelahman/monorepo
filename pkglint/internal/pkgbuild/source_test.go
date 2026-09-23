package pkgbuild

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestExpandBraces(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []string
	}{
		{"plain.tar.gz", []string{"plain.tar.gz"}},
		{"foo.tar.gz{,.sig}", []string{"foo.tar.gz", "foo.tar.gz.sig"}},
		{"foo.{tar.gz,zip}", []string{"foo.tar.gz", "foo.zip"}},
		{"a{1,2}b{3,4}", []string{"a1b3", "a1b4", "a2b3", "a2b4"}},
		{"${_tar}", []string{"${_tar}"}},                       // parameter braces, not a group
		{"${_tar}{,.asc}", []string{"${_tar}", "${_tar}.asc"}}, // mixed
		{"nocomma{x}", []string{"nocomma{x}"}},                 // bash leaves this literal too
		// Nested groups. The tar.gz case is how a PKGBUILD asks for an archive,
		// its checksum file and that file's signature in one element; splitting
		// on every comma instead would yield tar.gz twice and lose the .asc.
		{"tor.tar.gz{,.sha256sum{,.asc}}", []string{
			"tor.tar.gz", "tor.tar.gz.sha256sum", "tor.tar.gz.sha256sum.asc"}},
		{"{agilex{3,5},arria10}-x.qdz", []string{
			"agilex3-x.qdz", "agilex5-x.qdz", "arria10-x.qdz"}},
		{"pre{a,b{c,d},e}post", []string{
			"preapost", "prebcpost", "prebdpost", "preepost"}},
		{"{a{b}}", []string{"{a{b}}"}}, // no comma at any depth: literal
		{"un{balanced,", []string{"un{balanced,"}},
	} {
		if got := ExpandBraces(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("ExpandBraces(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// Brace groups multiply, so an element built from enough of them would expand
// without bound. Past the cap the element is left as written rather than
// allocating: the input is untrusted, and no real source array needs it.
func TestExpandBracesIsBounded(t *testing.T) {
	in := strings.Repeat("{a,b}", 40) // 2^40 if left unbounded
	got := ExpandBraces(in)
	if len(got) != 1 || got[0] != in {
		t.Errorf("ExpandBraces on %d nested groups produced %d values, want the input unexpanded", 40, len(got))
	}
}

func TestSourcesBraceExpansion(t *testing.T) {
	dir := t.TempDir()
	content := `pkgname=demo
pkgver=1.0.0
pkgrel=1
arch=('x86_64')
_tar=demo-$pkgver.tar.gz
source=("https://example.com/$_tar{,.sig}")
sha256sums=('9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08' 'SKIP')
`
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	pkg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	srcs := pkg.Sources()
	if len(srcs) != 2 {
		t.Fatalf("expected 2 expanded sources, got %d: %+v", len(srcs), srcs)
	}
	// Both entries expand from the single written element, so they share an
	// ElemIndex while getting distinct Index values.
	if srcs[0].URL != "https://example.com/demo-1.0.0.tar.gz" || srcs[0].Index != 0 || srcs[0].ElemIndex != 0 {
		t.Errorf("first source wrong: %+v", srcs[0])
	}
	if srcs[1].URL != "https://example.com/demo-1.0.0.tar.gz.sig" || srcs[1].Index != 1 || srcs[1].ElemIndex != 0 {
		t.Errorf("second source wrong: %+v", srcs[1])
	}
	// Checksums pair by expanded index, exactly as makepkg pairs them.
	if sums := pkg.SumsFor(srcs[1]); len(sums) != 1 || sums[0] != "SKIP" {
		t.Errorf("sig source should pair with the SKIP sum, got %v", sums)
	}
}

// TestSourcesArrayRefExpansion pins the ttf-ms-fonts shape: a source array
// built by prefixing every element of a helper array, whose real length only
// exists after expanding the array reference. Each expanded entry pairs with
// its checksum by index and reports the written element's position.
func TestSourcesArrayRefExpansion(t *testing.T) {
	pkg := loadPKGBUILD(t, `pkgname=demo
pkgver=1.0
_files=('one.bin'
        'two.bin'
        'three.bin')
_dlpath="https://example.com/pub"
source=("${_files[@]/#/$_dlpath/}")
sha256sums=('aaaa' 'bbbb' 'cccc')
`)
	srcs := pkg.Sources()
	if len(srcs) != 3 {
		t.Fatalf("Sources() = %d entries, want 3: %+v", len(srcs), srcs)
	}
	for i, want := range []string{"one.bin", "two.bin", "three.bin"} {
		e := srcs[i]
		if e.URL != "https://example.com/pub/"+want {
			t.Errorf("Sources()[%d].URL = %q, want https://example.com/pub/%s", i, e.URL, want)
		}
		if e.Index != i || e.ElemIndex != 0 {
			t.Errorf("Sources()[%d] Index/ElemIndex = %d/%d, want %d/0", i, e.Index, e.ElemIndex, i)
		}
		// All three genuinely occupy the one written element on line 7.
		if e.Pos.Line() != 7 || e.Pos.Col() != 9 {
			t.Errorf("Sources()[%d] at %d:%d, want 7:9", i, e.Pos.Line(), e.Pos.Col())
		}
	}
	if got := pkg.SumsFor(srcs[2]); len(got) != 1 || got[0] != "cccc" {
		t.Errorf("SumsFor(index 2) = %v, want [cccc]", got)
	}
	if pkg.Vars["source"].CountUnknown {
		t.Error("source.CountUnknown = true for a statically sized expansion")
	}
}

// TestArrayRefExpansionForms pins which whole-array reference forms expand,
// which keep only the element count, and which leave the count unknown.
func TestArrayRefExpansionForms(t *testing.T) {
	const files = "_files=('a.bin' 'b.bin' 'c.bin')\n"
	dyn3 := []string{"\x00", "\x00", "\x00"}
	for _, tc := range []struct {
		name    string
		decl    string   // assignments preceding source=
		elem    string   // the single written source element
		want    []string // nil: only check the count
		wantN   int
		unknown bool
	}{
		{"plain quoted", files, `"${_files[@]}"`, []string{"a.bin", "b.bin", "c.bin"}, 3, false},
		{"plain unquoted", files, `${_files[@]}`, []string{"a.bin", "b.bin", "c.bin"}, 3, false},
		{"unquoted star", files, `${_files[*]}`, []string{"a.bin", "b.bin", "c.bin"}, 3, false},
		{"prefix idiom keeps the ref unexpanded", files + "_url='https://x.example/d'\n",
			`"${_files[@]/#/$_url/}"`, []string{"$_url/a.bin", "$_url/b.bin", "$_url/c.bin"}, 3, false},
		{"suffix idiom", files, `"${_files[@]/%/.sig}"`, []string{"a.bin.sig", "b.bin.sig", "c.bin.sig"}, 3, false},
		{"inline prefix lands on the first element only", files,
			`"https://x.example/d/${_files[@]}"`, []string{"https://x.example/d/a.bin", "b.bin", "c.bin"}, 3, false},
		{"literal replacement", files, `"${_files[@]/.bin/.dat}"`, []string{"a.dat", "b.dat", "c.dat"}, 3, false},
		{"glob replacement keeps only the count", files, `"${_files[@]/?.bin/x}"`, dyn3, 3, false},
		{"suffix strip keeps only the count", files, `"${_files[@]%.bin}"`, dyn3, 3, false},
		{"quoted star joins into one word", files, `"${_files[*]}"`, nil, 1, false},
		{"length is a count, not a reference", files, `"${#_files[@]}"`, nil, 1, false},
		{"unknown array", "", `"${_missing[@]}"`, nil, 1, true},
		{"slice changes the count", files, `"${_files[@]:1}"`, nil, 1, true},
		{"two references", files, `"${_files[@]}${_files[@]}"`, nil, 1, true},
		{"empty array vanishes", "_files=()\n", `"${_files[@]}"`, []string{}, 0, false},
		{"empty array with surrounding text keeps a word", "_files=()\n",
			`"pre-${_files[@]}"`, []string{"pre-"}, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pkg := loadPKGBUILD(t, "pkgname=demo\n"+tc.decl+"source=("+tc.elem+")\n")
			v := pkg.Vars["source"]
			if v == nil {
				t.Fatal("source not recorded")
			}
			if len(v.Values) != tc.wantN {
				t.Fatalf("len(Values) = %d, want %d: %q", len(v.Values), tc.wantN, v.Values)
			}
			if tc.want != nil && !reflect.DeepEqual(append([]string{}, v.Values...), tc.want) {
				t.Errorf("Values = %q, want %q", v.Values, tc.want)
			}
			if v.CountUnknown != tc.unknown {
				t.Errorf("CountUnknown = %v, want %v", v.CountUnknown, tc.unknown)
			}
		})
	}
}

// TestSourcesArrayRefNeighbors pins that elements around an expanded array
// reference keep their own positions and element indices, and that a later
// `+=` append still merges in behind the expansion.
func TestSourcesArrayRefNeighbors(t *testing.T) {
	// `source=(` opens on line 3: elements sit at 3:9, 4:9, 5:9. The appended
	// element continues the written-element numbering (index 3) so fix code
	// can still address its text, but has no AST element in the merged Var and
	// falls back to the array position 3:1.
	pkg := loadPKGBUILD(t, `pkgname=demo
_files=('a.bin' 'b.bin')
source=('pre.tar.gz'
        "${_files[@]}"
        'post.tar.gz')
sha256sums=('s0' 's1' 's2' 's3' 's4')
source+=('extra.tar.gz')
`)
	srcs := pkg.Sources()
	if len(srcs) != 5 {
		t.Fatalf("Sources() = %d entries, want 5: %+v", len(srcs), srcs)
	}
	wants := []struct {
		url       string
		elemIndex int
		line, col uint
	}{
		{"pre.tar.gz", 0, 3, 9},
		{"a.bin", 1, 4, 9},
		{"b.bin", 1, 4, 9},
		{"post.tar.gz", 2, 5, 9},
		{"extra.tar.gz", 3, 3, 1},
	}
	for i, want := range wants {
		e := srcs[i]
		if e.URL != want.url || e.Index != i || e.ElemIndex != want.elemIndex {
			t.Errorf("Sources()[%d] = %q Index=%d ElemIndex=%d, want %q/%d/%d",
				i, e.URL, e.Index, e.ElemIndex, want.url, i, want.elemIndex)
		}
		if e.Pos.Line() != want.line || e.Pos.Col() != want.col {
			t.Errorf("Sources()[%d] at %d:%d, want %d:%d", i, e.Pos.Line(), e.Pos.Col(), want.line, want.col)
		}
	}
	if got := pkg.SumsFor(srcs[3]); len(got) != 1 || got[0] != "s3" {
		t.Errorf("SumsFor(post.tar.gz) = %v, want [s3]", got)
	}
}

// TestArrayRefAmplificationCapped pins the untrusted-input guards: chained
// whole-array references double values per assignment and chained replacements
// multiply content, and a hostile PKGBUILD must run out of budget, not memory.
// Past the cap the variable is CountUnknown, never silently miscounted.
func TestArrayRefAmplificationCapped(t *testing.T) {
	t.Run("value-count doubling", func(t *testing.T) {
		var b strings.Builder
		b.WriteString("pkgname=demo\na0=(x x x x x x x x)\n")
		for i := 1; i <= 30; i++ {
			fmt.Fprintf(&b, "a%d=(\"${a%d[@]}\" \"${a%d[@]}\")\n", i, i-1, i-1)
		}
		b.WriteString("source=(\"${a30[@]}\")\n")
		pkg := loadPKGBUILD(t, b.String())
		total := 0
		for _, v := range pkg.Vars {
			total += len(v.Values)
		}
		if total > 100_000 {
			t.Fatalf("expansion produced %d values; the cap did not hold", total)
		}
		if !pkg.Vars["source"].CountUnknown {
			t.Error("source.CountUnknown = false after a refused expansion chain")
		}
	})

	t.Run("content multiplication", func(t *testing.T) {
		var b strings.Builder
		b.WriteString("pkgname=demo\nb0=('" + strings.Repeat("a", 64) + "')\n")
		for i := 1; i <= 12; i++ {
			fmt.Fprintf(&b, "b%d=(\"${b%d[@]//a/aaaaaaaa}\")\n", i, i-1)
		}
		pkg := loadPKGBUILD(t, b.String())
		total := 0
		for _, v := range pkg.Vars {
			for _, s := range v.Values {
				total += len(s)
			}
		}
		if total > 8<<20 {
			t.Fatalf("expansion produced %d bytes; the cap did not hold", total)
		}
	})
}

// TestSourcesSplitPackagePkgname pins the cyrus-imapd shape: a split package
// whose source array references ${pkgname}, which bash expands to the
// array's first element.
func TestSourcesSplitPackagePkgname(t *testing.T) {
	dir := t.TempDir()
	content := `pkgbase=cyrus-imapd
pkgname=(cyrus-imapd cyrus-imapd-docs)
pkgver=3.6.1
pkgrel=1
arch=('x86_64')
source=(${pkgname}.service ${pkgname}.sysusers)
sha256sums=('9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08'
            '60303ae22b998861bce3b28f33eec1be758a213c86c93c076dbe9f558c11c752')
`
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	pkg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	srcs := pkg.Sources()
	if len(srcs) != 2 {
		t.Fatalf("expected 2 sources, got %d: %+v", len(srcs), srcs)
	}
	if srcs[0].Expanded != "cyrus-imapd.service" || !srcs[0].Local {
		t.Errorf("first source should expand to the local file cyrus-imapd.service: %+v", srcs[0])
	}
	if srcs[1].Expanded != "cyrus-imapd.sysusers" || !srcs[1].Local {
		t.Errorf("second source should expand to the local file cyrus-imapd.sysusers: %+v", srcs[1])
	}
}

// TestSourcesArchSuffixFilter pins that source_$CARCH counts only for an
// architecture the package declares. makepkg fetches no other, so a suffix
// naming none of them is an ordinary variable that collides with the namespace
// — and when arch itself cannot be pinned down, every suffix stays in: missing
// a real source array is worse than looking at one makepkg would skip.
func TestSourcesArchSuffixFilter(t *testing.T) {
	// urls returns url->arch for every entry, since Sources() walks a map and
	// the order between arrays is not fixed.
	urls := func(pkg *Package) map[string]string {
		out := map[string]string{}
		for _, e := range pkg.Sources() {
			out[e.URL] = e.Arch
		}
		return out
	}

	t.Run("skips a suffix no arch declares", func(t *testing.T) {
		pkg := loadPKGBUILD(t, `pkgname=demo
arch=('x86_64')
source_prefix="https://example.com/demo"
source=('common.conf')
source_x86_64=("${source_prefix}-amd64.tar.gz")
source_aarch64=("${source_prefix}-arm64.tar.gz")
`)
		want := map[string]string{
			"common.conf":                           "",
			"https://example.com/demo-amd64.tar.gz": "x86_64",
		}
		if got := urls(pkg); !reflect.DeepEqual(got, want) {
			t.Errorf("Sources() = %v, want %v", got, want)
		}
	})

	t.Run("keeps every suffix when a conditional sets arch", func(t *testing.T) {
		pkg := loadPKGBUILD(t, `pkgname=demo
if [[ $CARCH == aarch64 ]]; then arch=('aarch64'); else arch=('x86_64'); fi
source_x86_64=('amd64.tar.gz')
source_aarch64=('arm64.tar.gz')
`)
		want := map[string]string{"amd64.tar.gz": "x86_64", "arm64.tar.gz": "aarch64"}
		if got := urls(pkg); !reflect.DeepEqual(got, want) {
			t.Errorf("Sources() = %v, want %v", got, want)
		}
	})

	t.Run("keeps every suffix when arch names a variable", func(t *testing.T) {
		pkg := loadPKGBUILD(t, `pkgname=demo
arch=("$CARCH")
source_riscv64=('riscv.tar.gz')
`)
		want := map[string]string{"riscv.tar.gz": "riscv64"}
		if got := urls(pkg); !reflect.DeepEqual(got, want) {
			t.Errorf("Sources() = %v, want %v", got, want)
		}
	})

	t.Run("keeps every suffix when arch is unset", func(t *testing.T) {
		pkg := loadPKGBUILD(t, `pkgname=demo
source_x86_64=('amd64.tar.gz')
`)
		want := map[string]string{"amd64.tar.gz": "x86_64"}
		if got := urls(pkg); !reflect.DeepEqual(got, want) {
			t.Errorf("Sources() = %v, want %v", got, want)
		}
	})

	t.Run("counts an arch a brace group writes", func(t *testing.T) {
		pkg := loadPKGBUILD(t, `pkgname=demo
arch=({i,x}686)
source_i686=('x86.tar.gz')
source_armv7h=('arm.tar.gz')
`)
		want := map[string]string{"x86.tar.gz": "i686"}
		if got := urls(pkg); !reflect.DeepEqual(got, want) {
			t.Errorf("Sources() = %v, want %v", got, want)
		}
	})
}

// TestParseSourceEntry pins makepkg's [filename::]url[#fragment][?query]
// splitting for the well-defined orderings.
func TestParseSourceEntry(t *testing.T) {
	for _, tc := range []struct {
		name     string
		raw      string
		filename string
		url      string
		proto    string
		vcs      string
		fragment map[string]string
		query    string
		local    bool
	}{
		{
			name:     "plain https tarball",
			raw:      "https://example.com/foo.tar.gz",
			url:      "https://example.com/foo.tar.gz",
			proto:    "https",
			fragment: map[string]string{},
		},
		{
			name:     "plain http tarball",
			raw:      "http://example.com/x.tar.gz",
			url:      "http://example.com/x.tar.gz",
			proto:    "http",
			fragment: map[string]string{},
		},
		{
			name:     "explicit filename",
			raw:      "foo.tar.gz::https://example.com/x.tar.gz",
			filename: "foo.tar.gz",
			url:      "https://example.com/x.tar.gz",
			proto:    "https",
			fragment: map[string]string{},
		},
		{
			name:     "vcs proto",
			raw:      "git+https://github.com/example/foo.git",
			url:      "git+https://github.com/example/foo.git",
			proto:    "git+https",
			vcs:      "git",
			fragment: map[string]string{},
		},
		{
			name:     "bare vcs proto",
			raw:      "git://github.com/example/foo.git",
			url:      "git://github.com/example/foo.git",
			proto:    "git",
			vcs:      "git",
			fragment: map[string]string{},
		},
		{
			name:     "vcs with commit fragment",
			raw:      "git+https://github.com/example/foo.git#commit=abc123",
			url:      "git+https://github.com/example/foo.git",
			proto:    "git+https",
			vcs:      "git",
			fragment: map[string]string{"commit": "abc123"},
		},
		{
			// Canonical makepkg order. Guard against regressing the common
			// case while making the reversed order work.
			name:     "fragment then query",
			raw:      "git+https://x/y.git#tag=v1?signed",
			url:      "git+https://x/y.git",
			proto:    "git+https",
			vcs:      "git",
			fragment: map[string]string{"tag": "v1"},
			query:    "signed",
		},
		{
			// The reversed order used to swallow the whole "#commit=abc" into
			// Query, leaving Fragment empty — so a commit-pin check saw an
			// unpinned VCS source and misfired.
			name:     "query then fragment",
			raw:      "git+https://x/y.git?signed#commit=abc",
			url:      "git+https://x/y.git",
			proto:    "git+https",
			vcs:      "git",
			fragment: map[string]string{"commit": "abc"},
			query:    "signed",
		},
		{
			name:     "query only",
			raw:      "git+https://x/y.git?signed",
			url:      "git+https://x/y.git",
			proto:    "git+https",
			vcs:      "git",
			fragment: map[string]string{},
			query:    "signed",
		},
		{
			name:     "fragment only",
			raw:      "git+https://x/y.git#branch=main",
			url:      "git+https://x/y.git",
			proto:    "git+https",
			vcs:      "git",
			fragment: map[string]string{"branch": "main"},
		},
		{
			// Pathological, not valid makepkg input. Pinned only to prove the
			// tail loop consumes one delimiter per iteration and terminates:
			// the URL stops at the first delimiter, valueless "#b"/"#c"
			// segments add no fragment, and the last query wins.
			name:     "repeated delimiters terminate",
			raw:      "a#b#c?d?e",
			url:      "a",
			local:    true,
			fragment: map[string]string{},
			query:    "e",
		},
		{
			name:     "proto is lowercased",
			raw:      "HTTPS://example.com/x.tar.gz",
			url:      "HTTPS://example.com/x.tar.gz",
			proto:    "https",
			fragment: map[string]string{},
		},
		{
			name:     "local file",
			raw:      "local-file.patch",
			url:      "local-file.patch",
			local:    true,
			fragment: map[string]string{},
		},
		{
			name:     "local file with explicit name",
			raw:      "renamed.patch::local-file.patch",
			filename: "renamed.patch",
			url:      "local-file.patch",
			local:    true,
			fragment: map[string]string{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := parseSourceEntry(tc.raw, tc.raw)
			if e.Raw != tc.raw || e.Expanded != tc.raw {
				t.Errorf("Raw/Expanded = %q/%q, want %q", e.Raw, e.Expanded, tc.raw)
			}
			if e.Filename != tc.filename {
				t.Errorf("Filename = %q, want %q", e.Filename, tc.filename)
			}
			if e.URL != tc.url {
				t.Errorf("URL = %q, want %q", e.URL, tc.url)
			}
			if e.Proto != tc.proto {
				t.Errorf("Proto = %q, want %q", e.Proto, tc.proto)
			}
			if e.VCS != tc.vcs {
				t.Errorf("VCS = %q, want %q", e.VCS, tc.vcs)
			}
			if !reflect.DeepEqual(e.Fragment, tc.fragment) {
				t.Errorf("Fragment = %v, want %v", e.Fragment, tc.fragment)
			}
			if e.Query != tc.query {
				t.Errorf("Query = %q, want %q", e.Query, tc.query)
			}
			if e.Local != tc.local {
				t.Errorf("Local = %v, want %v", e.Local, tc.local)
			}
		})
	}
}

// TestSourcePositions pins that each entry carries its own written element's
// position. Every entry used to report the position of the `source=(` token,
// so a finding about the third source pointed at the first line of the array.
func TestSourcePositions(t *testing.T) {
	t.Run("one element per line", func(t *testing.T) {
		// `source=(` opens on line 3; the three elements sit on lines 3, 4, 5,
		// each indented to column 9.
		pkg := loadPKGBUILD(t, `pkgname=demo
pkgver=1.0.0
source=("https://example.com/a.tar.gz"
        "https://example.com/b.tar.gz"
        "https://example.com/c.tar.gz")
`)
		srcs := pkg.Sources()
		if len(srcs) != 3 {
			t.Fatalf("Sources() = %d entries, want 3: %+v", len(srcs), srcs)
		}
		for i, want := range []struct{ line, col uint }{{3, 9}, {4, 9}, {5, 9}} {
			if got := srcs[i].Pos; got.Line() != want.line || got.Col() != want.col {
				t.Errorf("Sources()[%d] (%s) at %d:%d, want %d:%d",
					i, srcs[i].URL, got.Line(), got.Col(), want.line, want.col)
			}
		}
	})

	t.Run("brace expansion shares the element position", func(t *testing.T) {
		// Both expansions come from one written element, so both report it:
		// they genuinely occupy the same source line.
		pkg := loadPKGBUILD(t, `pkgname=demo
source=("https://example.com/a.tar.gz"
        "https://example.com/b.tar.gz"{,.sig})
`)
		srcs := pkg.Sources()
		if len(srcs) != 3 {
			t.Fatalf("Sources() = %d entries, want 3: %+v", len(srcs), srcs)
		}
		for i, want := range []struct{ line, col uint }{{2, 9}, {3, 9}, {3, 9}} {
			if got := srcs[i].Pos; got.Line() != want.line || got.Col() != want.col {
				t.Errorf("Sources()[%d] (%s) at %d:%d, want %d:%d",
					i, srcs[i].URL, got.Line(), got.Col(), want.line, want.col)
			}
		}
		for i, want := range []int{0, 1, 1} {
			if got := srcs[i].ElemIndex; got != want {
				t.Errorf("Sources()[%d].ElemIndex = %d, want %d", i, got, want)
			}
		}
	})

	t.Run("appended elements fall back to the array position", func(t *testing.T) {
		// A merged `+=` Var keeps the first assignment's Assign, so appended
		// values have no AST element to take a position from. They report the
		// base array's position rather than a wrong one — the accepted
		// limitation of merging appends.
		pkg := loadPKGBUILD(t, `pkgname=demo
source=("https://example.com/a.tar.gz")
source+=("https://example.com/b.tar.gz")
`)
		srcs := pkg.Sources()
		if len(srcs) != 2 {
			t.Fatalf("Sources() = %d entries, want 2: %+v", len(srcs), srcs)
		}
		if got := srcs[0].Pos; got.Line() != 2 || got.Col() != 9 {
			t.Errorf("base element at %d:%d, want 2:9", got.Line(), got.Col())
		}
		if got := srcs[1].Pos; got.Line() != 2 || got.Col() != 1 {
			t.Errorf("appended element at %d:%d, want the array position 2:1", got.Line(), got.Col())
		}
	})

	t.Run("scalar source keeps the assignment position", func(t *testing.T) {
		// A scalar `source=` has no Array, so there is no element to index.
		pkg := loadPKGBUILD(t, `pkgname=demo
source=https://example.com/a.tar.gz
`)
		srcs := pkg.Sources()
		if len(srcs) != 1 {
			t.Fatalf("Sources() = %d entries, want 1: %+v", len(srcs), srcs)
		}
		if got := srcs[0].Pos; got.Line() != 2 || got.Col() != 1 {
			t.Errorf("scalar source at %d:%d, want 2:1", got.Line(), got.Col())
		}
	})
}

// TestSourceEntryHost pins hostname extraction, including the VCS prefix strip.
func TestSourceEntryHost(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want string
	}{
		{"https://Example.COM/foo.tar.gz", "example.com"},
		{"git+https://github.com/example/foo.git", "github.com"},
		{"local-file.patch", ""},
	} {
		if got := parseSourceEntry(tc.raw, tc.raw).Host(); got != tc.want {
			t.Errorf("Host(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

// TestChecksums pins how sums arrays are collected per arch and paired with
// source entries by index.
func TestChecksums(t *testing.T) {
	t.Run("base arrays pair by index", func(t *testing.T) {
		pkg := loadPKGBUILD(t, `pkgname=demo
pkgver=1.0.0
source=('a.tar.gz' 'b.tar.gz')
sha256sums=('aaaa' 'bbbb')
`)
		sums := pkg.Checksums("")
		if !reflect.DeepEqual(sums, map[string][]string{"sha256": {"aaaa", "bbbb"}}) {
			t.Fatalf(`Checksums("") = %v, want {sha256: [aaaa bbbb]}`, sums)
		}
		srcs := pkg.Sources()
		if len(srcs) != 2 {
			t.Fatalf("Sources() = %d entries, want 2", len(srcs))
		}
		for i, want := range []string{"aaaa", "bbbb"} {
			if got := pkg.SumsFor(srcs[i]); !reflect.DeepEqual(got, []string{want}) {
				t.Errorf("SumsFor(index %d) = %v, want [%s]", i, got, want)
			}
		}
	})

	t.Run("multiple algorithms", func(t *testing.T) {
		pkg := loadPKGBUILD(t, `pkgname=demo
source=('a.tar.gz')
md5sums=('mmmm')
sha256sums=('aaaa')
`)
		sums := pkg.Checksums("")
		if !reflect.DeepEqual(sums, map[string][]string{"md5": {"mmmm"}, "sha256": {"aaaa"}}) {
			t.Fatalf(`Checksums("") = %v, want {md5: [mmmm], sha256: [aaaa]}`, sums)
		}
		got := pkg.SumsFor(pkg.Sources()[0])
		sort.Strings(got) // SumsFor iterates a map; order is not part of the contract
		if !reflect.DeepEqual(got, []string{"aaaa", "mmmm"}) {
			t.Errorf("SumsFor = %v, want [aaaa mmmm]", got)
		}
	})

	t.Run("arch-suffixed arrays", func(t *testing.T) {
		pkg := loadPKGBUILD(t, `pkgname=demo
source_x86_64=('a.tar.gz')
sha256sums_x86_64=('cccc')
`)
		if sums := pkg.Checksums("x86_64"); !reflect.DeepEqual(sums, map[string][]string{"sha256": {"cccc"}}) {
			t.Fatalf(`Checksums("x86_64") = %v, want {sha256: [cccc]}`, sums)
		}
		if sums := pkg.Checksums(""); len(sums) != 0 {
			t.Errorf(`Checksums("") = %v, want empty`, sums)
		}
		srcs := pkg.Sources()
		if len(srcs) != 1 || srcs[0].Arch != "x86_64" {
			t.Fatalf("Sources() = %+v, want one entry with Arch=x86_64", srcs)
		}
		if got := pkg.SumsFor(srcs[0]); !reflect.DeepEqual(got, []string{"cccc"}) {
			t.Errorf("SumsFor = %v, want [cccc]", got)
		}
	})

	t.Run("missing sum for index yields none", func(t *testing.T) {
		pkg := loadPKGBUILD(t, `pkgname=demo
source=('a.tar.gz' 'b.tar.gz')
sha256sums=('aaaa')
`)
		srcs := pkg.Sources()
		if got := pkg.SumsFor(srcs[1]); len(got) != 0 {
			t.Errorf("SumsFor(index 1) = %v, want none", got)
		}
	})
}
