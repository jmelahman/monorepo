# Check Symlinks

Check for broken symbolic links.

`check-symlinks` is optmized for large codebases as well as small, incremental checks,

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

```shell
cargo install check-symlinks

```

## Usage

By default, checks all [unignored](https://github.com/BurntSushi/ripgrep/tree/master/crates/ignore#ignore) files recursively from the current working directory,

```shell
$ check-symlinks
"./broken_link" is not a valid symlink
```

File paths can also be passed,

```shell
$ check-symlinks broken_link doesnt_exist
"./broken_link" is not a valid symlink
```

_NOTE: file arguments which don't exist are ignored._
