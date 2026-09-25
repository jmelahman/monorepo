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
    rev: v1.3.0
    hooks:
      - id: gofmt
      - id: go-fix
      - id: go-mod-tidy
      - id: go-work-sync # only in repositories with a go.work
      - id: go-vet
      - id: go-test
      - id: go-get
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

### `go-work-sync`

For multi-module repositories with a `go.work`. Runs `go work sync`, which
raises each workspace module's requirements to the versions the workspace
build list selected, and fails when a `go.mod` changes. It runs whenever
`go.work`, a `go.mod`, or a `go.sum` changes. Add it to the config next to
`go.work`, since `go work sync` must run inside the workspace.

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

### `go-get`

Upgrades dependencies with `go get -u ./...` and fails when `go.mod` or
`go.sum` change. It is on the `manual` stage, so it never fires on a commit;
run it on demand:

```shell
prek run --stage manual go-get
```

Use `args: [-u=patch, ./...]` to stay within patch releases, or add `-t` to
upgrade test dependencies too. Follow up with `go-mod-tidy` (which runs on the
next commit touching `go.mod` anyway).

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
disabling Go's automatic toolchain switching. prek infers a minimum from this
repository's `go.mod` (`go 1.27`), so it downloads a current Go when the
system's is older; pre-commit doesn't, and uses the system `go` as is. If your
`go.mod` requires a newer Go than the one the hook environment selected, pin
one explicitly:

```yaml
- id: go-vet
  language_version: ">=1.26" # prek accepts ranges; pre-commit wants an exact version, e.g. '1.26.3'
```
