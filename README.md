# local-preview

[![Test status](https://github.com/jmelahman/local-preview/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/local-preview/actions)
[![Deploy Status](https://github.com/jmelahman/local-preview/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/local-preview/actions)
[![Docs](https://github.com/jmelahman/local-preview/actions/workflows/docs.yml/badge.svg)](https://jmelahman.github.io/local-preview/)
[![Go Reference](https://pkg.go.dev/badge/github.com/jmelahman/local-preview.svg)](https://pkg.go.dev/github.com/jmelahman/local-preview)
[![PyPI](https://img.shields.io/pypi/v/local-preview.svg)](https://pypi.org/project/local-preview/)

A local-first preview-deployment orchestrator: every commit of a registered
git repository becomes a servable preview at its own subdomain
(`<sha>-<repo>.preview.localhost:8080`), built once, deduplicated by
content, and served from a single binary.

`local-preview` resolves each commit into a *(frontend artifact, backend
artifact)* pair by content-addressing the relevant subtrees. Commits that
don't touch the frontend (or backend) reuse the existing artifact — and the
already-running backend process. Backend state follows git lineage: a new
backend version forks its data from the nearest deployed ancestor, so
previews feel continuous on a straight line of commits while divergent
branches can never corrupt each other.

## Install

```sh
uv tool install local-preview
```

This installs the binary to `~/.local/bin/preview`.
Make sure to have that on your `PATH`.

## Run

```sh
preview serve
```

The server listens on `:8080`. Open <http://localhost:8080/> for the
dashboard.

Or with [Docker
Compose](https://jmelahman.github.io/local-preview/guide/install):

```sh
docker compose up -d
```

## 📖 Documentation

Full documentation lives at **<https://jmelahman.github.io/local-preview/>**:

- [Quickstart](https://jmelahman.github.io/local-preview/guide/quickstart) — register a repo, deploy a commit, open the preview.
- [Configuration](https://jmelahman.github.io/local-preview/guide/configuration) — flags and env vars.
- [REST API](https://jmelahman.github.io/local-preview/reference/api) — endpoints exposed by the running server.
- [CLI](https://jmelahman.github.io/local-preview/reference/cli) — `serve`, `repo`, `deploy`, `install-hook`.
