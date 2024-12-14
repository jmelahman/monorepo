# Tag Release

[![Test status](https://github.com/jmelahman/tag/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/tag/actions)
[![Deploy Status](https://github.com/jmelahman/tag/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/tag/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/jmelahman/tag.svg)](https://pkg.go.dev/github.com/jmelahman/tag)
[![PyPI](https://img.shields.io/pypi/v/release-tag.svg)]()
[![Go Report Card](https://goreportcard.com/badge/github.com/jmelahman/tag)](https://goreportcard.com/report/github.com/jmelahman/tag)

Automatically create [semantic version](https://semver.org/) git tags.

```shell
$ tag --push
Next version: v1.0.1
Tag v1.0.1 created and pushed to remote.
```

### Autocomplete

`tag` provides autocomplete for `bash`, `fish`, `powershell` and `zsh` shells.
For example, to enable autocomplete for the `bash` shell,

```shell
tag completion bash | sudo tee /etc/bash_completion.d/tag > /dev/null
```

_Note: bash completion requires the [bash-completion](https://github.com/scop/bash-completion/) package be installed._

For more information, see `tag completion <shell> --help` for your respective `<shell>`.

## Install

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
