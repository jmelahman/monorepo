[![CI Status](https://github.com/jmelahman/monorepo/actions/workflows/main.yml/badge.svg)](https://github.com/jmelahman/monorepo/actions/workflows/main.yml)
[![Code style: black](https://img.shields.io/badge/code%20style-black-000000.svg)](https://github.com/psf/black)

# Projects

## Snapify

An executable to check if any packages installed with the host's package manager can be installed
as a [snap](https://snapcraft.io/) package.

The project is tracked using a [git-subtree](https://github.com/git/git/blob/master/contrib/subtree/git-subtree.txt).
To push changes upstream,

```shell
git subtree push --prefix subtrees/snapify git@github.com:jmelahman/python-snapify.git master
```

See also, [github://jmelahman/python-snapify](https://github.com/jmelahman/python-snapify).

## Pybazel

A python client for Bazel.

The project is tracked using a [git-subtree](https://github.com/git/git/blob/master/contrib/subtree/git-subtree.txt).
To push changes upstream,

```shell
git subtree push --prefix subtrees/pybazel git@github.com:jmelahman/pybazel.git master
```

See also, [github://jmelahman/pybazel](https://github.com/jmelahman/pybazel).

## Buildprint

Provides a blueprint print a buildkite pipeline.

The project is tracked using a [git-subtree](https://github.com/git/git/blob/master/contrib/subtree/git-subtree.txt).
To push changes upstream,

```shell
git subtree push --prefix subtrees/buildprint git@github.com:jmelahman/buildprint.git master
```

See also, [github://jmelahman/buildprint](https://github.com/jmelahman/buildprint).

# Tooling

## Dependencies

Python dependencies are specified in `third_party/requirements.in` and compiled to
`third_party/requirements.txt`.

To recompile `third_party/requirements.txt`, run,

```shell
bazel run //third_party:requirements.update
```

### Upgrading Dependencies

To upgrade all dependencies (dangerous),

```shell
bazel run //third_party:requirements.update -- --upgrade
```

To upgrade a single dependency (in this case, the `mypy` package),

```shell
bazel run //third_party:requirements.update -- --upgrade-package mypy
```

## Linting

### Mypy

Mypy is a static-type checker and linter for python.
There are three ways to run mypy:

1. standalone mypy
2. as a mypy daemon
3. as a bazel asepct

The preferred method is as a bazel aspect, but using a daemon or standalone might
be more performant or convenient depending on the workflow.

For either 1. or 2., you'll first have to install the build and runtime dependencies for the repo,

```
pip install -r tools/typing/mypy-requirements.txt
pip install -r third_party/requirements.txt
```

If using the mypy daemon, you'll then need to start it with the correct config,

```shell
dmypy start -- --config-file=tools/typing/mypy.ini
```

Then either:

```shell
mypy --config-file=tools/typing/mypy.ini
```

_Tip: if you copy this `tools/typing/.mypy.ini` to `~/.mypy.ini`, you don't need to pass the flag in either case._

or

```shell
dmypy check .
```

_Note: `dmypy check` must run from the toplevel of the repo unless you explicitly handle the status file._

_Note: With `mypy` and `dmypy`, mypy will use the local packages (`monorepo/pybazel`) rather than those installed in the site-packages whereas the oppisite is true for the bazel aspect._

To use the bazel aspect,

```
bazel build --config=mypy //...
```

_Tip: This also supports `ibazel`._

#### Generating python stubs

These packages are compiled with mypyc, so `.pyi` files are required for consumers to be able to reference the type hints.
These files are generated automatically using `stubgen`.

For example, to generate type stubs for the `pybazel` package, run,

```shell
stubgen pybazel --out subtrees/pybazel/
```

## Formatting

### Python

Formatting python is done by [black](https://github.com/psf/black).

To run the formatter,

```shell
bazel run //tools/format
```

or alternatively,

```shell
bazel build @pypi__310__black_22_12_0//:bin-black
bazel-bin/external/pypi__310__black_22_12_0/bin-black .
```

### BUILD

Formatting bazel `BUILD` and `.bzl` is done by [Buildifier](https://github.com/bazelbuild/buildtools/tree/master/buildifier).

To run the formatter,

```shell
bazel run //:buildifier
```

and verify with,

```shell
bazel run //:buildifier.check
```
