# Check Symlinks

Checks for broken symbolic links.

## Install

```shell
cargo install check-symlinks
```

## Usage

By default, checks all files recursively from the current working directory,

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
