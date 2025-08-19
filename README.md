[![CI Status](https://github.com/jmelahman/monorepo/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/monorepo/actions/workflows/test.yml)
[![Code style: black](https://img.shields.io/badge/code%20style-black-000000.svg)](https://github.com/psf/black)

# Projects

## Subtrees

Most projects are tracked as [git-subtrees](https://github.com/git/git/blob/master/contrib/subtree/git-subtree.txt).
This allows them to be developed uniformly while leaving operational tasks, such as deployments, independent.

By design, each projects's directory (referred to as the subtree's `<prefix>`) matches the upstream repository name.
For example, `connections/` → [github.com/jmelahman/connections](https://github.com/jmelahman/connections).
This is slightly more convenient to make shell functions since the `git-subtree` commands can be a bit cumbersome.

# Tooling

## Upgrading

### Github Actions

```
ratchet upgrade $(fd --hidden --type file --extension yml --full-path .github/workflows)
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

### Python

```shell
uvx ruff check
```

### Shell

```shell
./bin/shellcheck
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
./bin/shfmt
```
