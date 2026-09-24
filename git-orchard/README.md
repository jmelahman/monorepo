# `git-orchard`

[![pre-commit status](https://github.com/jmelahman/git-orchard/actions/workflows/pre-commit.yml/badge.svg)](https://github.com/jmelahman/git-orchard/actions)
[![Deploy Status](https://github.com/jmelahman/git-orchard/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/git-orchard/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/jmelahman/git-orchard.svg)](https://pkg.go.dev/github.com/jmelahman/git-orchard)
[![Arch User Repsoitory](https://img.shields.io/aur/version/git-orchard)](https://aur.archlinux.org/packages/git-orchard)
[![PyPI](https://img.shields.io/pypi/v/git-orchard.svg)](https://pypi.org/project/git-orchard/)
[![Go Report Card](https://goreportcard.com/badge/github.com/jmelahman/git-orchard)](https://goreportcard.com/report/github.com/jmelahman/git-orchard)

A command-line utility for managing [git-subtree](https://github.com/git/git/blob/master/contrib/subtree/git-subtree.adoc)s.

## Install

**AUR:**

`git-orchard` is available from the [Arch User Repository](https://aur.archlinux.org/packages/git-orchard).

```shell
yay -S git-orchard
```

**pip:**

`git-orchard` is available as a [pypi package](https://pypi.org/project/git-orchard/).

```shell
pip install git-orchard
```

**go:**

```shell
go install github.com/jmelahman/git-orchard@latest
```

## Usage

Subtrees are listed in a committed manifest at the repository root, `.gitsubtrees` or `.config/git-orchard/subtrees` (but not both), in the same syntax as `.gitmodules`:

```gitconfig
[subtree "tools/foo"]
	remote = git@github.com:owner/foo.git
	branch = master
```

The same keys in git's own configuration (e.g. `.git/config`) override it for one clone.
Pulls and adds are squashed unless the manifest sets `orchard.squash = false`.

```shell
git orchard init                        # list the subtrees already in git history
git orchard add tools/foo git@github.com:owner/foo.git
git orchard status                      # commits ahead/behind each upstream
git orchard pull [prefix...]            # merge upstream changes
git orchard push [prefix...]            # publish, fast-forward only
git orchard push --changed-since REV    # only subtrees changed since REV
git orchard push --tag tools/foo/v1.2.3 # publish as v1.2.3 upstream
git orchard release tools/foo          # tag the next version, e.g. tools/foo/v1.2.4, and push it to origin
git orchard release tools/foo v2.0.0   # or a version of your choosing
git orchard sync [prefix...]            # copy shared files into subtrees
```

Without a version, `release` picks one after the latest release, much as [tag](https://github.com/jmelahman/tag) does: the patch version incremented (`--minor` and `--major` increment those instead), or a pre-release's stable release; `--suffix rc` picks the next release candidate, e.g. `v1.2.4-rc`, then `v1.2.4-rc.1`.
Releases are the `<prefix>/v*` tags in the monorepo and on its remote, and the `v*` tags upstream, so releases from before the subtree count; `--dry-run` prints the pick.
`release` requires the upstream branch to contain the release already, so the upstream tag lands on its history; `--upstream` pushes the branch and tag there directly instead of leaving it to the action.
`push`, `pull` and `release` take `--no-verify` to skip git hooks.
`push --force` overwrites upstream branches and tags, leased on their value when the push starts so a concurrent update still fails it; the action never forces.
`release --force` moves an existing tag, unless the upstream already published it at another commit: the Go module proxy and release artifacts won't follow a moved release.

`push` splits each subtree out of the monorepo with `git subtree split`, which is deterministic, so the same history always gives the same commits and every push is a fast-forward.
An upstream with commits the monorepo doesn't have rejects the push until they're pulled in.

git-orchard only runs `git`, so credentials, SSH config and `url.<base>.insteadOf` rewrites apply as usual.

## Shared files

Subtrees that are published on their own each need their own copy of config like `.pre-commit-config.yaml` or `.github/dependabot.yml`.
`git orchard sync` keeps those copies in step with one source.
A subtree lists the profiles it shares, and each profile is a directory under `orchard.sharedDir` (`.config/git-orchard/shared` by default):

```gitconfig
[subtree "tools/foo"]
	remote = git@github.com:owner/foo.git
	shared = base
	shared = go
```

```
.config/git-orchard/shared/
  base/.github/dependabot.yml   # → tools/foo/.github/dependabot.yml
  base/.pre-commit-config.yaml
  go/.pre-commit-config.yaml
```

A file in a profile lands at the same path in the subtree, replacing it whole.
When the subtree's file marks a block for the profile, only the lines between the markers are replaced, and the rest of the file stays the subtree's own:

```yaml
repos:
  # BEGIN orchard:base
  # END orchard:base
  # BEGIN orchard:go
  # END orchard:go
  - repo: local # not shared
    hooks: [...]
```

Markers work in any comment syntax, since git-orchard only looks for `BEGIN orchard:<profile>` and `END orchard:<profile>` in the line.
Two profiles can share a file only through blocks.

`sync` exits 1 when it changes a file, like a formatter, and `--check` prints the differences without writing them.
To run it on every commit, add the hook to the monorepo's root pre-commit config (not to the subtrees', since their mirrors have no manifest):

```yaml
repos:
  - repo: https://github.com/jmelahman/git-orchard
    rev: v1.2.3
    hooks:
      - id: git-orchard-sync
```

## GitHub Action

This repository is also an action that mirrors a monorepo's subtrees on every push: changed subtrees are pushed to their upstreams, and a `<prefix>/<name>` tag is published to that prefix's upstream as `<name>`.

```yaml
on:
  push:
    branches: [master]
    tags: ["**/v*"] # `*` doesn't match `/`

jobs:
  mirror:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0 # splits need the full history
          persist-credentials: false
      - uses: jmelahman/git-orchard@v1
        with:
          app-client-id: ${{ vars.ORCHARD_APP_CLIENT_ID }}
          app-private-key: ${{ secrets.ORCHARD_APP_PRIVATE_KEY }}
```

The action pushes as a GitHub App, which `git orchard github-app` creates:

```shell
git orchard github-app  # add --org ORG for an organization's repositories
```

It opens a browser to create a private App under your account from a manifest (contents and workflows write, no webhook), then to install it: pick the upstream repositories there.
With the [GitHub CLI](https://cli.github.com) installed, it stores the App's client ID and private key on the monorepo (origin, or `--repo`) as the `ORCHARD_APP_CLIENT_ID` variable and `ORCHARD_APP_PRIVATE_KEY` secret; otherwise it writes the key to a file and prints the `gh` commands to store it.
The App and its key are yours; git-orchard runs no service.

The action mints a short-lived token from the App's installation for `owner` (default: the monorepo's owner).
Alternatively, pass `token`, e.g. a fine-grained token with "Contents" and "Workflows" read and write on the upstreams; GitHub refuses pushes that change `.github/workflows` without the latter.
Either way, GitHub remotes in the manifest are rewritten to use the token.
`changed-since` defaults to the start of the pushed range; set it empty to push every subtree.
The action builds git-orchard from its own source, so the CLI is always the version the action is pinned to.
