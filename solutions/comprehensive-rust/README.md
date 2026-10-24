# Comprehensive Rust

My solutions to the exercises in Google's [Comprehensive Rust](https://google.github.io/comprehensive-rust/) course.
Set up to work offline.

## Layout

- `exercises/NN-name/`: one crate per exercise, in course order. Each one starts as the course's starter code, where `todo!()` and `...` mark the parts to fill in.
  - Days 1–4: `01-fibonacci` through `20-ffi-wrapper`.
  - Concurrency: `21-dining-philosophers` through `24-chat-app`.
- `course/comprehensive-rust.pdf`: the whole course for reading offline. This folder is gitignored. To refresh it:

  ```shell
  curl -fL --create-dirs -o course/comprehensive-rust.pdf https://google.github.io/comprehensive-rust/comprehensive-rust.pdf
  ```

The course's Android, Chromium and bare-metal exercises are left out because they need those environments.

## Working on an exercise

```shell
docker compose up -d --build  # once; leaves the container running
docker compose exec rust zsh  # a shell in /workspace (this directory)

cargo run -p fibonacci        # exercises with a main
cargo test -p luhn            # exercises with tests
cargo clippy -p luhn
```

A starter fails to compile until you fill it in. That is expected for many of them, so build one package at a time with `-p` instead of the whole workspace.
The course PDF has a solution page after each exercise.

## Offline

The `Dockerfile` builds on the monorepo's devcontainer image. It adds the toolchain pinned in `rust-toolchain.toml` (with clippy, rustfmt, rust-analyzer, rust-src and rust-docs) and the crates every exercise depends on.
After the image is built once, `cargo build`, `cargo test` and rust-analyzer need no network.
The Book, Rust by Example and the standard library docs are installed too: `rustup doc --path --book` (or `--rust-by-example`, `--std`) prints where.

`22-link-checker` crawls real websites, so it only does something useful online.

Two changes need the network: adding a crate to an exercise, and bumping the toolchain.
After either, rebuild the image while online so it is cached again.
To bump the toolchain, change `rust-toolchain.toml` and the `rust` stage in `Dockerfile` together.
