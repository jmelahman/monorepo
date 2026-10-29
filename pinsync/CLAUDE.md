# pinsync — Claude Notes

A CLI that keeps every copy of a pinned version (pre-commit hook revs and
additional_dependencies, GitHub Action inputs) in step with the one place
that defines it. See README.md for the config and what counts as a copy.

## Commands

- Tests: `go test -race ./...`.
- Run the CLI: `go run . check` or `go run . sync`, from inside a git
  repository with a `.config/pinsync/config.toml`.
- Lint: `prek run --all-files`. It is the same set CI runs; prefer it over
  invoking golangci-lint, aligo or zizmor directly, since the hooks pin
  versions and provision the tools.

## Layout

- `main.go` — the cobra CLI. `main` does process plumbing only; everything
  testable lives in `run(args, stdout, stderr)`. It finds the repository
  root and its tracked files with git.
- `internal/config/` — loads and validates the config; unknown keys are
  errors, so a typo can't leave a copy unmanaged.
- `internal/pins/` — finds copies and rewrites stale ones. Files are parsed
  with yaml.v3 only to locate each version's node; edits are byte splices at
  those offsets, so comments and formatting survive. Never re-serialize a
  file.
- `internal/remote/` — resolves a tag to its commit with `git ls-remote`.
- `internal/version/` — ldflags-stamped version with a `runtime/debug`
  fallback.
- `pyproject.toml`, `hatch_build.py` — package the binary as a wheel on
  PyPI. The `orchard:` blocks here and in `.github/workflows/release.yml`,
  and `build-constraints.txt`, are copied from the monorepo's shared
  profiles; edit them there.

## Conventions

- `check` must stay offline: it compares a rev's `# frozen:` tag, and only
  `sync` resolves commits.
- Tests go in `package <pkg>_test` and exercise the public API; table-driven
  with `t.Parallel()` on both the parent and the subtests. `main_test.go`
  runs the commands end to end in a temporary git repository.
- Struct fields: wide first, `bool` last. `aligo check ./...` fails the build
  on a struct whose field order wastes memory.
- Sentinel errors are `ErrFoo` and matched with `errors.Is`; `errorlint`
  rejects `==` comparisons and `%v`-wrapped errors.
