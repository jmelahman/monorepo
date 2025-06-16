# Check Symlinks

[![Test status](https://github.com/jmelahman/check-symlinks/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/check-symlinks/actions)
[![Deploy Status](https://github.com/jmelahman/check-symlinks/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/check-symlinks/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/jmelahman/check-symlinks.svg)](https://pkg.go.dev/github.com/jmelahman/check-symlinks)
[![Arch User Repsoitory](https://img.shields.io/aur/version/check-symlinks)](https://aur.archlinux.org/packages/check-symlinks)
[![PyPI](https://img.shields.io/pypi/v/check-symlinks.svg)](https://pypi.org/project/check-symlinks/)
[![Go Report Card](https://goreportcard.com/badge/github.com/jmelahman/check-symlinks)](https://goreportcard.com/report/github.com/jmelahman/check-symlinks)

Check for broken symbolic links.

```shell
$ check-symlinks
Broken symlink: some/path/broken_link
```

`check-symlinks` is optimized for large codebases as well as small, incremental checks,

<p align="center">
  <picture align="center">
    <source media="(prefers-color-scheme: dark)" srcset="https://github.com/jmelahman/check-symlinks/assets/23436978/b6d5a6f1-d3ec-4786-a234-92840ec26fc4">
    <source media="(prefers-color-scheme: light)" srcset="https://github.com/jmelahman/check-symlinks/assets/23436978/5619ff9a-474b-4daa-a179-d8f6d8f046a5">
    <img alt="Shows a bar chart with benchmark results." src="https://github.com/jmelahman/check-symlinks/assets/23436978/5619ff9a-474b-4daa-a179-d8f6d8f046a5">
  </picture>
</p>

where the full commands are respectively,

```shell
fd --type symlink --exec sh -c 'test -e "$0"'

check-symlinks

git ls-files | xargs pre_commit_hooks/check_symlinks.py

while read file; do test -e "$test"; done < <(git ls-files)

find . -type l -not -path data ! -exec test -e {} \; -print0 | xargs --no-run-if-empty git ls-files
```

and `check_symlinks.py` is from [https://github.com/pre-commit/pre-commit-hooks](https://github.com/pre-commit/pre-commit-hooks/blob/main/pre_commit_hooks/check_symlinks.py).

## Install

**AUR:**

`check-symlinks` is available from the [Arch User Repository](https://aur.archlinux.org/packages/check-symlinks).

```shell
yay -S check-symlinks
```

**pip:**

`check-symlinks` is available as a [pypi package](https://pypi.org/project/check-symlinks/).

```shell
pip install check-symlinks
```

**go:**

```shell
go install github.com/jmelahman/check-symlinks@latest
```
