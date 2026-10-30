# vinder — Claude Notes

A CLI that generates a no-build project's `vendor/` directory from its npm
dependencies (installed by the project's package manager, bundled by
embedded esbuild), checks the committed one is current, and audits it
against OSV. README.md documents the config and commands.

## Commands

- Tests: `go test -race ./...`. The `internal/project` integration tests
  run real package managers against tarballs they pack themselves, so they
  need no network. `TestSync` runs under every one on PATH (bun, npm, nub,
  pnpm, yarn) and the rest under the first; with none they skip.
- Run the CLI: `go run . sync|check|audit|upgrade [--all] [dir...]`, `go run . github-app`.
- Lint: `prek run --all-files`. It is the same set CI runs; prefer it over
  invoking golangci-lint, aligo or zizmor directly, since the hooks pin
  versions and provision the tools.
- Push-stage hooks (`go test -race`, govulncheck):
  `prek run --all-files --hook-stage pre-push`.

## Layout

- `main.go` — the cobra CLI. `main` does process plumbing only; everything
  testable lives in `run(args, stdout, stderr)`.
- `internal/project/` — `vinder.toml` loading and validation (`config.go`),
  package-manager detection and install (`pm.go`), esbuild bundling +
  manifest (`generate.go`), sync, check and discovery
  (`sync.go`), cross-project version skew (`skew.go`), `sync --align`
  (`align.go`) and `upgrade` (`upgrade.go`). The align/upgrade integration
  test runs every package manager against a local registry.
- `internal/pkgjson/` — order- and indent-preserving package.json edits.
- `internal/registry/` — the npm registry's latest dist-tag;
  `registrytest/` serves packages over the registry protocol for tests.
- `internal/osv/` — the OSV batch query client.
- `githubapp.go`, `internal/githubapp/` — `vinder github-app`: creates the
  GitHub App the scheduled upgrade workflow opens PRs as, from a manifest.
- `action.yml` — the GitHub Action for scheduled upgrades: builds vinder
  from the action's source, runs upgrade and audit, opens the PR.
- `.github/npm/package.json` — fields for the npm package. The packing
  script beside it and the release jobs are git-orchard's shared `npm`
  profile; edit those in the monorepo.
- `internal/version/` — ldflags-stamped version with a `runtime/debug`
  fallback.
- `.golangci.yml` — golangci-lint v2. After editing it, run
  `prek run --all-files golangci-lint-config-verify`.

## Conventions

- Tests go in `package <pkg>_test` and exercise the public API; table-driven
  with `t.Parallel()` on both the parent and the subtests. Tests that run
  a real package manager go in `*_integration_test.go`, where lint rejects
  fakes.
- Struct fields: wide first, `bool` last. `aligo check ./...` fails the build
  on a struct whose field order wastes memory.
- Sentinel errors are `ErrFoo` and matched with `errors.Is`; `errorlint`
  rejects `==` comparisons and `%v`-wrapped errors.
- vinder owns `<out>/<key>/` and `<out>/vinder.json` and nothing else in the
  output directory; keep sync and check agreeing on that.
- Tool versions are pinned in `.pre-commit-config.yaml`; CI runs the hooks
  rather than re-declaring the tools. actionlint and zizmor are `rev`-pinned
  upstream hook repos, so `prek update` bumps those two.
