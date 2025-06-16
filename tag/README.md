# Tag Release

[![Test status](https://github.com/jmelahman/tag/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/tag/actions)
[![Deploy Status](https://github.com/jmelahman/tag/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/tag/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/jmelahman/tag.svg)](https://pkg.go.dev/github.com/jmelahman/tag)
[![Arch User Repsoitory](https://img.shields.io/aur/version/release-tag)](https://aur.archlinux.org/packages/release-tag)
[![PyPI](https://img.shields.io/pypi/v/release-tag.svg)]()
[![Go Report Card](https://goreportcard.com/badge/github.com/jmelahman/tag)](https://goreportcard.com/report/github.com/jmelahman/tag)

Automatically create [semantic version](https://semver.org/) git tags.

```text
$ tag
Push tag 'v1.0.1' to origin? (y/N): y
Tag 'v1.0.1' created and pushed to origin.
```

## Usage

By default, `tag` will increment the smallest digit following [SemVer precedence](https://semver.org/#semantic-versioning-specification-semver).
Incrementing a specific is achieved by passing the respective flag: `--major`, `--minor`, `--patch`.

Tags can be automatically pushed to a remote repository by passing `--push`.

`tag` supports [pre-release](https://semver.org/#spec-item-9) versions.
Creating a pre-release tag is achieved by the using the `--suffix` flag.
For example, `--suffix="alpha"` will create a tag like `v1.0.0-alpha`.
If the previous tag was for a pre-release, that suffix is preferred.
This behavior can be overridden by passing `--patch` or `--suffix=""`.
Only incrementing the trailing pre-release identifier is currently supported.

`tag` authoritatively discourages duplicate tags for a single commit.

### Autocomplete

`tag` provides autocomplete for `bash`, `fish`, `powershell` and `zsh` shells.
For example, to enable autocomplete for the `bash` shell,

```shell
tag completion bash | sudo tee /etc/bash_completion.d/tag > /dev/null
```

_Note: bash completion requires the [bash-completion](https://github.com/scop/bash-completion/) package be installed._

For more information, see `tag completion <shell> --help` for your respective `<shell>`.

## Install

**AUR:**

`tag` is available from the [Arch User Repository](https://aur.archlinux.org/packages/release-tag).

```shell
yay -S release-tag
```

**pip:**

`tag` is available as a [pypi package](https://pypi.org/project/release-tag/).

```shell
pip install release-tag
```

**go:**

```shell
go install github.com/jmelahman/tag@latest
```

**github:**

Prebuilt packages are available from [Github Releases](https://github.com/jmelahman/tag/releases).
