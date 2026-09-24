# `git-orchard`

[![Test status](https://github.com/jmelahman/git-orchard/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/git-orchard/actions)
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
git orchard push [prefix...]            # publish, never forced
git orchard push --changed-since REV    # only subtrees changed since REV
git orchard push --tag tools/foo/v1.2.3 # publish as v1.2.3 upstream
```

`push` splits each subtree out of the monorepo with `git subtree split`, which is deterministic, so the same history always gives the same commits and every push is a fast-forward.
An upstream with commits the monorepo doesn't have rejects the push until they're pulled in.

git-orchard only runs `git`, so credentials, SSH config and `url.<base>.insteadOf` rewrites apply as usual.

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
