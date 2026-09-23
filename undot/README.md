# undot

Declutter a project's root directory by taming configuration file sprawl.

```
before              after
──────              ─────
.claude/            .config/
.cursor/            .github/
.editorconfig       src/
.github/
.npmrc
.yarn/
src/
```

Every tool wants a dotfile at the repository root. undot moves them into a single
tracked `.config/` directory. Gitignored symlinks point back to the root, so every tool
keeps working. A [pre-commit](https://pre-commit.com) hook (or
[prek](https://github.com/j178/prek)) recreates the symlinks on every clone and checkout.
Teammates install nothing and notice nothing.

## Install

```sh
uv tool install undot   # or: pip install undot
```

Small static binaries (~200KB, instant startup) for Linux and macOS, also on
[GitHub releases](https://github.com/jmelahman/undot/releases).
The pre-commit hook (below) needs no install at all.

## Migrate an existing repository

```sh
undot migrate             # or name entries: undot migrate .claude .cursor
git add -A && git commit
pre-commit install
```

One idempotent command:
- moves the configs
- creates the symlinks
- updates `.gitignore` (existing rules that reached inside a moved directory get matching `.config/` patterns)
- registers the git hook below

## Clones and new repos set themselves up

`migrate` writes this hook config. After `clone`, `checkout`, `merge`, or `rebase`, it
restores any missing symlink:

```yaml
default_install_hook_types: [pre-commit, post-checkout, post-merge, post-rewrite]
repos:
  - repo: https://github.com/jmelahman/undot
    rev: v0.3.0
    hooks:
      - id: undot
```

## CI configs migrate too

CI checkouts don't run hooks. Most tools accept an explicit config path, so a file like
`.goreleaser.yaml` can live in `.config/` with no symlink at all:

```sh
undot migrate .goreleaser.yaml
echo 'nolink .goreleaser.yaml' >> .config/undot.conf
```

```yaml
- run: goreleaser release --clean --config .config/.goreleaser.yaml
```

## Guardrails

- `.git*` files are never migrated. `.github` and `.pre-commit-config.yaml` require
  `--force`, because a fresh clone must be able to bootstrap the hook.
- Caches, secrets, and CI-read configs (`.venv`, `.env`, `.golangci.yml`, ...) are
  skipped unless you name them.
- Conflicts are reported, never overwritten. Migrating a tracked *file* prints the
  `git rm --cached` command you'll want afterwards.

Tune the scan in `.config/undot.conf` or `~/.config/undot/undot.conf`, one
name or glob per line:

```
# never migrate these here
.idea*
# do migrate .env, despite the default skip
!.env
# keep in .config/ without a root symlink
nolink .goreleaser.yaml
```
