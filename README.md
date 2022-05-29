[![CI Status](https://github.com/jmelahman/monorepo/actions/workflows/main.yml/badge.svg)](https://github.com/jmelahman/monorepo/actions/workflows/main.yml)
[![Code style: black](https://img.shields.io/badge/code%20style-black-000000.svg)](https://github.com/psf/black)

# Projects

## Snapify

An executable to check if any packages installed with the host's package manager can be installed
as a [snap](https://snapcraft.io/) package.

The project is tracked using a [git-subtree](https://github.com/git/git/blob/master/contrib/subtree/git-subtree.txt).
To push changes upstream,

```shell
git subtree push --prefix snapify git@github.com:jmelahman/python-snapify.git master
```

See also, [github://jmelahman/python-snapify](https://github.com/jmelahman/python-snapify).

## Pybazel

A python client for Bazel.

The project is tracked using a [git-subtree](https://github.com/git/git/blob/master/contrib/subtree/git-subtree.txt).
To push changes upstream,

```shell
git subtree push --prefix pybazel git@github.com:jmelahman/pybazel.git master
```

See also, [github://jmelahman/pybazel](https://github.com/jmelahman/pybazel).

# Tooling

## Dependencies

Python dependencies are specified in `third_party/requirements.in` and compiled to
`third_party/requirements.txt`.

To recompile `third_party/requirements.txt`, run,

```shell
bazel run //third_party:requirements.update
```

### Upgrading Dependencies

// TODO

## Formatting

### Python

Formatting python is done by [black](https://github.com/psf/black).

To run the formatter,

```shell
bazel run //tools/format
```

### BUILD

Formatting bazel `BUILD` and `.bzl` is done by [Buildifier](https://github.com/bazelbuild/buildtools/tree/master/buildifier).

To run the formatter,

```shell
bazel run :buildifier
```
