# vinder

Vendored npm dependencies for projects that have no build step.

A committed `vendor/` directory usually has no record of where its code came
from, so nothing can tell it is out of date or vulnerable. vinder makes
`vendor/` a generated artifact: package.json and its lockfile pin the
versions, `vinder.toml` says what goes into `vendor/`, and vinder builds it
with [esbuild](https://esbuild.github.io), checks it, and audits it against
[OSV](https://osv.dev). The project still runs straight from its files.

## Install

```sh
go install github.com/jmelahman/vinder@latest
# or a prebuilt binary from npm
bun add -d vinder       # npm install -D vinder, pnpm add -D vinder
```

vinder installs with the project's own package manager (bun, npm,
[Nub](https://nubjs.com), pnpm or Yarn), which has to be on `PATH`. It picks
one from package.json's `packageManager` field, or else from the lockfile.

## Quick start

```sh
vinder add canvas-confetti
```

In a project with neither file, that creates package.json, on the first
package manager on `PATH`, and `vinder.toml`, then writes the bundle:

```html
<script type="module">
  import confetti from "./vendor/canvas-confetti/canvas-confetti.mjs";
  confetti();
</script>
```

Commit package.json, the lockfile, `vinder.toml` and `vendor/`. `add` vendors
a package's main entry as one ES module. For anything else, such as a global,
a stylesheet or a license, edit its entry in `vinder.toml` and run `vinder
sync`. A CommonJS package bundled this way has only a default export. See
[examples/](examples) for both kinds of project.

## Configuration

`vinder.toml` sits next to package.json:

```toml
# out = "vendor"   # the default

[deps.toastui-editor]                 # -> vendor/toastui-editor/
package = "@toast-ui/editor"          # the package being vendored
embeds  = ["dompurify@2.3.3"]         # code it inlines, for the audit

[deps.toastui-editor.bundle]          # output = entry, built by esbuild
"toastui-editor-all.min.js" = "./vendor.src/toastui-editor.js"
"toastui-editor.min.css"    = "@toast-ui/editor/dist/toastui-editor.css"

[deps.toastui-editor.copy]            # output = source, copied as is
LICENSE = "./vendor.src/toastui-editor.LICENSE"

[deps.toastui-editor.ignore]          # advisory ID or name@version = reason
"dompurify@2.3.3" = "never runs: app.js replaces the sanitizer"
```

- A path starting with `./` is in the project; anything else is in
  `node_modules`. A `bundle` entry that is only a package name resolves as an
  import of it would, through the package's `exports`.
- `bundle` outputs are minified for the browser: `.js` as an IIFE, `.mjs` as
  an ES module, `.css` with its imports inlined and its images and fonts copied beside it. An entry that
  imports only what the project uses lets esbuild tree-shake the rest; one
  that should set a global does so itself (`window.toastui = { Editor }`).
- Change a dependency's code with the package manager's patch command
  (`bun patch`, `pnpm patch`, ...), never by editing `vendor/`.

## Usage

```sh
vinder add <package>       # depend on a package and vendor it
vinder sync                # install, regenerate vendor/, update the lockfile
vinder check               # fail if vendor/ differs from what the lockfile makes
vinder audit               # fail on OSV advisories for anything vendored
vinder sync --align --all  # move projects onto one version of each shared package
vinder upgrade --all       # bump to latest, re-resolve, align; prints a PR body
```

Each command but `add` takes project directories (the current one by default)
or `--all`, every `vinder.toml` in the git repository; `add` takes `--dir`.

- **add** takes `name` or `name@version`, with any version npm would take.
  Without one it keeps a dependency package.json already has, and otherwise
  asks for `^` the latest release. A tag or a partial version (`@next`, `@1`)
  goes in as `^` the version it means, so `upgrade` can move it; anything else
  (`@1.2.3`, `@^1.2.0`, `@npm:other@1`) goes in as written. It refuses to
  replace a `vendor/<key>` that vinder did not write. If anything fails, package.json, the lockfile and
  `vinder.toml` are put back.
- **sync** never runs lifecycle scripts, and replaces only the
  `vendor/<key>/` directories it owns and `vendor/vinder.json`.
- **check** rebuilds from the lockfile into a temporary directory and lists
  every file that is modified, missing or extra. Across several projects it
  also fails if one package is vendored at more than one version.
- **audit** reads only `vendor/vinder.json`, so it needs no install.
- **upgrade** raises each direct dependency with a plain spec (`1.2.3`,
  `^1.2.3`, `~1.2.3`) to its latest release, re-resolves every transitive
  dependency, syncs and aligns. On failure, package.json and the lockfile are
  put back. `--cooldown-days <n>` holds direct and transitive dependencies to
  releases at least that old. It fails, rather than move a dependency back, if
  a spec in package.json allows only younger releases, and on a package manager
  that can't enforce it: Yarn 1, bun before 1.3, pnpm before 10.16 or Yarn 2+
  before 4.10.

### What the audit covers

`vendor/vinder.json` lists the packages whose code actually ships: each
vendored package, everything esbuild bundled in, and `embeds`. A package
tree-shaken out of every bundle is not audited.

`embeds` is for code a package inlines in its own dist files, which no
lockfile can see: `@toast-ui/editor` ships its own DOMPurify 2.3.3 whatever
the lockfile says.

An `ignore` entry needs a reason. A `name@version` key covers every advisory
for that version, including future ones; an ID covers just that advisory.

## pre-commit

With [pre-commit](https://pre-commit.com) or [prek](https://prek.j178.dev):

```yaml
repos:
  - repo: https://github.com/jmelahman/vinder
    rev: v0.1.4
    hooks:
      - id: vinder-check
      - id: vinder-audit
```

`vinder-check` runs `check --all` on every commit. `vinder-audit` runs
`audit --all` only when a `vinder.json` or `vinder.toml` changes, since it
goes to the network. Both need the projects' package managers on `PATH`;
the `-system` variants use a vinder already on `PATH` instead of building
one.

## Scheduled upgrades

Dependabot can't regenerate `vendor/`, so its PRs fail `vinder check`; leave
vinder projects out of `dependabot.yml` and use the action instead. It runs
`upgrade --all`, adds any `audit` findings to the PR body, and opens the PR:

```yaml
on:
  schedule:
    - cron: "23 6 * * 1"
  workflow_dispatch:
permissions: {}
jobs:
  upgrade:
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@v6
        with:
          persist-credentials: false
      - uses: oven-sh/setup-bun@v2 # the projects' package managers
      - uses: jmelahman/vinder@v0.1.4
        with:
          app-client-id: ${{ vars.VINDER_APP_CLIENT_ID }}
          app-private-key: ${{ secrets.VINDER_APP_PRIVATE_KEY }}
          cooldown-days: 7
```

A PR opened with `GITHUB_TOKEN` doesn't trigger CI, so the action opens it
as a GitHub App. `vinder github-app` creates the App and stores those two
values in the repository with `gh`; then install it from the page it prints.
Alternatively, pass a `token` that can push and open PRs. See
[action.yml](action.yml) for the other inputs.

## Develop

```sh
go test -race ./...    # under each package manager on PATH
prek run --all-files
prek run --all-files --hook-stage pre-push
```
