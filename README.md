# Check Symlinks

Checks for broken symbolic links.

## Install

```shell
cargo install check-symlinks
```

## Usage

It currently only supports checking all files recursively from your current working directory,

```shell
$ check-symlinks
"./testdata/broken_link" is not a valid symlink
```
