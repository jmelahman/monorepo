# vinder

Vendored npm dependencies for projects that have no build step.

A project that serves third-party browser code from a committed `vendor/`
directory usually has no record of where that code came from, so nothing
can tell it is out of date or vulnerable. vinder makes `vendor/` a generated
artifact: package.json and its lockfile pin the versions, `vinder.toml` says
what goes into `vendor/`, and vinder regenerates it, checks it, and audits
it against [OSV](https://osv.dev). The project still runs straight from its
files; node_modules exists only while vinder runs.

## Install

```sh
go install github.com/jmelahman/vinder@latest
# or, from npm, as a prebuilt binary
bun add -d vinder       # npm install -D vinder, pnpm add -D vinder
```

The npm package runs the binary for the platform from an optional
dependency (`@jmelahman/vinder-linux-x64` and so on), the way esbuild does,
so it needs no install script and works with bun and pnpm, which skip them.

vinder bundles with [esbuild](https://esbuild.github.io), which is built
in, and installs with the project's own package manager, which has to be on
`PATH`. It supports bun, npm, [Nub](https://nubjs.com), pnpm and Yarn, and
picks one the way corepack does: the `packageManager` field of package.json
(or `devEngines.packageManager`), and otherwise whichever lockfile is
present. Lockfiles from two package managers and no `packageManager` field
is an error rather than a guess.

## Usage

```sh
vinder sync            # install, regenerate vendor/, update the lockfile
vinder check           # fail if vendor/ differs from what the lockfile makes
vinder check --all     # ...in any project, or if two vendor different versions
vinder audit           # report OSV advisories for everything vendored
vinder audit --all     # every project in the git repository
vinder sync --align --all  # move projects onto one version of each shared package
vinder upgrade --all   # bump to latest, re-resolve, align; prints a PR body
```

Each command takes project directories as arguments (the current directory
by default) or `--all`, which finds every `vinder.toml` in the enclosing git
repository, skipping `node_modules` and hidden directories.

- **sync** installs in the project without running lifecycle scripts
  (`--ignore-scripts`, or `YARN_ENABLE_SCRIPTS=false` for Yarn 2+), builds
  and copies every output, and writes `vendor/vinder.json`. It replaces only
  the directories it owns (one per `[deps.<key>]`) and the manifest, and
  removes the directory of a key that has been dropped from the config.
  Anything else in `vendor/` is left alone.
- **check** does the same from the lockfile alone (`--frozen-lockfile`,
  `npm ci` or `yarn install --immutable`) into a temporary directory and
  lists every file that is `modified`, `missing` or `extra`.
  It exits 1 if there are any, so it catches hand edits, a config change
  that was never synced, and a package.json bump without a lockfile update.
  Given several projects (or `--all`), it also compares their manifests
  and exits 1 if one package, vendored or bundled in, appears at more than
  one version, listing which projects have which. Each project keeps its
  own package.json and lockfile, so it still installs on its own; this is
  what keeps them in step.
- **audit** reads only the manifests, so it needs no install. It sends one
  OSV batch query for every package in them and exits 1 if any has an
  advisory that is not ignored.
- **sync --align** syncs, then settles every package `check` would call
  skewed. It picks the highest version vendored anywhere that satisfies
  every range asked of the package: by each project's package.json and
  overrides, and by every vendored package that depends on it. A project on
  another version is moved to it, by its dependency spec if the package is
  a direct dependency (keeping `^` or `~`), and otherwise by an override
  (`overrides` for npm, bun and nub, `pnpm.overrides`, or Yarn's
  `resolutions`), which `vinder.json` records under `aligned` as vinder's.
  The changed projects are synced again. A package no one version satisfies
  is reported and left alone.
- **upgrade** is Dependabot for vendored dependencies. It raises each
  direct dependency with a plain spec (`1.2.3`, `^1.2.3`, `~1.2.3`) to the
  registry's latest release (`--registry`, npmjs.org by default), drops the
  overrides an earlier alignment added, deletes the lockfile so every
  transitive dependency resolves to the newest release its ranges allow,
  syncs, and aligns. Ranges, tags and `file:`/git specs are left alone, as
  are overrides you wrote. It prints what changed as Markdown. If a step
  fails, package.json and the lockfile are put back.

### Scheduled upgrades

Dependabot can bump package.json and the lockfile but can't regenerate
`vendor/`, so its PRs fail `vinder check`. The vinder GitHub Action does
its job instead: it builds vinder from its own source, runs `upgrade`, runs
`audit` and adds any advisories to the PR body rather than failing, and
opens or updates the pull request.

A pull request opened with `GITHUB_TOKEN` doesn't trigger CI, so the
action opens it as a GitHub App. `vinder github-app` creates one: it opens
GitHub on an App manifest (contents and pull requests write), takes the
credentials back, and stores the client ID as the `VINDER_APP_CLIENT_ID`
variable and the private key as the `VINDER_APP_PRIVATE_KEY` secret of the
repository (`--repo`, the `origin` remote by default) with `gh`. Then
install the App on the repository from the page it prints. `--org` creates
it under an organization, and `--no-secrets` writes the key to a file and
prints the `gh` commands instead.

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
      - uses: oven-sh/setup-bun@v2 # or the projects' package managers
      - uses: jmelahman/vinder@v0.1.4
        with:
          app-client-id: ${{ vars.VINDER_APP_CLIENT_ID }}
          app-private-key: ${{ secrets.VINDER_APP_PRIVATE_KEY }}
```

The projects' package managers have to be set up first. `token` takes the
place of the App with a token that can push and open pull requests;
`directories` limits the upgrade to some projects (every one by default);
`audit: false` skips the audit; and `branch`, `title` and `labels` shape the
pull request (`vinder/upgrade`, "Upgrade vendored npm dependencies",
`dependencies`). The `pull-request-number` and `pull-request-url` outputs
name what it opened. Leave the vinder projects out of `dependabot.yml`.

### pre-commit

To enforce this on commit with [pre-commit](https://pre-commit.com) or
[prek](https://prek.j178.dev):

```yaml
repos:
  - repo: https://github.com/jmelahman/vinder
    rev: v0.1.0
    hooks:
      - id: vinder-check
      - id: vinder-audit
```

`vinder-check` runs `check --all` on every commit; with the package
manager's cache warm it takes a fraction of a second. `vinder-audit` runs
`audit --all` only when a `vinder.json` or `vinder.toml` changes, since it
goes to the network; run it on a schedule in CI to hear about new
advisories. Both need the projects' package managers on `PATH`.
`vinder-check-system` and `vinder-audit-system` use a vinder already on
`PATH` instead of building one.

## Configuration

`vinder.toml` sits next to the project's package.json:

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

- A source or entry starting with `./` is a file in the project, and
  anything else a path in `node_modules`.
- `bundle` outputs are minified by esbuild for the browser: `.js` as an
  IIFE, `.mjs` as an ES module, `.css` with its imports inlined and the
  images and fonts its `url()`s name copied beside it under a content hash.
  License comments are kept, at the end of the file. Bundling is how a
  dependency gets smaller: the entry imports only what the project uses,
  and the rest is tree-shaken away. An entry that should set a global does
  so itself (`window.toastui = { Editor }`).
- To change a dependency's code, patch it with the package manager
  (`bun patch`, `pnpm patch`, `nub patch`, `yarn patch`, or
  [patch-package](https://github.com/ds300/patch-package) with npm, which
  needs its postinstall script run by hand as vinder skips scripts). The
  patch is applied on install, so the change survives upgrades or
  fails loudly when it no longer applies. Don't edit `vendor/`: `check`
  will flag it.
- `overrides` in package.json pins a transitive dependency, for instance
  one a newer release broke.

### What the audit covers

`vendor/vinder.json` records, for each dependency, the packages whose code
is actually in its files: the package itself, every package esbuild's metafile
says a bundle pulled in, and `embeds`. A package that is in the lockfile but
tree-shaken out of every bundle is not audited, because none of its code
ships.

`embeds` exists because some packages inline other packages in their own
dist files, and no lockfile can see that. `@toast-ui/editor`, for example,
ships its own copy of DOMPurify 2.3.3 whatever version the lockfile resolves.
Finding these takes reading the dist files: version strings and license
banners are the usual clues.

An `ignore` entry needs a reason. A `name@version` key covers every
advisory for that version, including ones published later. An ID covers
just that advisory.

## Develop

```sh
go test -race ./...                        # under each package manager on PATH
prek run --all-files                       # lint, as CI runs it
prek run --all-files --hook-stage pre-push
```
