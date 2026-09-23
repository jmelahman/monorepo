# go-pre-commit-hooks

Hooks for running standard golang tools without system dependencies.

These hooks use `language: golang`, so the hook framework provisions the Go
toolchain itself: a suitable system `go` is used when present, and otherwise a
toolchain is downloaded automatically. No `language: system` escape hatch, no
"install go first" prerequisite.

Requires [prek](https://prek.j178.dev) (any recent version) or
[pre-commit](https://pre-commit.com) >= 3.0.0.

## Usage

```yaml
repos:
  - repo: https://github.com/jmelahman/go-pre-commit-hooks
    rev: v1.0.0
    hooks:
      - id: gofmt
      - id: go-fix
      - id: go-mod-tidy
      - id: go-vet
      - id: go-test
```

## Hooks

### `gofmt`

Rewrites staged Go files with `gofmt -l -w`. `gofmt` itself always exits 0;
the hook fails because the framework detects modified files. Pass extra flags
with `args`, e.g. `args: [-s]` to also simplify code.

### `go-fix`

Applies `go fix ./...` suggested fixes in place and fails when a file changes.
On Go >= 1.26 this is the analyzer-based fixer with modernizers (`interface{}`
-> `any`, redundant loop-variable re-declarations, etc.); on older toolchains
the legacy `go fix` is a harmless near-no-op. Use `args: [-diff, ./...]` to
report the patch without rewriting.

### `go-mod-tidy`

Runs `go mod tidy` and fails when `go.mod` or `go.sum` change. May touch the
network to fill in missing requirements or checksums. Use `args: [-diff]`
(Go >= 1.23) to report changes without rewriting.

### `go-vet`

Runs `go vet ./...` once from the repository root whenever Go sources,
`go.mod`, or `go.sum` change. Override `args` to vet specific packages or add
build flags, e.g. `args: [-tags=integration, ./...]`.

### `go-test`

Runs `go test ./...` under the same trigger conditions as `go-vet`. Tune the
invocation with `args` — but always re-include `./...`, since with no package
argument `go test` only tests the repository root. Tests are often too slow
for every commit; to mirror a CI invocation with the race detector on push
only:

```yaml
- id: go-test
  args: [-race, ./...]
  stages: [pre-push]
```

(Remember `default_install_hook_types: [pre-commit, pre-push]` so the pre-push
hook actually gets installed.)

## Scope

Only tools bundled with the Go toolchain belong here. Third-party tools
(`goimports`, `gofumpt`, `staticcheck`, `golangci-lint`, ...) have their own
hook repositories, or can be bolted onto any repo via `additional_dependencies`.

There is deliberately no `go-build` hook: `go build ./...` writes an
executable into the working tree when the repository is a single `main`
package, and compilation is already covered — `go vet` type-checks every
package including test files, and `go-test` builds for real.

## Toolchain selection

Both prek and pre-commit (>= 4.1.0) run these hooks with `GOTOOLCHAIN=local`,
disabling Go's automatic toolchain switching. If your `go.mod` requires a
newer Go than the one the hook environment selected, pin one explicitly:

```yaml
- id: go-vet
  language_version: ">=1.26" # prek accepts ranges; pre-commit wants an exact version, e.g. '1.26.3'
```
