# Tag Release

[![Test status](https://github.com/jmelahman/tag/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/tag/actions)
[![Deploy Status](https://github.com/jmelahman/tag/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/tag/actions)
[![Arch User Repsoitory](https://img.shields.io/aur/version/release-tag)](https://aur.archlinux.org/packages/release-tag)
[![PyPI](https://img.shields.io/pypi/v/release-tag.svg)](https://pypi.org/project/release-tag/)

Automatically create [semantic version](https://semver.org/) git tags.

```text
$ tag
Push tag 'v1.0.1' to origin? (y/N): y
Tag 'v1.0.1' created and pushed to origin.
```

## Usage

By default, `tag` will increment the smallest digit following [SemVer precedence](https://semver.org/#semantic-versioning-specification-semver).
Incrementing a specific version is achieved by passing the respective flag: `--major`, `--minor`, `--patch`.

Tags can be automatically pushed to a remote repository by passing `--push`.

`tag` supports [pre-release](https://semver.org/#spec-item-9) versions.
Creating a pre-release tag is achieved by the using the `--suffix` flag.
For example, `--suffix="alpha"` will create a tag like `v1.0.0-alpha`.
If the previous tag was for a pre-release, that suffix is preferred.
This behavior can be overridden by passing `--patch` or `--suffix=""`.
Only incrementing the trailing pre-release identifier is currently supported.

`tag` authoritatively discourages duplicate tags for a single commit.

For the most up-to-date options, run `tag --help`,

```
$ tag --help
Calculate the next semantic version tag

Usage: tag [OPTIONS] [COMMAND]

Commands:
  completion  Generate completion script
  help        Print this message or the help of the given subcommand(s)

Options:
      --major                increment the major version
      --minor                increment the minor version
      --patch                increment the patch version
      --push                 create and push the tag to remote
      --print-only           print the next tag and exit
      --check                validate that the tag at HEAD has its previous version as an ancestor
      --no-fetch             skip fetching tags from remote
      --allow-untagged       allow HEAD to be untagged when using --check
      --debug                enable debug logging
      --prefix <PREFIX>      set a prefix for the tag [default: ""]
      --suffix <SUFFIX>      set the pre-release suffix (e.g., rc, alpha, beta) [default: ""]
      --metadata <METADATA>  set the build metadata [default: ""]
      --remote <REMOTE>      remote repository to push tag to [default: origin]
  -v, --version              version for tag
  -h, --help                 Print help
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

**cargo:**

```shell
cargo install --locked --git https://github.com/jmelahman/tag
```

**github:**

Prebuilt packages are available from [Github Releases](https://github.com/jmelahman/tag/releases).

### Pre-commit

In your `.pre-commit-config.yaml`,

```yaml
default_install_hook_types:
  - pre-push
repos:
  - repo: https://github.com/jmelahman/tag
    rev: a069af54b5de5379d3581e3fc027395d1ad0a982  # frozen: v0.5.0
    hooks:
      - id: tag-check
```

By default, it only runs on the [`pre-push`](git-scm.com/book/en/v2/Customizing-Git-Git-Hooks) stage,
so you may need to install the `pre-push` hook after staging your changes.
