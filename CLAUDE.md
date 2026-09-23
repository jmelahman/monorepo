# golang-template — Claude Notes

A Go template repo: a small importable package in `greeter/` (the "SDK"), a
CLI at the module root that consumes it, and the tooling that checks both.

## Commands

- Tests: `go test ./...` (`scripts/modules.sh go test -race ./...` to cover
  every module in a multi-module checkout).
- Run the CLI: `go run . [-greeting X] [-shout] [name]`.
- Lint: `prek run --all-files`. It is the same set CI runs; prefer it over
  invoking golangci-lint, aligo or zizmor directly, since the hooks pin
  versions and provision the tools.
- Push-stage hooks (`go test -race`, govulncheck):
  `prek run --all-files --hook-stage pre-push`.

## Layout

- `greeter/` — the public package (doc comment, API and functional options
  in `greeter.go`). Keep exported symbols documented: `revive`'s `exported`
  rule is on. New packages go in their own top-level directory; `main.go`
  and `main_test.go` are the only `.go` files at the root.
- `main.go` — the CLI, at the module root so `go install <module>@latest`
  works and the binary is named after the repo. `main` does process plumbing
  only; everything testable lives in `run(args, stdout, stderr)`. A second
  binary would go in `cmd/<name>/`.
- `internal/version/` — ldflags-stamped version with a `runtime/debug`
  fallback.
- `scripts/modules.sh` — lists the repo's Go modules, or runs a command in
  each. Used by the lint hooks and by CI; `./...` does not span a workspace,
  so per-module looping is what makes those checks complete.
- `.golangci.yml` — golangci-lint v2. After editing it, run
  `prek run --all-files golangci-lint-config-verify`.

## Conventions

- Tests go in `package <pkg>_test` and exercise the public API; table-driven
  with `t.Parallel()` on both the parent and the subtests.
- New exported behavior gets an `Example` function in `example_test.go` with
  an `// Output:` comment — it compiles as a test and renders on pkg.go.dev.
- Struct fields: wide first, `bool` last. `aligo check ./...` fails the build
  on a struct whose field order wastes memory.
- Sentinel errors are `ErrFoo` and matched with `errors.Is`; `errorlint`
  rejects `==` comparisons and `%v`-wrapped errors.
- Tool versions are pinned in `.pre-commit-config.yaml`; CI runs the hooks
  rather than re-declaring the tools. govulncheck is the exception, pinned
  again in `test.yml` and `release.yml`. Dependabot sees none of them. actionlint and zizmor are `rev`-pinned
  upstream hook repos, so `prek auto-update` bumps those two.
