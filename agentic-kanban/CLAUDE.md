# Agentic Kanban — Claude Notes

A Go backend (`/workspace`) plus a React/Vite frontend (`/workspace/web`).
Source of truth for run commands is `.vscode/tasks.json`; this file translates
them for use inside this devcontainer.

## Running the app

Both processes are long-running. Start them with `run_in_background: true`,
then poll until their ports answer before driving the UI.

**Backend** (`:7474`):

```bash
wgo run . serve
```

**Frontend** (`:5173`, talks to backend in the same container):

```bash
cd web && npm install && npm run dev -- --host 0.0.0.0
```

**Frontend against a host backend** (`:5174`, points at a backend running on
the host at `localhost:7474`):

```bash
cd web && KANBAN_BACKEND=localhost:7474 npm install && npm run dev -- --host 0.0.0.0 --port 5174
```

Wait for both to be reachable before navigating:

```bash
until curl -sf -m 1 http://localhost:7474/api/boards >/dev/null \
   && curl -sf -m 1 http://localhost:5173/ >/dev/null; do sleep 2; done
```

## Reproducing fresh-install issues

Boot a clean instance with no on-disk DB:

```bash
wgo run . serve --in-memory
```

The server logs `WARNING: --in-memory set` at startup and uses an ephemeral
SQLite database (shared cache, single connection) for the lifetime of the
process. Each launch starts from zero, and shutting the process down
discards everything — including any boards or tickets the UI created.
Frontend (`:5173`) and Playwright MCP (`browser_navigate
http://localhost:5173/`) work as usual.

## Tests / typecheck / lint

- Go: `go test ./...`
- Frontend types: `cd web && npm run typecheck`
- Frontend lint/format (Biome): `cd web && npm run check` (`check:fix` to
  auto-apply safe fixes). The `prek` `biome` hook runs the same check on
  staged `web/**/*.{ts,tsx,js,jsx,json}` files.
- Playwright E2E: `cd web && npm run test:e2e`. Read
  `web/tests/e2e/README.md` before adding or modifying a spec.
- Pre-commit hooks: `prek run --all-files` (run before committing).

## Perf benchmark

`internal/db/perfbench_test.go` is a build-tagged comparison of hot DB
queries with vs. without the `idx_*` indexes from `schema.sql`. It seeds
an in-memory SQLite at n=100/1000/5000 tickets, times each query, drops
the indexes, re-times, and prints a speedup table.

```bash
go test -tags=perfbench -run TestPerfReport -v ./internal/db/
```

The `perfbench` build tag keeps it out of the default `go test ./...`.
Use it to validate future schema/query changes — a regression on the
`MaxPosition` or `ListArchived` rows is the canary (those are 30–95×
faster with indexes at n≥1000; everything else is a wash on small tables).

## Metrics

`internal/metrics` exposes a shared Prometheus registry served at
`GET /metrics`. See `docs/guide/observability.md` for the signal catalog
(HTTP, GitHub REST API, local git CLI, SQLite, Go runtime), PromQL
examples, and recording rules.

Two rules when extending instrumentation:

- New HTTP client that calls GitHub → wrap its `Transport` with
  `metrics.WrapGitHubTransport(nil)`, or it won't show up in the
  rate-limit gauges.
- New git shell-out in `internal/git` worth timing → call
  `metrics.ObserveGitCommand("<operation>", start, err)` with `start` taken
  just before the command; keep the `operation` label set small and fixed.

`compose.yaml` bundles a Prometheus service reverse-proxied at `/prometheus/`
(driven by `$KANBAN_PROMETHEUS_URL`). From inside this devcontainer (attached
to `kanban-net`) it's also reachable directly at
`http://prometheus:9090/prometheus/` — the server runs with
`--web.external-url=/prometheus`, so the API lives under
`/prometheus/api/v1/...` (a bare `/api/v1/...` returns 404). Useful probes:

```bash
# Build / version
curl -sf http://prometheus:9090/prometheus/api/v1/status/buildinfo

# Active scrape targets (expect kanban:7474 -> up)
curl -sf 'http://prometheus:9090/prometheus/api/v1/targets?state=active'

# Instant query
curl -sf --data-urlencode 'query=up' \
  http://prometheus:9090/prometheus/api/v1/query
```

## Driving the UI with Playwright MCP

`.mcp.json` registers `@playwright/mcp --headless --isolated`. Use the
`mcp__playwright__browser_*` tools (e.g. `browser_navigate
http://localhost:5173/`, then `browser_snapshot`) — never spawn `npx
playwright` ad-hoc.

`@playwright/mcp` defaults to the `chrome` channel which looks at
`/opt/google/chrome/chrome`; the devcontainer Dockerfile symlinks that to the
bundled Chromium under `$PLAYWRIGHT_BROWSERS_PATH=/ms-playwright`.

## Layout

- `main.go`, `cmd/`, `internal/` — Go server, MCP, CLI subcommands.
- `web/` — Vite + React 19 + Tailwind 4 frontend.
- `docs/` — VitePress site (`guide/`, `reference/api.md`, `reference/cli.md`,
  `reference/mcp.md`).
- `.kanban.toml` — maps the task labels above to container ports so kanban
  proxies them to host ports `13000–13099` when run as a session.
- `.devcontainer/` — image, firewall (allowlists `storage.googleapis.com`,
  `registry.npmjs.org`, GitHub, `proxy.golang.org`, anthropic, etc.).

## Recurring regression notes

Tripwires for code that's bitten us before. Full write-ups live in
`REGRESSIONS.md` (kept out of the published `docs/` site — it's internal
lore, not user docs). If you're touching any area below, read its entry there
before changing it. Several of these fire from changes that don't look risky
(adding an SSE event, a handler that updates a couple of columns), so don't
skip the check. When you fix something new and likely to recur, add the full
entry to `REGRESSIONS.md` and a one-line title here.

- `ghostty-web` terminal dispose poisons the WASM heap — terminal teardown
- `ghostty-web` canvas doesn't fill its container — terminal sizing/fit
- `sessions` row has multiple writers — session lifecycle vs PR poller
- `sessions` rows don't track container liveness — acting on a session from its row alone
- Closing a PTY broker doesn't end its process — replacing a PTY in a live container
- Per-origin SSE streams starve the WebSocket pool — board subscriptions
- A deleted board can refetch its `/state` on a loop — board-level events
- Build Cop boards on the same repo share one jobs cache — per-tick cache eviction
- Exec sites hardcode `/workspace` — anything that needs a container working directory

## Documentation upkeep

User-facing changes need a docs update in the same PR. Match the change to
the page:

- New/changed CLI flags or subcommands → `docs/reference/cli.md`.
- New/changed HTTP endpoints or request/response shapes → `docs/reference/api.md`.
- New/changed MCP tools → `docs/reference/mcp.md`.
- Config keys (`.kanban.toml`, env vars, `--in-memory`, etc.) → `docs/guide/configuration.md`.
- Install/setup steps → `docs/guide/install.md` or `docs/guide/quickstart.md`.

If a feature doesn't fit an existing page, add one under `docs/guide/` and
link it from `docs/.vitepress/config.*`. Skip docs only for purely internal
refactors with no observable behavior change.

## Network notes

Outbound HTTP from this container is allowlisted. If a fetch fails with
`No route to host`, the destination is firewalled — don't try to work around
it; pick an allowed mirror or tell the user.
