package rules

import (
	"strings"
	"testing"
)

func countSubstring(s, sub string) int { return strings.Count(s, sub) }

// mustClear runs the fixed source back through the linter and fails when the
// rule the fix exists for is still reported. Every fixer test here ends with
// it: an edit that leaves the finding standing is a diff for nothing.
func mustClear(t *testing.T, id, fixed string) {
	t.Helper()
	if fixed == "" {
		t.Fatalf("%s: the fix produced no change", id)
	}
	if n := ruleIDs(lint(t, map[string]string{"PKGBUILD": fixed}))[id]; n != 0 {
		t.Errorf("fixed PKGBUILD still has %d %s finding(s):\n%s", n, id, fixed)
	}
}

func TestFixCMakePrefix(t *testing.T) {
	t.Run("appended to the configure", func(t *testing.T) {
		body := `
build() {
  cmake -B build -S "$srcdir/demo-$pkgver"
  cmake --build build
}`
		// The prefix moves every installed file, so it is not a safe-level
		// rewrite.
		if got := fixOnly(t, map[string]string{"PKGBUILD": pkgbuildWith("", body)}, FixSafe, nil, "PB950"); len(got) != 0 {
			t.Errorf("FixSafe should not apply the unsafe PB950 fix, got:\n%s", got["PKGBUILD"])
		}
		got := fixPKGBUILD(t, body, FixUnsafe, nil)
		mustContain(t, got, `cmake -B build -S "$srcdir/demo-$pkgver" -DCMAKE_INSTALL_PREFIX=/usr`)
		// --build is not a configure and must be left exactly as it was.
		mustContain(t, got, "cmake --build build\n")
		mustClear(t, "PB950", got)
	})
	t.Run("a $srcdir value is not a hidden flag", func(t *testing.T) {
		// The unreadable word is a path, not an argument list: declining here
		// would cost the fix most of the real invocations it exists for.
		got := fixPKGBUILD(t, `
build() {
  cmake -S "$srcdir/demo" -B build
}`, FixUnsafe, nil)
		mustContain(t, got, "-DCMAKE_INSTALL_PREFIX=/usr")
	})
	t.Run("a flags variable from outside the file declines the fix", func(t *testing.T) {
		// bash splits $CMAKE_EXTRA_ARGS into however many words it holds, and
		// nothing in the PKGBUILD says what those are — one of them may be the
		// prefix, which cmake would then take twice. (A flags variable the
		// file itself assigns is readable, and gets the fix.)
		files := map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  cmake -B build $CMAKE_EXTRA_ARGS
}`)}
		if n := ruleIDs(lint(t, files))["PB950"]; n != 1 {
			t.Errorf("PB950 fired %d times on an unprefixed configure, want exactly 1", n)
		}
		if got := fixOnly(t, files, FixUnsafe, nil, "PB950"); len(got) != 0 {
			t.Errorf("a flag list pkglint cannot read must not be added to, got:\n%s", got["PKGBUILD"])
		}
	})
	t.Run("a flags variable the file assigns is readable and gets the fix", func(t *testing.T) {
		got := fixPKGBUILD(t, `
_cmake_opts='-DBUILD_TESTING=OFF'
build() {
  cmake -B build $_cmake_opts
}`, FixUnsafe, nil)
		mustContain(t, got, "cmake -B build $_cmake_opts -DCMAKE_INSTALL_PREFIX=/usr")
		mustClear(t, "PB950", got)
	})
	t.Run("a splatted array stands the rule down entirely", func(t *testing.T) {
		files := map[string]string{"PKGBUILD": pkgbuildWith("", `
_opts=(-DBUILD_TESTING=OFF)
build() {
  cmake -B build "${_opts[@]}"
}`)}
		if n := ruleIDs(lint(t, files))["PB950"]; n != 0 {
			t.Errorf("PB950 fired %d times where the flags are unreadable, want 0", n)
		}
		if got := fixOnly(t, files, FixUnsafe, nil, "PB950"); len(got) != 0 {
			t.Errorf("no finding, so no edit; got:\n%s", got["PKGBUILD"])
		}
	})
}

func TestFixCMakeBuildType(t *testing.T) {
	t.Run("only the value is rewritten", func(t *testing.T) {
		body := `
build() {
  cmake -B build -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX=/usr
}`
		if got := fixOnly(t, map[string]string{"PKGBUILD": pkgbuildWith("", body)}, FixSafe, nil, "PB951"); len(got) != 0 {
			t.Errorf("FixSafe should not apply the unsafe PB951 fix, got:\n%s", got["PKGBUILD"])
		}
		got := fixPKGBUILD(t, body, FixUnsafe, nil)
		mustContain(t, got, "-DCMAKE_BUILD_TYPE=None -DCMAKE_INSTALL_PREFIX=/usr")
		mustClear(t, "PB951", got)
	})
	t.Run("the typed spelling keeps its type", func(t *testing.T) {
		got := fixPKGBUILD(t, `
build() {
  cmake -B build -DCMAKE_BUILD_TYPE:STRING=Release -DCMAKE_INSTALL_PREFIX=/usr
}`, FixUnsafe, nil)
		mustContain(t, got, "-DCMAKE_BUILD_TYPE:STRING=None")
		mustClear(t, "PB951", got)
	})
	t.Run("the quoted spelling keeps its quotes", func(t *testing.T) {
		got := fixPKGBUILD(t, `
build() {
  cmake -B build "-DCMAKE_BUILD_TYPE=Release" -DCMAKE_INSTALL_PREFIX=/usr
}`, FixUnsafe, nil)
		mustContain(t, got, `"-DCMAKE_BUILD_TYPE=None"`)
		mustClear(t, "PB951", got)
	})
	t.Run("the split -D spelling rewrites the value word", func(t *testing.T) {
		got := fixPKGBUILD(t, `
build() {
  cmake -B build -D CMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX=/usr
}`, FixUnsafe, nil)
		mustContain(t, got, "-D CMAKE_BUILD_TYPE=None")
		mustClear(t, "PB951", got)
	})
}

func TestFixMesonPrefix(t *testing.T) {
	t.Run("appended to the setup", func(t *testing.T) {
		body := `
build() {
  meson setup build
  meson compile -C build
}`
		if got := fixOnly(t, map[string]string{"PKGBUILD": pkgbuildWith("", body)}, FixSafe, nil, "PB953"); len(got) != 0 {
			t.Errorf("FixSafe should not apply the unsafe PB953 fix, got:\n%s", got["PKGBUILD"])
		}
		got := fixPKGBUILD(t, body, FixUnsafe, nil)
		mustContain(t, got, "meson setup build --prefix=/usr")
		mustContain(t, got, "meson compile -C build\n")
		mustClear(t, "PB953", got)
	})
	t.Run("the legacy spelling with no setup verb", func(t *testing.T) {
		got := fixPKGBUILD(t, `
build() {
  meson build
}`, FixUnsafe, nil)
		mustContain(t, got, "meson build --prefix=/usr")
		mustClear(t, "PB953", got)
	})
	t.Run("a flags variable from outside the file declines the fix", func(t *testing.T) {
		files := map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  meson setup build $MESON_EXTRA_ARGS
}`)}
		if n := ruleIDs(lint(t, files))["PB953"]; n != 1 {
			t.Errorf("PB953 fired %d times, want exactly 1", n)
		}
		if got := fixOnly(t, files, FixUnsafe, nil, "PB953"); len(got) != 0 {
			t.Errorf("a flag list pkglint cannot read must not be added to, got:\n%s", got["PKGBUILD"])
		}
	})
}

func TestFixCargoInstallTracked(t *testing.T) {
	// The fixtures carry --locked so PB203's fix does not also fire and make
	// the assertions about where --no-track lands harder to read.
	t.Run("appended", func(t *testing.T) {
		body := `
package() {
  cargo install --locked --root "$pkgdir/usr" --path .
}`
		if got := fixOnly(t, map[string]string{"PKGBUILD": pkgbuildWith("", body)}, FixSafe, nil, "PB941"); len(got) != 0 {
			t.Errorf("FixSafe should not apply the unsafe PB941 fix, got:\n%s", got["PKGBUILD"])
		}
		got := fixPKGBUILD(t, body, FixUnsafe, nil)
		mustContain(t, got, `cargo install --locked --root "$pkgdir/usr" --path . --no-track`)
		mustClear(t, "PB941", got)
	})
	t.Run("written in front of the separator", func(t *testing.T) {
		// Past the separator the words belong to the installed program, which
		// has no --no-track and stops the build when handed one.
		got := fixPKGBUILD(t, `
package() {
  cargo install --locked --path . -- --features extra
}`, FixUnsafe, nil)
		mustContain(t, got, "cargo install --locked --path . --no-track -- --features extra")
		mustClear(t, "PB941", got)
	})
}

func TestFixNpmUserCache(t *testing.T) {
	t.Run("appended", func(t *testing.T) {
		body := `
build() {
  npm ci
}`
		if got := fixOnly(t, map[string]string{"PKGBUILD": pkgbuildWith("", body)}, FixSafe, nil, "PB980"); len(got) != 0 {
			t.Errorf("FixSafe should not apply the unsafe PB980 fix, got:\n%s", got["PKGBUILD"])
		}
		got := fixPKGBUILD(t, body, FixUnsafe, nil)
		mustContain(t, got, `npm ci --cache "$srcdir/npm-cache"`)
		mustClear(t, "PB980", got)
	})
	t.Run("a flags variable from outside the file declines the fix", func(t *testing.T) {
		files := map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  npm ci $NPM_EXTRA_ARGS
}`)}
		if n := ruleIDs(lint(t, files))["PB980"]; n != 1 {
			t.Errorf("PB980 fired %d times, want exactly 1", n)
		}
		if got := fixOnly(t, files, FixUnsafe, nil, "PB980"); len(got) != 0 {
			t.Errorf("a flag list pkglint cannot read must not be added to, got:\n%s", got["PKGBUILD"])
		}
	})
}

func TestFixGoCgoFlags(t *testing.T) {
	t.Run("exports written at the top of build()", func(t *testing.T) {
		body := `
build() {
  cd "demo-$pkgver"
  go build -o demo .
}`
		// The safe level fixes other things in this build (PB916's
		// -modcacherw among them); what it must not do is export the flags.
		safe := fixAll(t, map[string]string{"PKGBUILD": pkgbuildWith("", body)}, FixSafe, nil)
		mustNotContain(t, safe["PKGBUILD"], "CGO_CFLAGS")
		got := fixPKGBUILD(t, body, FixUnsafe, nil)
		mustContain(t, got, `  export CGO_CPPFLAGS="$CPPFLAGS"
  export CGO_CFLAGS="$CFLAGS"
  export CGO_CXXFLAGS="$CXXFLAGS"
  export CGO_LDFLAGS="$LDFLAGS"
  cd "demo-$pkgver"`)
		mustClear(t, "PB917", got)
	})
	t.Run("one block for a build with several go commands", func(t *testing.T) {
		got := fixPKGBUILD(t, `
build() {
  go build -o demo ./cmd/demo
  go build -o helper ./cmd/helper
}`, FixUnsafe, nil)
		if n := countSubstring(got, "export CGO_CFLAGS"); n != 1 {
			t.Errorf("wrote %d export blocks, want exactly 1:\n%s", n, got)
		}
		mustClear(t, "PB917", got)
	})
	t.Run("a one-line body declines the fix", func(t *testing.T) {
		// Whole lines cannot be inserted where the body has no line of its
		// own, so the finding stands rather than the fix breaking the build.
		files := map[string]string{"PKGBUILD": pkgbuildWith("", `
build() { go build -o demo .; }`)}
		if n := ruleIDs(lint(t, files))["PB917"]; n != 1 {
			t.Errorf("PB917 fired %d times, want exactly 1", n)
		}
		got := fixAll(t, files, FixUnsafe, nil)
		mustNotContain(t, got["PKGBUILD"], "CGO_CFLAGS")
		if n := ruleIDs(lint(t, map[string]string{"PKGBUILD": got["PKGBUILD"]}))["PB917"]; n != 1 {
			t.Errorf("the finding must stand when the fix declines, got %d", n)
		}
	})
	t.Run("a directive at the go command blocks the fix", func(t *testing.T) {
		// The edit inserts at the top of build(), but the finding — and the
		// directive waiving it — is at the go command; the edit answers there.
		files := map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  cd "demo-$pkgver"
  # pkglint: ignore=PB917
  go build -o demo .
}`)}
		if n := ruleIDs(lint(t, files))["PB917"]; n != 0 {
			t.Fatalf("the finding should be suppressed, got %d", n)
		}
		if got := fixOnly(t, files, FixUnsafe, nil, "PB917"); len(got) != 0 {
			t.Errorf("a suppressed finding must not be fixed, got:\n%s", got["PKGBUILD"])
		}
	})
	t.Run("CGO_ENABLED=0 needs no exports", func(t *testing.T) {
		got := fixAll(t, map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  export CGO_ENABLED=0
  go build -o demo .
}`)}, FixUnsafe, nil)
		mustNotContain(t, got["PKGBUILD"], "CGO_CFLAGS")
	})
}
