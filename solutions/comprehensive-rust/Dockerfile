# The monorepo's devcontainer image (templates/fullstack-template/.devcontainer)
# plus a Rust toolchain and every exercise's crates, so the course works offline
# once this is built.
#
# .dockerignore cuts the context down to the manifests, so editing a solution
# never rebuilds this.

# Declared as a stage (not inline COPY --from) so Dependabot bumps its version
# and digest pin. Keep rust-toolchain.toml's channel in step.
FROM rust:1.99.1-slim-trixie@sha256:1a939748a7cbab172de91a014e3804a97af1394a8c844faa67fce9401c4e40fb AS rust

FROM ghcr.io/jmelahman/devcontainer:latest

# compose.yaml runs as root; see the note there.
USER root

COPY --from=rust /usr/local/rustup /usr/local/rustup
COPY --from=rust /usr/local/cargo /usr/local/cargo
ENV RUSTUP_HOME=/usr/local/rustup \
    CARGO_HOME=/usr/local/cargo \
    PATH="/usr/local/cargo/bin:${PATH}"

# rust-docs is The Book, Rust by Example and the std docs (`rustup doc`).
COPY rust-toolchain.toml /tmp/toolchain/
RUN cd /tmp/toolchain && rustup toolchain install && rm -rf /tmp/toolchain

# Cargo needs a target to accept a manifest, so each exercise gets an empty
# lib.rs; fetching never compiles it.
COPY . /tmp/prefetch/
RUN cd /tmp/prefetch \
  && for d in exercises/*/; do mkdir -p "$d/src" && touch "$d/src/lib.rs"; done \
  && cargo fetch --locked \
  && rm -rf /tmp/prefetch
