# preview.toml

The contract a target repository declares at its root so the orchestrator can
build and run it. It is always read from the committed tree of the deployed
sha (`git show <sha>:preview.toml`) — never from a checkout — so old commits
build with the manifest they shipped with.

Unknown keys are a hard error, so typos fail the deploy instead of silently
changing nothing.

## Example

The shape this repository itself uses (Go backend at the root, Vite frontend
in `web/`):

```toml
[frontend]
path  = "web"                    # hash root + build cwd (repo-relative)
build = [["npm", "ci"], ["npm", "run", "build"]]
dist  = "dist"                   # build output, relative to path

[backend]
path          = "."              # hash root + build cwd
exclude       = ["docs/", "*.md", ".github/"]   # hashing only
build         = [["go", "build", "-o", "bin/server", "."]]
run           = ["./bin/server", "--addr", ":{port}", "--data-dir", "{state_dir}"]
health_path   = "/api/health"
start_timeout = "20s"            # optional

[artifacts.cli]                  # a downloadable binary, never run
path    = "."
exclude = ["docs/", "*.md", ".github/"]
build   = [["go", "build", "-o", "bin/preview", "."]]
files   = ["bin/preview"]
```

## `[frontend]`

| Key | Required | Description |
| --- | --- | --- |
| `path` | yes | Subtree that defines the frontend hash; build commands run with this as their working directory |
| `build` | yes | Build steps as argv arrays (no shell; use `["sh", "-c", "..."]` explicitly if you need one) |
| `dist` | static mode | Directory the build produces, relative to `path`; published as the static bundle |
| `image` | no | Container image the build steps run in (see [Build images](#build-images)) |
| `run` | process mode | Server command; the frontend becomes a supervised process receiving all non-API traffic (see [Process-mode frontends](#process-mode-frontends)) |
| `health_path` | with `run` | Path polled until it returns 200 after start |
| `start_timeout` | no | How long the process gets to become healthy (default `20s`) |
| `idle_timeout` | no | Idle period without proxied requests before the process is stopped (default `30m`) |
| `run_image` | no | Container image the server process runs in (see [Runtime images](#runtime-images)) |
| `env` | no | Environment variables for the process (see [Env placeholders](#env-placeholders)) |

A static bundle must be deploy-agnostic: base path `/`, relative `/api`
calls, no per-deploy values baked in at build time. One bundle is served
under every subdomain that references it.

### Process-mode frontends

With `run` set the frontend is a server (SSR apps like Next.js standalone),
not a static bundle: the whole built `path` tree is published as the
artifact, the command runs from it on demand exactly like a backend, and
the proxy forwards every non-API path to it. `dist` is unused. The process
must bind `{port}` — on all interfaces (`0.0.0.0`) when it runs containered
(`run_image`, or the [devcontainer default](#devcontainer-default)).

## `[backend]`

| Key | Required | Description |
| --- | --- | --- |
| `path` | yes | Subtree that defines the backend hash (minus `frontend.path` and `exclude`); build cwd; the built subtree becomes the artifact |
| `exclude` | no | Patterns removed from the backend hash: `dir/` prefixes, or globs matched against the full path and basename (`*.md`) |
| `build` | yes | Build steps as argv arrays |
| `init` | no | Idempotent steps run once per backend artifact before its first `run`, re-run after a failed start that skipped them (see [Init commands](#init-commands)) |
| `run` | yes | Command that starts the server, executed with the artifact directory as cwd |
| `health_path` | yes | Path polled until it returns 200 after start |
| `start_timeout` | no | How long the process gets to become healthy (default `20s`) |
| `idle_timeout` | no | Idle period without proxied requests before the process is stopped (default `30m`) |
| `init_timeout` | no | Total time budget for all `init` steps (default `2m`) |
| `image` | no | Container image the build steps (not the server) run in (see [Build images](#build-images)) |
| `run_image` | no | Container image the server process runs in (see [Runtime images](#runtime-images)) |
| `env` | no | Environment variables for the process (see [Env placeholders](#env-placeholders)) |
| `strip_api_prefix` | no | Remove the leading `/api` before proxying, for backends whose routes aren't mounted under `/api` |
| `extra_routes` | no | Additional path prefixes proxied to the backend unstripped (e.g. `["/openapi.json", "/auth/saml"]`) |

### Run templating

Two placeholders are substituted into every `run` argv element:

| Placeholder | Value |
| --- | --- |
| `{port}` | The port assigned to this process — bind `0.0.0.0:{port}`, which works on every runtime (only a pure host process may narrow to `127.0.0.1`; containered runs — `run_image` or the [devcontainer default](#devcontainer-default) — need all interfaces) |
| `{state_dir}` | The artifact's mutable state directory (see [state lineage](/guide/concepts#state-follows-git-lineage)) |

### Env placeholders

`env` values are expanded at process start (never at hash time — the
literal string is what's hashed):

| Placeholder | Value | Sides |
| --- | --- | --- |
| `{port}` | The side's assigned port | both |
| `{state_dir}` | The backend's state directory | backend |
| `{repo}` | The registered repo name | both |
| `{hash}` | The side's own 12-char artifact hash | both |
| `{backend_url}` | Base URL the frontend uses to reach this deploy's backend | frontend |
| `{secret:NAME}` | The value of `PREVIEW_SECRET_NAME` from the serving node's environment | both |

`{hash}` is per-artifact, not per-commit: processes are shared across
deploys with the same hash (the same reason state dirs fork per artifact),
so `POSTGRES_DB = "preview_{hash}"` gives isolation that follows artifact
identity. `{backend_url}` requires `frontend.run` and both sides on the
same runtime (both `run_image` or neither).

`{secret:NAME}` keeps credentials out of the manifest itself: the value is
read at process start from the orchestrator's own environment, namespaced as
`PREVIEW_SECRET_NAME` — put the real value in that variable on every serving
node (control and workers), typically via the boot-time env file. The prefix
is deliberate: manifests are repo-controlled, so unrestricted pass-through
would let a manifest read control-plane env like the SSO client secret. A
referenced secret that is unset fails the start loudly rather than exporting
an empty credential. The stored run config keeps the placeholder, so the
secret value never lands in the database or crosses the worker wire.

::: warning `{state_dir}` and worker tiers
State directories are node-local. On a single node (`--role=all`) they fork
along git lineage, but a worker tier (`--role=control` + workers) gives each
worker a fresh, empty state dir per artifact — no lineage forking, and state
does not follow a preview across workers. Repos serving from a worker tier
should keep per-commit state in external services keyed by `{hash}` (as in
the `POSTGRES_DB` example above) rather than in `{state_dir}`.
:::

### Init commands

One-time setup — schema migrations, seed data — that shouldn't re-run on
every cold start:

```toml
[backend]
init = [["alembic", "upgrade", "head"]]
run  = ["uvicorn", "app:app", "--port", "{port}"]
```

`init` steps run **once per backend artifact**, sequentially, with the
artifact directory as cwd, after the artifact's state dir is provisioned and
before its first `run`. Because artifacts are immutable and each one owns its
state dir exclusively, later cold starts of the same artifact skip init
entirely. A new backend hash is a new artifact, so
its init runs again — against state [freshly forked](/guide/concepts#state-follows-git-lineage)
from its ancestor, which is exactly when migrations have real work to do.

No port is assigned until the server starts, so `{port}` in an init step is
rejected at parse time and only `{state_dir}` is templated. Init sees the
backend's `env` with `{state_dir}`, `{repo}`, and `{hash}` expanded;
variables whose values reference `{port}` are omitted entirely rather than
given a bogus value. All steps share one `init_timeout` budget (default
`2m`), separate from `start_timeout` — the health-check window needs no
padding for worst-case migration time.

With `run_image` set, each init step runs to completion in a one-shot
container using the same image, mounts, and external `networks` as the
server container, so migrations reach whatever the server will.

**Init steps must be idempotent**, because "once" is not a promise: a start
that skipped init and then failed — the process exited before it went
healthy, or never answered its health check — *revokes* the recorded success,
and the next start runs init again from the first step.

The reason is that init's effects need not live in the state dir this system
owns. A step that creates a per-preview database on a shared Postgres, a
bucket prefix, or a queue has written to a service that can lose it —
someone reaps old databases, an external environment is rebuilt — while the
"done" flag keeps claiming the effect exists. Without revocation such a
preview crash-loops forever, since nothing re-runs the step that would
recreate it. Writing each step to check-then-create (`CREATE DATABASE` only
if absent, `alembic upgrade head`, `mkdir -p`) makes the repair free, and
costs at most one extra idempotent init per failed cold start. An init that
already ran in the failing attempt is never revoked — the flag is only
withdrawn when init was skipped.

If a step fails or times out, the start attempt fails and the **next** start
retries init from the first step — nothing is recorded until every step
exits 0. Steps should therefore tolerate a partially-applied predecessor
(alembic and most migration tools already do). Init output is written to the
head of the run log, so the first cold start's log begins with migration
output.

## `[artifacts.<name>]`

Prebuilt downloadable outputs — e.g. a CLI that ships from the same
monorepo as the backend and frontend, built per commit and downloaded
instead of run. Declare any number of named artifacts:

```toml
[artifacts.cli]
path  = "cli"                    # hash root + build cwd
build = [["go", "build", "-o", "bin/mycli", "."]]
files = ["bin/mycli"]            # build outputs published for download
```

| Key | Required | Description |
| --- | --- | --- |
| `path` | yes | Subtree that defines the artifact hash (minus `frontend.path` and `exclude` — the same partition rule as `[backend]`); build commands run with this as their working directory |
| `exclude` | no | Patterns removed from the hash, same syntax as `backend.exclude` |
| `build` | yes | Build steps as argv arrays |
| `files` | yes | Files the build produces, relative to `path`, published for download. Served flat by base name, so base names must be unique within one artifact |
| `image` | no | Container image the build steps run in (see [Build images](#build-images)) |

The name must be a lowercase label (letters, digits, inner hyphens) — it
appears in download URLs. Artifacts are content-addressed exactly like the
two sides: a commit that doesn't touch an artifact's partition (or its
manifest section) reuses the cached build. Nothing is ever run — there is
no run command, health check, state dir, or env.

Artifacts never gate the preview: the frontend and backend builds make the
deploy ready, then artifacts build on afterwards, surfacing per-artifact
`building`/`ready`/`failed` states in the
[API](/reference/api#post-api-deploys) and dashboard. An artifact build
failure (including a build that doesn't produce every declared file) fails
only that artifact — the error lands on the artifact, its build log has the
detail, and the deploy stays ready.

A build matrix is just more build steps and more files — cross-compile once
per platform and declare every output:

```toml
[artifacts.cli]
path  = "cli"
build = [
  ["env", "CGO_ENABLED=0", "GOOS=linux",  "GOARCH=amd64", "go", "build", "-o", "bin/mycli-linux-amd64", "."],
  ["env", "CGO_ENABLED=0", "GOOS=darwin", "GOARCH=arm64", "go", "build", "-o", "bin/mycli-darwin-arm64", "."],
]
files = ["bin/mycli-linux-amd64", "bin/mycli-darwin-arm64"]
```

Ready deploys carry their artifacts in the [API](/reference/api#deploys)
with a per-file download URL (files listed sorted by name), the dashboard
shows one download button per artifact — a direct download for a
single-file artifact, a dropdown menu of its files for a matrix like the
one above — and `preview deploy` prints the URLs alongside the preview
URL.

## Runtime images

With `run_image` set on a side, its server process runs inside that stock
container image — the artifact (and the backend's state dir) bind-mounted
at their host paths, the port published to the host loopback — instead of
directly on the server's host. This is how apps whose runtime the server
doesn't have (Node, Python, …) run under the toolchain-less composed
server. Unlike build `image` steps there is no host fallback: `run_image`
with no reachable daemon fails the start with a clear error. (A side with
no `run_image` still defaults into the repo's devcontainer when the commit
carries one — see [Devcontainer default](#devcontainer-default), which
*does* fall back to the host.)

A process-mode frontend and its backend share a per-deploy bridge network,
created by whichever side starts first and removed when the last one exits:
the backend is reachable from the frontend container by the DNS alias
`backend`, which is what `{backend_url}` resolves to. Both containers also
join every network listed in the top-level `networks` key:

```toml
# Existing external docker networks every containered process joins — how
# previews reach shared dependencies you run yourself (e.g. a target
# repo's own deps compose). Not part of any artifact hash.
networks = ["onyx_default"]
```

See [external dependencies](/guide/external-dependencies) for the full
workflow.

## Alternate locations

Sources are tried in order at each deployed commit; the standalone server
looks for:

1. `preview.toml` at the repo root
2. a `[previews]` table in `.kanban.toml` (the agentic-kanban convention)
3. a [local manifest](#local-manifests-repos-you-can-t-change) on the
   server's machine

Applications embedding the orchestrator can replace the in-repo list via
`Options.ManifestSources` — typically a table inside their own config file,
so target repos need only one file. agentic-kanban, for example, accepts the
same schema under a `[previews]` table in `.kanban.toml`:

```toml
[previews.frontend]
path  = "web"
build = [["npm", "ci"], ["npm", "run", "build"]]
dist  = "dist"

[previews.backend]
path        = "."
build       = [["go", "build", "-o", "bin/server", "."]]
run         = ["./bin/server", "serve", "--addr", "127.0.0.1:{port}", "--data-dir", "{state_dir}"]
health_path = "/api/health"
```

In-repo sources are always read from the committed tree. Artifact hashes
cover the parsed manifest, not its location, so moving unchanged config
between sources doesn't invalidate caches.

## Local manifests (repos you can't change)

To onboard a repository without pushing anything upstream, put its manifest
(plain `preview.toml` schema, no table) on the server's machine at:

```
~/.config/preview/manifests/<repo>.toml
```

where `<repo>` is the name the repo was registered under. The directory
root honors `$PREVIEW_CONFIG_DIR` and `$XDG_CONFIG_HOME` (see
[configuration](/guide/configuration#local-manifests)).

The local file is the last source tried — a committed manifest always wins.
Unlike in-repo sources it is read from disk at build time, not from the
deployed commit, so every commit builds with the current local manifest and
edits apply to the next build (use rebuild to pick them up for an
already-ready deploy; artifact hashes cover the parsed manifest, so changes
rebuild only the sides they touch).

## Build images

With `image` set on a side, its build steps run inside one-shot containers
(the extracted commit tree bind-mounted, output streamed into the build
log) instead of on the server's host — reproducible toolchains that don't
care what the host has installed, including when the server itself runs in
a toolchain-less container. The image is part of the manifest section, so
it feeds the artifact hash: bumping the image rebuilds.

Requires a reachable Docker daemon (`$DOCKER_HOST`, `/var/run/docker.sock`,
or the rootless per-user socket). If none is reachable, the build logs a
warning and falls back to host execution. Toolchain caches (npm, Go) are
kept warm on a named volume per image. Embedding applications with their
own runners (agentic-kanban's devcontainer builds) honor `image` when set —
an explicit image beats environment discovery. The standalone server does
its own discovery for sides without one: the repo's devcontainer, below.

## Devcontainer default

Sides that pin no `image`/`run_image` don't go straight to the host: when
the deployed commit carries a devcontainer config
(`.devcontainer/devcontainer.json` or `.devcontainer.json`, read from the
committed tree like the manifest), its image becomes the default
environment for both build steps and server processes. Per side the
precedence is explicit `image`/`run_image` → devcontainer → host.

What's honored:

- `image` — Dockerfile-built devcontainers aren't supported and fall back
  to the host with a note in the build log.
- `mounts` of `type=volume` — the devcontainer's named cache volumes are
  mounted into build and run containers, so preview builds share the warm
  toolchain caches your interactive sandbox already populated. Bind mounts
  reference paths personal to a developer's machine and are dropped.
- `remoteUser` / `containerUser` — only to derive the `HOME` steps run
  with, so toolchains (go, npm) resolve their caches onto the mounted
  volume targets.
- `${localEnv:VAR}` and `${localEnv:VAR:default}` are expanded against the
  server's environment; entries still referencing other variables
  (`${localWorkspaceFolder}`, …) are skipped.

Everything else — features, `runArgs`, `containerEnv`, lifecycle
commands — is ignored: previews want the toolchain, not the sandbox.

Unlike `run_image` there is always a host fallback: with no reachable
docker daemon, builds and runs execute on the host and the logs say why. A
server must bind `0.0.0.0:{port}` to work under the devcontainer runtime
(which also works on the host — see [Run templating](#run-templating)).

The resolved config (image, cache volumes, home) feeds the artifact hash
of every side that would use it, so bumping the devcontainer image
rebuilds — while edits to the ignored keys (lifecycle commands, bind
mounts) rebuild nothing.

Opt out per side with an explicit `image`/`run_image`, or for the whole
repo with the top-level key:

```toml
devcontainer = false
```

## Hashing caveats

Each side's hash covers its declared partition **plus its own manifest
section** — editing a build command, `env` value, or `run_image` rebuilds
that side — plus the resolved [devcontainer](#devcontainer-default) when
the side would use it. Files outside the declared partition don't bust the
cache; if your backend depends on files across the whole tree, use
`path = "."`. The top-level `networks` list is the one run-time-only
exception: it feeds no hash, and changes apply to the next process start
after a redeploy.
