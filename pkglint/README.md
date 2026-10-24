# pkglint

A security-focused linter for Arch Linux packages.

```shell
$ pkglint ~/pkgbuilds/somepkg
somepkg: grade F, 3 finding(s)
  PKGBUILD:16:3: critical [PB304] a network download is piped straight into bash and executed
  PKGBUILD:11:1: error [PB101] remote source "http://..." has no checksum (SKIP): the download is never verified
  PKGBUILD:11:1: error [PB104] source "http://..." is fetched over unencrypted http:// with nothing verifying what arrives

1 package linted: 1 with findings
1 finding(s) fixable with --unsafe-fix
```

- **Never runs the package.** PKGBUILDs are parsed into a real bash syntax tree, not
  sourced, so quoting and line-continuation tricks that fool regex scanners don't work.
  Built packages are inspected without loading or executing anything in them.
- **Checks what matters for trust:** unverified sources, builds that fetch undeclared
  code, download-and-execute, writes outside the package, and persistence in install
  scripts.
- **Covers namcap and makepkg too:** namcap's PKGBUILD and built-package checks, plus
  makepkg's own build-breaking metadata errors, all in one report.
- **Fixes what it can.** `--fix` applies the safe rewrites, and `--unsafe-fix` applies
  the ones that change build behavior.

The [AUR Report Card](https://jamison.lahman.dev/pkglint/) runs pkglint nightly over
the official repositories and the AUR's most popular and recently updated packages.

## Install

**AUR**

```shell
yay -S pkglint
```

**PyPI**

```shell
uv tool install pkglint
```

**Go**

```shell
go install github.com/jmelahman/pkglint@latest
```

**GitHub Releases**

Prebuilt binaries for Linux and macOS (amd64 and arm64) are attached to every
[release](https://github.com/jmelahman/pkglint/releases/latest).

**pre-commit hook**

```yaml
repos:
  - repo: https://github.com/jmelahman/pkglint
    rev: 18319ffe6c6ea34d19fe0660ac2e99578cb883f0 # frozen: v1.7.0
    hooks:
      - id: pkglint
```

<details>
<summary>Other hook ids and arguments</summary>

| Hook id          | Runs                                                                      |
| ---------------- | ------------------------------------------------------------------------- |
| `pkglint`        | pkglint, built from source                                                |
| `pkglint-system` | the `pkglint` already on `$PATH`                                          |
| `pkglint-fix`    | the safe auto-fixes, in place                                             |
| `pkglint-build`  | a full build, then lints the result ([manual stage](#build-and-validate)) |

Pass flags with `args:`, e.g. `args: [--ignore, PB105, --fail-on, critical]`.

</details>

**yay hook (yay ≥ 13)**

To lint every AUR package before yay builds it, add the snippet below to
`~/.config/yay/init.lua`. It runs after each PKGBUILD is downloaded, before the
diff/edit menus or any build, and asks before continuing if pkglint finds anything.

<details>
<summary><code>init.lua</code> snippet</summary>

```lua
-- pkglint: lint each AUR PKGBUILD before yay shows menus or builds anything.
-- PKGLINT_FAIL_ON overrides the threshold per run (info|warn|error|critical).
local function shquote(s) return "'" .. s:gsub("'", [['\'']]) .. "'" end

yay.create_autocmd("AURPreInstall", {
  desc = "pkglint: refuse packages with static findings at or above the threshold",
  callback = function(event)
    local fail_on = os.getenv("PKGLINT_FAIL_ON") or "error"
    -- --no-inline-ignores: the maintainer's "# pkglint: ignore=" doesn't get to
    -- hide findings from the person installing the package.
    local status = os.execute("pkglint --no-inline-ignores --fail-on "
      .. shquote(fail_on) .. " " .. shquote(event.data.dir))
    if status == 0 then return end
    io.write(event.match .. ": pkglint reported findings. Continue anyway? [Y/n] ")
    io.flush()
    -- Enter continues; "n" or no terminal to read from (EOF) refuses.
    local answer = io.read("*l")
    if answer == nil or answer:match("^%s*[nN]") then
      yay.abort("refused by pkglint")
    end
  end,
})
```

</details>

## Usage

```shell
pkglint [flags] [path ...]     # package dirs, PKGBUILD files, or *.pkg.tar.* (default: .)

  --format text|json|sarif     # output format (sarif for code scanning)
  --fail-on SEVERITY           # exit 1 at or above: info, warn (default), error, critical, never
  --ignore PB105,PB206         # disable rules
  --select PB101,PB304         # check only these rules
  --fix                        # apply safe auto-fixes in place
  --unsafe-fix                 # also apply behavior-changing fixes
  --diff                       # with a fix flag: show changes instead of writing them
  --offline                    # with --fix: skip fixes that need the network
  --add-ignores                # suppress every current finding with an inline directive
  --no-inline-ignores          # disregard ignore directives (audit an untrusted package)
  --verbose                    # also list packages with no findings
  --color auto|always|never

pkglint explain PB101          # one rule's docs, an example, and how to fix or suppress it
pkglint --rules                # every rule
```

### Suppressing findings

```bash
# pkglint: ignore=PB204
go build -o "$pkgname" .
```

A directive covers its own line and the next. To adopt pkglint on an existing package
without fixing everything at once, run `--add-ignores` once. Directives that stop
matching a finding are flagged and removed by `--fix`, so a fixed problem can't stay
silenced if it comes back.

### Auto-fixing

`--fix` rewrites what it can and prints every change. Run it with `--diff` to preview.
Safe fixes keep the build's behavior or restore a security default. Unsafe fixes, such
as upgrading `http://` to `https://` or adding `--locked` to `cargo`, change what the
build does, so review them. A fix that can't see the whole picture is skipped rather
than half-applied. For findings without a mechanical fix, pkglint prints a hint like
`updpkgsums` or `makepkg --printsrcinfo` instead.

`pkglint --rules` and the [rule reference](https://jamison.lahman.dev/pkglint/rules/)
show which rules each flag fixes.

### Build and validate

The built-package rules (PB8xx) need a built package:

```shell
pkglint build ~/pkgbuilds/somepkg
```

This lints the PKGBUILD, runs `makepkg`, and then lints the archive it produces.
`pkglint build --fix` also fixes PKGBUILD issues found only in the build output, such
as a linked library missing from `depends`.

This is the only pkglint command that runs PKGBUILD code. It **refuses to build when
the static findings reach `--fail-on`**, and always on a critical one. `--force`
overrides the refusal. Nothing in the PKGBUILD can talk its way past that check.

How the build is isolated, and what you can change:

- The gate ignores the file's own `# pkglint: ignore=` directives. A `curl | bash`
  with `ignore=PB304` above it is still refused. The printed report still honors
  them.
- `makepkg` flags and words that would build or source a different file (`-p`,
  `-D`, `--config`, `BUILDFILE=`, `PKGDEST=`, `BUILDDIR=`, …) are rejected, and a
  file argument must be the `PKGBUILD` itself.
- The packaging tree is left untouched: archives go to a temporary `PKGDEST` (use
  `--keep <dir>` to keep them), and the `PKGBUILD` is read-only during the build.
- Sources are cached in `${XDG_CACHE_HOME:-~/.cache}/pkglint/sources` and the build
  happens under `$TMPDIR`. Set `$SRCDEST` or `$BUILDDIR` to move either.
- Dependencies are not installed and nothing is signed, so the hook never prompts
  for a password. Pass `-- -s` (or `--makepkg-arg=-s` in pre-commit `args:`) to
  install dependencies. In a container, pkglint intercepts `-s`: makepkg gets no
  sudo there. pkglint reads the dependency lists from the PKGBUILD statically
  and installs them as root with `pacman -Syu` before the package directory is
  copied in. It refuses a PKGBUILD whose lists cannot be read without running
  it, for example lists built by a command substitution, `eval`, `source`, or
  conditional assignment. For those, use an image with the dependencies
  already installed.
- Without `makepkg` on the host, or with `--docker`/`--image`, the build runs in a
  container (`$PKGLINT_BUILD_IMAGE`, `$PKGLINT_BUILD_RUNNER` = `docker` or
  `podman`). The container is a convenience, not a sandbox. The PKGBUILD's code
  still runs, and you are trusting the image.
- To lint a directory named `build`, write `pkglint ./build`.

## Rules

| Rules       | Category      | Asks                                                                     |
| ----------- | ------------- | ------------------------------------------------------------------------ |
| PB101–PB116 | Integrity     | Is every source verified, pinned, and fetched over a secure transport?   |
| PB201–PB210 | Hermeticity   | Does the build download anything it didn't declare and lock?             |
| PB301–PB310 | Execution     | Does it `eval`, decode-and-run, or pipe a download into a shell?         |
| PB401–PB405 | Filesystem    | Does it write outside the package, escalate privileges, or set setuid?   |
| PB501–PB505 | Scriptlets    | Do the root-run `.install` scripts reach the network or persist?         |
| PB601–PB603 | Consistency   | Does `.SRCINFO` match? Does it claim to replace core packages?           |
| PB701–PB711 | Correctness   | Would makepkg reject it?                                                 |
| PB801–PB842 | Built package | Is the binary hardened and stripped, with all its dependencies declared? |
| PB901–PB984 | Style         | Does it follow namcap and the Arch packaging guidelines?                 |

Run `pkglint explain <rule>` or see the
[rule reference](https://jamison.lahman.dev/pkglint/rules/) for each rule's details.

## License

GPLv3. See [LICENSE](LICENSE).
