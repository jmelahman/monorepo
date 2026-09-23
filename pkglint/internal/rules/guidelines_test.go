package rules

import "testing"

func TestSelfProvidesConflicts(t *testing.T) {
	t.Run("PB918 provides own name", func(t *testing.T) {
		expectRule(t, "PB918", map[string]string{"PKGBUILD": pkgbuildWith("", `provides=("$pkgname")`)})
	})
	t.Run("PB918 versioned self-provide is still self", func(t *testing.T) {
		expectRule(t, "PB918", map[string]string{"PKGBUILD": pkgbuildWith("", `provides=("demo=$pkgver")`)})
	})
	t.Run("PB918 arch-suffixed provides counts", func(t *testing.T) {
		expectRule(t, "PB918", map[string]string{"PKGBUILD": pkgbuildWith("", `provides_x86_64=('demo')`)})
	})
	t.Run("PB918 providing another capability is fine", func(t *testing.T) {
		expectNoRule(t, "PB918", map[string]string{"PKGBUILD": pkgbuildWith("", `provides=('demo-cli')`)})
	})
	t.Run("PB918 split package naming a sibling split", func(t *testing.T) {
		expectRule(t, "PB918", map[string]string{"PKGBUILD": pkgbuildWith(`pkgbase=demo
pkgname=(demo demo-extra)
pkgver=1.0.0
pkgrel=1
arch=('x86_64')
url='https://example.com/demo'
license=('MIT')
provides=('demo')`, `
package_demo() { :; }
package_demo-extra() { :; }`)})
	})
	t.Run("PB919 conflicts with own name", func(t *testing.T) {
		expectRule(t, "PB919", map[string]string{"PKGBUILD": pkgbuildWith("", `conflicts=("$pkgname")`)})
	})
	t.Run("PB919 conflicting with a real other package is fine", func(t *testing.T) {
		expectNoRule(t, "PB919", map[string]string{"PKGBUILD": pkgbuildWith("", `conflicts=('demo-legacy')`)})
	})
}

func TestNonSPDXLicense(t *testing.T) {
	for _, tc := range []struct {
		name    string
		license string
		want    int
	}{
		{"legacy GPL", `license=('GPL')`, 1},
		{"legacy BSD and Apache both flagged", `license=('BSD' 'Apache')`, 2},
		{"legacy token inside an expression", `license=('GPL2 OR MIT')`, 1},
		{"bare custom", `license=('custom')`, 1},
		{"SPDX id is fine", `license=('GPL-2.0-or-later')`, 0},
		{"SPDX expression is fine", `license=('Apache-2.0 OR MIT')`, 0},
		{"LicenseRef is fine", `license=('LicenseRef-demo-eula')`, 0},
		{"custom:name is tolerated", `license=('custom:demo-eula')`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ids := ruleIDs(lint(t, map[string]string{"PKGBUILD": pkgbuildWith("", tc.license)}))
			if got := ids["PB920"]; got != tc.want {
				t.Errorf("got %d PB920 findings, want %d", got, tc.want)
			}
		})
	}
}

func TestReservedDirInstalls(t *testing.T) {
	t.Run("PB921 install into usr/local", func(t *testing.T) {
		expectRule(t, "PB921", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  install -Dm755 demo "$pkgdir/usr/local/bin/demo"
}`)})
	})
	t.Run("PB921 braced pkgdir spelling", func(t *testing.T) {
		expectRule(t, "PB921", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  mkdir -p "${pkgdir}/usr/local/share/demo"
}`)})
	})
	t.Run("PB921 split package function counts", func(t *testing.T) {
		expectRule(t, "PB921", map[string]string{"PKGBUILD": pkgbuildWith("", `
package_demo() {
  cp -r out "$pkgdir/usr/local"
}`)})
	})
	t.Run("PB921 usr/bin is fine", func(t *testing.T) {
		expectNoRule(t, "PB921", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  install -Dm755 demo "$pkgdir/usr/bin/demo"
}`)})
	})
	t.Run("PB922 install into usr/libexec", func(t *testing.T) {
		expectRule(t, "PB922", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  install -Dm755 helper "$pkgdir/usr/libexec/demo/helper"
}`)})
	})
	t.Run("PB922 usr/lib is fine", func(t *testing.T) {
		expectNoRule(t, "PB922", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  install -Dm755 helper "$pkgdir/usr/lib/demo/helper"
}`)})
	})
}
