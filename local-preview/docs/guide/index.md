# Introduction

local-preview is a self-hostable preview-deployment orchestrator. You
register git repositories with it, and every commit of those repos can be
served as a preview at its own subdomain, like
`a1b2c3d.myapp.preview.localhost:8080`.

The server is one Go binary. It contains a reverse proxy that routes
previews by Host header, a build queue producing content-addressed
artifacts, a supervisor that runs backend processes on demand, a JSON REST
API under `/api/`, a CLI, a dashboard, and a SQLite database (pure Go, no
CGO).

Target applications declare how to build and run themselves in a small
[`preview.toml`](/reference/preview-toml) at their repo root. That file is
the entire contract between your app and the orchestrator.

Start with [Install](/guide/install), then walk through the
[Quickstart](/guide/quickstart). The [Concepts](/guide/concepts) page
explains the ideas that keep previews cheap: content addressing, backend
sharing, and lineage-forked state. To use the orchestrator inside another
Go program instead of running it standalone, see
[Embedding](/guide/embedding).

## Developing

To hack on local-preview itself, run the backend and the dashboard
separately for hot reload:

```sh
# Terminal 1 — backend on :8080 (wgo restarts on save; plain `go run` works too)
wgo run . serve

# Terminal 2 — dashboard on :5173, proxying /api to the backend
cd web && npm install && npm run dev
```
