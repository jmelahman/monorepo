# golang-template

[![Test status](https://github.com/jmelahman/golang-template/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/golang-template/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/jmelahman/golang-template.svg)](https://pkg.go.dev/github.com/jmelahman/golang-template)

A template repository for a Go application, a Go SDK, or a repo that grows
into several of both. The tooling is the point; the sample package is a
deliberately tiny greeter so every layer — public API, functional options,
sentinel error, table test, runnable example, CLI, release — has one worked
example to copy from.

The root is `main.go` plus configuration; the rest is `greeter/` (the
importable package) and `internal/`. Keeping `main` at the root is what
makes `go install github.com/you/repo@latest` work and names the binary
after the repo — add `cmd/<name>/` only when there is a second binary.

## What's included

**Checks, everywhere the same**

| Tool                                                             | Local (prek) | CI                               |
| ---------------------------------------------------------------- | ------------ | -------------------------------- |
| `gofmt -s`, `go fix`, `go mod tidy`, `go vet`                    | pre-commit   | `pre-commit` job                 |
| `go test -race`                                                  | pre-push     | `test` job                       |
| [golangci-lint](https://golangci-lint.run) (v2, `.golangci.yml`) | pre-commit   | `pre-commit` job                 |
| [aligo](https://github.com/essentialkaos/aligo) struct alignment | pre-commit   | `pre-commit` job                 |
| [govulncheck](https://go.dev/blog/govulncheck)                   | pre-push     | `test` job, and again on release |
| [actionlint](https://github.com/rhysd/actionlint)                | pre-commit   | `pre-commit` job                 |
| [zizmor](https://docs.zizmor.sh) workflow audit                  | pre-commit   | `pre-commit` job                 |

CI is two jobs. `test` builds and tests; `pre-commit` runs the hooks above
through [prek](https://prek.j178.dev), at the versions `.pre-commit-config.yaml`
pins — so the linters run once, from one definition, instead of being
re-declared as workflow steps with a second set of version pins.

The Go toolchain hooks come from
[jmelahman/go-pre-commit-hooks](https://github.com/jmelahman/go-pre-commit-hooks),
and the third-party tools are `language: golang` (or, for zizmor,
`language: python`) hooks with pinned `additional_dependencies`. Nothing in `.pre-commit-config.yaml` needs a
system install: the hook framework provisions Go and every tool. Clone,
`prek install`, commit.

**Monorepo-friendly test running**

One script, `scripts/modules.sh`. With no arguments it prints every Go
module in the repo — the `use` directives of `go.work` when there is a
workspace, otherwise every `go.mod` in the tree. With arguments it runs that
command in each of them:

```sh
scripts/modules.sh                # .
scripts/modules.sh go test ./...  # in every module, reporting the first failure
```

That loop is what makes the lint hooks and the CI steps multi-module-correct.
It is not optional dressing: Go's own `./...` does not span a workspace.
From a workspace root that is not itself a module, `go vet ./...` and
`go test ./...` fail outright (`directory prefix . does not contain modules
listed in go.work`), and from a module root they silently skip the nested
modules. Per-module invocation is the only thing that covers everything.

A repo with one `go.mod` gets a list of one and behaves like a plain
single-module repo, so nothing here needs to be unwound if you never add a
second module.

**Release**

`.goreleaser.yaml` builds the root package for linux/darwin/windows × amd64/arm64
and stamps the version into `internal/version` with ldflags; `release.yml`
runs it on a `v*.*.*` tag after a govulncheck audit.

## Using this template

1. Create a repo from this template (GitHub → "Use this template").
2. Find-and-replace `github.com/jmelahman/golang-template` with your module
   path (`go.mod`, the imports in `main.go` and `greeter/`,
   `.goreleaser.yaml`). The binary takes its name from the module path, and
   the CLI's usage text from the binary, so there is nothing else to rename.
3. Pick a shape:
   - **SDK / library**: delete `main.go`, `main_test.go`, `internal/version/`,
     `.goreleaser.yaml` and `.github/workflows/release.yml`. Tagging is the
     release. Keep the package comment and `example_test.go` — they are what
     pkg.go.dev shows.
   - **Application**: keep them, and configure a `release` environment in
     the repo settings.

   The CLI uses the standard library's `flag`, so the template has no
   third-party dependencies and no `go.sum`. That is the right default for
   one command with a few flags. Reach for
   [cobra](https://github.com/spf13/cobra) when you actually need
   subcommands, shell completion or generated man pages — it costs six
   modules (a markdown renderer among them) and about 1.2 MB of binary.
   `run(args, stdout, stderr) error` maps onto cobra's `RunE` plus
   `SetOut`/`SetErr`, so the switch stays confined to `main.go` and
   `main_test.go` keeps working.

4. Rename `greeter/` to your package (the directory name and the `package`
   clause should match, so plain imports read well), and replace this README
   and the LICENSE with your own.
5. `prek install && prek run --all-files`.

### Growing into a monorepo

Add the module, commit `go.work` and `go.work.sum` (the `.gitignore` entries
are there for single-module repos — drop them), and add a `gomod` block to
`.github/dependabot.yml` for the new directory. CI picks the module up on its
own.

Three hooks shell out to the toolchain from the repo root — `go-mod-tidy`,
`go-vet` and `go-test` — and assume a `go.mod` there. None of them spans a
workspace (see above), so replace each with a local hook that loops:

```yaml
- id: go-mod-tidy
  name: go mod tidy
  entry: scripts/modules.sh go mod tidy
  language: golang
  types_or: [go, go-mod, go-sum]
  pass_filenames: false
```

## Develop

```sh
go test ./...                             # or scripts/modules.sh go test ./...
go run . -greeting Howdy gopher           # HOWDY, GOPHER! with -shout
prek run --all-files                      # everything CI runs, minus pre-push hooks
prek run --all-files --hook-stage pre-push
```

Tool versions live in `.pre-commit-config.yaml` (`additional_dependencies`),
which CI reuses — the one exception is govulncheck, pinned again in
`test.yml` and `release.yml` because those run it outside the hooks. Bump
those three together; Dependabot sees none of them.

Tool provenance: the Go tools pin a version and let the module checksum
database verify the bytes; actionlint and zizmor come from their upstream
hook repos pinned to a git SHA, which is a hash by construction. What a SHA
pin does _not_ cover is the wheel zizmor's hook installs from PyPI — that is
resolved by version, like any other Python dependency. Pinning those five
per-platform wheels by `#sha256` is possible and this template used to do
it, but it has to be redone by hand on every bump and nothing warns you when
it drifts; upstream's repo is bumped by `prek auto-update` instead.
