# local-preview — Claude Notes

A local-first preview-deployment orchestrator: a Go backend (repo root) plus
a React/Vite dashboard (`web/`). Registered git repos get a preview per
commit at `<sha>-<repo>.preview.localhost:8080` — content-addressed builds,
on-demand backend processes, lineage-forked state. Architecture notes live in
`docs/guide/concepts.md`; the target-repo contract in
`docs/reference/preview-toml.md`.
Source of truth for run commands is `.vscode/tasks.json`; this file translates
them for direct shell use.

## Running the app

Both processes are long-running. Start them with `run_in_background: true`,
then poll until their ports answer before driving the UI.

**Backend** (`:8080`):

```bash
wgo run . serve
```

**Frontend** (`:5173`, proxies `/api` to the backend):

```bash
cd web && npm install && npm run dev -- --host 0.0.0.0
```

**Frontend against a non-default backend** (`:5174`):

```bash
cd web && PREVIEW_BACKEND=localhost:8080 npm install && npm run dev -- --host 0.0.0.0 --port 5174
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

The server logs `WARNING: --in-memory set` at startup and uses an ephemeral
SQLite database for the lifetime of the process. Each launch starts from
zero, and shutting the process down discards everything.

## Tests / typecheck / lint

- Go: `go test ./...`
- Frontend types: `cd web && npm run typecheck`
- Frontend lint/format (Biome): `cd web && npm run check` (`check:fix` to
  auto-apply safe fixes). The `prek` `biome` hook runs the same check on
  staged `web/**/*.{ts,tsx,js,jsx,json}` files.
- Playwright E2E: `cd web && npm run test:e2e` (boots the real backend with
  `--in-memory` plus the Vite dev server).
- Pre-commit hooks: `prek run --all-files` (run before committing).

## Driving the UI with Playwright MCP

`.mcp.json` registers `@playwright/mcp --headless --isolated`. Use the
`mcp__playwright__browser_*` tools (e.g. `browser_navigate
http://localhost:5173/`, then `browser_snapshot`) — never spawn `npx
playwright` ad-hoc.

## Layout

- `main.go`, `cmd/`, `internal/` — Go server and CLI subcommands.
- `web/` — Vite + React 19 + Tailwind 4 frontend, embedded into the binary
  with `-tags embed`.
- `docs/` — VitePress site (`guide/`, `reference/api.md`, `reference/cli.md`).
- `.kanban.toml` — maps the task labels above to container ports for
  agentic-kanban sessions.
- `.devcontainer/` — dev sandbox image with an opt-in network firewall.

## Recurring regression notes

Tripwires for code that's bitten us before. Full write-ups live in
`REGRESSIONS.md` (kept out of the published `docs/` site — it's internal
lore, not user docs). When you fix something new and likely to recur, add the
full entry to `REGRESSIONS.md` and a one-line title here.

- Container tests in CI see the runner host's daemon, not their own
  filesystem — bind mounts silently resolve on the host; probe before
  asserting on bind-mounted output.
- Side publishes rename their subtree out of the shared scratch dir —
  anything reading the extracted tree (checksums, post-publish steps) must
  run before `PublishFrontend`/`PublishBackend`, or take its own extraction
  (as the post-ready artifact phase does).
- React's `autoFocus` prop loses the focus race against `<dialog>.showModal()`
  — opt a control into initial focus with `data-autofocus` (which `Modal`
  focuses after opening) instead.
- On-disk leftovers must never gate DB-owned decisions — deletes clean disk
  best-effort, so creates replace orphaned dirs; only DB rows may conflict.
- A startup backlog must not be enqueued from the goroutine that serves HTTP —
  a bounded-channel send before `ListenAndServe` wedges the whole server.
- State derived from a live-process table can't report a process that died —
  a failure record has to outlive the process, or "crashed" reads as "idle".
- Uploads must hash only the side being uploaded — computing every side's hash
  (via `resolveHashes`) makes a one-side upload fail on an unrelated partition.
- GitHub OIDC upload auth needs a custom `aud` (the default GH audience is
  org-wide) and must verify the token before the repo lookup (else uploads leak
  which repos exist).
- A GitHub OIDC token authenticates but authorizes nothing on its own — accept
  one only on a route that names a repo, and bind it to that repo's registered
  source, or any repo's CI can act on any other's.
- A derived runtime state can't be filtered by a stored-status predicate —
  "crashed" lives in the supervisor, not the `status` column, so a listing
  filter has to resolve it into a row predicate (never post-filter a page:
  that breaks paging and `X-Total-Count`).
- SSO's preview-access cookie must stay a separate scope from the apex session
  and be stripped in the proxy `Rewrite` — else the untrusted previewed backend
  receives (and can replay) the control-plane credential.
- A truncated artifact must never land under a content-addressed key — the S3
  tier's skip-if-exists would make it permanent; compress to a temp file (abort
  before put), record the size, and verify it on hydrate.
- Absence of on-disk artifact files must not be read as eviction — once local
  disk is a cache, a swept-but-live deploy must hydrate-and-serve, not 410;
  eviction is a DB fact, residency a cache fact (use `EvictCacheToWatermark`,
  never `RemoveBackend`, and hydrate on serve before failing).
- The proxy routes to a `Backends` interface returning `host:port`, not a bare
  port — one orchestrator, two transports (loopback `LocalBackends` and remote
  `workerapi.Client`); don't reintroduce a port-only assumption or a second
  orchestrator path.
- The worker API (`internal/workerapi`) starts arbitrary preview processes — a
  remote-code-execution surface. It must stay shared-secret authed and bound to
  a private listener only; never expose it via the ALB/apex router.
- Preview containers run untrusted branch code, so the instance's IMDS must be
  one hop out of their reach — set `http_put_response_hop_limit = 1` on every
  node's `metadata_options`. The orchestrator runs `--network host` (reaches
  IMDS as the host, hop 1), but a preview on the docker bridge is one hop
  further; the AWS default of 2 lets it assume the instance role and read S3
  artifacts + the SSM `PREVIEW_SECRET_*` deps credentials. Previews never need
  IMDS — they reach the deps by private IP.
- An unknown `user_data` silently defeats `user_data_replace_on_change` — a
  bucket name read from a resource makes the rendered script `(known after
  apply)`, so terraform plans an in-place update that cloud-init never re-runs;
  keep every value reaching `user_data` known at plan time (a `locals` literal).
- Fleet placement must co-place a process-mode frontend with its backend — the
  pair shares a per-deploy docker network that lives on one node, so a frontend
  hashes on its backend's hash (`Peer`), never its own. Splitting them across
  workers breaks `{backend_url}` and the deploy network.
- Every path that extracts an untrusted repo tree must reject escaping symlinks
  — `gitrepo.Archive` recreated a committed `leak -> /etc/passwd` verbatim, then
  the public frontend file server / artifact publisher followed it; apply the
  same absolute-reject + within-root rule `store.ExtractTar` already enforces.
  One deliberate carve-out: backend artifacts are executed payloads whose venv
  symlinks legitimately point into their run image, so they extract via
  `ExtractTarPayload` (names/hardlinks still strict) — don't re-unify the two
  policies in either direction.
- A worker resolves run specs from the wire, not its own DB — artifact rows
  never replicate; the ensure request carries the control-resolved `WireSpec`
  (state dir as identity, sticky per-node `InitDone`), and the regression test
  must drive a real `supervise.Manager` over an empty DB, never just `fakeSup`.
- Every publish path must persist to the durable tier in the same breath —
  reconcile is a repair pass, not a route; uploads that skipped `enqueuePersist`
  502'd on workers ("not present in durable tier") until the next tick.
- A deploy going `ready` must mean a serve-only worker could hydrate it — the
  async persist races auto-start/first-serve, so gate `SetDeployReady` on a
  synchronous `ensureDurable` (skip-if-exists) of fe/be, never mark ready off a
  merely-published, not-yet-durable artifact.
- Containered preview ports publish on loopback by default — a worker must
  also publish on its routable address (derived from `--worker-listen`) or the
  control node's proxy dials a port nothing answers; local health checks still
  poll 127.0.0.1, so the loopback binding must survive alongside it.
- Never mix inline SG rules with standalone rule resources on one group —
  Terraform wipes the standalone ones on the next in-place SG update (a retype
  deleted the deps ingress in production); give each rule owner its own group.
- Onyx SSO on previews: the canonical host's host-only session cookie must be
  widened to `Domain=.<preview-domain>` in the proxy (previews run
  `AUTH_BACKEND=jwt` and only validate the shared-secret JWT), and the
  cross-host `onyx_return` marker — which a previewed backend can set — must be
  validated to our own domain before the post-login redirect, or it's an open
  redirect. onyx's `sanitize_next_url` blocks cross-host `next`, so the return
  hop lives in the proxy, not onyx.
- The onyx-login return-dance must widen the session cookie itself, not lean on
  the login response — an already-logged-in user hits the canonical host with a
  host-only cookie the preview never receives, so the dance has to mint a
  `Domain=.<preview-domain>` copy before redirecting or the browser loops
  `preview/app → canonical/auth/login → preview/app` forever.
- Scale-from-zero rides on unserved demand, not load — `LoadRatio` is degenerate
  at zero workers (idle and saturated both read as full); publish `FleetLoad`
  only when a worker exists (else the scale-in alarm fires on an empty fleet),
  drive scale-in protection off the live fleet (no `DescribeAutoScalingInstances`
  grant), and keep the ASG name reaching `user_data` a plan-time literal.
- Unserved demand is a boolean per preview, not a request counter — dedupe by
  placement key (retries/refreshes of one preview read as 1) and respond with a
  single +1 step, because once any worker registers `place()` stops returning
  `ErrNoWorker` and every waiting preview lands on it (a proportional ladder
  boots extra nodes into a fleet that no longer reports demand); whatever
  registers demand must retry `ErrNoWorker` until its node arrives, or the node
  it summoned boots to nothing (the pre-warm's `startWithRetry`).
- Worker re-registration (every ~20s) must be idempotent in the fleet registry
  — replacing live `workerState` zeroes heartbeat freshness, blinding placement
  ~25% of wall-clock and registering phantom demand on small fleets; only a
  changed instance-id (new machine, same IP) may replace.
- A control-DB statistic doesn't exist in fleet mode unless workers ship it —
  process events ride the heartbeat (drained at-most-once) and replay with
  their ORIGINAL occurred_at (percentiles are timestamp deltas).
- Manifest env is baked into the artifact at build time — an env-borne
  operational fix (e.g. DB pool caps) never reaches already-built deploys;
  pair it with a server-side guard that covers legacy artifacts (the
  deps Postgres `idle_session_timeout`, safe under onyx's pool_pre_ping).
- A `nofail` data volume must gate the service that bind-mounts it — without
  `RequiresMountsFor=`, a boot that races the volume attach starts the
  orchestrator on an empty dir with a fresh SQLite DB, and a later mount can't
  fix it (bind mounts don't follow). Latent until reboots are routine.
- A worker's init result must flow back to the control DB — a successful
  backend ensure proves init succeeded (the client's `InitMarker` adopts it);
  a per-artifact fact recorded only worker-side means every fresh worker
  re-runs the work, and a scale-to-zero fleet makes fresh workers the norm.
- A response-writer wrapper must forward Flush/Hijack and expose Unwrap —
  embedding `http.ResponseWriter` erases those capabilities (the logging
  recorder broke WebSocket upgrades and SSE flushes through the apex), and
  `http.ResponseController` only reaches past a wrapper via `Unwrap`.
- Joined arg lists spliced into systemd/shell lines need per-arg quoting —
  `server_args` rendered unquoted into `ExecStart` word-split the first
  multi-word flag value and crash-looped the orchestrator on deploy.
- Every per-deploy docker network needs a release on container exit — the
  daemon's stock address pools hold ~30 networks, so a create-only network
  per backend hash exhausts them in a working day; the reaper removes it
  last-one-out (docker refuses while the peer is attached) under `netMu`,
  which must span lookup→`StartContainer` (endpoints attach at start).
- An init-done flag may only record an effect this node owns — init that
  writes an external service (the per-preview Postgres DB) must be revocable
  and re-runnable; a reaped database behind a permanent `init_done_at`
  crash-looped previews until the deploy was deleted.
- A worker's disk is only a cache and must be capped by default — the cache
  sweeper only runs with `--cache-max-artifact-bytes` set, and a worker left
  at `0` hydrates every preview it ever serves until hydration itself dies
  with ENOSPC; `--role worker` derives the cap from the data filesystem.

## Documentation upkeep

User-facing changes need a docs update in the same PR. Match the change to
the page:

- New/changed CLI flags or subcommands → `docs/reference/cli.md`.
- New/changed HTTP endpoints or request/response shapes → `docs/reference/api.md`.
- Config keys (env vars, `--in-memory`, etc.) → `docs/guide/configuration.md`.
- Install/setup steps → `docs/guide/install.md` or `docs/guide/quickstart.md`.

If a feature doesn't fit an existing page, add one under `docs/guide/` and
link it from `docs/.vitepress/config.ts`. Skip docs only for purely internal
refactors with no observable behavior change.
