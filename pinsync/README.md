# pinsync

[![Go Reference](https://pkg.go.dev/badge/github.com/jmelahman/pinsync.svg)](https://pkg.go.dev/github.com/jmelahman/pinsync)

Keep every copy of a pinned version in step with the one place that defines
it.

A tool's version tends to be written in several places: a pre-commit hook's
`rev`, `uv==` in another hook's `additional_dependencies`, the `version`
input of a setup action in CI. Dependency bots bump one of them and the rest
drift. pinsync gives each pin one source — a lockfile or a hook rev your
bumper already updates — and rewrites every copy to match.

## Install

From [PyPI](https://pypi.org/project/pinsync/), as a prebuilt binary:

```sh
uv tool install pinsync
```

Or from source:

```sh
go install github.com/jmelahman/pinsync@latest
```

Binaries for each platform are also attached to every
[release](https://github.com/jmelahman/pinsync/releases).

## Configuration

`.config/pinsync/config.toml`, at the root of the git repository:

```toml
# The version is the uv-pre-commit rev in the root .pre-commit-config.yaml,
# which `prek autoupdate` bumps.
[pins.uv]
source = "pre-commit"
repo = "https://github.com/astral-sh/uv-pre-commit"
action = "astral-sh/setup-uv"        # steps' `version` input follows it

# The version uv.lock locks git-orchard at.
[pins.git-orchard]
source = "uv.lock"
repo = "https://github.com/jmelahman/git-orchard"   # this hook's rev follows it

[pins.prek]
source = "uv.lock"
action = "j178/prek-action"
input = "prek-version"               # the input, when it isn't `version`

# A version nothing else defines.
[pins.shfmt]
version = "3.14.1"
package = "mvdan.cc/sh/v3/cmd/shfmt"
```

A pin's source is one of:

- `source = "uv.lock"` — the version `uv.lock`, at the root, locks `package`
  at.
- `source = "pre-commit"` — the tag in the `# frozen:` comment (or else the
  `rev`) of `repo` in the root `.pre-commit-config.yaml`.
- `version = "..."` — the version itself.

Its copies, in every file git tracks:

- **Hook revs.** The `rev` of `repo` in any `.pre-commit-config.yaml`. A rev
  pinned to a commit needs a `# frozen: <tag>` comment, as `prek autoupdate
  --freeze` writes; pinsync rewrites both.
- **Hook dependencies.** `package==version` (Python, by normalized name) and
  `package@version` (Go, npm) in any hook's `additional_dependencies`.
  `package` defaults to the pin's name.
- **Action inputs.** `input` (default `version`) of any step that uses
  `action`, in `.github/workflows/` and composite `action.yml` files. An
  input set by an expression is left alone.

A copy keeps its own spelling: a `v` prefix, quotes, and the comments and
formatting around it.

## Usage

```sh
pinsync check   # list the copies that differ from their pin; exit 1 if any
pinsync sync    # rewrite them; exit 1 if anything changed
```

`check` needs no network: it reads a rev's version from its `# frozen:`
comment. `sync` looks up the commit of a rev's new tag with `git ls-remote`.

### pre-commit

`sync` exits 1 when it changes a file, like a formatter:

```yaml
repos:
  - repo: https://github.com/jmelahman/pinsync
    rev: v0.1.0
    hooks:
      - id: pinsync
```

Run it before anything that copies the files it rewrites, such as
[git-orchard](https://github.com/jmelahman/git-orchard)'s `git-orchard-sync`.
