# Prebuilt Go Runtime

[![Deploy Status](https://github.com/jmelahman/go-bin/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/go-bin/actions/workflows/release.yml)
[![Go Latest Release](https://img.shields.io/github/v/tag/golang/go?sort=semver&logo=go)](https://github.com/golang/go/tags)
[![PyPI](https://img.shields.io/pypi/v/go-bin.svg)](https://pypi.org/project/go-bin/)

## Overview

This package provides a prebuilt Go runtime for integrating Golang artifacts into Python projects.

## Features

- Automatic Go runtime download for multiple platforms
- Cross-platform support (Windows, Linux, macOS)
- Declarative integration with python build systems

### Using the `go` binary

One potential use case for `go-bin` is to facilitate managing the go binary.
Rather than [managing multiple go installations](https://go.dev/doc/manage-install),
utilize tools such as [uv](https://docs.astral.sh/uv/) to switch between environments.

Without needing anything other than `uv`, building a golang binary becomes as simple as,

```shell
uvx --from=go-bin go build ./...
```

### Using `go-bin` for packaging

Whether you're building standalone go binaries or writing [c-extensions in golang](https://words.filippo.io/building-python-modules-with-go-1-5),
this package allows a declarative and hermetic way to build golang source code.

Simply define a build dependency on `go-bin`,

```toml
[build-system]
requires = ["hatchling", "go-bin~=1.23.4"]
build-backend = "hatchling.build"
```

_It is recommended to use [compatible release versions (`~=`)](https://peps.python.org/pep-0440/#version-specifiers). Major, minor, and patch versions of `go-bin` will always correlate with Go versions while the latter digit is reserved for changes in packaging._

then use it in your build scripts as if it were your system's version of `go`.
For an example, see [`github.com/jmelahman/connections`](https://github.com/jmelahman/connections).
