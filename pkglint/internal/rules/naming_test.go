package rules

import "testing"

// namedHeader builds a PKGBUILD head for the prefix-family tests around a
// package name and arch value.
func namedHeader(name, arch, extra string) string {
	return `pkgname=` + name + `
pkgver=1.0.0
pkgrel=1
pkgdesc='A demonstration package'
arch=(` + arch + `)
url='https://example.com/demo'
license=('MIT')
` + extra + `source=("https://example.com/demo-$pkgver.tar.gz")
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')`
}

func TestFontPackageRules(t *testing.T) {
	t.Run("PB970 concrete arch on a font package", func(t *testing.T) {
		expectRule(t, "PB970", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("ttf-demo", "'x86_64'", ""), "")})
		expectRule(t, "PB970", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("otf-demo", "'x86_64'", ""), "")})
	})
	t.Run("PB970 any is the guideline", func(t *testing.T) {
		expectNoRule(t, "PB970", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("ttf-demo", "'any'", ""), "")})
	})
	t.Run("PB970 non-font packages are exempt", func(t *testing.T) {
		expectNoRule(t, "PB970", map[string]string{"PKGBUILD": pkgbuildWith("", "")})
	})
	t.Run("PB971 font package with depends", func(t *testing.T) {
		expectRule(t, "PB971", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("ttf-demo", "'any'", "depends=('fontconfig')\n"), "")})
	})
	t.Run("PB971 depends on a non-font package is not this rule's business", func(t *testing.T) {
		expectNoRule(t, "PB971", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("demo", "'x86_64'", "depends=('glibc')\n"), "")})
	})
	t.Run("PB972 fonts.google.com source", func(t *testing.T) {
		expectRule(t, "PB972", map[string]string{"PKGBUILD": pkgbuildWith(`pkgname=ttf-demo
pkgver=1.0.0
pkgrel=1
arch=('any')
url='https://example.com/demo'
license=('MIT')
source=("https://fonts.google.com/download?family=Demo")
sha256sums=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')`, "")})
	})
	t.Run("PB972 a stable release URL is fine", func(t *testing.T) {
		expectNoRule(t, "PB972", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("ttf-demo", "'any'", ""), "")})
	})
}

func TestDkmsRules(t *testing.T) {
	t.Run("PB973 -dkms without dkms in depends", func(t *testing.T) {
		expectRule(t, "PB973", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("demo-dkms", "'x86_64'", ""), "")})
	})
	t.Run("PB973 dkms declared", func(t *testing.T) {
		expectNoRule(t, "PB973", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("demo-dkms", "'x86_64'", "depends=('dkms')\n"), "")})
	})
	t.Run("PB973 depends declared inside a package function stands down", func(t *testing.T) {
		// The zfs-dkms shape: the split's depends live in package_*().
		expectNoRule(t, "PB973", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("demo-dkms", "'x86_64'", ""), `
package_demo-dkms() {
  depends=('dkms')
  install -Dm644 dkms.conf "$pkgdir/usr/src/demo-1.0.0/dkms.conf"
}`)})
	})
	t.Run("PB974 kernel headers pinned", func(t *testing.T) {
		expectRule(t, "PB974", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("demo-dkms", "'x86_64'", "depends=('dkms' 'linux-headers')\n"), "")})
		expectRule(t, "PB974", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("demo-dkms", "'x86_64'", "depends=('dkms' 'linux-lts-headers')\n"), "")})
	})
	t.Run("PB974 linux-api-headers is userspace, not a kernel", func(t *testing.T) {
		expectNoRule(t, "PB974", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("demo-dkms", "'x86_64'", "depends=('dkms' 'linux-api-headers')\n"), "")})
	})
	t.Run("PB974 non-dkms packages may depend on headers", func(t *testing.T) {
		expectNoRule(t, "PB974", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("demo", "'x86_64'", "depends=('linux-headers')\n"), "")})
	})
}

func TestLib32Rules(t *testing.T) {
	t.Run("PB975 pkgdesc without the suffix", func(t *testing.T) {
		expectRule(t, "PB975", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("lib32-demo", "'x86_64'", ""), "")})
	})
	t.Run("PB975 suffixed pkgdesc", func(t *testing.T) {
		expectNoRule(t, "PB975", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("lib32-demo", "'x86_64'", "pkgdesc='A demonstration library (32-bit)'\n"), "")})
		// lib32-nvidia-utils-beta qualifies the marker.
		expectNoRule(t, "PB975", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("lib32-demo", "'x86_64'", "pkgdesc='A demonstration library (32-bit, beta version)'\n"), "")})
	})
	t.Run("PB976 build without -m32", func(t *testing.T) {
		expectRule(t, "PB976", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("lib32-demo", "'x86_64'", ""), `
build() {
  ./configure --libdir=/usr/lib32
  make
}`)})
	})
	t.Run("PB976 -m32 in exported flags", func(t *testing.T) {
		expectNoRule(t, "PB976", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("lib32-demo", "'x86_64'", ""), `
build() {
  export CFLAGS="$CFLAGS -m32" LDFLAGS="$LDFLAGS -m32"
  ./configure --libdir=/usr/lib32
  make
}`)})
	})
	t.Run("PB976 repackaging without build() compiles nothing", func(t *testing.T) {
		expectNoRule(t, "PB976", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("lib32-demo", "'x86_64'", ""), `
package() {
  install -Dm755 lib/demo.so "$pkgdir/usr/lib32/demo.so"
}`)})
	})
}

func TestMingwRules(t *testing.T) {
	t.Run("PB977 missing options", func(t *testing.T) {
		expectRule(t, "PB977", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("mingw-w64-demo", "'any'", ""), "")})
		expectRule(t, "PB977", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("mingw-w64-demo", "'any'", "options=('!strip')\n"), "")})
	})
	t.Run("PB977 all three declared", func(t *testing.T) {
		expectNoRule(t, "PB977", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("mingw-w64-demo", "'any'", "options=('!strip' 'staticlibs' '!buildflags')\n"), "")})
	})
	t.Run("PB978 pkgdesc without the suffix", func(t *testing.T) {
		expectRule(t, "PB978", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("mingw-w64-demo", "'any'", "options=('!strip' 'staticlibs' '!buildflags')\n"), "")})
	})
	t.Run("PB978 suffixed pkgdesc", func(t *testing.T) {
		expectNoRule(t, "PB978", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("mingw-w64-demo", "'any'",
			"pkgdesc='A demonstration library (mingw-w64)'\noptions=('!strip' 'staticlibs' '!buildflags')\n"), "")})
	})
}

func TestNodeJavaCLRRules(t *testing.T) {
	t.Run("PB979 npm without makedepends", func(t *testing.T) {
		expectRule(t, "PB979", map[string]string{"PKGBUILD": pkgbuildWith("", `
prepare() {
  npm ci --cache "$srcdir/npm-cache"
}`)})
	})
	t.Run("PB979 npm declared", func(t *testing.T) {
		expectNoRule(t, "PB979", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('npm')
prepare() {
  npm ci --cache "$srcdir/npm-cache"
}`)})
	})
	t.Run("PB980 npm ci without --cache", func(t *testing.T) {
		expectRule(t, "PB980", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('npm')
prepare() {
  npm ci
}`)})
	})
	t.Run("PB980 --cache in both spellings", func(t *testing.T) {
		expectNoRule(t, "PB980", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('npm')
prepare() {
  npm ci --cache "$srcdir/npm-cache"
}`)})
		expectNoRule(t, "PB980", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('npm')
prepare() {
  npm ci --cache="$srcdir/npm-cache"
}`)})
	})
	t.Run("PB980 npm_config_cache in the environment", func(t *testing.T) {
		expectNoRule(t, "PB980", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('npm')
prepare() {
  export npm_config_cache="$srcdir/npm-cache"
  npm ci
}`)})
		expectNoRule(t, "PB980", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('npm')
prepare() {
  npm_config_cache="$srcdir/npm-cache" npm install
}`)})
		// Set but not exported, npm never sees it.
		expectRule(t, "PB980", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('npm')
prepare() {
  npm_config_cache="$srcdir/npm-cache"
  npm ci
}`)})
	})
	t.Run("PB980 npm run is not a download", func(t *testing.T) {
		expectNoRule(t, "PB980", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('npm')
build() {
  npm run build
}`)})
	})

	t.Run("PB981 jar install without a JVM dependency", func(t *testing.T) {
		expectRule(t, "PB981", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  install -Dm644 demo.jar "$pkgdir/usr/share/java/demo/demo.jar"
}`)})
	})
	t.Run("PB981 javac counts as java use", func(t *testing.T) {
		expectRule(t, "PB981", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  javac Demo.java
}`)})
	})
	t.Run("PB981 java-runtime satisfies it", func(t *testing.T) {
		expectNoRule(t, "PB981", map[string]string{"PKGBUILD": pkgbuildWith("", `
depends=('java-runtime')
package() {
  install -Dm644 demo.jar "$pkgdir/usr/share/java/demo/demo.jar"
}`)})
	})
	t.Run("PB981 depends declared inside a package function stands down", func(t *testing.T) {
		// The ant shape: a split-style PKGBUILD sets depends in package_*(),
		// beyond static reading.
		expectNoRule(t, "PB981", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  depends=('java-environment')
  install -Dm644 demo.jar "$pkgdir/usr/share/java/demo/demo.jar"
}`)})
	})
	t.Run("PB981 a concrete jre satisfies it", func(t *testing.T) {
		expectNoRule(t, "PB981", map[string]string{"PKGBUILD": pkgbuildWith("", `
depends=('jre-openjdk')
package() {
  install -Dm644 demo.jar "$pkgdir/usr/share/java/demo/demo.jar"
}`)})
	})

	t.Run("PB982 mono package without any/!strip", func(t *testing.T) {
		expectRule(t, "PB982", map[string]string{"PKGBUILD": pkgbuildWith("", `
depends=('mono')
build() {
  xbuild demo.sln
}`)})
	})
	t.Run("PB982 declared metadata satisfies it", func(t *testing.T) {
		expectNoRule(t, "PB982", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("demo", "'any'",
			"depends=('mono')\noptions=('!strip')\n"), `
build() {
  xbuild demo.sln
}`)})
	})
	t.Run("PB982 non-CLR packages are exempt", func(t *testing.T) {
		expectNoRule(t, "PB982", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  make
}`)})
	})
}

func TestHaskellPhpArchRules(t *testing.T) {
	t.Run("PB983 haskell arch any", func(t *testing.T) {
		expectRule(t, "PB983", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("haskell-demo", "'any'", ""), "")})
	})
	t.Run("PB983 concrete arch is correct", func(t *testing.T) {
		expectNoRule(t, "PB983", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("haskell-demo", "'x86_64'", ""), "")})
	})
	t.Run("PB984 pure-php package on a concrete arch", func(t *testing.T) {
		expectRule(t, "PB984", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("php-demo", "'x86_64'", ""), `
package() {
  install -Dm644 demo.php "$pkgdir/usr/share/webapps/demo/demo.php"
}`)})
	})
	t.Run("PB984 compiled extension is architecture-specific", func(t *testing.T) {
		expectNoRule(t, "PB984", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("php-demo", "'x86_64'", ""), `
build() {
  phpize
  ./configure
  make
}`)})
	})
	t.Run("PB984 any is correct for pure php", func(t *testing.T) {
		expectNoRule(t, "PB984", map[string]string{"PKGBUILD": pkgbuildWith(namedHeader("php-demo", "'any'", ""), "")})
	})
}
