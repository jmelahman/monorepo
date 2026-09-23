package rules

// example is an illustrative pair of PKGBUILD snippets for a rule: Bad is a
// construct the rule flags, Good the preferred alternative. These are for
// documentation on the report card site only; the actual detection logic lives
// in each rule's Check. Registry() attaches these to the corresponding Rule.
type example struct {
	Bad  string
	Good string
}

var examples = map[string]example{
	"PB101": {
		Bad: `source=("https://example.com/foo-$pkgver.tar.gz")
sha256sums=('SKIP')`,
		Good: `source=("https://example.com/foo-$pkgver.tar.gz")
sha256sums=('9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08')`,
	},
	"PB102": {
		Bad: `source=("https://example.com/foo-$pkgver.tar.gz")
md5sums=('d41d8cd98f00b204e9800998ecf8427e')`,
		Good: `source=("https://example.com/foo-$pkgver.tar.gz")
sha256sums=('9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08')`,
	},
	"PB103": {
		Bad:  `source=("git+https://github.com/example/foo.git#tag=v1.2.3")`,
		Good: `source=("git+https://github.com/example/foo.git#commit=3f2b1a0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e5f4a")`,
	},
	"PB104": {
		Bad:  `source=("http://example.com/foo-$pkgver.tar.gz")`,
		Good: `source=("https://example.com/foo-$pkgver.tar.gz")`,
	},
	"PB105": {
		Bad: `url="https://example.com"
source=("https://cdn.unrelated.example/foo-$pkgver.tar.gz")`,
		Good: `url="https://example.com"
source=("https://example.com/releases/foo-$pkgver.tar.gz")`,
	},
	"PB106": {
		Bad: `DLAGENTS=('https::/usr/bin/curl -fsSL -- %u')`,
		Good: `# Rely on makepkg's built-in download agents; do not override DLAGENTS.
source=("https://example.com/foo-$pkgver.tar.gz")`,
	},
	"PB107": {
		Bad:  `install="foo.install"   # ...but no foo.install is committed alongside the PKGBUILD`,
		Good: `install="foo.install"   # foo.install ships in the same directory as the PKGBUILD`,
	},
	"PB108": {
		Bad: `VCSCLIENTS=('git::/tmp/evil-git')   # makepkg runs this to fetch git sources
source=("git+https://github.com/example/foo.git")`,
		Good: `# Leave makepkg.conf variables to makepkg.conf; declare only package fields here.
source=("git+https://github.com/example/foo.git#commit=3f2b1a0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e5f4a")`,
	},
	"PB109": {
		Bad: `url="https://github.com/upstream/foo"
source=("git+https://github.com/somebodyelse/foo.git#commit=3f2b1a0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e5f4a")`,
		Good: `url="https://github.com/upstream/foo"
source=("git+https://github.com/upstream/foo.git#commit=3f2b1a0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e5f4a")`,
	},
	"PB110": {
		Bad: `source=("a.tar.gz" "b.tar.gz")
sha256sums=('9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08')`,
		Good: `source=("a.tar.gz" "b.tar.gz")
sha256sums=('9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08'
            '2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae')`,
	},
	"PB111": {
		Bad: `source=("https://example.com/foo-$pkgver.tar.gz"
        "https://example.com/foo-$pkgver.tar.gz.sig")
sha256sums=('9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08' 'SKIP')
# validpgpkeys is never set: any key in the builder's keyring passes`,
		Good: `source=("https://example.com/foo-$pkgver.tar.gz"
        "https://example.com/foo-$pkgver.tar.gz.sig")
sha256sums=('9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08' 'SKIP')
validpgpkeys=('ABAF11C65A2970B130ABE3C479BE3E4300411886')   # upstream's release key`,
	},
	"PB112": {
		Bad: `source=("https://example.com/foo-$pkgver.tar.gz"
        "http://example.com/foo-$pkgver.tar.gz.sig")   # signature over plain http`,
		Good: `source=("https://example.com/foo-$pkgver.tar.gz"
        "https://example.com/foo-$pkgver.tar.gz.sig")`,
	},
	"PB113": {
		Bad: `source=("https://example.com/foo-$pkgver.tar.gz")
sha256sums=('9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08')
validpgpkeys=('ABAF11C65A2970B130ABE3C479BE3E4300411886')   # verifies nothing`,
		Good: `source=("https://example.com/foo-$pkgver.tar.gz"
        "https://example.com/foo-$pkgver.tar.gz.sig")   # now the key has something to check
sha256sums=('9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08' 'SKIP')
validpgpkeys=('ABAF11C65A2970B130ABE3C479BE3E4300411886')`,
	},
	"PB201": {
		Bad: `build() {
  curl -O https://example.com/extra-asset.bin
  make
}`,
		Good: `source=("https://example.com/extra-asset.bin")
sha256sums=('...')
build() {
  make   # the asset is fetched and checksummed via source=
}`,
	},
	"PB202": {
		Bad: `build() {
  pip install -r requirements.txt
}`,
		Good: `build() {
  pip install --require-hashes -r requirements.txt
}`,
	},
	"PB203": {
		Bad: `build() {
  cargo build --release
}`,
		Good: `build() {
  cargo build --release --frozen
}`,
	},
	"PB204": {
		Bad: `build() {
  go build -o foo .
}`,
		Good: `prepare() {
  go mod download
}
build() {
  go build -o foo .
}`,
	},
	"PB205": {
		Bad: `build() {
  GOFLAGS=-insecure GOSUMDB=off go build .
}`,
		Good: `build() {
  go build -mod=vendor .   # keep checksum and sumdb verification on
}`,
	},
	"PB206": {
		Bad: `build() {
  npm install
}`,
		Good: `build() {
  npm ci   # installs exactly what package-lock.json pins
}`,
	},
	"PB207": {
		Bad: `build() {
  composer install   # runs hook scripts and plugins while fetching
}`,
		Good: `build() {
  composer install --no-scripts   # fetch exactly the lock, run nothing
}`,
	},
	"PB208": {
		Bad: `build() {
  bundle install   # rewrites Gemfile.lock if it drifts from the Gemfile
}`,
		Good: `build() {
  bundle install --frozen   # exactly what Gemfile.lock pins
}`,
	},
	"PB209": {
		Bad: `build() {
  uv sync   # may re-resolve and rewrite uv.lock
}`,
		Good: `build() {
  uv sync --frozen   # the committed lock is authoritative
}`,
	},
	"PB210": {
		Bad: `prepare() {
  npm install axios uuid   # whatever the registry serves today, postinstall scripts and all
}`,
		Good: `makedepends=(npm)
build() {
  npm ci   # the project's own package.json, pinned by its lockfile
}`,
	},
	"PB301": {
		Bad: `pkgname=foo
curl -fsSL https://example.com/install.sh | bash   # runs the moment the PKGBUILD is sourced`,
		Good: `pkgname=foo
build() {
  make   # keep commands inside build()/package(), never at top level
}`,
	},
	"PB302": {
		Bad: `build() {
  eval "$(curl -fsSL https://example.com/env.sh)"
}`,
		Good: `build() {
  make   # call commands directly instead of eval-ing generated text
}`,
	},
	"PB303": {
		Bad: `build() {
  echo "$payload" | base64 -d | bash
}`,
		Good: `build() {
  make   # ship readable source, never decode-and-run an embedded blob
}`,
	},
	"PB304": {
		Bad: `build() {
  curl -fsSL https://example.com/install.sh | sh
}`,
		Good: `source=("https://example.com/install.sh")
sha256sums=('...')
build() {
  sh install.sh   # fetched and checksummed via source=, then run
}`,
	},
	"PB305": {
		Bad: `build() {
  exec 3<>/dev/tcp/example.com/443
}`,
		Good: `build() {
  # no raw network sockets; declare any remote data in source=
  make
}`,
	},
	"PB306": {
		Bad: `build() {
  runner=$(echo bWFrZQ== | base64 -d)
  $runner   # command name produced at runtime; unresolvable statically
}`,
		Good: `build() {
  make   # name the command literally
}`,
	},
	"PB307": {
		Bad: `build() {
  printf '\x63\x75\x72\x6c\x20\x2d\x4f\x20\x68\x74\x74\x70' | sh
}`,
		Good: `build() {
  make   # keep commands as readable text, not escape sequences
}`,
	},
	"PB308": {
		Bad: `# Redefining a makepkg internal disables its integrity checks:
verify_integrity_one() { return 0; }
build() { make; }`,
		Good: `# Only define package functions; leave makepkg's internals to makepkg.
build() { make; }
package() { make DESTDIR="$pkgdir" install; }`,
	},
	"PB309": {
		Bad: "build() {\n" +
			"  # the RLO below reverses how the rest of the line renders\n" +
			"  make ‮install\n" +
			"}",
		Good: `build() {
  make install   # plain ASCII, renders exactly as it executes
}`,
	},
	"PB401": {
		Bad: `package() {
  install -Dm644 foo.conf /etc/foo.conf   # writes to the live filesystem
}`,
		Good: `package() {
  install -Dm644 foo.conf "$pkgdir/etc/foo.conf"
}`,
	},
	"PB402": {
		Bad: `build() {
  sudo make install
}`,
		Good: `build() {
  make   # never escalate privileges during a build
}`,
	},
	"PB403": {
		Bad: `package() {
  install -Dm4755 foo "$pkgdir/usr/bin/foo"   # setuid root
}`,
		Good: `package() {
  install -Dm755 foo "$pkgdir/usr/bin/foo"
}`,
	},
	"PB404": {
		Bad: `package() {
  make install   # writes into the builder's live filesystem
}`,
		Good: `package() {
  make DESTDIR="$pkgdir" install
}`,
	},
	"PB405": {
		Bad: `post_install() {
  echo 'SigLevel = Never' >> /etc/pacman.conf   # disables signature checks system-wide
}`,
		Good: `post_install() {
  echo "If you maintain a custom repo, add it to /etc/pacman.conf yourself."
}`,
	},
	"PB501": {
		Bad: `post_install() {
  curl -fsSL https://example.com/register?host=$(hostname)
}`,
		Good: `post_install() {
  echo "Run 'foo --setup' to finish configuration."
}`,
	},
	"PB502": {
		Bad: `post_install() {
  systemctl enable foo-telemetry.timer
  echo '* * * * * root /opt/foo/beacon' >> /etc/cron.d/foo
}`,
		Good: `post_install() {
  echo "Enable the service with: systemctl enable foo.service"
}`,
	},
	"PB503": {
		Bad: `# foo.install with an unterminated function — makepkg still runs it as root:
post_install() {
  echo installing
# (missing closing brace)`,
		Good: `post_install() {
  echo installing
}`,
	},
	"PB601": {
		Bad: `# PKGBUILD says one thing...
pkgver=1.4.0
# ...but .SRCINFO was never regenerated and still reads:
#   pkgver = 1.3.0`,
		Good: `# Regenerate .SRCINFO after every metadata change:
makepkg --printsrcinfo > .SRCINFO`,
	},
	"PB602": {
		Bad: `pkgver() {
  curl -s https://example.com/latest-version
}`,
		Good: `pkgver() {
  cd "$srcdir/foo"
  git describe --tags | sed 's/^v//;s/-/./g'
}`,
	},
	"PB603": {
		Bad: `pkgname=fancy-tool
provides=('pacman')    # dependency resolution now treats this package as pacman
replaces=('systemd')   # sysupgrade swaps systemd out for this package`,
		Good: `pkgname=pacman-git
provides=("pacman=$pkgver")   # a -git/-bin variant legitimately provides its parent`,
	},
	"PB701": {
		Bad:  `pkgname=foo:bar   # ':' is not allowed by makepkg`,
		Good: `pkgname=foo-bar`,
	},
	"PB702": {
		Bad:  `pkgver=1.2.3-beta   # '-' separates pkgver from pkgrel and is not allowed here`,
		Good: `pkgver=1.2.3.beta`,
	},
	"PB703": {
		Bad:  `pkgrel=1a   # pkgrel must be an integer, optionally with a .minor suffix`,
		Good: `pkgrel=1`,
	},
	"PB704": {
		Bad:  `epoch=1.0   # epoch must be a non-negative integer`,
		Good: `epoch=1`,
	},
	"PB705": {
		Bad:  `backup=('/etc/foo.conf')   # backup paths are relative to /`,
		Good: `backup=('etc/foo.conf')`,
	},
	"PB706": {
		Bad:  `options=('!striped')   # typo: not a known makepkg option`,
		Good: `options=('!strip')`,
	},
	"PB707": {
		Bad:  `provides=('libfoo<2')   # a provide is a concrete capability, not a range`,
		Good: `provides=('libfoo=1.9')`,
	},
	"PB708": {
		Bad:  `depends=gtk3   # list fields must be arrays`,
		Good: `depends=('gtk3')`,
	},
	"PB709": {
		Bad: `package() {
  makedepends=('go')   # makedepends can't be overridden per-package
  make DESTDIR="$pkgdir" install
}`,
		Good: `makedepends=('go')
package() {
  depends=('glibc')   # only packaging metadata may be set here
  make DESTDIR="$pkgdir" install
}`,
	},
	"PB710": {
		Bad:  `arch=('any' 'x86_64')   # 'any' cannot be combined with concrete architectures`,
		Good: `arch=('x86_64')`,
	},
	"PB711": {
		Bad: `source=("git+https://github.com/example/demo.git#commit=3f2b1a0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e5f4a")
# nothing installs git, so a clean chroot build fails at the fetch step`,
		Good: `makedepends=('git')
source=("git+https://github.com/example/demo.git#commit=3f2b1a0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e5f4a")`,
	},
	"PB114": {
		Bad: `source=("https://example.com/demo-$pkgver.tar.gz")
sha256sums=('deadbeef')   # 8 characters can never be a sha256 digest`,
		Good: `source=("https://example.com/demo-$pkgver.tar.gz")
sha256sums=('9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08')`,
	},
	"PB504": {
		Bad: `post_install() {
  gtk-update-icon-cache -q /usr/share/icons/hicolor
}`,
		Good: `post_install() {
  echo "run 'demo --setup' once to finish configuration"
}`,
	},
	"PB901": {
		Bad:  `source=("https://example.com/demo-$pkgver-x86_64.tar.gz")`,
		Good: `source=("https://example.com/demo-$pkgver-$CARCH.tar.gz")`,
	},
	"PB902": {
		Bad:  `gitcommit='3f2b1a0'   # could collide with a future makepkg field`,
		Good: `_gitcommit='3f2b1a0'`,
	},
	"PB903": {
		Bad: `package() {
  install -Dm755 demo "$startdir/pkg/usr/bin/demo"
}`,
		Good: `package() {
  install -Dm755 demo "$pkgdir/usr/bin/demo"
}`,
	},
	"PB904": {
		Bad: `depends=('libfoo')
makedepends=('libfoo' 'cmake')   # libfoo is installed at build time already`,
		Good: `depends=('libfoo')
makedepends=('cmake')`,
	},
	"PB905": {
		Bad:  `source=("https://heanet.dl.sourceforge.net/project/demo/demo-$pkgver.tar.gz")`,
		Good: `source=("https://downloads.sourceforge.net/project/demo/demo-$pkgver.tar.gz")`,
	},
	"PB906": {
		Bad:  `pkgdesc='The demo package'`,
		Good: `pkgdesc='Fast demonstration tool for benchmarks'`,
	},
	"PB907": {
		Bad: `build() {
  msg "building demo"
  make
}`,
		Good: `build() {
  echo "building demo"
  make
}`,
	},
	"PB908": {
		Bad: `# Contributor: Somebody Else <somebody@example.com>
# ...but no '# Maintainer:' line anywhere in the file`,
		Good: `# Maintainer: Sam Coder <sam@example.com>`,
	},
	"PB909": {
		Bad:  `pkgname=Demo-Tool`,
		Good: `pkgname=demo-tool`,
	},
	"PB910": {
		Bad:  `pkgdesc=''   # empty — and url= is never set at all`,
		Good: `url='https://github.com/example/demo'`,
	},
	"PB911": {
		Bad:  `source=("https://github.com/example/demo/archive/v$pkgver.tar.gz")   # saves as v1.0.0.tar.gz`,
		Good: `source=("$pkgname-$pkgver.tar.gz::https://github.com/example/demo/archive/v$pkgver.tar.gz")`,
	},
	"PB912": {
		Bad: `depends=('ffmpeg')
optdepends=('ffmpeg: video export')   # already a hard dependency`,
		Good: `optdepends=('ffmpeg: video export')`,
	},
	"PB913": {
		Bad: `build() {
  # pkglint: ignore=PB203
  cargo build --locked --release   # --locked is present, so nothing suppresses
}`,
		Good: `build() {
  cargo build --locked --release
}`,
	},
	"PB914": {
		Bad: `build() {
  go build -o demo .
}`,
		Good: `build() {
  export GOFLAGS="-buildmode=pie -trimpath -mod=readonly -modcacherw"
  go build -o demo .
}`,
	},
	"PB915": {
		Bad: `build() {
  go build -buildmode=pie -o demo .   # $srcdir paths end up inside the binary
}`,
		Good: `build() {
  go build -buildmode=pie -trimpath -o demo .
}`,
	},
	"PB916": {
		Bad: `prepare() {
  go mod download
}`,
		Good: `prepare() {
  go mod download -modcacherw
}`,
	},
	"PB917": {
		Bad: `build() {
  export GOFLAGS="-buildmode=pie -trimpath -modcacherw"
  go build -o demo .
}`,
		Good: `build() {
  export CGO_CFLAGS="$CFLAGS" CGO_LDFLAGS="$LDFLAGS"
  export GOFLAGS="-buildmode=pie -trimpath -modcacherw"
  go build -o demo .
}`,
	},
	"PB918": {
		Bad:  `provides=("$pkgname")   # a package always provides itself`,
		Good: `provides=('demo-cli')   # a capability other packages can depend on`,
	},
	"PB919": {
		Bad:  `conflicts=("$pkgname")   # a package can never conflict with itself`,
		Good: `conflicts=('demo-legacy')   # the package this one actually displaces`,
	},
	"PB920": {
		Bad:  `license=('GPL')   # which GPL? or-later or only?`,
		Good: `license=('GPL-2.0-or-later')`,
	},
	"PB921": {
		Bad: `package() {
  install -Dm755 demo "$pkgdir/usr/local/bin/demo"
}`,
		Good: `package() {
  install -Dm755 demo "$pkgdir/usr/bin/demo"
}`,
	},
	"PB922": {
		Bad: `package() {
  install -Dm755 helper "$pkgdir/usr/libexec/demo/helper"
}`,
		Good: `package() {
  install -Dm755 helper "$pkgdir/usr/lib/demo/helper"
}`,
	},
	"PB930": {
		Bad: `checkdepends=('python-tox')
check() {
  tox
}`,
		Good: `checkdepends=('python-pytest')
check() {
  pytest
}`,
	},
	"PB931": {
		Bad:  `checkdepends=('python-pytest' 'python-pytest-cov')`,
		Good: `checkdepends=('python-pytest')`,
	},
	"PB932": {
		Bad: `source=("https://files.pythonhosted.org/packages/py3/d/demo/demo-1.0-py3-none-any.whl")
sha256sums=('9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08')`,
		Good: `source=("https://files.pythonhosted.org/packages/source/d/demo/demo-1.0.tar.gz")
sha256sums=('9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08')`,
	},
	"PB933": {
		Bad: `build() {
  python -m build --wheel --no-isolation
}`,
		Good: `makedepends=('python-build' 'python-installer')
build() {
  python -m build --wheel --no-isolation
}`,
	},
	"PB934": {
		Bad: `build() {
  python setup.py build
}`,
		Good: `makedepends=('python-build' 'python-installer')
build() {
  python -m build --wheel --no-isolation
}`,
	},
	"PB940": {
		Bad: `check() {
  cargo test --release --locked
}`,
		Good: `check() {
  cargo test --locked
}`,
	},
	"PB941": {
		Bad: `package() {
  cargo install --locked --root "$pkgdir/usr" --path .
}`,
		Good: `package() {
  cargo install --locked --no-track --root "$pkgdir/usr" --path .
}`,
	},
	"PB942": {
		Bad: `build() {
  cargo build --locked
}`,
		Good: `build() {
  cargo build --release --locked
}`,
	},
	"PB944": {
		Bad: `build() {
  cargo build --release --locked
}`,
		Good: `makedepends=('rust')
build() {
  cargo build --release --locked
}`,
	},
	"PB950": {
		Bad: `makedepends=('cmake')
build() {
  cmake -B build -S .   # prefix silently defaults to /usr/local
  cmake --build build
}`,
		Good: `makedepends=('cmake')
build() {
  cmake -B build -S . -DCMAKE_INSTALL_PREFIX=/usr -DCMAKE_BUILD_TYPE=None
  cmake --build build
}`,
	},
	"PB951": {
		Bad: `makedepends=('cmake')
build() {
  cmake -B build -S . -DCMAKE_INSTALL_PREFIX=/usr -DCMAKE_BUILD_TYPE=Release
  cmake --build build
}`,
		Good: `makedepends=('cmake')
build() {
  cmake -B build -S . -DCMAKE_INSTALL_PREFIX=/usr -DCMAKE_BUILD_TYPE=None
  cmake --build build
}`,
	},
	"PB952": {
		Bad: `build() {
  cmake -B build -S . -DCMAKE_INSTALL_PREFIX=/usr -DCMAKE_BUILD_TYPE=None
  cmake --build build
}`,
		Good: `makedepends=('cmake')
build() {
  cmake -B build -S . -DCMAKE_INSTALL_PREFIX=/usr -DCMAKE_BUILD_TYPE=None
  cmake --build build
}`,
	},
	"PB953": {
		Bad: `makedepends=('meson')
build() {
  meson setup build   # prefix silently defaults to /usr/local
}`,
		Good: `makedepends=('meson')
build() {
  meson setup build --prefix=/usr
}`,
	},
	"PB954": {
		Bad: `build() {
  meson setup build --prefix=/usr
}`,
		Good: `makedepends=('meson')
build() {
  meson setup build --prefix=/usr
}`,
	},
	"PB955": {
		Bad: `makedepends=('meson')
build() {
  meson setup build --prefix=/usr
  ninja -C build
}`,
		Good: `makedepends=('meson')
build() {
  meson setup build --prefix=/usr
  meson compile -C build
}`,
	},
	"PB960": {
		Bad: `pkgname=demo-git
makedepends=('git')
source=("git+https://example.com/demo.git")
sha256sums=('SKIP')
# no pkgver(): every build ships pkgver=1.0.0 whatever the checkout holds`,
		Good: `pkgname=demo-git
makedepends=('git')
source=("git+https://example.com/demo.git")
sha256sums=('SKIP')

pkgver() {
  cd demo
  git describe --long --tags | sed 's/\([^-]*-g\)/r\1/;s/-/./g'
}`,
	},
	"PB961": {
		Bad: `pkgname=demo-git
makedepends=('git')
source=("git+https://example.com/demo.git")
sha256sums=('SKIP')

pkgver() {
  cd demo
  git describe --long --tags | sed 's/\([^-]*-g\)/r\1/;s/-/./g'
}`,
		Good: `pkgname=demo-git
provides=('demo')
conflicts=('demo')
makedepends=('git')
source=("git+https://example.com/demo.git")
sha256sums=('SKIP')

pkgver() {
  cd demo
  git describe --long --tags | sed 's/\([^-]*-g\)/r\1/;s/-/./g'
}`,
	},
	"PB962": {
		Bad: `source=("$pkgname-$pkgver::git+https://example.com/demo.git#commit=3f2b1a0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e5f4a")
sha256sums=('SKIP')`,
		Good: `source=("$pkgname::git+https://example.com/demo.git#commit=3f2b1a0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e5f4a")
sha256sums=('SKIP')`,
	},
	"PB963": {
		Bad: `pkgname=demo-git   # ...but the source is a release tarball
source=("https://example.com/demo-1.0.0.tar.gz")
sha256sums=('9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08')`,
		Good: `pkgname=demo-git
provides=('demo')
conflicts=('demo')
makedepends=('git')
source=("git+https://example.com/demo.git")
sha256sums=('SKIP')

pkgver() {
  cd demo
  git describe --long --tags | sed 's/\([^-]*-g\)/r\1/;s/-/./g'
}`,
	},
	"PB970": {
		Bad: `pkgname=ttf-demo   # font files are the same bytes on every machine
# arch=('x86_64') from above stays in effect`,
		Good: `pkgname=ttf-demo
arch=('any')`,
	},
	"PB971": {
		Bad: `pkgname=ttf-demo
arch=('any')
depends=('fontconfig')   # fontconfig finds installed fonts on its own`,
		Good: `pkgname=ttf-demo
arch=('any')`,
	},
	"PB972": {
		Bad: `source=("https://fonts.google.com/download?family=Demo")
sha256sums=('9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08')`,
		Good: `source=("https://github.com/example/demo-font/releases/download/v1.0/demo-v1.0.zip")
sha256sums=('9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08')`,
	},
	"PB973": {
		Bad: `pkgname=demo-dkms
# depends=(): the module sources ship, but nothing will ever build them`,
		Good: `pkgname=demo-dkms
depends=('dkms')`,
	},
	"PB974": {
		Bad: `pkgname=demo-dkms
depends=('dkms' 'linux-headers')   # pins the stock kernel's headers`,
		Good: `pkgname=demo-dkms
depends=('dkms')   # dkms pulls the right headers for every installed kernel`,
	},
	"PB975": {
		Bad: `pkgname=lib32-demo
pkgdesc='Demonstration library'`,
		Good: `pkgname=lib32-demo
pkgdesc='Demonstration library (32-bit)'`,
	},
	"PB976": {
		Bad: `pkgname=lib32-demo
pkgdesc='Demonstration library (32-bit)'
build() {
  ./configure --libdir=/usr/lib32
  make
}`,
		Good: `pkgname=lib32-demo
pkgdesc='Demonstration library (32-bit)'
build() {
  export CFLAGS="$CFLAGS -m32" CXXFLAGS="$CXXFLAGS -m32" LDFLAGS="$LDFLAGS -m32"
  ./configure --libdir=/usr/lib32
  make
}`,
	},
	"PB977": {
		Bad: `pkgname=mingw-w64-demo
pkgdesc='Demonstration library (mingw-w64)'`,
		Good: `pkgname=mingw-w64-demo
pkgdesc='Demonstration library (mingw-w64)'
options=('!strip' 'staticlibs' '!buildflags')`,
	},
	"PB978": {
		Bad: `pkgname=mingw-w64-demo
pkgdesc='Demonstration library'
options=('!strip' 'staticlibs' '!buildflags')`,
		Good: `pkgname=mingw-w64-demo
pkgdesc='Demonstration library (mingw-w64)'
options=('!strip' 'staticlibs' '!buildflags')`,
	},
	"PB979": {
		Bad: `prepare() {
  npm ci --cache "$srcdir/npm-cache"
}`,
		Good: `makedepends=('npm')
prepare() {
  npm ci --cache "$srcdir/npm-cache"
}`,
	},
	"PB980": {
		Bad: `makedepends=('npm')
prepare() {
  npm ci   # tarballs land in the build user's ~/.npm
}`,
		Good: `makedepends=('npm')
prepare() {
  npm ci --cache "$srcdir/npm-cache"
}`,
	},
	"PB981": {
		Bad: `package() {
  install -Dm644 demo.jar "$pkgdir/usr/share/java/demo/demo.jar"
}   # nothing in depends provides a JVM to run it`,
		Good: `depends=('java-runtime')
package() {
  install -Dm644 demo.jar "$pkgdir/usr/share/java/demo/demo.jar"
}`,
	},
	"PB982": {
		Bad: `depends=('mono')
build() {
  xbuild demo.sln   # CIL assemblies, but arch/options say native binary
}`,
		Good: `depends=('mono')
arch=('any')
options=('!strip')
build() {
  xbuild demo.sln
}`,
	},
	"PB983": {
		Bad: `pkgname=haskell-demo
arch=('any')   # GHC output is native code with a baked-in ABI hash`,
		Good: `pkgname=haskell-demo
arch=('x86_64')`,
	},
	"PB984": {
		Bad: `pkgname=php-demo
# arch=('x86_64') from above, but nothing here compiles anything
package() {
  install -Dm644 demo.php "$pkgdir/usr/share/webapps/demo/demo.php"
}`,
		Good: `pkgname=php-demo
arch=('any')
package() {
  install -Dm644 demo.php "$pkgdir/usr/share/webapps/demo/demo.php"
}`,
	},

	// PB8xx examples illustrate what in the PKGBUILD produced the offending
	// package contents; the rules themselves run on built .pkg.tar.* archives,
	// so these snippets are documentation, verified by the package-rule tests
	// rather than the example round-trip suite.
	"PB801": {
		Bad: `arch=('any')
package() {
  install -Dm755 prebuilt/tool "$pkgdir/usr/bin/tool"   # a compiled ELF binary
}`,
		Good: `arch=('x86_64')
package() {
  install -Dm755 prebuilt/tool "$pkgdir/usr/bin/tool"
}`,
	},
	"PB802": {
		Bad: `package() {
  install -Dm755 helper "$pkgdir/usr/share/demo/helper"   # ELF outside usr/bin, usr/lib
}`,
		Good: `package() {
  install -Dm755 helper "$pkgdir/usr/lib/demo/helper"
}`,
	},
	"PB803": {
		Bad: `# hand-written assembly without a .note.GNU-stack section makes the
# linker request an executable stack for the whole binary
build() {
  make
}`,
		Good: `build() {
  export LDFLAGS="$LDFLAGS -Wl,-z,noexecstack"
  make
}`,
	},
	"PB804": {
		Bad: `build() {
  gcc -shared -o libdemo.so demo.c   # non-PIC objects force text relocations
}`,
		Good: `build() {
  gcc -shared -fPIC -o libdemo.so demo.c
}`,
	},
	"PB805": {
		Bad: `options=(!buildflags)   # drops Arch's default -Wl,-z,relro,-z,now hardening`,
		Good: `# keep the default buildflags; Arch's LDFLAGS already enable full RELRO
build() {
  make
}`,
	},
	"PB806": {
		Bad: `build() {
  export LDFLAGS="-no-pie"
  make
}`,
		Good: `# keep the default toolchain flags; Arch builds executables as PIE
build() {
  make
}`,
	},
	"PB807": {
		Bad:  `options=(!strip)   # ships every binary with its full symbol table`,
		Good: `options=(strip debug)   # symbols go to the -debug split package`,
	},
	"PB808": {
		Bad: `build() {
  export LDFLAGS="-Wl,-rpath,/home/builder/demo/lib"
  make
}`,
		Good: `build() {
  export LDFLAGS="-Wl,-rpath,\$ORIGIN/../lib/demo"
  make
}`,
	},
	"PB809": {
		Bad:  `depends=()   # but usr/bin/demo links libpng16.so.16`,
		Good: `depends=('libpng')`,
	},
	"PB810": {
		Bad: `# the build links every library pkg-config mentions, used or not:
# usr/bin/demo lists libxml2.so.2 in DT_NEEDED but imports nothing from it
build() {
  make
}`,
		Good: `build() {
  export LDFLAGS="$LDFLAGS -Wl,--as-needed"
  make
}`,
	},
	"PB811": {
		Bad:  `depends=('libfoo.so')   # unversioned: matches every future ABI`,
		Good: `depends=('libfoo.so=2-64')`,
	},
	"PB812": {
		Bad:  `depends=()   # but the package installs scripts beginning #!/usr/bin/python`,
		Good: `depends=('python')`,
	},
	"PB813": {
		Bad: `depends=('qt5-base' 'libpng')   # the qt frontend was dropped a year ago;
                                # nothing links or runs qt5-base anymore`,
		Good: `depends=('libpng')`,
	},
	"PB814": {
		Bad: `package() {
  install -Dm644 demo.gschema.xml "$pkgdir/usr/share/glib-2.0/schemas/demo.gschema.xml"
}   # ...but depends does not include dconf`,
		Good: `depends=('dconf')
package() {
  install -Dm644 demo.gschema.xml "$pkgdir/usr/share/glib-2.0/schemas/demo.gschema.xml"
}`,
	},
	"PB815": {
		Bad: `depends=('desktop-file-utils')   # only there for update-desktop-database,
                                 # which pacman's hook runs anyway`,
		Good: `# ship the .desktop file; pacman's hook updates the database
package() {
  install -Dm644 demo.desktop "$pkgdir/usr/share/applications/demo.desktop"
}`,
	},
	"PB816": {
		Bad: `# usr/lib/pkgconfig/demo.pc says "Requires: libcrypto"
depends=()   # ...but nothing in depends provides openssl's .pc files`,
		Good: `depends=('openssl')`,
	},
	"PB817": {
		Bad: `# built with a PKGBUILD that never set url= or pkgdesc=,
# so .PKGINFO ships blanks and pacman -Qi shows empty fields`,
		Good: `pkgdesc='Fast demonstration tool'
url='https://github.com/example/demo'`,
	},
	"PB820": {
		Bad: `package() {
  install -d "$pkgdir/run/demo"   # tmpfs: gone at first boot
}`,
		Good: `package() {
  install -Dm644 demo.tmpfiles "$pkgdir/usr/lib/tmpfiles.d/demo.conf"
}`,
	},
	"PB821": {
		Bad: `package() {
  chmod 777 "$pkgdir/var/lib/demo"   # world-writable
}`,
		Good: `package() {
  chmod 755 "$pkgdir/var/lib/demo"
}`,
	},
	"PB822": {
		Bad: `package() {
  cp -a ~/demo-data "$pkgdir/usr/share/demo"   # preserves the build user's ownership
}`,
		Good: `package() {
  cp -a --no-preserve=ownership demo-data "$pkgdir/usr/share/demo"
}`,
	},
	"PB823": {
		Bad: `package() {
  install -d "$pkgdir/var/log/demo"   # ships as an empty directory
}`,
		Good: `# create runtime directories at boot instead of shipping them empty
package() {
  install -Dm644 demo.tmpfiles "$pkgdir/usr/lib/tmpfiles.d/demo.conf"
}`,
	},
	"PB824": {
		Bad: `package() {
  # upstream tarball contains "README (kopie).txt" with a non-ASCII name
  cp -r docs "$pkgdir/usr/share/doc/$pkgname"
}`,
		Good: `package() {
  install -Dm644 docs/README.txt "$pkgdir/usr/share/doc/$pkgname/README.txt"
}`,
	},
	"PB825": {
		Bad: `package() {
  ln "$pkgdir/usr/bin/demo" "$pkgdir/usr/lib/demo/demo"   # hard link across directories
}`,
		Good: `package() {
  ln -s /usr/bin/demo "$pkgdir/usr/lib/demo/demo"
}`,
	},
	"PB826": {
		Bad: `package() {
  ln -s /usr/share/demo/old-name "$pkgdir/usr/bin/demo"   # target was renamed upstream
}`,
		Good: `package() {
  ln -s /usr/share/demo/demo.sh "$pkgdir/usr/bin/demo"   # target ships in this package
}`,
	},
	"PB827": {
		Bad: `package() {
  make DESTDIR="$pkgdir" install   # leaves libtool .la files in usr/lib
}`,
		Good: `package() {
  make DESTDIR="$pkgdir" install
  find "$pkgdir" -name '*.la' -delete
}`,
	},
	"PB828": {
		Bad: `package() {
  make DESTDIR="$pkgdir" install   # perl's install step wrote perllocal.pod
}`,
		Good: `package() {
  make DESTDIR="$pkgdir" install
  find "$pkgdir" -name perllocal.pod -delete
}`,
	},
	"PB829": {
		Bad: `package() {
  make DESTDIR="$pkgdir" install-info   # writes usr/share/info/dir
}`,
		Good: `package() {
  make DESTDIR="$pkgdir" install-info
  rm -f "$pkgdir/usr/share/info/dir"
}`,
	},
	"PB830": {
		Bad: `package() {
  python -m compileall "$pkgdir"
  sed -i "s/@VERSION@/$pkgver/" "$pkgdir"/usr/lib/python*/site-packages/demo/version.py
}   # the .py is now newer than its .pyc`,
		Good: `package() {
  sed -i "s/@VERSION@/$pkgver/" "$pkgdir"/usr/lib/python*/site-packages/demo/version.py
  python -m compileall "$pkgdir"
}`,
	},
	"PB831": {
		Bad: `package() {
  python -m installer --destdir="$pkgdir" dist/*.whl   # the wheel ships a top-level tests/
}`,
		Good: `package() {
  python -m installer --destdir="$pkgdir" dist/*.whl
  rm -r "$pkgdir"/usr/lib/python*/site-packages/tests
}`,
	},
	"PB832": {
		Bad: `package() {
  install -Dm644 demo.service "$pkgdir/etc/systemd/system/demo.service"
}`,
		Good: `package() {
  install -Dm644 demo.service "$pkgdir/usr/lib/systemd/system/demo.service"
}`,
	},
	"PB833": {
		Bad: `package() {
  install -Dm644 demo.conf "$pkgdir/etc/dbus-1/system.d/demo.conf"
}`,
		Good: `package() {
  install -Dm644 demo.conf "$pkgdir/usr/share/dbus-1/system.d/demo.conf"
}`,
	},
	"PB842": {
		Bad: `package() {
  install -Dm644 demo.jar "$pkgdir/usr/share/demo/demo.jar"   # private jar copy
}`,
		Good: `package() {
  install -Dm644 demo.jar "$pkgdir/usr/share/java/demo/demo.jar"
  ln -s /usr/share/java/demo/demo.jar "$pkgdir/usr/share/demo/demo.jar"
}`,
	},
	"PB834": {
		Bad: `license=('LicenseRef-demo-eula')
package() {
  install -Dm755 demo "$pkgdir/usr/bin/demo"   # the EULA text is never installed
}`,
		Good: `license=('LicenseRef-demo-eula')
package() {
  install -Dm755 demo "$pkgdir/usr/bin/demo"
  install -Dm644 EULA "$pkgdir/usr/share/licenses/$pkgname/EULA"
}`,
	},
	"PB835": {
		Bad: `backup=('etc/demo.conf')
package() {
  install -Dm755 demo "$pkgdir/usr/bin/demo"   # etc/demo.conf is never installed
}`,
		Good: `backup=('etc/demo.conf')
package() {
  install -Dm755 demo "$pkgdir/usr/bin/demo"
  install -Dm644 demo.conf "$pkgdir/etc/demo.conf"
}`,
	},
	"PB836": {
		Bad: `package() {
  install -Dm755 demo "$pkgdir/usr/bin/demo"
  cp -r manual/ "$pkgdir/usr/share/doc/$pkgname"   # 300 MB of HTML for a 2 MB tool
}`,
		Good: `# move the manual to a demo-docs split package
package_demo-docs() {
  cp -r manual/ "$pkgdir/usr/share/doc/demo"
}`,
	},
	"PB837": {
		Bad: `package() {
  cp -r docs/_build "$pkgdir/usr/share/doc/$pkgname"   # includes .doctrees/environment.pickle
}`,
		Good: `package() {
  cp -r docs/_build/html "$pkgdir/usr/share/doc/$pkgname"
}`,
	},
	"PB838": {
		Bad: `package() {
  update-mime-database "$pkgdir/usr/share/mime"   # bakes generated caches into the package
}`,
		Good: `package() {
  install -Dm644 demo-mime.xml "$pkgdir/usr/share/mime/packages/demo.xml"
}   # pacman's hook regenerates the database on the user's system`,
	},
	"PB839": {
		Bad: `build() {
  ./configure --prefix=/usr   # an ancient scrollkeeper hook creates var/lib/scrollkeeper
  make
}`,
		Good: `build() {
  ./configure --prefix=/usr --disable-scrollkeeper
  make
}`,
	},
}
