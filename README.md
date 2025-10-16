[![CI Status](https://github.com/jmelahman/monorepo/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/monorepo/actions/workflows/test.yml)
[![Dependabot Updates](https://github.com/jmelahman/monorepo/actions/workflows/dependabot/dependabot-updates/badge.svg)](https://github.com/jmelahman/monorepo/actions/workflows/dependabot/dependabot-updates)
[![Code style: black](https://img.shields.io/badge/code%20style-black-000000.svg)](https://github.com/psf/black)

# Projects

## Finished Projects

These projects are still in active development but may be considered "stable",

- [check-symlinks](https://github.com/jmelahman/check-symlinks) [![Test status](https://github.com/jmelahman/check-symlinks/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/check-symlinks/actions/workflows/test.yml) [![Deploy Status](https://github.com/jmelahman/check-symlinks/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/check-symlinks/actions/workflows/release.yml)
- [connections](https://github.com/jmelahman/connections) [![Test status](https://github.com/jmelahman/connections/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/connections/actions/workflows/test.yml) [![Deploy Status](https://github.com/jmelahman/connections/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/connections/actions/workflows/release.yml)
- [connections-ssh](https://github.com/jmelahman/connections-ssh) [![Test status](https://github.com/jmelahman/connections-ssh/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/connections-ssh/actions/workflows/test.yml) [![Deploy Status](https://github.com/jmelahman/connections-ssh/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/connections-ssh/actions/workflows/release.yml)
- [docker-status](https://github.com/jmelahman/docker-status)
- [go-bin](https://github.com/jmelahman/go-bin) [![Deploy Status](https://github.com/jmelahman/go-bin/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/go-bin/actions/workflows/release.yml)
- homelab
- [jmelahman.github.io](https://github.com/jmelahman/jmelahman.github.io) [![Deploy Status](https://github.com/jmelahman/jmelahman.github.io/actions/workflows/pages/pages-build-deployment/badge.svg)](https://github.com/jmelahman/jmelahman.github.io/actions/workflows/pages/pages-build-deployment)
- [manygo](https://github.com/jmelahman/manygo) [![Test status](https://github.com/jmelahman/manygo/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/manygo/actions/workflows/test.yml) [![Deploy Status](https://github.com/jmelahman/manygo/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/manygo/actions/workflows/release.yml)
- [nature-sounds](https://github.com/jmelahman/nature-sounds) [![Test status](https://github.com/jmelahman/nature-sounds/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/nature-sounds/actions/workflows/test.yml) [![Deploy Status](https://github.com/jmelahman/nature-sounds/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/nature-sounds/actions/workflows/release.yml)
- [PKGBUILDs](https://github.com/jmelahman/pkgbuilds) [![Nightly updates status](https://github.com/jmelahman/pkgbuilds/actions/workflows/nightly.yml/badge.svg)](https://github.com/jmelahman/pkgbuilds/actions/workflows/nightly.yml) [![Deploy Status](https://github.com/jmelahman/pkgbuilds/actions/workflows/deploy.yml/badge.svg)](https://github.com/jmelahman/pkgbuilds/actions/workflows/deploy.yml)
- [tag](https://github.com/jmelahman/tag) [![Test status](https://github.com/jmelahman/tag/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/tag/actions/workflows/test.yml) [![Deploy Status](https://github.com/jmelahman/tag/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/tag/actions/workflows/release.yml)
- [work](https://github.com/jmelahman/work) [![Test status](https://github.com/jmelahman/work/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/work/actions/workflows/test.yml) [![Deploy Status](https://github.com/jmelahman/work/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/work/actions/workflows/release.yml)


## Unfinished Projects

- cycle-cli
- dashboard
- git-orchard
- runtainer

## Subtrees

Most projects are tracked as [git-subtrees](https://github.com/git/git/blob/master/contrib/subtree/git-subtree.txt).
This allows them to be developed uniformly while leaving operational tasks, such as deployments, independent.

By design, each projects's directory (referred to as the subtree's `<prefix>`) matches the upstream repository name.
For example, `connections/` → [github.com/jmelahman/connections](https://github.com/jmelahman/connections).
This is slightly more convenient to make shell functions since the `git-subtree` commands can be a bit cumbersome.

Update all upstreams with this command,

```shell
for d in */; do gsp "${d%/}"; done
```

And pulling from upstreams with,

```shell
for d in */; do gspull "${d%/}" -m "Update ${d%/}"; done
```

_See my [dotfiles](https://github.com/jmelahman/dotfiles/blob/a1a3e8abd2f746b5e24919f189d7df1d5f2d5911/.zshrc#L176-L203) for the `gsp` and `gspull` aliases._

# Tooling

## Upgrading

### Github Actions

```
ratchet upgrade $(fd --hidden --type file --extension yml --full-path .github/workflows)
```
### Golang

```shell
find . -name go.mod -execdir go get -u ./... \;
```

## Linting

### Generic

Check for broken symlinks,

```shell
uvx check-symlinks
```
### Golang

```shell
find . -name go.mod -execdir golangci-lint run ./... \;
```

### Github Actions

```
uvx zizmor $(fd --hidden --type file --extension yml --full-path .github/workflows)
```

### Python

```shell
uvx ruff check
```

### Shell

```shell
./tools/bin/shellcheck
```

## Type-checking

### Python

```shell
uv sync
uvx ty check
```

## Formatting

### Python

```shell
uvx ruff format
```

### Shell

```shell
./tools/bin/shfmt
```
