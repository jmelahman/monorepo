# AgileCBT — Claude Notes

A Go backend (repo root) plus a React/Vite frontend (`web/`). Run commands
below translate `.vscode/tasks.json` (the source of truth) for shell use.

## Running the app

Both processes are long-running. Start them with `run_in_background: true`,
then poll until their ports answer before driving the UI.

**Backend** (`:8080`):

```bash
wgo run . serve
```

**Frontend** (`:5173`, proxies `/api` to the backend):

```bash
cd web && bun install && bun run dev --host 0.0.0.0
```

**Frontend against a non-default backend** (`:5174`):

```bash
cd web && bun install && APP_BACKEND=localhost:8080 bun run dev --host 0.0.0.0 --port 5174
```

Wait for both to be reachable before navigating:

```bash
until curl -sf -m 1 http://localhost:8080/api/health >/dev/null \
   && curl -sf -m 1 http://localhost:5173/ >/dev/null; do sleep 2; done
```

## Reproducing fresh-install issues

Boot a clean instance with no on-disk DB:

```bash
wgo run . serve --in-memory
```

It logs `WARNING: --in-memory set` and uses an ephemeral SQLite DB: each
launch starts from zero and shutdown discards everything.

## Tests / typecheck / lint

- Go: `go test ./...`
- Frontend types: `cd web && bun run typecheck`
- Frontend lint/format (Biome): `cd web && bun run check` (`check:fix` to
  auto-apply safe fixes). The `prek` `biome` hook runs the same check on
  staged `web/**/*.{ts,tsx,js,jsx,json}` files.
- Playwright E2E: `cd web && bun run test:e2e` (first run `test:e2e:install`
  for Chromium). Boots the backend `--in-memory` on :8095 with `APP_LLM=ollama`
  pointed at scripted `tests/e2e/fake-ollama.mjs` on :11499, plus Vite on :5177,
  so a running dev stack is never reused. Tests share one DB: don't assume it's
  empty (unique titles, `.last()`).
- Pre-commit hooks: `prek run --all-files` (run before committing).

## QA via Sonnet subagents

Delegate token-heavy QA where an occasional miss is acceptable (Playwright
MCP walkthroughs, `bun run test:e2e` triage, API/MCP smoke tests) to an
`Agent` with `model: "sonnet"`. Give it the URL, steps, and pass criteria
(start the dev stack first or tell it how); ask for a short verdict with
evidence (failing step, error text), not raw tool output. Keep code changes
and final calls in the main session, and verify results that look wrong.

## MCP

`.mcp.json` registers:

- **playwright** — `@playwright/mcp --headless --isolated`. Use the
  `mcp__playwright__browser_*` tools (`browser_navigate`, `browser_snapshot`);
  never spawn `bunx playwright` ad-hoc.
- **agilecbt** — streamable HTTP at `http://localhost:8080/mcp` (same tools
  as the in-app curator). Needs `agilecbt serve` running.

## Layout

- `main.go`, `cmd/`, `internal/` — Go server and CLI subcommands.
- `web/` — Vite + React 19 + Tailwind 4 frontend, embedded into the binary
  with `-tags embed`.
- `docs/` — VitePress site (`guide/`, `reference/api.md`, `reference/cli.md`).
- `.kanban.toml` — maps task labels to container ports for agentic-kanban.
- `.devcontainer/` — dev sandbox image with an opt-in network firewall.

## Documentation upkeep

User-facing changes need a docs update in the same PR:

- New/changed CLI flags or subcommands → `docs/reference/cli.md`.
- New/changed HTTP endpoints or request/response shapes → `docs/reference/api.md`.
- Config keys (env vars, `--in-memory`, etc.) → `docs/guide/configuration.md`.
- Install/setup steps → `docs/guide/install.md` or `docs/guide/quickstart.md`.

Otherwise add a page under `docs/guide/` linked from `docs/.vitepress/config.ts`.
Skip docs only for internal refactors with no observable behavior change.
