# Personal Monorepo

[![Code style: black](https://img.shields.io/badge/code%20style-black-000000.svg)](https://github.com/psf/black)

## Tooling

### Dependencies

Python dependencies are specified in `third_party/requirements.in` and compiled to
`third_party/requirements.txt`.

To recompile `third_party/requirements.txt`, run,

```shell
bazel run //third_pary:requirements.update
```

#### Upgrading Dependencies

// TODO

### Formatting

#### Python

Formatting python is done by [black](https://github.com/psf/black).

To run the formatter,

```shell
bazel run //tools/format
```

#### BUILD

Formatting bazel `BUILD` and `.bzl` is done by [Buildifier](https://github.com/bazelbuild/buildtools/tree/master/buildifier).

To run the formatter,

```shell
bazel run :buildifier
```
