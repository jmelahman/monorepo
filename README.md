# Fullstack Template

[![Test status](https://github.com/jmelahman/fullstack-template/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/fullstack-template/actions)
[![Deploy Status](https://github.com/jmelahman/fullstack-template/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/fullstack-template/actions)
[![Docs](https://github.com/jmelahman/fullstack-template/actions/workflows/docs.yml/badge.svg)](https://jmelahman.github.io/fullstack-template/)
[![Go Reference](https://pkg.go.dev/badge/github.com/jmelahman/fullstack-template.svg)](https://pkg.go.dev/github.com/jmelahman/fullstack-template)
[![PyPI](https://img.shields.io/pypi/v/fullstack-template.svg)](https://pypi.org/project/fullstack-template/)

A template repository for fullstack applications: a Go backend with an
embedded SQLite database serving an embedded React frontend — server, web UI,
REST API, and CLI in one self-contained binary.

The sample app is a deliberately tiny "items" CRUD so every layer (schema,
store, handlers, HTTP client, CLI, React UI, E2E test) has one worked example
to copy from.

## What's included

**Backend** (`main.go`, `cmd/`, `internal/`)

- Cobra CLI: `app serve` plus noun-verb client subcommands (`app item list`)
  that talk to a running server over HTTP.
- `net/http` with Go 1.22+ pattern routing, JSON helpers, panic-recovery and
  request-logging middleware.
- SQLite via `modernc.org/sqlite` (pure Go, no CGO) with an idempotent
  embedded schema and an `--in-memory` mode for tests and demos.
- Version stamping via ldflags with a `runtime/debug` fallback.

**Frontend** (`web/`)

- Vite + React 19 + Tailwind 4 + TanStack Query, typechecked with TypeScript.
- Biome for lint + format; theme tokens with dark/light support.
- Playwright E2E suite that boots the real backend (`--in-memory`) and the
  Vite dev server.
- Production build embedded into the Go binary with `-tags embed`
  (`web/static_embed.go`).

**Docs** (`docs/`)

- VitePress site with guide/reference structure, llms.txt generation, and a
  GitHub Pages deploy workflow.

**Packaging & ops**

- GoReleaser (linux/darwin/windows × amd64/arm64) publishing GitHub releases.
- Multi-arch Docker image (distroless-style nonroot Alpine) + `compose.yaml`
  + `docker-bake.hcl`.
- PyPI wheels per platform via hatch + `go-bin` + `manygo`
  (`uv tool install <name>` installs the Go binary).
- prek (pre-commit) hooks: builtin checks, actionlint, ripsecrets,
  govulncheck (with a documented allowlist wrapper), npm audit, Biome.
- GitHub Actions: tests + lint + image smoke test, release pipeline, docs
  deploy, zizmor, Dependabot.
- `.devcontainer/` dev sandbox with an opt-in default-deny network firewall.
- `.kanban.toml` task/port mapping for [agentic-kanban](https://github.com/jmelahman/agentic-kanban) sessions.

## Using this template

1. Create a repo from this template (GitHub → "Use this template").
2. Find-and-replace the placeholder identifiers:
   - `github.com/jmelahman/fullstack-template` → your Go module path
     (`go.mod`, all imports, ldflags in `Dockerfile` / `.goreleaser.yaml` /
     `hatch_build.py`).
   - `fullstack-template` → your project name (`pyproject.toml`, docs,
     workflows, badges).
   - `lahmanja/fullstack-template` → your Docker Hub repository
     (`compose.yaml`, `docker-bake.hcl`, `release.yml`).
   - `app` → your binary name (`.goreleaser.yaml`, `pyproject.toml`,
     `Dockerfile`, `internal/config`, docs).
   - `APP_` → your env-var prefix (`internal/config`, `cmd/server/cli.go`,
     `vite.config.ts`, `playwright.config.ts`).
3. The CI/devcontainer image is `lahmanja/devcontainer` (rebuilt by
   `devcontainer.yml`, manual dispatch only); point it at your own registry
   or swap the container jobs for setup-go/setup-node.
4. Configure repo settings: a `release` environment, `DOCKERHUB_USERNAME` /
   `DOCKERHUB_TOKEN` secrets, PyPI trusted publishing, and GitHub Pages
   (source: GitHub Actions).
5. Regenerate lockfiles (`npm install` in `web/` and `docs/`) and run
   `prek run --all-files`.
6. Replace this README (and the LICENSE, if GPLv3 doesn't fit) with your
   project's own.

## Install

```sh
uv tool install fullstack-template
```

This installs the binary to `~/.local/bin/app`.
Make sure to have that on your `PATH`.

## Run

```sh
app serve
```

The server listens on `:8080`. Open <http://localhost:8080/>.

Or with Docker:

```sh
docker compose up -d --build
```

## Develop

```sh
# Backend on :8080 (wgo restarts on save; plain `go run` works too)
wgo run . serve

# Frontend on :5173, proxying /api to the backend
cd web && npm install && npm run dev
```

### Docs site

The `docs/` VitePress site deploys to GitHub Pages automatically: pushes to
`master` that touch `docs/**` run `.github/workflows/docs.yml`, which builds
the site and publishes it through the Pages Actions deploy (no `gh-pages`
branch). Run it locally with:

```sh
cd docs && npm install && npm run docs:dev   # http://localhost:5175
```

One-time setup on a new repo: enable Pages with the "GitHub Actions" source
(Settings → Pages, or `gh api -X POST repos/<owner>/<repo>/pages -f
build_type=workflow`), keep `base` in `docs/.vitepress/config.ts` in sync
with the repo name (it's the URL prefix Pages serves the site under), and
trigger the first deploy with `gh workflow run docs.yml`.

See `CLAUDE.md` for the full development reference (tests, lint, E2E, docs
conventions).

## 📖 Documentation

Full documentation lives at **<https://jmelahman.github.io/fullstack-template/>**:

- [Quickstart](https://jmelahman.github.io/fullstack-template/guide/quickstart) — serve, add items, script the CLI.
- [Configuration](https://jmelahman.github.io/fullstack-template/guide/configuration) — flags and env vars.
- [REST API](https://jmelahman.github.io/fullstack-template/reference/api) — endpoints exposed by the running server.
- [CLI](https://jmelahman.github.io/fullstack-template/reference/cli) — `serve`, `item list`, `item create`.
