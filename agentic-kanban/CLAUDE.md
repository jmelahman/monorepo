# Agentic Kanban — Claude Notes

A Go backend (repo root) plus a React/Vite frontend (`web/`). Run commands
below translate `.vscode/tasks.json` (the source of truth) for shell use.

## Running the app

Both processes are long-running. Start them with `run_in_background: true`,
then poll until their ports answer before driving the UI.

```bash
wgo run . serve                                          # backend :7474
cd web && npm install && npm run dev -- --host 0.0.0.0   # frontend :5173
```

Against a backend on the host, add `KANBAN_BACKEND=localhost:7474` and
`--port 5174` to the frontend command. Wait for both before navigating:

```bash
until curl -sf -m 1 http://localhost:7474/api/boards >/dev/null \
   && curl -sf -m 1 http://localhost:5173/ >/dev/null; do sleep 2; done
```

For fresh-install issues, run `wgo run . serve --in-memory`: it logs
`WARNING: --in-memory set` and uses an ephemeral SQLite DB — each launch
starts from zero and shutdown discards everything.

## Tests / typecheck / lint

- Go: `go test ./...`
- Frontend types: `cd web && npm run typecheck`
- Frontend lint/format (Biome): `cd web && npm run check` (`check:fix` to
  auto-apply safe fixes). The `prek` `biome` hook runs the same check.
- Playwright E2E: `cd web && npm run test:e2e`. Read
  `web/tests/e2e/README.md` before adding or modifying a spec.
- Pre-commit hooks: `prek run --all-files` (run before committing).
- DB perf: `go test -tags=perfbench -run TestPerfReport -v ./internal/db/`
  times hot queries with vs. without `idx_*` indexes. Run it for
  schema/query changes; `MaxPosition`/`ListArchived` regressing is the canary.

## Metrics

`internal/metrics` serves a Prometheus registry at `GET /metrics`; signal
catalog and PromQL in `docs/guide/observability.md`. When extending:

- New HTTP client calling GitHub → wrap its `Transport` with
  `metrics.WrapGitHubTransport(nil)` or it's missing from rate-limit gauges.
- New timed git shell-out in `internal/git` → `metrics.ObserveGitCommand("<op>",
  start, err)`; keep the `op` label set small and fixed.

The `compose.yaml` Prometheus is reachable from the devcontainer at
`http://prometheus:9090/prometheus/` — its API lives under
`/prometheus/api/v1/...` (bare `/api/v1/...` 404s).

## Playwright MCP

`.mcp.json` registers `@playwright/mcp --headless --isolated`. Use the
`mcp__playwright__browser_*` tools (`browser_navigate`, `browser_snapshot`);
never spawn `npx playwright` ad-hoc.

## Layout

- `main.go`, `cmd/`, `internal/` — Go server, MCP, CLI subcommands.
- `web/` — Vite + React 19 + Tailwind 4 frontend.
- `docs/` — VitePress site (`guide/`, `reference/{api,cli,mcp}.md`).
- `.kanban.toml` — maps task labels to container ports (host `13000–13099`).
- `.devcontainer/` — image plus an outbound allowlist firewall. `No route to
  host` means firewalled — don't work around it; use an allowed mirror or ask.

## Recurring regression notes

`REGRESSIONS.md` records traps that have bitten us before (session row
writers, PTY/terminal teardown, SSE subscriptions, worktrees, container
execs, and more). Before finishing a change, skim its headings and read any
entry that touches what you changed. When you fix something likely to recur,
add an entry there and a `See REGRESSIONS.md: "<title>"` comment at the code.

## Documentation upkeep

User-facing changes need a docs update in the same PR:

- CLI flags or subcommands → `docs/reference/cli.md`.
- HTTP endpoints or request/response shapes → `docs/reference/api.md`.
- MCP tools → `docs/reference/mcp.md`.
- Config keys (`.kanban.toml`, env vars, `--in-memory`) → `docs/guide/configuration.md`.
- Install/setup steps → `docs/guide/install.md` or `docs/guide/quickstart.md`.

Otherwise add a page under `docs/guide/` linked from `docs/.vitepress/config.ts`.
Skip docs only for internal refactors with no observable behavior change.
