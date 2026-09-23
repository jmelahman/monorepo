package rules

import "testing"

func TestRustGuidelineRules(t *testing.T) {
	t.Run("PB940 cargo test --release", func(t *testing.T) {
		expectRule(t, "PB940", map[string]string{"PKGBUILD": pkgbuildWith("", `
check() {
  cargo test --release --locked
}`)})
	})
	t.Run("PB940 cargo check --release", func(t *testing.T) {
		expectRule(t, "PB940", map[string]string{"PKGBUILD": pkgbuildWith("", `
check() {
  cargo check --release --locked
}`)})
	})
	t.Run("PB940 debug-profile tests are the guideline", func(t *testing.T) {
		expectNoRule(t, "PB940", map[string]string{"PKGBUILD": pkgbuildWith("", `
check() {
  cargo test --locked
}`)})
	})
	t.Run("PB940 -r is the same flag", func(t *testing.T) {
		expectRule(t, "PB940", map[string]string{"PKGBUILD": pkgbuildWith("", `
check() {
  cargo test -r --locked
}`)})
	})
	t.Run("PB940 --release reaching cargo through a variable still counts", func(t *testing.T) {
		expectRule(t, "PB940", map[string]string{"PKGBUILD": pkgbuildWith("", `
_cargo_flags="--release --locked"
check() {
  cargo test $_cargo_flags
}`)})
	})
	t.Run("PB940 --release after the separator is the test binary's", func(t *testing.T) {
		expectNoRule(t, "PB940", map[string]string{"PKGBUILD": pkgbuildWith("", `
_test_args="-- --release"
check() {
  cargo test --locked $_test_args
}`)})
	})
	t.Run("PB940 --release inside an array counts too", func(t *testing.T) {
		// The lib32-rav1e shape, where check() and package() each build their
		// own option array and --release only belongs in one of them.
		expectRule(t, "PB940", map[string]string{"PKGBUILD": pkgbuildWith("", `
check() {
  local cargo_options=(--release --frozen --offline)
  cargo test "${cargo_options[@]}"
}`)})
	})
	t.Run("PB940 release build in build() is not a test", func(t *testing.T) {
		expectNoRule(t, "PB940", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  cargo build --release --locked
}`)})
	})

	t.Run("PB941 cargo install without --no-track", func(t *testing.T) {
		expectRule(t, "PB941", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  cargo install --locked --root "$pkgdir/usr" --path .
}`)})
	})
	t.Run("PB941 --no-track is the fix", func(t *testing.T) {
		expectNoRule(t, "PB941", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  cargo install --locked --no-track --root "$pkgdir/usr" --path .
}`)})
	})
	t.Run("PB941 --no-track inside a variable counts", func(t *testing.T) {
		expectNoRule(t, "PB941", map[string]string{"PKGBUILD": pkgbuildWith("", `
_cargo_flags="--locked --no-track"
package() {
  cargo install $_cargo_flags --root "$pkgdir/usr" --path .
}`)})
	})
	t.Run("PB941 --no-track inside an array counts", func(t *testing.T) {
		expectNoRule(t, "PB941", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  _flags=(--locked --no-track)
  cargo install "${_flags[@]}" --root "$pkgdir/usr" --path .
}`)})
	})
	t.Run("PB941 flags pkglint cannot read stand down", func(t *testing.T) {
		expectNoRule(t, "PB941", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  cargo install $CARGO_INSTALL_FLAGS --root "$pkgdir/usr" --path .
}`)})
	})
	t.Run("PB941 an array without --no-track still reports", func(t *testing.T) {
		expectRule(t, "PB941", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  _flags=(--locked --frozen)
  cargo install "${_flags[@]}" --root "$pkgdir/usr" --path .
}`)})
	})
	t.Run("PB941 an unreadable install root is a value, not a flag", func(t *testing.T) {
		// $pkgdir is makepkg's, never the file's, so the root argument stays an
		// expansion — the guidelines' own invocation must still be readable.
		expectRule(t, "PB941", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  cargo install --locked --root "$pkgdir/usr" --path .
}`)})
	})

	t.Run("PB942 cargo build without --release", func(t *testing.T) {
		expectRule(t, "PB942", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  cargo build --locked
}`)})
	})
	t.Run("PB942 --release is the guideline", func(t *testing.T) {
		expectNoRule(t, "PB942", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  cargo build --release --locked
}`)})
	})
	t.Run("PB942 -r asks for the same profile", func(t *testing.T) {
		expectNoRule(t, "PB942", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  cargo build -r --locked
}`)})
	})
	t.Run("PB942 an explicit profile is a decision", func(t *testing.T) {
		expectNoRule(t, "PB942", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  cargo build --profile dist --locked
}`)})
	})
	t.Run("PB942 a release build alongside it is the shipped one", func(t *testing.T) {
		// Both profiles on purpose: the dev binary is what the test suite
		// needs, and the release one is what the package installs.
		expectNoRule(t, "PB942", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  cargo build --locked
  cargo build --locked --release
}`)})
	})
	t.Run("PB942 --release inside a variable is still --release", func(t *testing.T) {
		// The flags-in-one-place shape: build() and check() share a scalar,
		// which bash splits into words before cargo sees it.
		expectNoRule(t, "PB942", map[string]string{"PKGBUILD": pkgbuildWith("", `
_cargo_flags="--release --locked"
build() {
  cargo build $_cargo_flags
}`)})
	})
	t.Run("PB942 a profile inside a variable is a decision too", func(t *testing.T) {
		expectNoRule(t, "PB942", map[string]string{"PKGBUILD": pkgbuildWith("", `
_cargo_flags="--locked --profile dist"
build() {
  cargo build $_cargo_flags
}`)})
	})
	t.Run("PB942 --release inside an array is still --release", func(t *testing.T) {
		// The i3status-rust-full-git shape: build() collects its flags in an
		// array it reuses across cargo invocations, --release among them, and
		// package() installs out of target/release. Each element is one word
		// however it is quoted, so the feature list stays a feature list.
		expectNoRule(t, "PB942", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  myflags=(
    --release
    --features 'pulseaudio notmuch'
  )
  cargo build "${myflags[@]}" --package demo
}`)})
	})
	t.Run("PB942 an array without --release still reports", func(t *testing.T) {
		expectRule(t, "PB942", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  myflags=(--locked --features 'pulseaudio notmuch')
  cargo build "${myflags[@]}" --package demo
}`)})
	})
	t.Run("PB942 --release from a local counts", func(t *testing.T) {
		// An assignment inside the function takes the file-level name out of
		// scope, so the argument reaches the rule as an expansion and has to
		// be read back out of the function.
		expectNoRule(t, "PB942", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  local flags="--release --locked"
  cargo build $flags
}`)})
	})
	t.Run("PB942 one element of an array is not the array", func(t *testing.T) {
		// Which element ${myflags[0]} is depends on order pkglint does not
		// track, so the word stays unread and could be the missing --release.
		expectNoRule(t, "PB942", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  myflags=(--release --locked)
  cargo build "${myflags[0]}"
}`)})
	})
	t.Run("PB942 a word that is more than an expansion stands down", func(t *testing.T) {
		// Two variables glued into one word are not one variable's contents.
		expectNoRule(t, "PB942", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  local a=-- b=release
  cargo build --locked "$a$b"
}`)})
	})
	t.Run("PB942 flags from the environment stand down", func(t *testing.T) {
		expectNoRule(t, "PB942", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  cargo build $CARGO_ARGS
}`)})
	})
	t.Run("PB942 a readable dev build beside an unreadable one still reports", func(t *testing.T) {
		expectRule(t, "PB942", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  cargo build --locked
  cargo build $CARGO_ARGS
}`)})
	})
	t.Run("PB942 an expansion past the separator is the compiler's", func(t *testing.T) {
		// cargo reads no flags of its own after --, so nothing in there can be
		// the --release this build is missing.
		expectRule(t, "PB942", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  cargo build --locked -- $RUSTC_ARGS
}`)})
	})
	t.Run("PB942 an opaque value is not a hidden flag", func(t *testing.T) {
		// Only the flag words matter: a feature list nobody can resolve says
		// nothing about the profile, and the build is still on the dev one.
		expectRule(t, "PB942", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  cargo build --locked --features "${_features[@]}"
}`)})
	})
	t.Run("PB942 cargo build outside build() is not the shipped artifact", func(t *testing.T) {
		expectNoRule(t, "PB942", map[string]string{"PKGBUILD": pkgbuildWith("", `
prepare() {
  cargo build --locked
}`)})
	})

	t.Run("PB944 cargo without rust in makedepends", func(t *testing.T) {
		files := map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  cargo build --release --locked
}
check() {
  cargo test --locked
}`)}
		if n := ruleIDs(lint(t, files))["PB944"]; n != 1 {
			t.Errorf("PB944 fired %d times, want exactly 1", n)
		}
	})
	t.Run("PB944 rust in makedepends satisfies it", func(t *testing.T) {
		expectNoRule(t, "PB944", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('rust')
build() {
  cargo build --release --locked
}`)})
	})
	t.Run("PB944 rustup counts too", func(t *testing.T) {
		expectNoRule(t, "PB944", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('rustup')
build() {
  cargo build --release --locked
}`)})
	})
	t.Run("PB944 rust in depends counts", func(t *testing.T) {
		expectNoRule(t, "PB944", map[string]string{"PKGBUILD": pkgbuildWith("", `
depends=('rust')
build() {
  cargo build --release --locked
}`)})
	})
}
