# pkglint

A security-focused linter for Arch Linux packages.

pkglint statically analyzes PKGBUILDs and their install scriptlets — **without ever
sourcing them** — and reports findings about source integrity, build hermeticity, code
execution, and persistence patterns, condensed into a letter grade per package. It also
reproduces makepkg's own build-breaking metadata checks, so a PKGBUILD that would fail to
build is caught (and, where the fix is mechanical, rewritten) before you run makepkg. It is
built on a real bash AST ([mvdan.cc/sh](https://github.com/mvdan/sh)), so the
quoting/line-continuation tricks that evade regex-based scanners don't work here.

Built packages (`*.pkg.tar.zst` and friends) are inputs too: pkglint inspects the archive
the way namcap does — ELF hardening, stripping, dependencies inferred from linked
libraries and shebangs, packaged `.INSTALL` scriptlets, filesystem hygiene — while
**never executing anything from the package** (ELF files are parsed, not loaded; no `ldd`).

```shell
$ pkglint ~/pkgbuilds/somepkg
somepkg: grade F, 3 finding(s)
  PKGBUILD:16:3: critical [PB304] a network download is piped straight into bash and executed
  PKGBUILD:11:1: error [PB101] remote source "http://..." has no checksum (SKIP): the download is never verified
  PKGBUILD:24:3: warn [PB403] chmod 4755 creates a setuid/setgid file

1 package linted: 1 with findings
1 finding(s) fixable with --unsafe-fix
```

Packages with nothing to report are only counted in the closing summary line
(`--verbose` lists them individually). The line after it tallies the findings a
fix run will attempt, split by the flag that applies them.

## Install

**AUR**

```shell
yay -S pkglint
```

**PyPi**

```shell
uv tool install pkglint
```

**Go**

```shell
go install github.com/jmelahman/pkglint@latest
```

**Github Releases**

Prebuilt binaries for Linux and macOS (amd64 and arm64) are attached to every [release](https://github.com/jmelahman/pkglint/releases/latest).

**Commit hook**

```yaml
repos:
  - repo: https://github.com/jmelahman/pkglint
    rev: a3b3651b15d3c2b04b2548a39925643a625d5f4e # frozen: v1.6.1
    hooks:
      - id: pkglint
      - id: pkglint-build
```

Four hook ids are available: `pkglint` builds from source with the Go
toolchain, `pkglint-system` runs whatever `pkglint` is already on `$PATH`,
`pkglint-fix` applies the safe auto-fixes in place (offline by default), and
`pkglint-build` builds each PKGBUILD and lints the package it produces (see
below). Tune any of them with e.g. `args: [--ignore, PB105, --fail-on, critical]`.

## Usage

```shell
pkglint [flags] [path ...]     # paths are package dirs, PKGBUILD files,
                               # or built packages (*.pkg.tar.*) (default: .)

  --format text|json|sarif     # output format (sarif = SARIF 2.1.0, for code scanning)
  --fail-on SEVERITY           # exit 1 at or above: info, warn (default), error, critical, never
  --ignore PB105,PB206         # disable rules
  --select PB101,PB304         # check only these rules (--ignore still subtracts from them)
  --color auto|always|never    # colorize text output (auto = only on a terminal; honors NO_COLOR)
  --verbose                    # list packages with no findings individually, not just in the summary
  --rules                      # list every rule with its documentation
  --fix                        # apply safe auto-fixes in place
  --unsafe-fix                 # also apply behavior-changing fixes (implies --fix)
  --diff                       # with --fix/--unsafe-fix/--add-ignores: show changes instead of writing
  --offline                    # with --fix: skip fixes needing network (VCS ref resolution, https probing)
  --add-ignores                # insert ignore directives suppressing every current finding
  --no-inline-ignores          # disregard ignore directives (audit an untrusted package)

pkglint explain PB101 ...      # print one rule's documentation, example, and
                               # how to fix or suppress it (ID or name, any case)
```

Suppress a reviewed, intentional finding inline:

```bash
# pkglint: ignore=PB204
go build -o "$pkgname" .
```

The directive covers its own line and the line below it, in the file it appears
in only. `--add-ignores` writes one for every finding still reported, so a
package can adopt pkglint before fixing everything it flags; `--diff` previews
the insertions. A directive that no longer matches a finding is itself flagged
(PB913, `stale-ignore-directive`) and deleted by `--fix`, so a fixed issue
cannot leave behind a comment that would silence its return. When reviewing a
package you don't trust, `--no-inline-ignores` reports whatever the maintainer
suppressed.

### Auto-fixing

`--fix` rewrites what it can and prints every change; `--diff` previews without
writing. Fixes come in two tiers:

**Safe — `--fix`**

| Rules                                    | What it does                                                                                                                                                        |
| ---------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| PB102                                    | Add `sha256sums` beside a weak `md5sums`/`sha1sums`, computed from sources already on disk (see below)                                                              |
| PB103                                    | Pin a mutable VCS tag or branch to the commit it names right now (via `git ls-remote`)                                                                              |
| PB205, PB916                             | Delete Go verification-disabling env settings; insert `-modcacherw` so the module cache stays removable                                                             |
| PB705, PB708                             | Strip a leading slash from `backup` entries; wrap a scalar list field (`depends=foo`) in an array                                                                   |
| PB711, PB933, PB944, PB952, PB954, PB979 | Declare in `makedepends` a build tool the build already invokes: the VCS client a source needs, `python-build`/`python-installer`, `rust`, `cmake`, `meson`, `npm`  |
| PB904, PB912, PB918, PB919               | Delete dependency metadata the rule proves does nothing: a `makedepends`/`optdepends` entry already in `depends`, a package naming itself in `provides`/`conflicts` |
| PB913                                    | Remove stale ignore directives                                                                                                                                      |

**Unsafe — `--unsafe-fix`**

| Rules                      | What it does                                                                                                                                                                                                                                                                                                            |
| -------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| PB104, PB112               | Upgrade an insecure source transport (`http://`/`ftp://` → `https://`, bare `git://` → `git+https://`), signature sources included, and only after a headers-only request (or `git ls-remote`) confirms the https URL answers, so an unserved one keeps the finding — reachable still isn't identical, so rebuild after |
| PB203, PB204, PB206–PB209  | Pin a build to its lockfile: `--locked` on `cargo` (fails the build if no `Cargo.lock` ships), a `go mod download` prepare() step, `npm install`→`ci`, `yarn install --immutable`, `--frozen-lockfile` on `pnpm`/`bun install`, `--no-scripts` on `composer install`, `--frozen` on `bundle install` and `uv sync`      |
| PB914, PB915, PB917        | Restore Arch's Go build flags: insert `-buildmode=pie` and `-trimpath`, and export `CGO_CFLAGS`/`CGO_LDFLAGS` so the hardening flags reach C code                                                                                                                                                                       |
| PB940, PB941, PB942        | Cargo profile and install flags: drop `--release` from `cargo test`/`cargo check` so the suite runs with debug assertions and overflow checks on, insert it into a build() `cargo build` so the package ships the optimized profile, append `--no-track` to `cargo install`                                             |
| PB950, PB951, PB953, PB980 | Point a build system at Arch's layout: `-DCMAKE_INSTALL_PREFIX=/usr`, `-DCMAKE_BUILD_TYPE=None`, `--prefix=/usr` on `meson setup`, and an npm `--cache` inside `$srcdir` instead of the invoking user's `~/.npm`                                                                                                        |
| PB961, PB973, PB981        | Add installed-runtime metadata the guidelines require: the `provides`/`conflicts` pair a `-git` package owes its release counterpart (written together or not at all), `dkms`, `java-runtime`                                                                                                                           |
| PB931, PB971, PB974        | Delete a declared dependency the rule calls unnecessary: a pytest lint plugin from `checkdepends`, every `depends` entry of a font package, a `-dkms` package's pinned `linux*-headers`                                                                                                                                 |
| PB403                      | Drop setuid/setgid mode bits                                                                                                                                                                                                                                                                                            |
| PB902                      | Prefix a custom variable with an underscore (`pyname` → `_pyname`), declaration and every reference at once                                                                                                                                                                                                             |

Safe fixes preserve behavior or restore a security default; unsafe fixes are
mechanical but change what the build does, so review them. Every fix stands
down where it cannot see the whole picture — a flag arriving through an
expansion, an array it cannot re-render faithfully, a rename it cannot prove
complete — rather than leave a half-applied edit. An inline `# pkglint: ignore=`
on a finding's line also suppresses its fix. Findings whose remediation isn't a
mechanical rewrite print a one-line suggestion instead (`updpkgsums` for
checksums, `makepkg --printsrcinfo` for a stale `.SRCINFO`), computed from what
is left _after_ fixing.

**From the build — `pkglint build --fix`**

| Rules | What it does                                                                                                                                  |
| ----- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| PB809 | Add to `depends` the package that owns a library the built binaries link and nothing in `depends` reaches (`usr/bin/demo` → `libc.so.6` → `glibc`) |

The built-package rules judge an artifact, so their fixes live on the one
command that has both: `pkglint build --fix` lints the PKGBUILD, builds it,
lints the archive, and writes the answer back into the PKGBUILD it built
(`--diff` previews, `--unsafe-fix` widens the tier as it does elsewhere). It
only touches a PKGBUILD that declares exactly one package, since a split
PKGBUILD's `depends` belongs to each `package_<name>()` rather than to the
file, and only the `error` gaps — a library reached transitively or through an
`optdepends` is a judgement the maintainer makes. The gate runs first, as
always: a build that is refused produces no archive and fixes nothing. The
PKGBUILD-scope fixes above stay with `pkglint --fix`.

The PB102 fix hashes sources **already downloaded** into the package directory
or `$SRCDEST` — pkglint never fetches a source — and writes a digest only after
re-computing the existing `md5`/`sha1` from the same bytes and finding it
matches, so the new `sha256sums` covers exactly what the weak digest already
vouched for. Sources that aren't present or a digest that doesn't match leave
the finding standing, and `updpkgsums` remains the way to close it. The weak
array is kept; the edit is purely additive.

### Build and validate

The built-package rules need a built package. `pkglint build` produces one:

```shell
$ pkglint build ~/pkgbuilds/somepkg
```

It lints the PKGBUILD, hands the package to `makepkg`, and then lints the
archive that comes out. `pkglint-build` is the same thing as a hook; it is
`stages: [manual]` because a full build has no business running on every commit.

This is the one pkglint command that executes a PKGBUILD, because that is what
`makepkg` does. It is a separately named verb — `pkglint <path>` never reaches
it — and it **refuses to build a package whose static findings reach
`--fail-on`**, always refusing on a critical finding no matter what `--fail-on`
says. `--force` overrides that; the findings still count toward the exit code.

Because that gate decides whether to run code, the PKGBUILD gets no say in it:

- The gate **disregards the file's own `# pkglint: ignore=` directives** — a
  `curl | bash` with an `ignore=PB304` above it is still a `curl | bash`. The
  report printed beside the refusal still honours them; only the decision to
  execute is taken on the unsuppressed findings.
- A file argument must be a `PKGBUILD`, since `makepkg` builds the `PKGBUILD`
  in its working directory regardless of which file pkglint linted.
- `makepkg`'s `-p` and `-D`/`--dir` are rejected, for the same reason.

Because `build` is a verb, a package directory named `./build` is spelled
`pkglint ./build`.

The packaging tree is left exactly as it was found: `makepkg` writes into a
temporary `PKGDEST`, and the `PKGBUILD` is held read-only so a `pkgver()`
package cannot rewrite the file that was gated. `--keep <dir>` moves the
archives out. Sources are cached in `${XDG_CACHE_HOME:-~/.cache}/pkglint/sources`
and the build tree goes under `$TMPDIR`; export `$SRCDEST` or `$BUILDDIR` to
relocate either. Dependencies are **not** synced, since that needs root and a
hook that prompts for a password is a hook that hangs — pass `-- -s` (or
`--makepkg-arg=-s`, the form that survives pre-commit's `args:`) to opt in.
`--nosign` is passed for the same reason.

Without `makepkg` on the host — or with `--docker` or an explicit `--image` —
the build runs in a container. Name the image with `--image` or
`$PKGLINT_BUILD_IMAGE` and pick the runtime with `$PKGLINT_BUILD_RUNNER`
(`docker` by default, else `podman`). The package directory is bind-mounted
read-only and the archives come back out with `<runner> cp`, owned by you. The
image is your trust decision, and the container is a convenience, not a
sandbox — the PKGBUILD's own code still runs. `-- -s` inside one needs an
image with passwordless `sudo pacman`, which stock `archlinux:base-devel` lacks.

## Rules

| Group         | Rules       | What they catch                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| ------------- | ----------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Integrity     | PB101–PB114 | SKIP/weak/malformed checksums, unpinned VCS sources, unencrypted transports, source/url domain and forge-owner mismatches, DLAGENTS and other makepkg.conf overrides, checksum-count mismatches, missing install scripts, PGP signatures without pinned keys, insecure signature transport, unused validpgpkeys                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| Hermeticity   | PB201–PB210 | network access outside `prepare()`, `pip`/`uv pip` without `--require-hashes`, unlocked `cargo`/npm/yarn/pnpm/bun/composer/bundler/uv/poetry installs, implicit Go module downloads and mutable `@latest` refs, disabled checksum databases, packages fetched by name from a registry (`npm install axios`, `npx`, `pip install requests`, `gem install`, `cargo install` and a dozen more) with nothing verifying what arrives                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| Execution     | PB301–PB309 | top-level code, `eval`, decode-and-execute, download-and-execute (including `eval "$(curl ...)"` and `source <(wget ...)"` variants), `/dev/tcp`, unresolvable command names, embedded payloads, makepkg-internal function overrides, hidden bidi/zero-width characters                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| Filesystem    | PB401–PB405 | writes outside `$srcdir`/`$pkgdir`, privilege escalation, setuid files and setcap capability grants, install steps that skip `$pkgdir`, writes to pacman/dynamic-linker/sudoers config                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| Scriptlets    | PB501–PB504 | network access and persistence (crontabs, systemd units, shell profiles, login-capable users) in `.install` files running as root, unparseable scriptlets, commands pacman hooks already run                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| Consistency   | PB601–PB603 | PKGBUILD / .SRCINFO drift, network access in `pkgver()`, provides/replaces/conflicts claims on core system packages                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| Correctness   | PB701–PB711 | makepkg build-breakers: invalid pkgname/pkgver/pkgrel/epoch, backup leading slash, unknown `options`, `provides` comparison operators, scalar-vs-array field types, schema variables set inside `package()`, missing/duplicate/mixed `arch`, VCS sources without their client in makedepends                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| Built package | PB801–PB842 | everything namcap checks in a `.pkg.tar.*`: ELF hardening (executable stacks, text relocations, missing RELRO, non-PIE executables, insecure RPATH/RUNPATH), unstripped binaries, missing/unused library and interpreter dependencies (resolved through pacman's database, statically — no `ldd`), stale soname declarations, pkg-config requirements, FHS layout, permissions and ownership, dangling symlinks and cross-directory hardlinks, `.la`/`perllocal.pod`/info `dir`/MIME-cache landmines, stale python bytecode, systemd/D-Bus units under `/etc`, missing license and backup files — plus the full scriptlet analysis over the packaged `.INSTALL`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| Style         | PB901–PB984 | namcap's PKGBUILD conventions (hardcoded architectures, custom variables without `_` prefix, `$startdir`, redundant makedepends, missing Maintainer/pkgdesc/url/license, pre-SPDX license identifiers, stale ignore directives, …) plus the published [Arch package guidelines](https://wiki.archlinux.org/title/Arch_package_guidelines) and the per-ecosystem guidelines: [Go](https://wiki.archlinux.org/title/Go_package_guidelines), [Python](https://wiki.archlinux.org/title/Python_package_guidelines), [Rust](https://wiki.archlinux.org/title/Rust_package_guidelines), [CMake](https://wiki.archlinux.org/title/CMake_package_guidelines)/[Meson](https://wiki.archlinux.org/title/Meson_package_guidelines), [VCS](https://wiki.archlinux.org/title/VCS_package_guidelines), [fonts](https://wiki.archlinux.org/title/Font_package_guidelines), [DKMS](https://wiki.archlinux.org/title/DKMS_package_guidelines), [lib32](https://wiki.archlinux.org/title/32-bit_package_guidelines), [MinGW](https://wiki.archlinux.org/title/MinGW_package_guidelines), [Node.js](https://wiki.archlinux.org/title/Node.js_package_guidelines), [Java](https://wiki.archlinux.org/title/Java_package_guidelines), [CLR](https://wiki.archlinux.org/title/CLR_package_guidelines), [Haskell](https://wiki.archlinux.org/title/Haskell_package_guidelines) and PHP |

`pkglint --rules` prints the full documentation for each, with its severity and
the flag that auto-fixes it. `pkglint explain PB101` (or `pkglint explain
skipped-checksum`) prints one rule's page — documentation, a flagged snippet
beside the preferred spelling, and how to fix or suppress it — the same
reference the [rule reference](https://jamison.lahman.dev/pkglint/rules/)
publishes.

### Relationship to namcap

pkglint covers namcap's rule set — both the PKGBUILD checks and the built-package
checks — with a few deliberate differences: nothing from the analyzed package is ever
executed (namcap runs `ldd -r -u` on packaged binaries; pkglint compares dynamic symbol
tables instead), findings the lint host cannot verify are reported informationally
instead of as hard errors, and everything is folded into the same graded, suppressible,
JSON/SARIF-capable reporting the PKGBUILD rules use. Dependency inference reads pacman's
local database directly and skips just those rules on non-Arch hosts.

Grading: any critical → **F**, any error → **D**, 3+ warns → **C**, 1–2 warns → **B**,
otherwise **A**.

A grade is a **static hygiene score, not a malware verdict** — it measures how reviewable
and reproducible a PKGBUILD is. A low grade means "worth reviewing", never "malicious",
and a high grade is not an endorsement. Static analysis cannot catch a malicious upstream
release pinned with a perfectly valid checksum.

## License

GPLv3 — see [LICENSE](LICENSE).
