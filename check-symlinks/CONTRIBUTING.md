# Contributing

`check-symlinks` is written in [Solod](https://solod.dev), a strict subset of Go that
translates to C: release binaries are a few hundred kilobytes, start instantly, and
have no runtime dependencies.

## Building and testing

The source is written once, in the Solod subset, and builds with either toolchain:

```sh
so build -o check-symlinks-so ./so       # native binary (needs solod + a C compiler)
so test ./so                             # test suite under solod
so test -sanitize ./so                   # solod tests under ASan/UBSan
go build .                               # Go binary via gocompat (needs only Go)
go -C so run -modfile=gotest.mod ./test  # the same test suite against gocompat
go test ./...                            # end-to-end suite over testdata/
prek run --all-files                     # all of the above, as defined in .pre-commit-config.yaml
```

Install the solod toolchain with `go install solod.dev/cmd/so@latest`.

`so test` regenerates `so/test/main.go` from the `TestXxx` functions in `so/test/`;
that file is generated but committed, so run `so test ./so` after adding a test and
commit the result. CI fails on a dirty tree for exactly this reason.

## How the two toolchains coexist

- The root module's `go.mod` replaces `solod.dev` with `gocompat/`, a Go-stdlib-backed
  implementation of the solod APIs this tool uses. The real solod.dev packages are
  transpiler stubs that only work once translated to C, so this replacement is what
  makes a plain `go build` — and the pre-commit hook, which pre-commit builds with
  `go install ./...` — produce a working binary.
- The nested module `so/` requires the real `solod.dev` and pulls in the same `cli/`
  package via a directory replace; `so build ./so` and `so test ./so` run there.
  (Replace directives only apply in the main module, so the root's gocompat
  replacement is inert during solod builds.)

`go install github.com/jmelahman/check-symlinks@version` is refused by Go because of
the replace directive; that's intentional — use the pre-commit hook, a release binary,
or `go install ./...` from a checkout.

Two solod constraints are worth knowing before editing `cli/`:

- Errors are sentinel values compared with `==`; there is no wrapping, `errors.Is`, or
  `errors.As`. `gocompat/so/os` normalizes the Go stdlib's errors back to those
  sentinels so both toolchains agree.
- Package-level functions in `package main` become unprefixed C symbols, so names like
  `write`, `link`, or `mkdir` collide with the declarations in `<unistd.h>`. The test
  helpers are named `writeText`, `makeLink`, `makeDir` for this reason.

## Releasing

Releases are cut by pushing a `v*.*.*` tag. The tag must match `Version` in
`cli/cli.go` and the `rev:` in the README's pre-commit snippet
(`scripts/check-version.sh` guards this — solod has no ldflags-style stamping, so the
constant is the source of truth). The workflow then publishes:

- a GitHub release with static binaries via GoReleaser (`scripts/sobuild` swaps
  `go build` for `so build` + `zig cc`, targeting musl for Linux), and
- PyPI wheels for linux/macOS × amd64/arm64 plus an sdist, built by the hatch hook
  in `hatch_build.py` with the same toolchain.

Build dependencies are pinned exactly — directly in `pyproject.toml` and
`hatch_build.py` (the central pin registry), transitively (with hashes) in the
generated `build-constraints.txt` that CI passes to `uv build --build-constraints`.
After changing a pin, regenerate it with `scripts/gen-build-constraints.sh`;
`scripts/check-version.sh` fails when it's stale.

`goreleaser release --snapshot --clean --skip=publish --config .config/.goreleaser.yaml`
dry-runs the GitHub release artifacts locally.

## Benchmarks

`scripts/gen-benchmark-chart.py` holds the measurements behind the README chart and
the hyperfine commands that produced them, and regenerates
`docs/benchmark-{light,dark}.svg`.
