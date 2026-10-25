package rules

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestCMakeGuidelineRules(t *testing.T) {
	t.Run("PB950 configure without a prefix", func(t *testing.T) {
		expectRule(t, "PB950", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('cmake')
build() {
  cmake -B build -S .
  cmake --build build
}`)})
	})
	t.Run("PB950 --build and --install are not configures", func(t *testing.T) {
		expectNoRule(t, "PB950", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('cmake')
build() {
  cmake -B build -S . -DCMAKE_INSTALL_PREFIX=/usr
  cmake --build build
}
package() {
  DESTDIR="$pkgdir" cmake --install build
}`)})
	})
	t.Run("PB950 split -D spelling counts", func(t *testing.T) {
		expectNoRule(t, "PB950", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('cmake')
build() {
  cmake -B build -S . -D CMAKE_INSTALL_PREFIX=/usr
}`)})
	})
	t.Run("PB950 an explicit /opt prefix is a decision", func(t *testing.T) {
		expectNoRule(t, "PB950", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('cmake')
build() {
  cmake -B build -S . -DCMAKE_INSTALL_PREFIX=/opt/demo
}`)})
	})
	t.Run("PB950 typed define spelling counts", func(t *testing.T) {
		expectNoRule(t, "PB950", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('cmake')
build() {
  cmake -B build -S . -DCMAKE_INSTALL_PREFIX:PATH=/usr
}`)})
	})
	t.Run("PB950 a staging prefix decides the install location", func(t *testing.T) {
		// The wasi-libc++ shape.
		expectNoRule(t, "PB950", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('cmake')
build() {
  cmake -B build -S . -DCMAKE_STAGING_PREFIX=/usr/share/wasi-sysroot
}`)})
	})
	t.Run("PB950 cmake's --install-prefix flag counts", func(t *testing.T) {
		expectNoRule(t, "PB950", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('cmake')
build() {
  cmake -B build -S . --install-prefix /usr
  cmake -B build2 -S . --install-prefix=/usr
}`)})
	})
	t.Run("PB950 a splatted flag array may carry the prefix", func(t *testing.T) {
		// The octopi shape: the flags live in a local array.
		expectNoRule(t, "PB950", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('cmake')
build() {
  local _cmake_args=(
    -DCMAKE_INSTALL_PREFIX=/usr
    -DCMAKE_BUILD_TYPE=None
  )
  cmake -B build -S . "${_cmake_args[@]}"
}`)})
	})
	t.Run("PB950 a bundled-deps configure beside a prefixed one is fine", func(t *testing.T) {
		// The neovim shape: cmake.deps installs nothing, the real tree sets
		// the prefix.
		expectNoRule(t, "PB950", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('cmake')
build() {
  cmake -S cmake.deps -B .deps -DUSE_BUNDLED=OFF
  cmake --build .deps
  cmake -B build -S . -DCMAKE_INSTALL_PREFIX=/usr
  cmake --build build
}`)})
	})

	t.Run("PB951 Release build type", func(t *testing.T) {
		expectRule(t, "PB951", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('cmake')
build() {
  cmake -B build -S . -DCMAKE_INSTALL_PREFIX=/usr -DCMAKE_BUILD_TYPE=Release
}`)})
	})
	t.Run("PB951 typed Release spelling counts", func(t *testing.T) {
		expectRule(t, "PB951", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('cmake')
build() {
  cmake -B build -S . -DCMAKE_INSTALL_PREFIX=/usr -DCMAKE_BUILD_TYPE:STRING=Release
}`)})
	})
	t.Run("PB951 explicitly emptied flags leave Release the only -O", func(t *testing.T) {
		// The llvm device-runtime shape: cmake never reads $CFLAGS here.
		expectNoRule(t, "PB951", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('cmake')
build() {
  cmake -B build -S . -DCMAKE_INSTALL_PREFIX=/usr -DCMAKE_BUILD_TYPE=Release -DCMAKE_C_FLAGS= -DCMAKE_CXX_FLAGS=
}`)})
	})
	t.Run("PB951 a hand-written C flag list leaves Release the only -O", func(t *testing.T) {
		expectNoRule(t, "PB951", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('cmake')
build() {
  cmake -B build -S . -DCMAKE_INSTALL_PREFIX=/usr -DCMAKE_BUILD_TYPE=Release \
    -DCMAKE_C_FLAGS="-fno-exceptions --sysroot=/usr/share/wasi-sysroot"
}`)})
	})
	t.Run("PB951 Release flags defined by the configure are the PKGBUILD's choice", func(t *testing.T) {
		expectNoRule(t, "PB951", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('cmake')
build() {
  cmake -B build -S . -DCMAKE_BUILD_TYPE=Release -DCMAKE_C_FLAGS_RELEASE="$CFLAGS" -DCMAKE_CXX_FLAGS_RELEASE="$CXXFLAGS"
}`)})
	})
	t.Run("PB951 Release flags defined to something else still override Arch's", func(t *testing.T) {
		expectRule(t, "PB951", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('cmake')
build() {
  cmake -B build -S . -DCMAKE_INSTALL_PREFIX=/usr -DCMAKE_BUILD_TYPE=Release -DCMAKE_CXX_FLAGS_RELEASE="-O3 -DNDEBUG"
}`)})
	})
	t.Run("PB951 explicit flags that carry $CFLAGS still lose -O2", func(t *testing.T) {
		expectRule(t, "PB951", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('cmake')
build() {
  cmake -B build -S . -DCMAKE_INSTALL_PREFIX=/usr -DCMAKE_BUILD_TYPE=Release -DCMAKE_C_FLAGS="${CFLAGS} -fPIC"
}`)})
	})
	t.Run("PB951 a variable built from $CFLAGS still loses -O2", func(t *testing.T) {
		// The lib32-llvm shape.
		expectRule(t, "PB951", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('cmake')
build() {
  local LIB32_CFLAGS="$CFLAGS -m32"
  cmake -B build -S . -DCMAKE_INSTALL_PREFIX=/usr -DCMAKE_BUILD_TYPE=Release -DCMAKE_C_FLAGS="$LIB32_CFLAGS"
}`)})
	})
	t.Run("PB951 a variable built without $CFLAGS leaves Release the only -O", func(t *testing.T) {
		expectNoRule(t, "PB951", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('cmake')
build() {
  local _cflags="-fno-exceptions"
  cmake -B build -S . -DCMAKE_INSTALL_PREFIX=/usr -DCMAKE_BUILD_TYPE=Release -DCMAKE_C_FLAGS="$_cflags"
}`)})
	})
	t.Run("PB951 a variable it cannot follow may carry $CFLAGS", func(t *testing.T) {
		expectRule(t, "PB951", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('cmake')
build() {
  cmake -B build -S . -DCMAKE_INSTALL_PREFIX=/usr -DCMAKE_BUILD_TYPE=Release -DCMAKE_C_FLAGS="$MYSTERY_FLAGS"
}`)})
	})
	for _, tc := range []struct{ name, body string }{
		{"a substituted $CFLAGS", `  cmake -B build -DCMAKE_BUILD_TYPE=Release -DCMAKE_C_FLAGS="${CFLAGS/-O2/-O3}"`},
		{"a defaulted $CXXFLAGS", `  cmake -B build -DCMAKE_BUILD_TYPE=Release -DCMAKE_CXX_FLAGS="${CXXFLAGS:-} -DFOO"`},
		{"a variable substituting $CFLAGS", `  local _f="${CFLAGS/-O2/-O3}"
  cmake -B build -DCMAKE_BUILD_TYPE=Release -DCMAKE_C_FLAGS="$_f"`},
		{"an array holding $CFLAGS", `  local _f=("$CFLAGS" -m32)
  cmake -B build -DCMAKE_BUILD_TYPE=Release -DCMAKE_C_FLAGS="${_f[*]}"`},
		{"a command substitution", `  cmake -B build -DCMAKE_BUILD_TYPE=Release -DCMAKE_C_FLAGS="$(get_flags)"`},
		{"C++ flags from $CXXFLAGS beside hand-written C flags", `  cmake -B build -DCMAKE_BUILD_TYPE=Release -DCMAKE_C_FLAGS="-O0" -DCMAKE_CXX_FLAGS="$CXXFLAGS"`},
	} {
		t.Run("PB951 sees Arch's flags through "+tc.name, func(t *testing.T) {
			expectRule(t, "PB951", map[string]string{"PKGBUILD": pkgbuildWith("", "\nmakedepends=('cmake')\nbuild() {\n"+tc.body+"\n}")})
		})
	}
	t.Run("PB951 sees a top-level CFLAGS the file extends", func(t *testing.T) {
		// Rendering substitutes top-level assignments; the reference must be
		// read from the source word or it vanishes.
		expectRule(t, "PB951", map[string]string{"PKGBUILD": pkgbuildWith(`CFLAGS+=" -fPIC"`, `
makedepends=('cmake')
build() {
  cmake -B build -DCMAKE_BUILD_TYPE=Release -DCMAKE_C_FLAGS="$CFLAGS"
}`)})
	})
	for _, tc := range []struct{ name, body string }{
		{"a list built up from literals", `  local _flags="-a"
  _flags="$_flags -b"
  cmake -B build -DCMAKE_BUILD_TYPE=Release -DCMAKE_C_FLAGS="$_flags"`},
		{"makepkg's path variables", `  cmake -B build -DCMAKE_BUILD_TYPE=Release -DCMAKE_C_FLAGS="-I$srcdir/include --sysroot=${srcdir}/sysroot"`},
	} {
		t.Run("PB951 hand-written flags through "+tc.name, func(t *testing.T) {
			expectNoRule(t, "PB951", map[string]string{"PKGBUILD": pkgbuildWith("", "\nmakedepends=('cmake')\nbuild() {\n"+tc.body+"\n}")})
		})
	}
	t.Run("PB951 a wide variable chain is read in linear time", func(t *testing.T) {
		// Each variable references the next many times; following every
		// reference anew is exponential in the chain's depth.
		var b strings.Builder
		b.WriteString("\nmakedepends=('cmake')\nbuild() {\n  local _v0=\"-x\"\n")
		for i := 1; i <= 8; i++ {
			fmt.Fprintf(&b, "  local _v%d=\"%s\"\n", i, strings.Repeat(fmt.Sprintf("$_v%d ", i-1), 100))
		}
		b.WriteString("  cmake -B build -DCMAKE_BUILD_TYPE=Release -DCMAKE_C_FLAGS=\"$_v8\"\n}")
		start := time.Now()
		expectNoRule(t, "PB951", map[string]string{"PKGBUILD": pkgbuildWith("", b.String())})
		if d := time.Since(start); d > 5*time.Second {
			t.Errorf("took %v", d)
		}
	})
	t.Run("PB951 None is the guideline", func(t *testing.T) {
		expectNoRule(t, "PB951", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('cmake')
build() {
  cmake -B build -S . -DCMAKE_INSTALL_PREFIX=/usr -DCMAKE_BUILD_TYPE=None
}`)})
	})

	t.Run("PB952 cmake without makedepends", func(t *testing.T) {
		expectRule(t, "PB952", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  cmake -B build -S . -DCMAKE_INSTALL_PREFIX=/usr
}`)})
	})
	t.Run("PB952 declared makedepends satisfy it", func(t *testing.T) {
		expectNoRule(t, "PB952", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('cmake')
build() {
  cmake -B build -S . -DCMAKE_INSTALL_PREFIX=/usr
}`)})
	})
	t.Run("PB952 a cross wrapper package satisfies it", func(t *testing.T) {
		// mingw-w64-cmake pulls cmake in; the raw `cmake --build` still works.
		expectNoRule(t, "PB952", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('mingw-w64-cmake')
build() {
  cmake --build build
}`)})
	})

	t.Run("PB953 meson setup without --prefix", func(t *testing.T) {
		expectRule(t, "PB953", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('meson')
build() {
  meson setup build
}`)})
	})
	t.Run("PB953 --prefix in any spelling counts", func(t *testing.T) {
		for _, flag := range []string{"--prefix=/usr", "--prefix /usr", "-Dprefix=/usr"} {
			expectNoRule(t, "PB953", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('meson')
build() {
  meson setup build `+flag+`
}`)})
		}
	})
	t.Run("PB953 meson compile is not a configure", func(t *testing.T) {
		expectNoRule(t, "PB953", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('meson')
build() {
  meson setup build --prefix=/usr
  meson compile -C build
}`)})
	})
	t.Run("PB953 legacy no-subcommand spelling is a configure", func(t *testing.T) {
		expectRule(t, "PB953", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('meson')
build() {
  meson build
}`)})
	})

	t.Run("PB954 meson without makedepends", func(t *testing.T) {
		expectRule(t, "PB954", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  meson setup build --prefix=/usr
}`)})
	})
	t.Run("PB954 arch-meson counts as meson usage", func(t *testing.T) {
		expectRule(t, "PB954", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  arch-meson . build
}`)})
	})
	t.Run("PB954 declared makedepends satisfy it", func(t *testing.T) {
		expectNoRule(t, "PB954", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('meson')
build() {
  meson setup build --prefix=/usr
}`)})
	})
	t.Run("PB954 a cross wrapper package satisfies it", func(t *testing.T) {
		expectNoRule(t, "PB954", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('mingw-w64-meson')
build() {
  meson compile -C build
}`)})
	})

	t.Run("PB955 bare ninja in a meson build", func(t *testing.T) {
		expectRule(t, "PB955", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('meson')
build() {
  meson setup build --prefix=/usr
  ninja -C build
}`)})
	})
	t.Run("PB955 meson compile is the wrapper", func(t *testing.T) {
		expectNoRule(t, "PB955", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('meson')
build() {
  meson setup build --prefix=/usr
  meson compile -C build
}`)})
	})
	t.Run("PB955 ninja without meson is another build system", func(t *testing.T) {
		expectNoRule(t, "PB955", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('cmake' 'ninja')
build() {
  cmake -B build -S . -G Ninja -DCMAKE_INSTALL_PREFIX=/usr
  ninja -C build
}`)})
	})
}
