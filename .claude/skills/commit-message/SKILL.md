---
name: commit-message
description: Conventional Commits format for this repo. Load before running `git commit` or writing any commit message.
---

# Use Conventional Commits

- Write subjects as `<type>(<scope>): <description>`, e.g. `fix(opencycle): stop the rider clipping through terraces`.
- Types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`.
- The scope is the project the change touches, usually its directory name. Omit it for repo-wide changes.
- Mark breaking changes with `!` before the colon, e.g. `feat(kanban)!: rename the board config keys`.
