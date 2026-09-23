package rules

import "testing"

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
