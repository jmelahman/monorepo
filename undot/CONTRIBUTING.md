# Contributing

undot is written in Rust with no dependencies beyond the standard library: release
binaries are a few hundred kilobytes, start instantly, and have no runtime dependencies,
and the pre-commit hook builds from source without touching a package registry.

## Building and testing

```sh
cargo build --release     # binary at target/release/undot
cargo test                # unit tests, plus tests/cli.rs driving the real binary
prek run --all-files      # everything CI runs, as defined in .pre-commit-config.yaml
```

## Releasing

Releases are cut by pushing a `v*.*.*` tag, which must match `version` in `Cargo.toml`
(`scripts/check-version.sh` guards this, and that `Cargo.lock` and the `rev:` shown in
`README.md` and `docs/index.html` agree). The hook rev that `undot migrate` writes comes
from the same version. The workflow then publishes:

- a GitHub release with static binaries for linux/macOS × amd64/arm64, targeting musl
  on Linux, and
- PyPI wheels for the same platforms plus an sdist, built by
  [maturin](https://www.maturin.rs). The Linux wheels are tagged both manylinux and
  musllinux, since a static binary runs on either.

The Rust toolchain and maturin are pinned exactly in the workflow, and maturin also in
`pyproject.toml`; `scripts/check-version.sh` fails if those two disagree.
