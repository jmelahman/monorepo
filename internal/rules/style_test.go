package rules

import (
	"strings"
	"testing"
)

// styleHeader is a minimal clean PKGBUILD head for style-rule tests. It
// carries a maintainer tag, metadata and a renamed source so the style rules
// under test are the only ones expected to fire on what each test adds.
const styleHeader = `# Maintainer: Sam Coder <sam@example.com>
pkgname=demo
pkgver=1.0.0
pkgrel=1
pkgdesc='A demonstration tool'
arch=('x86_64')
url='https://github.com/example/demo'
license=('MIT')
source=("$pkgname-$pkgver.tar.gz::https://github.com/example/demo/archive/v$pkgver.tar.gz")
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')
`

func styleLint(t *testing.T, snippet string) map[string]int {
	t.Helper()
	return ruleIDs(lint(t, map[string]string{"PKGBUILD": styleHeader + snippet}))
}

func TestSpecificHostArch(t *testing.T) {
	for _, tc := range []struct {
		name    string
		snippet string
		want    int
	}{
		{"hardcoded arch in build flags", "build() {\n  ./configure --host=x86_64-pc-linux-gnu\n}\n", 1},
		{"hardcoded arch in extra source", `source+=("https://example.com/demo-x86_64.bin")` + "\n", 1},
		{"CARCH on the same line is the fix", "build() {\n  ./configure --host=$CARCH-pc-linux-gnu\n}\n", 0},
		{"arch assignment itself is exempt", "", 0}, // header's arch=('x86_64')
		{"arch-suffixed field is exempt", `source_x86_64=("https://example.com/demo-x86_64.bin")` + "\n" +
			`sha256sums_x86_64=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')` + "\n", 0},
		{"same arch twice on a line reports once", "build() {\n  cp x86_64/demo out/x86_64/demo\n}\n", 1},
		{"comments are not flagged", "# works only on x86_64 for now\n", 0},

		// A $CARCH dispatch is the portable idiom; its per-architecture arms
		// necessarily sit below the line naming $CARCH.
		{"case arms under $CARCH are exempt", "build() {\n  case \"$CARCH\" in\n" +
			"    x86_64)  _target=amd64 ;;\n    aarch64) _target=arm64 ;;\n  esac\n}\n", 0},
		{"if branches testing $CARCH are exempt", "build() {\n  if [[ $CARCH == x86_64 ]]; then\n" +
			"    _target=amd64\n  else\n    _target=arm64\n  fi\n}\n", 0},
		{"case arms under uname -m are exempt", "build() {\n  case $(uname -m) in\n" +
			"    x86_64) _target=amd64 ;;\n  esac\n}\n", 0},
		{"a case on something else still reports", "build() {\n  case \"$_flavor\" in\n" +
			"    full) _target=x86_64 ;;\n  esac\n}\n", 1},

		// Foreign cross-compilation targets are not the host.
		{"mingw target triple is exempt", `_architectures='i686-w64-mingw32 x86_64-w64-mingw32'` + "\n", 0},
		{"bare-metal target triple is exempt", "makedepends=('arm-none-eabi-gcc')\n", 0},
		{"debian multiarch path is exempt", "build() {\n  cp /usr/lib/x86_64-linux-gnu/libdemo.so .\n}\n", 0},
		{"android cross package is exempt", "depends=('android-aarch64-qt6-base')\n", 0},

		// Names Arch never sets $CARCH to.
		{"i386 is not an Arch architecture", "build() {\n  make demo-i386\n}\n", 0},
		{"ppc is not an Arch architecture", "build() {\n  make demo-ppc\n}\n", 0},

		// arch=() belongs wherever it is written, not just at top level.
		{"arch=() in a split package function is exempt",
			"package_demo() {\n  arch=('x86_64' 'aarch64')\n  install -Dm755 demo \"$pkgdir/usr/bin/demo\"\n}\n", 0},
		{"arch=() shadowed by a later arch=() is exempt", "arch=('aarch64')\n", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := styleLint(t, tc.snippet)["PB901"]; got != tc.want {
				t.Errorf("got %d PB901 findings, want %d", got, tc.want)
			}
		})
	}
}

func TestUnprefixedCustomVars(t *testing.T) {
	for _, tc := range []struct {
		name    string
		snippet string
		want    int
	}{
		{"lowercase custom var", "commitish='abc123'\n", 1},
		{"underscore prefix is the fix", "_commitish='abc123'\n", 0},
		{"uppercase names are PB108's concern", "MYDIR='/tmp'\n", 0},
		{"schema fields are known", "makedepends=('cmake')\nnoextract=('demo.bin')\n", 0},
		{"arch-suffixed schema fields are known", "depends_x86_64=('glibc')\n", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := styleLint(t, tc.snippet)["PB902"]; got != tc.want {
				t.Errorf("got %d PB902 findings, want %d", got, tc.want)
			}
		})
	}
}

func TestStartdirReference(t *testing.T) {
	findings := lint(t, map[string]string{"PKGBUILD": styleHeader + `package() {
  cd "$startdir/src/demo"
  install -Dm755 demo "${startdir}/pkg/usr/bin/demo"
  echo "$startdir"
}
`})
	var msgs []string
	for _, f := range findings {
		if f.RuleID == "PB903" {
			msgs = append(msgs, f.Message)
		}
	}
	if len(msgs) != 3 {
		t.Fatalf("got %d PB903 findings, want 3: %v", len(msgs), msgs)
	}
	if !strings.Contains(msgs[0], "$srcdir") || !strings.Contains(msgs[1], "$pkgdir") {
		t.Errorf("expected targeted srcdir/pkgdir suggestions, got %v", msgs)
	}
}

func TestRedundantMakedepends(t *testing.T) {
	for _, tc := range []struct {
		name    string
		snippet string
		want    int
	}{
		{"same package in both", "depends=('libfoo')\nmakedepends=('libfoo')\n", 1},
		{"versioned depends still covers a bare makedep", "depends=('libfoo>=2')\nmakedepends=('libfoo')\n", 1},
		{"differing constraint is deliberate", "depends=('libfoo')\nmakedepends=('libfoo>=2')\n", 0},
		{"identical constraint is redundant", "depends=('libfoo>=2')\nmakedepends=('libfoo>=2')\n", 1},
		{"disjoint sets are fine", "depends=('libfoo')\nmakedepends=('cmake')\n", 0},
		{"arch-suffixed makedepends checked too", "depends=('libfoo')\nmakedepends_x86_64=('libfoo')\n", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := styleLint(t, tc.snippet)["PB904"]; got != tc.want {
				t.Errorf("got %d PB904 findings, want %d", got, tc.want)
			}
		})
	}
}

func TestSourceforgeMirror(t *testing.T) {
	for _, tc := range []struct {
		name    string
		snippet string
		want    int
	}{
		{"pinned mirror", `source=("https://heanet.dl.sourceforge.net/project/demo/demo-1.0.tar.gz")` + "\n", 1},
		{"bare dl host", `source=("https://dl.sf.net/project/demo/demo-1.0.tar.gz")` + "\n", 1},
		{"downloads redirector is correct", `source=("https://downloads.sourceforge.net/project/demo/demo-1.0.tar.gz")` + "\n", 0},
		{"unrelated host untouched", "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := styleLint(t, tc.snippet)["PB905"]; got != tc.want {
				t.Errorf("got %d PB905 findings, want %d", got, tc.want)
			}
		})
	}
}

func TestPkgnameInDesc(t *testing.T) {
	for _, tc := range []struct {
		name    string
		snippet string
		want    int
	}{
		{"name repeated as a word", "pkgdesc='The demo tool'\n", 1},
		{"case-insensitive match", "pkgdesc='Demo for benchmarks'\n", 1},
		{"substring of a longer word is fine", "pkgdesc='A demonstration tool'\n", 0},
		{"split package names count", "pkgname=('demo' 'demo-docs')\npkgdesc='Files for demo-docs users'\n", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := styleLint(t, tc.snippet)["PB906"]; got != tc.want {
				t.Errorf("got %d PB906 findings, want %d", got, tc.want)
			}
		})
	}
}

func TestMakepkgInternalFunctions(t *testing.T) {
	for _, tc := range []struct {
		name    string
		snippet string
		want    int
	}{
		{"msg inside build", "build() {\n  msg 'compiling'\n  make\n}\n", 1},
		{"error with argument", "check() {\n  error 'tests failed'\n}\n", 1},
		{"user-defined function of the same name", "error() {\n  printf '%s\\n' \"$1\" >&2\n}\nbuild() {\n  error 'custom'\n}\n", 0},
		{"bare word without arguments is not a call to the helper", "build() {\n  make || error\n}\n", 0},
		{"echo is fine", "build() {\n  echo 'compiling'\n  make\n}\n", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := styleLint(t, tc.snippet)["PB907"]; got != tc.want {
				t.Errorf("got %d PB907 findings, want %d", got, tc.want)
			}
		})
	}
}

func TestMaintainerComment(t *testing.T) {
	noTag := strings.Replace(styleHeader, "# Maintainer: Sam Coder <sam@example.com>\n", "", 1)
	if got := ruleIDs(lint(t, map[string]string{"PKGBUILD": noTag}))["PB908"]; got != 1 {
		t.Errorf("got %d PB908 findings without a maintainer tag, want 1", got)
	}
	if got := styleLint(t, "")["PB908"]; got != 0 {
		t.Errorf("got %d PB908 findings with a maintainer tag, want 0", got)
	}
	// A qualified tag still names the maintainer (electronmail-bin); a tag
	// without its colon does not parse as one.
	for tag, want := range map[string]int{
		"# Maintainer (since 5.1.8): Sam Coder": 0,
		"# Maintainers: Sam Coder":              0,
		"# Maintainer Sam Coder":                1,
	} {
		h := strings.Replace(styleHeader, "# Maintainer: Sam Coder", tag, 1)
		if got := ruleIDs(lint(t, map[string]string{"PKGBUILD": h}))["PB908"]; got != want {
			t.Errorf("%q: got %d PB908 findings, want %d", tag, got, want)
		}
	}
	// Namcap accepts arbitrary spacing around the tag.
	spaced := strings.Replace(styleHeader, "# Maintainer: Sam Coder", "#  maintainer  : Sam Coder", 1)
	if got := ruleIDs(lint(t, map[string]string{"PKGBUILD": spaced}))["PB908"]; got != 0 {
		t.Errorf("got %d PB908 findings with a spaced lowercase tag, want 0", got)
	}
}

func TestUppercasePkgname(t *testing.T) {
	if got := styleLint(t, "pkgname=Demo-Tool\n")["PB909"]; got != 1 {
		t.Errorf("got %d PB909 findings for an uppercase name, want 1", got)
	}
	if got := styleLint(t, "pkgbase=DemoSuite\npkgname=('demo')\n")["PB909"]; got != 1 {
		t.Errorf("got %d PB909 findings for an uppercase pkgbase, want 1", got)
	}
	if got := styleLint(t, "")["PB909"]; got != 0 {
		t.Errorf("got %d PB909 findings for a lowercase name, want 0", got)
	}
}

func TestMissingMetadata(t *testing.T) {
	noURL := strings.Replace(styleHeader, "url='https://github.com/example/demo'\n", "", 1)
	if got := ruleIDs(lint(t, map[string]string{"PKGBUILD": noURL}))["PB910"]; got != 1 {
		t.Errorf("got %d PB910 findings without url, want 1", got)
	}
	// A split package that sets the field inside its package function is fine.
	noURLSplit := noURL + "package_demo() {\n  url='https://github.com/example/demo'\n}\n"
	if got := ruleIDs(lint(t, map[string]string{"PKGBUILD": noURLSplit}))["PB910"]; got != 0 {
		t.Errorf("got %d PB910 findings when a package function sets url, want 0", got)
	}
	if got := styleLint(t, "pkgdesc=''\n")["PB910"]; got != 1 {
		t.Errorf("got %d PB910 findings for an empty pkgdesc, want 1", got)
	}
	// makepkg sources the file top to bottom before reading any metadata, so a
	// field set in a top-level branch is set — the parser just cannot say to
	// what, since which branch runs depends on the build host.
	condURL := noURL + "if [[ $CARCH == x86_64 ]]; then\n  url='https://example.com/x86'\nelse\n  url='https://example.com'\nfi\n"
	if got := ruleIDs(lint(t, map[string]string{"PKGBUILD": condURL}))["PB910"]; got != 0 {
		t.Errorf("got %d PB910 findings when a top-level conditional sets url, want 0", got)
	}
}

func TestNonUniqueSourceName(t *testing.T) {
	for _, tc := range []struct {
		name    string
		snippet string
		want    int
	}{
		{"bare version tarball", `source=("https://github.com/example/demo/archive/v1.2.3.tar.gz")` + "\n", 1},
		{"bare date tarball", `source=("https://example.com/releases/20240101.tar.gz")` + "\n", 1},
		{"renamed with ::", "", 0}, // header's source is renamed
		{"name-bearing basename", `source=("https://example.com/demo-1.2.3.tar.gz")` + "\n", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := styleLint(t, tc.snippet)["PB911"]; got != tc.want {
				t.Errorf("got %d PB911 findings, want %d", got, tc.want)
			}
		})
	}
}

func TestDuplicatedOptdepends(t *testing.T) {
	for _, tc := range []struct {
		name    string
		snippet string
		want    int
	}{
		{"same package in both", "depends=('ffmpeg')\noptdepends=('ffmpeg: video export')\n", 1},
		{"constraint on the depends side still duplicates", "depends=('ffmpeg>=6')\noptdepends=('ffmpeg: video export')\n", 1},
		{"distinct packages", "depends=('ffmpeg')\noptdepends=('gimp: image editing')\n", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := styleLint(t, tc.snippet)["PB912"]; got != tc.want {
				t.Errorf("got %d PB912 findings, want %d", got, tc.want)
			}
		})
	}
}

func TestVCSMakedepends(t *testing.T) {
	pinned := `source=("git+https://github.com/example/demo.git#commit=3f2b1a0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e5f4a")` + "\n"
	for _, tc := range []struct {
		name    string
		snippet string
		want    int
	}{
		{"git source without git makedep", pinned, 1},
		{"git in makedepends satisfies", "makedepends=('git')\n" + pinned, 0},
		{"git in depends satisfies too", "depends=('git')\n" + pinned, 0},
		{"two protocols report once each", "source=(" +
			`"git+https://github.com/example/demo.git#commit=3f2b1a0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e5f4a"` + " " +
			`"hg+https://hg.example.com/demo#revision=3f2b1a0c9d8e"` + ")\n", 2},
		{"plain tarball needs nothing", "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := styleLint(t, tc.snippet)["PB711"]; got != tc.want {
				t.Errorf("got %d PB711 findings, want %d", got, tc.want)
			}
		})
	}
}

func TestMalformedChecksums(t *testing.T) {
	for _, tc := range []struct {
		name    string
		snippet string
		want    int
	}{
		{"truncated sha256", "sha256sums=('deadbeef')\n", 1},
		{"non-hex sha256 of the right length", "sha256sums=('zzzzbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')\n", 1},
		{"SKIP is exempt", "sha256sums=('SKIP')\n", 0},
		{"md5 length checked per algorithm", "md5sums=('d41d8cd98f00b204e9800998ecf8427e')\n", 0},
		{"sha512 truncated", "sha512sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')\n", 1},
		{"arch-suffixed sums checked", "source_x86_64=(\"https://example.com/demo-x86_64.bin\")\nsha256sums_x86_64=('deadbeef')\n", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := styleLint(t, tc.snippet)["PB114"]; got != tc.want {
				t.Errorf("got %d PB114 findings, want %d", got, tc.want)
			}
		})
	}
}

func TestHookCoveredCommands(t *testing.T) {
	files := map[string]string{
		"PKGBUILD":     styleHeader + "install=demo.install\n",
		"demo.install": "post_install() {\n  update-desktop-database -q\n  gtk-update-icon-cache -q /usr/share/icons/hicolor\n}\n",
	}
	if got := ruleIDs(lint(t, files))["PB504"]; got != 2 {
		t.Errorf("got %d PB504 findings, want 2", got)
	}
	// The same commands inside the PKGBUILD (not a scriptlet) are not flagged.
	inBuild := styleHeader + "package() {\n  update-desktop-database -q\n}\n"
	if got := ruleIDs(lint(t, map[string]string{"PKGBUILD": inBuild}))["PB504"]; got != 0 {
		t.Errorf("got %d PB504 findings in a PKGBUILD body, want 0", got)
	}
}
