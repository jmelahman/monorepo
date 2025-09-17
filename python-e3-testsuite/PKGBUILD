# Maintainer: Jamison Lahman <jamison+aur@lahman.dev>
# Contributor: Pierre-Marie de Rodat  <pmderodat@kawie.fr>

pkgname='python-e3-testsuite'
_pkgname=${pkgname#python-}
pkgver=27.2
pkgrel=3
pkgdesc="Generic Testsuite Driver in Python"

arch=('any')
url="https://github.com/AdaCore/e3-testsuite"
license=('GPL3')

depends=('python' 'python-e3-core')
makedepends=('python-pip')

source=(
  "https://files.pythonhosted.org/packages/py3/${_pkgname::1}/$_pkgname/${_pkgname/-/_}-$pkgver-py3-none-any.whl"
)
sha256sums=('ba162cc37c12ea011650975b522df828e74940ce46a20adf97ee352bc2072fc0')

package() {
    cd "$srcdir" || exit
    python -m pip install --root="$pkgdir/" --no-deps --ignore-installed "${srcdir}/${source[0]##*/}"
}
