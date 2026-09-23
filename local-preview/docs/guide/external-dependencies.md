# External dependencies

Real applications need more than one process and more than local state:
databases, search engines, caches, object stores. local-preview does not
orchestrate those — you run them once, and every preview's processes
connect to them. What local-preview adds per commit is the app's own
processes (an SSR frontend and an API backend), built content-addressed,
started on demand, and wired to your shared dependencies through manifest
`env`.

## The model

- **Shared dependencies, run by you.** Start the target repo's own deps
  stack (usually its docker compose) once. Previews of every commit talk
  to the same Postgres/Redis/search instances.
- **Per-commit app processes.** The manifest's `[frontend]` and
  `[backend]` build per commit as usual; with `run_image` set they run in
  stock runtime containers, so the server needs no Node or Python.
- **Wiring via env.** Dependency endpoints are plain `env` values. Use the
  `{hash}` placeholder where you want per-artifact isolation on a shared
  server (e.g. one Postgres database per backend artifact).
- **Networks.** The top-level `networks` key joins every containered
  process to your deps stack's docker network, so compose service names
  (`relational_db`, `cache`, …) resolve as-is.

## Worked example: onyx

[onyx](https://github.com/onyx-dot-app/onyx) is a Next.js standalone web
server plus a FastAPI API that expects Postgres, OpenSearch, Redis, MinIO,
and a model server. It carries no `preview.toml`, so the manifest lives
out-of-repo at `~/.config/preview/manifests/onyx.toml` (see
[local manifests](/reference/preview-toml#local-manifests-repos-you-can-t-change)).

1. **Start the deps** with onyx's own compose, pinning the project name so
   the network name is deterministic:

   ```bash
   cd onyx/deployment/docker_compose
   docker compose -p onyx -f docker-compose.yml -f docker-compose.dev.yml \
     up -d relational_db opensearch cache minio inference_model_server
   docker network ls   # → onyx_default
   ```

2. **Write the manifest** — the shape ships in this repo's docs; the key
   moves are `run` + `run_image` on both sides, `networks =
   ["onyx_default"]`, compose service names in `env`,
   `POSTGRES_DB = "preview_{hash}"` for isolation, and
   `INTERNAL_URL = "{backend_url}"` so the web server finds its API
   sibling over the per-deploy network.

3. **Deploy.** `preview deploy main --repo onyx`. The first build is cold
   (bun install, uv sync); subsequent commits reuse every unchanged side.
   Page traffic hits the Next server, `/api/*` reaches FastAPI with the
   prefix stripped (`strip_api_prefix = true`), and `/openapi.json`,
   `/auth/saml`, `/scim` pass through via `extra_routes`.

## On a shared server

"You run them once" holds on a laptop, where the compose stack is one command
away and you'd notice it stop. On a server nobody logs into, hand-started
containers don't survive a rebuild. The
[Terraform example](/guide/deploy-terraform#dependencies-live-beside-the-server)
takes the deps stack as a `compose_stacks` input and runs it as a systemd unit
with its data on the persistent volume, so the stack is part of the
deployment rather than something someone remembered to start.

## Shared-state caveats

Previews sharing dependencies share their state. `{hash}`-templated env
gives you per-artifact databases where you want isolation; anything not
templated is genuinely shared, including schema migrations different
commits may disagree about. Choose per value.

## Composed-server networking

Published container ports bind to the **docker host's** loopback. A
`preview serve` running directly on that host reaches them as-is; the
composed server must run with `network_mode: host` (see the note in
`compose.yaml`) so its loopback is the daemon host's.
