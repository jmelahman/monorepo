# `git orchard sync`: share configs across subtrees

## Context

Every subtree is mirrored to a standalone repo, and each mirror has to work standalone. Today the configs have drifted:

- **`.pre-commit-config.yaml`** (25 copies) comes in two kinds. About 12 Go subtrees hold only the module-scoped `go-*` hooks, so their mirrors have no gofmt, whitespace or secrets checks. Others (agentic-kanban, agilecbt, 5-wild, local-preview, tag) copy the root's builtin hooks, so in workspace mode those hooks run twice.
- **`test.yml`** has about 6 variants across 17 copies. `release.yml` has 16 copies and `dependabot.yml` has 7.
- **`hatch_build.py`** has 7 variants across 13 copies.

Keeping shared files the same across subtrees is part of maintaining a subtree monorepo, so this becomes a git-orchard feature. It uses no templating: whole files are copied, or marked blocks are synced.

## Model: profile directories

```
.config/git-orchard/shared/        # orchard.sharedDir, default shown
  base/.pre-commit-config.yaml     # builtins, ripsecrets, actionlint, zizmor, gofmt
  base/.github/dependabot.yml
  go/.pre-commit-config.yaml       # go-mod-tidy, go-vet, go-test, go-get
  go-cli/.github/workflows/test.yml
  fullstack/.github/workflows/test.yml
```
```gitconfig
[subtree "git-orchard"]
	remote = git@github.com:jmelahman/git-orchard.git
	branch = master
	shared = base
	shared = go
	shared = go-cli
```

Rules:
- Every file under `shared/<profile>/` lands at the same relative path in each subtree that lists `<profile>`.
- **Block mode:** if the destination contains the lines `BEGIN orchard:<profile>` and `END orchard:<profile>`, only the lines between them are replaced with the source. The marker lines stay as they are, so the comment syntax is whatever the file already uses (`#`, `//`, …).
- **Whole-file mode:** otherwise, the destination is replaced entirely, or created if it doesn't exist. New files take the source file's mode.
- **Errors:**
  - Two profiles write the same path and at least one of them is in whole-file mode.
  - A duplicate or unclosed marker.
  - A subtree lists a profile directory that doesn't exist.
- **Project-owned content:** anything outside the markers belongs to the subtree, e.g. local hooks like biome or check-version, or the name and description in `pyproject.toml`.

## Changes to git-orchard

### `config/config.go` and `config_test.go`
- Add `Subtree.Shared []string`. In `builder.apply`, append `subtree.*.shared` values; `git config --list` already returns repeated keys in order.
- Add `Config.SharedDir`, read from `orchard.sharedDir` with default `.config/git-orchard/shared`. Parse it next to `squash`.
- The regexp in `Load` already covers `orchard|subtree`, so it needs no change.
- Update the package doc comment, the `root.go` Long text and the README.

### `sync/` package (new)
- `Plan(repoDir string, cfg config.Config, subtrees []config.Subtree) ([]Change, error)` walks each listed profile directory and renders `Change{Path, Old, New []byte, Mode}` in memory. `Apply(changes)` writes them.
- Block replacement is a small line-based function with its own table tests.

### `cmd/sync.go` (new), registered in `cmd/root.go`
`git orchard sync [prefix...] [--check]`:
- Selects prefixes with the existing `Config.Select`, reached through `openOrchard()`.
- By default it writes the changed files, prints their paths, and **exits 1 if anything changed**, the way fixers like `end-of-file-fixer` do.
- `--check` prints a unified diff for each out-of-date file and writes nothing. Use it in CI.

### `.pre-commit-hooks.yaml` (new, at the git-orchard root)
```yaml
- id: git-orchard-sync
  name: git orchard sync
  description: Sync shared files from .config/git-orchard/shared into subtrees.
  entry: git-orchard sync
  language: golang
  pass_filenames: false
  always_run: true
```
- The hook must run only in the monorepo root, not inside subtree projects, because a mirror has no manifest. Say this in the README.
- If the manifest has no `shared` keys, the command does nothing and exits 0.
- Check that `language: golang` builds the `git-orchard` binary name. The module's main package is at the repo root, so `go install` produces `git-orchard`.

## Monorepo rollout, after the git-orchard release

1. **Enable the hook.** Add `git-orchard-sync` to the root `.pre-commit-config.yaml`, pinned by frozen `rev` like go-pre-commit-hooks.
2. **Pre-commit profiles.**
   - Create `base/` and `go/`, add markers to each subtree's `.pre-commit-config.yaml`, and add `shared =` lines to the manifest.
   - Add `orphan: true` to subtree configs. prek 0.5.3 supports it, so the root project skips files that belong to a subtree and hooks don't run twice. Update the comment in the root `exclude` block.
3. **Workflows.** Reduce the ~6 `test.yml` variants to a few profiles (`go-cli`, `fullstack`, …) in whole-file mode. Then do `release.yml` and `dependabot.yml`.
4. **Other configs.** `.goreleaser.yaml`, `biome.json`, `.golangci.yml`, and a `[build-system]` block in `pyproject.toml`.
5. **`hatch_build.py`.** This is code, so move it into a hatch build-hook plugin in `manygo`, configured with `[tool.hatch.build.hooks.manygo]`, and delete the copies. The larger variants (agentic-kanban's frontend build, check-symlinks/undot's `.so` build) keep a thin local hook.
6. **Templates.** Add `templates/golang-template` and `fullstack-template` to the manifest with the matching profiles.

Fix by hand: `agilecbt/pyproject.toml` has `name = "fullstack-template"`.

**Updating revisions:** run `prek auto-update --freeze` in one subtree, copy the new revisions into `shared/<profile>/.pre-commit-config.yaml`, and commit. The hook then spreads the change to every other subtree.

## Critical files
- `git-orchard/config/config.go` and `config_test.go`
- `git-orchard/sync/` (new)
- `git-orchard/cmd/sync.go` (new) and `cmd/root.go`
- `git-orchard/.pre-commit-hooks.yaml` (new)
- `git-orchard/README.md`
- Monorepo: `.config/git-orchard/subtrees`, `.config/git-orchard/shared/` (new), root and subtree `.pre-commit-config.yaml`

## Verification
- `go test ./...` in `git-orchard`:
  - Config: repeated `shared` values, and the `sharedDir` default and override.
  - Sync, using a temp repo with profiles:
    - Whole-file create and update.
    - Block replacement with `#` and `//` markers.
    - Two blocks in one file.
    - Errors: a conflict between two whole-file profiles, a missing profile directory, duplicate or unclosed markers.
  - Exit codes: 1 after writing changes, 0 when nothing changed, and `--check` writes nothing.
- Hook: `prek try-repo ./git-orchard git-orchard-sync --all-files` from the monorepo root.
- Monorepo: after each rollout step, `prek run --all-files` should pass, with no hook reported twice for subtree files.
- Standalone mirror: run `git subtree split --prefix=git-orchard -b tmp-split`, open that branch in a scratch worktree, and run `prek run --all-files`. The full set of checks should run without the root config.
- Drift check: edit a synced block by hand, commit, and confirm the hook rewrites it and fails. The next commit attempt should pass.
