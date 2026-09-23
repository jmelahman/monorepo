# Introduction

Fullstack Template is a starting point for applications that pair a Go backend
with a React frontend, shipped as one self-contained binary.

The backend embeds:

- a JSON REST API under `/api/`
- a SQLite database (pure Go, no CGO)
- the production build of the frontend (with `-tags embed`)

The `web/` frontend is a Vite + React 19 + Tailwind 4 app that proxies `/api`
to the backend during development.

## Layout

| Path | Purpose |
| --- | --- |
| `main.go`, `cmd/`, `internal/` | Go server and CLI subcommands |
| `web/` | Vite + React frontend, embedded into the binary at release |
| `docs/` | This VitePress site |
| `.github/`, `.goreleaser.yaml`, `pyproject.toml` | CI and release automation |

Continue with [Install](/guide/install) or the [Quickstart](/guide/quickstart).
