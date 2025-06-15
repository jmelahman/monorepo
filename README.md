[![CI Status](https://github.com/jmelahman/monorepo/actions/workflows/main.yml/badge.svg)](https://github.com/jmelahman/monorepo/actions/workflows/main.yml)
[![Code style: black](https://img.shields.io/badge/code%20style-black-000000.svg)](https://github.com/psf/black)

# Projects

## Subtrees

Most projects are tracked as [git-subtree](https://github.com/git/git/blob/master/contrib/subtree/git-subtree.txt)s.
This allows them to be developed uniformly while leaving operational tasks delegated.

# Tooling

## Linting

### Generic

Check for broken symlinks,

```shell
uvx check-symlinks
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
