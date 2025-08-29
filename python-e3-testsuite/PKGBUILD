# Maintainer: Jamison Lahman <jamison+aur@lahman.dev>
# Contributor: Pierre-Marie de Rodat  <pmderodat@kawie.fr>

pkgname='python-e3-testsuite'
pkgver=27.2
pkgrel=1
pkgdesc="Generic Testsuite Driver in Python"

arch=('any')
url="https://github.com/AdaCore/e3-testsuite"
license=('GPL3')

depends=('python-e3-core')
makedepends=('python-pip')

_name='e3_testsuite'
source=(
  "https://files.pythonhosted.org/packages/b7/1b/a7da16a13b13f6b0ae77fb658f8470d465b569842a79a84469ffbc7273e5/$_name-$pkgver-py3-none-any.whl"
)
sha256sums=('ba162cc37c12ea011650975b522df828e74940ce46a20adf97ee352bc2072fc0')

package() {
    cd "$srcdir/$_name-$pkgver" || exit
    python -m pip install --root="$pkgdir/" --no-deps --ignore-installed "${srcdir}/${source[0]##*/}"
}
