[![CI Status](https://github.com/jmelahman/monorepo/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/monorepo/actions/workflows/test.yml)
[![Dependabot Updates](https://github.com/jmelahman/monorepo/actions/workflows/dependabot/dependabot-updates/badge.svg)](https://github.com/jmelahman/monorepo/actions/workflows/dependabot/dependabot-updates)
[![Code style: black](https://img.shields.io/badge/code%20style-black-000000.svg)](https://github.com/psf/black)

# Projects

## Finished Projects

These projects are still in active development but may be considered "stable",

- [5-wild](https://github.com/jmelahman/5-wild)
- [agentic-kanban](https://github.com/jmelahman/agentic-kanban)
- [check-symlinks](https://github.com/jmelahman/check-symlinks) [![Test status](https://github.com/jmelahman/check-symlinks/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/check-symlinks/actions/workflows/test.yml) [![Deploy Status](https://github.com/jmelahman/check-symlinks/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/check-symlinks/actions/workflows/release.yml)
- [connections](https://github.com/jmelahman/connections) [![Test status](https://github.com/jmelahman/connections/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/connections/actions/workflows/test.yml) [![Deploy Status](https://github.com/jmelahman/connections/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/connections/actions/workflows/release.yml)
- [connections-ssh](https://github.com/jmelahman/connections-ssh) [![Test status](https://github.com/jmelahman/connections-ssh/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/connections-ssh/actions/workflows/test.yml) [![Deploy Status](https://github.com/jmelahman/connections-ssh/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/connections-ssh/actions/workflows/release.yml)
- [docker-status](https://github.com/jmelahman/docker-status)
- [go-bin](https://github.com/jmelahman/go-bin) [![Deploy Status](https://github.com/jmelahman/go-bin/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/go-bin/actions/workflows/release.yml)
- homelab
- [jmelahman.github.io](https://github.com/jmelahman/jmelahman.github.io) [![Deploy Status](https://github.com/jmelahman/jmelahman.github.io/actions/workflows/pages/pages-build-deployment/badge.svg)](https://github.com/jmelahman/jmelahman.github.io/actions/workflows/pages/pages-build-deployment)
- [local-preview](https://github.com/jmelahman/local-preview)
- [manygo](https://github.com/jmelahman/manygo) [![Test status](https://github.com/jmelahman/manygo/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/manygo/actions/workflows/test.yml) [![Deploy Status](https://github.com/jmelahman/manygo/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/manygo/actions/workflows/release.yml)
- [nature-sounds](https://github.com/jmelahman/nature-sounds) [![Test status](https://github.com/jmelahman/nature-sounds/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/nature-sounds/actions/workflows/test.yml) [![Deploy Status](https://github.com/jmelahman/nature-sounds/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/nature-sounds/actions/workflows/release.yml)
- [pkglint](https://github.com/jmelahman/pkglint)
- [skills](https://github.com/jmelahman/skills)
- [tag](https://github.com/jmelahman/tag) [![Test status](https://github.com/jmelahman/tag/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/tag/actions/workflows/test.yml) [![Deploy Status](https://github.com/jmelahman/tag/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/tag/actions/workflows/release.yml)
- [typesafe-sdk-go](https://github.com/jmelahman/typesafe-sdk-go)
- [undot](https://github.com/jmelahman/undot)
- [work](https://github.com/jmelahman/work) [![Test status](https://github.com/jmelahman/work/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/work/actions/workflows/test.yml) [![Deploy Status](https://github.com/jmelahman/work/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/work/actions/workflows/release.yml)

## Unfinished Projects

- [AgileCBT](https://github.com/jmelahman/AgileCBT)
- [cycle-cli](https://github.com/jmelahman/cycle-cli)
- [git-orchard](https://github.com/jmelahman/git-orchard)

## Browser Extensions

- [extensions/github-token-diff-extension](https://github.com/jmelahman/github-token-diff-extension)

## Pre-commit Hooks

- [pre-commit-hooks/go-pre-commit-hooks](https://github.com/jmelahman/go-pre-commit-hooks)

## Templates

- [templates/fullstack-template](https://github.com/jmelahman/fullstack-template)
- [templates/golang-template](https://github.com/jmelahman/golang-template)
- [templates/PKGBUILDs-template](https://github.com/jmelahman/PKGBUILDs-template)

## Subtrees

Most projects are tracked as [git-subtrees](https://github.com/git/git/blob/master/contrib/subtree/git-subtree.txt).
This allows them to be developed uniformly while leaving operational tasks, such as deployments, independent.

By design, the last component of each project's directory (referred to as the subtree's `<prefix>`) matches the upstream repository name.
For example, `connections/` → [github.com/jmelahman/connections](https://github.com/jmelahman/connections).
This is slightly more convenient to make shell functions since the `git-subtree` commands can be a bit cumbersome.

Update all upstreams with this command,

```shell
for d in $(git log --format=%b | sed -n 's/^git-subtree-dir: //p' | sort -u); do [ -d "$d" ] && gsp "$d"; done
```

And pulling from upstreams with,

```shell
for d in $(git log --format=%b | sed -n 's/^git-subtree-dir: //p' | sort -u); do [ -d "$d" ] && gspull "$d" -m "Update $d"; done
```

New projects are added with `git subtree add --squash --prefix=[<group>/]<name> git@github.com:jmelahman/<name>.git master` (and to `go.work`, for Go modules).

_See my [dotfiles](https://github.com/jmelahman/dotfiles/blob/a1a3e8abd2f746b5e24919f189d7df1d5f2d5911/.zshrc#L176-L203) for the `gsp` and `gspull` aliases._

# Tooling

## Upgrading

### Hooks

```shell
prek auto-update --freeze
```

### Github Actions

```
ratchet upgrade $(fd --hidden --type file --extension yml --full-path .github/workflows)
```

### Golang

```shell
find . -name go.mod -execdir go get -u ./... \;
```

## Checks

Linting, formatting and type-checking run as [prek](https://prek.j178.dev) hooks (see `.pre-commit-config.yaml`), with tools pinned by the config or `uv.lock` rather than taken from the host.

```shell
prek install        # once per clone
prek run            # staged files
prek run --all-files
```

The monorepo is a prek [workspace](https://prek.j178.dev/workspace/): each project's own `.pre-commit-config.yaml` runs from that project's directory, and the root config also runs on every file in the tree.
Module-scoped Go checks (`go vet`, `go test`, `golangci-lint`, ...) live in each module's config, since they have to run from a module root.
