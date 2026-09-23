---
layout: home

hero:
  name: Fullstack Template
  text: A template for fullstack Go + React applications.
  tagline: A Go backend with an embedded SQLite database serving an embedded React frontend — server, web UI, REST API, and CLI in one self-contained binary.
  actions:
    - theme: brand
      text: Get Started
      link: /guide/install
    - theme: alt
      text: Quickstart
      link: /guide/quickstart
    - theme: alt
      text: View on GitHub
      link: https://github.com/jmelahman/fullstack-template

features:
  - title: One small binary
    details: Server, web UI, REST API, and CLI ship in a single self-contained Go binary. No runtime, no node_modules, no sidecar processes — also distributed as a Docker image and a Python wrapper on PyPI.
  - title: Modern frontend toolchain
    details: Vite + React 19 + Tailwind 4, typechecked with TypeScript, linted and formatted with Biome, and covered by a Playwright E2E suite that boots the real backend.
  - title: Local SQLite storage
    details: A pure-Go SQLite driver (no CGO) with an idempotent schema. Use --in-memory for ephemeral runs and tests.
  - title: REST + CLI
    details: A small JSON API plus noun-verb CLI subcommands for scripting against a running server.
  - title: Release automation included
    details: GoReleaser for GitHub releases, multi-arch Docker images, and PyPI wheels built per-platform with a Go build hook — all wired into GitHub Actions.
  - title: Guardrails from day one
    details: prek (pre-commit) hooks, govulncheck, npm audit, actionlint, zizmor, ripsecrets, and Dependabot are configured out of the box.
---
