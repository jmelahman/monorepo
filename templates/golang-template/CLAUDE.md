# golang-template — Claude Notes

A Go template repo: a small importable package in `greeter/` (the "SDK"), a
CLI at the module root that consumes it, and the tooling that checks both.

## Commands

- Tests: `go test ./...`.
- Run the CLI: `go run . [-greeting X] [-shout] [name]`.
- Lint: `prek run --all-files`. It is the same set CI runs; prefer it over
  invoking golangci-lint or zizmor directly, since the hooks pin
  versions and provision the tools.

## Layout

- `greeter/` — the public package (doc comment, API and functional options
  in `greeter.go`). Keep exported symbols documented. New packages go in their own
  top-level directory; `main.go` and `main_test.go` are the only `.go` files
  at the root.
- `main.go` — the CLI, at the module root so `go install <module>@latest`
  works and the binary is named after the repo. `main` does process plumbing
  only; everything testable lives in `run(args, stdout, stderr)`. A second
  binary would go in `cmd/<name>/`.
- `internal/version/` — ldflags-stamped version with a `runtime/debug`
  fallback.
- `.golangci.yml` — golangci-lint v2, run by the `golangci-lint-full` hook.
  The comment at its foot lists the linters left out and why.

## Conventions

- Tests go in `package <pkg>_test` and exercise the public API; table-driven
  with `t.Parallel()` on both the parent and the subtests.
- New exported behavior gets an `Example` function in `example_test.go` with
  an `// Output:` comment — it compiles as a test and renders on pkg.go.dev.
- Struct fields: wide first, `bool` last.
- Sentinel errors are `ErrFoo` and matched with `errors.Is`; `errorlint`
  rejects `==` comparisons and `%v`-wrapped errors.
- Tool versions are pinned in `.pre-commit-config.yaml`; CI runs the hooks
  rather than re-declaring the tools. govulncheck is the exception, pinned
  again in `release.yml`. Dependabot sees none of them. actionlint and zizmor are `rev`-pinned
  upstream hook repos, so `prek update` bumps those two.
