# monorepo-template — Claude Notes

A monorepo whose projects are git subtrees, each mirrored to its own
repository by git-orchard. See README.md for the full picture.

## Commands

- Lint and test: `prek run --from-ref origin/master --to-ref HEAD` (what CI
  runs on a pull request), or `prek run --all-files`. Prefer these over
  invoking tools directly, since the hooks pin versions and provision the
  tools.
- Shared config: `git orchard sync` after editing anything under
  `.config/git-orchard/shared/`. Commit the synced files with the change.
- Release a project: `git orchard release <prefix>` (`--dry-run` first).

## Conventions

- Never edit a synced block (`BEGIN orchard:<profile>` … `END orchard:<profile>`)
  or a file copied whole from a profile inside a subtree. Edit the profile in
  `.config/git-orchard/shared/` and sync; the `git-orchard-sync` hook fails
  otherwise.
- Hooks that need a project root (tests, vet, type checks) belong in that
  project's `.pre-commit-config.yaml`, not the root one.
- Never force-push a mirror; its history is the split of this repository's.
