package rules

import "testing"

func TestPythonGuidelineRules(t *testing.T) {
	t.Run("PB930 tox in check", func(t *testing.T) {
		expectRule(t, "PB930", map[string]string{"PKGBUILD": pkgbuildWith("", `
check() {
  tox
}`)})
	})
	t.Run("PB930 tox in checkdepends", func(t *testing.T) {
		expectRule(t, "PB930", map[string]string{"PKGBUILD": pkgbuildWith("", `checkdepends=('python-tox')`)})
		expectRule(t, "PB930", map[string]string{"PKGBUILD": pkgbuildWith("", `checkdepends=('tox')`)})
	})
	t.Run("PB930 pytest is the guideline flow", func(t *testing.T) {
		expectNoRule(t, "PB930", map[string]string{"PKGBUILD": pkgbuildWith("", `
checkdepends=('python-pytest')
check() {
  pytest
}`)})
	})

	t.Run("PB931 coverage plugin in checkdepends", func(t *testing.T) {
		expectRule(t, "PB931", map[string]string{"PKGBUILD": pkgbuildWith("", `checkdepends=('python-pytest' 'python-pytest-cov')`)})
	})
	t.Run("PB931 plain pytest is fine", func(t *testing.T) {
		expectNoRule(t, "PB931", map[string]string{"PKGBUILD": pkgbuildWith("", `checkdepends=('python-pytest')`)})
	})

	t.Run("PB932 wheel source", func(t *testing.T) {
		expectRule(t, "PB932", map[string]string{"PKGBUILD": pkgbuildWith("", `source+=("https://files.pythonhosted.org/packages/py3/d/demo/demo-1.0-py3-none-any.whl")
sha256sums+=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')`)})
	})
	t.Run("PB932 renamed wheel source", func(t *testing.T) {
		expectRule(t, "PB932", map[string]string{"PKGBUILD": pkgbuildWith("", `source+=("$pkgname-$pkgver.whl::https://example.com/demo/download")
sha256sums+=('deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef')`)})
	})
	t.Run("PB932 sdist is fine", func(t *testing.T) {
		expectNoRule(t, "PB932", map[string]string{"PKGBUILD": pkgbuildWith("", "")})
	})

	t.Run("PB933 python -m build without python-build", func(t *testing.T) {
		expectRule(t, "PB933", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  python -m build --wheel --no-isolation
}`)})
	})
	t.Run("PB933 python -m installer without python-installer", func(t *testing.T) {
		expectRule(t, "PB933", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('python-build')
build() {
  python -m build --wheel --no-isolation
}
package() {
  python -m installer --destdir="$pkgdir" dist/*.whl
}`)})
	})
	t.Run("PB933 declared makedepends satisfy it", func(t *testing.T) {
		expectNoRule(t, "PB933", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('python-build' 'python-installer')
build() {
  python -m build --wheel --no-isolation
}
package() {
  python -m installer --destdir="$pkgdir" dist/*.whl
}`)})
	})
	t.Run("PB933 other -m modules are not build backends", func(t *testing.T) {
		expectNoRule(t, "PB933", map[string]string{"PKGBUILD": pkgbuildWith("", `
check() {
  python -m pytest
}`)})
	})

	t.Run("PB934 setup.py build", func(t *testing.T) {
		expectRule(t, "PB934", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  python setup.py build
}`)})
	})
	t.Run("PB934 setup.py install in package", func(t *testing.T) {
		expectRule(t, "PB934", map[string]string{"PKGBUILD": pkgbuildWith("", `
package() {
  python setup.py install --root="$pkgdir" --optimize=1
}`)})
	})
	t.Run("PB934 setup.py without a distutils verb is left alone", func(t *testing.T) {
		expectNoRule(t, "PB934", map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  python setup.py --version
}`)})
	})
	t.Run("PB934 the python -m build flow is fine", func(t *testing.T) {
		expectNoRule(t, "PB934", map[string]string{"PKGBUILD": pkgbuildWith("", `
makedepends=('python-build')
build() {
  python -m build --wheel --no-isolation
}`)})
	})
}
