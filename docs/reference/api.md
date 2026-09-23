# REST API

All endpoints are JSON over HTTP, rooted at `/api/` on the apex host (any
Host that isn't a preview subdomain). Errors return a JSON body of the form
`{"error": "message"}`. JSON request bodies are capped at 1&nbsp;MiB; a
larger body is rejected with `413` before it is decoded. (Upload bodies are a
separate, larger cap — see [Uploads](#uploads).)

Responses larger than 1&nbsp;KiB are gzipped when the client sends
`Accept-Encoding: gzip`; smaller ones are sent plain (compressing them costs
more than it saves). Artifact downloads, pre-encoded responses, event
streams, and range responses are never re-encoded. Dashboard `/assets/`
files are served with `Cache-Control: immutable` (their names are
content-hashed); preview responses are served `no-cache` so a `--rebuild`
of the same commit is picked up on the next request via a `304`
revalidation.

## Authentication

By default the API is **open** — no credentials required. Starting the server
with [GitHub SSO](/guide/sso) (`--sso-github-client-id`) gates every `/api/`
endpoint behind either a browser **session cookie** or an
`Authorization: Bearer <github-pat>` header, both resolved to a GitHub identity
and checked against the allowlist.

Exempt regardless of SSO: `GET /api/health`, the `/api/auth/*` endpoints below,
`POST /api/webhooks/github` (HMAC-authenticated), and
`POST /api/repos/{repo}/uploads/*` (GitHub Actions OIDC).

`POST /api/deploys` and `GET /api/deploys/{id}` additionally accept a [GitHub
Actions OIDC token](/guide/uploads#authenticating-with-github-actions-oidc) as
their bearer credential — the two calls `preview upload … --oidc --deploy` makes
to deploy what it just uploaded and wait for it, without a session. Such a token
reaches only the repo whose registered source is the GitHub repository it was
minted for: naming another repo on the create gets `403`, and reading another
repo's deploy gets `404` rather than `403`, so the sequential IDs stay
unenumerable.

No other endpoint accepts one. A token says which workflow minted it, which
authorizes nothing until it is checked against a repo, so only routes that
identify one are eligible.

A gated request with
no valid credential gets `401`; a valid session on a state-changing
(`POST`/`PUT`/`PATCH`/`DELETE`) request whose `Origin` isn't the dashboard gets
`403` (CSRF defense). A signed-in account that isn't on the allowlist gets `403`
at the callback.

### `GET /api/auth/login`

Starts the OAuth flow: sets a short-lived state cookie and `302`s to GitHub.
`404` when SSO is disabled. Accepts an optional same-origin `?return_to=` path.

### `GET /api/auth/callback`

GitHub's redirect target. Verifies state, exchanges the code, applies the
allowlist (`403` on rejection), sets the session cookie, and `302`s home.

### `POST /api/auth/logout`

Deletes the current session and clears its cookie. `204`.

### `GET /api/auth/me`

Returns the signed-in identity, or `{"anonymous": true}` when SSO is disabled,
or `401` when SSO is on and the caller isn't signed in.

```json
{ "login": "octocat", "email": "octo@example.com", "avatar_url": "https://…" }
```

### `GET /api/auth/preview-grant`

Internal to the [preview handshake](/guide/sso#viewing-previews): mints a
single-use code and redirects back to a preview URL. Requires an apex session
(bounces through `login` otherwise) and only ever redirects to a preview host.

## Health

### `GET /api/health`

`preview_domain` is the base domain previews are served under, as resolved at
startup from [`--preview-domain`](/reference/cli#preview-serve) /
`$PREVIEW_DOMAIN`, or from the host of
[`--preview-base-url`](/guide/configuration#hosting-behind-a-proxy) when
that's set.

```json
{ "status": "ok", "version": "v0.1.0", "preview_domain": "preview.localhost" }
```

## Repos

### `POST /api/repos`

Registers a repository: the server mirror-clones `source` (a local path or
clone URL) **in the background** and responds immediately. `name` must be a
lowercase DNS label — it becomes the subdomain segment. `watch` and
`watch_branches` are optional and enable
[watching](/guide/triggers#watched-repos) from the start; `backfill: true`
additionally deploys the branch tips the repo already has.

Request:

```json
{ "name": "myapp", "source": "/home/me/code/myapp" }
```

Response: `202 Accepted` with the repo in status `cloning`. `400` for an
invalid name/source, `409` if the name is taken by a registered repo
(including one whose clone failed — delete it to retry). Only current
registrations conflict: a mirror clone left on disk by a deleted registration
is replaced, so deleting a repo always frees its name.

```json
{
  "id": 1,
  "name": "myapp",
  "source": "/home/me/code/myapp",
  "watch": false,
  "watch_branches": "",
  "status": "cloning",
  "created_at": "2026-01-01T00:00:00Z"
}
```

`status` progresses `cloning → ready` (or `failed`, with `error` carrying
the clone failure). Poll `GET /api/repos/{name}` to follow it; while
cloning, the response may also include `progress` — the transport's latest
human-readable progress line (e.g. `"Receiving objects: 42%"`), when the
source's server reports one. A repo accepts deploys only once `ready`
(`POST /api/deploys` answers `409` before that), and a watched repo starts
polling on its own as soon as the clone lands. Clones interrupted by a
server restart resume at the next startup.

### `GET /api/repos`

Returns all repos. `GET /api/repos/{name}` returns one (`404` if missing).

### `PATCH /api/repos/{name}`

Updates a repo's watch settings; fields absent from the body keep their
current value. `watch: true` polls the repo every `--poll-interval` and
deploys branch tips as they move; `watch_branches` narrows which branches as
comma-separated globs (`""` = all; a single `*` doesn't cross `/` but a `**`
segment does; a `!` prefix excludes, so `"!main"` is every branch but `main`,
and `"!gh-readonly-queue/**"` drops merge-queue refs). The filter also gates
[webhook](#post-api-webhooks-github) deploys, regardless of `watch`.

Switching watching on starts from the repo's current state — the tips that
already exist are recorded and not deployed. Send `backfill: true` to deploy
those too. The field is ignored unless this request is what turns watching
on, so changing `watch_branches` later never re-deploys existing tips.

Request:

```json
{ "watch": true, "watch_branches": "main,release/*" }
```

Response: `200 OK` with the updated repo. `400` for an invalid branch
pattern or an empty body, `404` if the name isn't registered.

### `DELETE /api/repos/{name}`

Unregisters a repository: stops its preview backends, then deletes its
deploys, artifacts, state directories, build logs, and mirror clone. The
name is immediately reusable.

Response: `204 No Content`. `404` if the name isn't registered.

## Deploys

### `POST /api/deploys`

Requests a deploy of `ref` (branch, tag, or full/abbreviated sha) in `repo`.
Idempotent per commit: re-posting a sha whose deploy is queued, building, or
ready returns the existing deploy; failed and evicted deploys are re-queued.
`rebuild: true` rebuilds artifacts even when cached.

Request:

```json
{ "repo": "myapp", "ref": "main", "rebuild": false }
```

Response: `202 Accepted` with the deploy. `404` for an unknown repo, `409`
for a repo that isn't `ready` (still cloning, or its clone failed), `400`
for an unresolvable ref.

Accepts a GitHub Actions OIDC token in place of a session — see
[Authentication](#authentication) — which is what makes `--oidc --deploy` work
from CI. Such a token gets `403` if `repo`'s registered source is not the
repository it was minted for.

```json
{
  "id": 7,
  "repo": "myapp",
  "sha": "a1b2c3d4…",
  "short_sha": "a1b2c3d",
  "ref": "main",
  "branch": "main",
  "author_name": "Ada Lovelace",
  "author_email": "ada@example.com",
  "created_by": "ada",
  "fe_hash": "…",
  "be_hash": "…",
  "status": "queued",
  "attempt_count": 0,
  "created_at": "2026-01-01T00:00:00Z",
  "updated_at": "2026-01-01T00:00:00Z"
}
```

`status` is one of `queued`, `building`, `ready`, `failed`, `evicted`. Ready
deploys additionally carry `preview_url` (built from the server's public
preview base — see [hosting behind a
proxy](/guide/configuration#hosting-behind-a-proxy)) and `process` (the live backend
state: `running` means warm, `starting` means a start is in flight, `idle`
means the process will start on the first request — processes start on
demand — and `crashed` means the last run or start attempt ended
unexpectedly), and `fe_process` (same states) when the frontend is a
[process](/reference/preview-toml#process-mode-frontends) rather than a
static bundle. Deploys with no backend never carry `process`.

A `crashed` side also carries the reason in `process_error` (or
`fe_process_error`): the exit status of a process that died on its own, or
the failure of a start attempt that never became healthy. It exists because
a service that stopped answering is otherwise indistinguishable from one
nobody has requested yet — both would read `idle`. Crashed is not a wedged
state: the next request starts the process like any other cold start, and
that attempt (or a [stop](#post-api-deploys-id-stop)) clears the reason.
Processes are shared per artifact hash, so every deploy on that hash reports
the same crash. Run logs outlive the process — `GET
/api/deploys/{id}/logs/run` has the output it died with.

Ready deploys whose manifest declares
[downloadable artifacts](/reference/preview-toml#artifacts-name) also carry
`artifacts`, one entry per name with a download URL per file:

```json
"artifacts": [
  {
    "name": "cli",
    "hash": "…",
    "status": "ready",
    "files": [
      { "name": "mycli", "size": 5242880, "url": "/api/deploys/7/artifacts/cli/mycli" }
    ]
  }
]
```

Artifacts build after the deploy itself turns ready — only the frontend and
backend gate readiness, so a slow artifact never delays the preview. Each
entry's `status` is `building` (its `files` list is still empty), `ready`,
or `failed` (an `error` field carries the build failure summary; the deploy
itself stays ready). Cached artifacts are `ready` immediately.

`ref` is what the deploy request asked for (empty when it was a sha);
`branch`, `author_name`, and `author_email` are captured from git when the
deploy is created. `branch` is best-effort: the requested ref when it is a
branch, otherwise the first branch whose tip is the commit — deploys of
commits that are no longer any branch's tip leave it empty. Deploys made
before these fields existed omit them.

`created_by` is the identity that *triggered* the deploy, an audit trail
distinct from the commit's git author: the signed-in GitHub login for a
session request, the GitHub Actions actor for a CI request authenticated with
an OIDC token, the push event's sender for a webhook, or empty for a deploy
the automatic [poller](/guide/triggers#watched-repos) created
(which has no triggering identity). A bearer-PAT request also records empty —
the token is verified but its identity is not currently threaded to the row.

### `GET /api/deploys`

Returns deploys, newest first. Narrow the list with any combination of:

| Query param | Match |
| --- | --- |
| `?repo=<name>` | exact repo name |
| `?branch=<name>` | exact branch name |
| `?author=<text>` | case-insensitive substring of the commit author's name or email |
| `?status=<status>` | exact build status — `queued`, `building`, `ready`, `failed`, or `evicted` — plus `crashed` (anything else is `400`); see below |
| `?q=<text>` | free-text search: a commit-sha prefix, or a case-insensitive substring of the repo, branch, ref, or author |
| `?limit=<n>` | at most the newest `n` deploys, applied after the other filters (a non-positive or non-integer value is `400`) |
| `?offset=<n>` | skip the newest `n` matches before returning any (a negative or non-integer value is `400`) |

`?status=crashed` is the one value that isn't a stored build status: it
matches ready deploys whose supervised process died (the state `process` /
`fe_process` report as `"crashed"`), resolved against the live supervisor at
request time. Those deploys are **excluded** from `?status=ready`, so the
two are disjoint and "ready" means a preview that can still serve.

Every response carries an **`X-Total-Count`** header: how many deploys match
the filter in total, ignoring `limit` and `offset`. That's what a pager needs
to know whether another page exists.

`?author=ada@example.com` is the "only my deployments" filter; `?q=` is the
search box behind the dashboard's deployments list, which pages through the
results with `?limit=` and `?offset=` so its poll stays bounded as deploys
accumulate.

Paging is by descending id, so a deploy created between two page fetches
shifts the window rather than corrupting it.

### `GET /api/deploys/{id}`

Returns one deploy (`404` if missing).

### `POST /api/deploys/{id}/stop`

Stops the deploy's supervised processes (backend and, for process-mode
frontends, frontend) without removing it. Because processes are shared per
artifact hash, any sibling deploy on the same hash stops too; each cold-starts
again on its next request. Build artifacts and the deploy row are untouched.

Response: `200 OK` with the deploy (its `process`/`fe_process` now read
`idle`). `404` if the deploy doesn't exist. A no-op — succeeds — when nothing
was running. Stopping also acknowledges a `crashed` side: the state and its
`process_error` clear.

### `DELETE /api/deploys/{id}`

Hard-deletes a deploy: it removes the row, then stops and garbage-collects any
build artifacts, backend state, and process bookkeeping that no surviving
deploy still references. Artifacts and state are content-addressed and shared,
so a hash another deploy still uses is left intact. The deploy's `short_sha`
subdomain is freed and re-deploying the commit builds fresh.

Response: `204 No Content`. `404` if the deploy doesn't exist. On-disk cleanup
is best-effort once the row is gone — leftovers are unreachable and only cost
disk, so failures are logged rather than surfaced.

### `GET /api/deploys/{id}/logs`

Returns a plain-text snapshot of the frontend, backend, and artifact build
logs. Deploys that share an artifact share that artifact's build log.

### `GET /api/deploys/{id}/logs/run`

Returns an incremental slice of a **run log** — the supervised process's
combined stdout+stderr (init output included), the docker-logs view of a
preview. Run logs outlive their process, so crash output stays readable
after an exit.

| Query param | Meaning |
| --- | --- |
| `side` | `be` (default) or `fe` (process-mode frontends only; `404` for static frontends) |
| `attempt` | The attempt whose bytes the client already has |
| `offset` | How many bytes of it the client already has |

```json
{
  "side": "be",
  "attempt": 3,
  "offset": 4096,
  "content": "listening on 127.0.0.1:41234\n…",
  "truncated": false,
  "process": "running"
}
```

Each response covers the **latest start attempt** of the deploy's artifact
(processes are shared per artifact, so deploys with the same hash share a
run log; `attempt` counts that artifact's starts, `0` meaning it never
started). Echo `attempt` and `offset` back to receive only bytes appended
since — the polling loop behind the dashboard's live tail. When the
requested attempt is stale (the process restarted) the response resets to a
tail of the new attempt's log, with `truncated: true` if history beyond the
last 256 KiB was skipped. `process` is the side's live state
(`idle`/`starting`/`running`/`crashed`) — a `crashed` tail is the output the
process died with.

### `GET /api/deploys/{id}/stats`

Live resource usage of the deploy's supervised processes — the docker-stats
view. A side the deploy doesn't have is `null`.

```json
{
  "backend": {
    "state": "running",
    "runtime": "host",
    "cpu_percent": 1.2,
    "memory_bytes": 5709824,
    "memory_limit_bytes": 32887226368,
    "started_at": "2026-08-06T22:09:49Z"
  },
  "frontend": null
}
```

`state` is always present; the sampled fields appear only while the process
runs. A `crashed` state adds `error` with the exit status or start failure
behind it. `cpu_percent` is percent of one core (docker-stats convention — it can
exceed 100 on multi-core use) and needs two samples, so it appears from the
second request onward; poll at a steady interval for stable readings.
`memory_limit_bytes` is the container's cgroup limit or, for host
processes, total system memory. `runtime` is `host` or `container`. Host
processes are sampled from `/proc` (process-group-wide, so forked children
count); on non-Linux hosts only container-mode processes report samples.

### `GET /api/deploys/{id}/exec`

Opens an interactive exec session inside the deploy's supervised container —
the transport behind `preview exec`. The request is a WebSocket upgrade; the
established socket carries a binary frame protocol multiplexing stdin,
stdout, stderr, terminal resizes, and the final exit code (see
`internal/execstream`). On a control node the session is forwarded to the
worker running the process.

Query parameters:

| Param | Description |
| --- | --- |
| `side` | `be` (default) or `fe` (process-mode frontend) |
| `cmd` | The argv, one repeated parameter per element (required) |
| `tty` | `1` allocates a pseudo-terminal |
| `stdin` | `1` attaches client input |
| `term` | Client's `TERM`, exported into the session when `tty=1` |

Everything checkable is rejected before the upgrade as a plain HTTP error:
`400` for a missing `cmd`, `404` for an unknown deploy or side, `409` when
the process isn't running (open the preview to start it, then retry).
Failures after the upgrade — including "this preview runs as a host process,
not a container" — arrive as an error frame on the socket.

### `GET /api/deploys/{id}/artifacts/{name}/{file}`

Downloads one file of a ready deploy's named
[downloadable artifact](/reference/preview-toml#artifacts-name), as
`application/octet-stream` with an attachment disposition. The URL is what
the deploy's `artifacts` field lists. `404` for an unknown deploy,
artifact, or file; `409` while the deploy isn't ready, while the artifact
is still building (they build after the deploy turns ready), or when its
build failed.

## Uploads

Publish a CI-built side into the content-addressed store so a deploy serves it
without rebuilding — see [uploading prebuilt artifacts](/guide/uploads). The
server is the authority on the hash: it resolves `ref → sha`, reads the
manifest at that commit, and computes the same content-address a build would
target, then lands the uploaded bytes in that slot. An upload touches no deploy
row — it primes the store, and any deploy of a commit sharing the hash then
skips that build.

### `POST /api/repos/{repo}/uploads/frontend`
### `POST /api/repos/{repo}/uploads/backend`
### `POST /api/repos/{repo}/uploads/artifacts/{name}`

The request body is a **tar** stream, optionally gzip-compressed (both are
accepted; the server sniffs). Its entries are relative to the published root:
the `dist` tree (or the built `path` tree for a process-mode frontend) for
`frontend`, the built `backend.path` tree for `backend`, and the artifact's
declared `files` at their `path`-relative locations for an artifact.

The body is size-capped: an upload streaming more than `--max-upload-bytes`
(default 2 GiB) of compressed body is rejected with `413`, and extraction aborts
with `413` if the decompressed tar exceeds the same cap — a gzip bomb is stopped
before it fills the disk. Raise the limit for larger artifacts (see
[configuration](/guide/configuration#flags)).

| Query param | Meaning |
| --- | --- |
| `ref` | Required — the branch, tag, or (abbreviated) sha to resolve and hash |
| `overwrite` | Optional bool; replace an already-present artifact instead of no-op'ing |

Response: `200 OK`. `published` is `false` when the artifact was already
present and `overwrite` wasn't set (an idempotent no-op).

```json
{
  "sha": "a1b2c3d4…",
  "short_sha": "a1b2c3d",
  "side": "frontend",
  "hash": "…",
  "published": true
}
```

An artifact upload additionally echoes the published files:

```json
{
  "sha": "a1b2c3d4…", "short_sha": "a1b2c3d",
  "side": "artifact", "name": "cli", "hash": "…", "published": true,
  "files": [ { "name": "mycli", "size": 5242880 } ]
}
```

Errors: `404` for an unknown repo or an artifact name the manifest doesn't
declare; `409` if the repo isn't `ready` (still cloning, or its clone failed);
`413` if the request body or its decompressed size exceeds `--max-upload-bytes`;
`400` for a missing/unresolvable `ref`, a manifest error, a malformed tar, or a
tar missing a declared artifact file.

#### Authentication

By default there is no authentication — the server trusts the uploader's bytes
for the commit exactly as it trusts its own build output; run it where only
your CI can reach it.

Starting the server with `--github-oidc-audience` (see
[configuration](/guide/configuration#flags)) turns the upload endpoints
into authenticated ones: every request must carry an
`Authorization: Bearer <token>` header holding a [GitHub Actions OIDC
token](/guide/uploads#authenticating-with-github-actions-oidc). The server
verifies its signature, issuer, audience, and expiry, then authorizes it only
against the repo whose registered `source` is the same GitHub repository the
token's `repository` claim names. Additional statuses then apply:

- `401` — no bearer token was presented.
- `403` — the token is invalid/expired, or its `repository` doesn't match the
  target repo's `source`.

## Storage & retention

### `GET /api/storage`

Reports the instance's disk usage, by category and by repo. Sizes are
measured by walking the data directory on each call.

```json
{
  "total_bytes": 123456789,
  "artifacts_bytes": 100000000,
  "state_bytes": 2000000,
  "logs_bytes": 400000,
  "mirror_bytes": 20000000,
  "tmp_bytes": 0,
  "db_bytes": 1056789,
  "durable_tier_configured": true,
  "durable_bytes": 340000000,
  "repos": [
    {
      "repo": "myapp",
      "artifacts_bytes": 100000000,
      "state_bytes": 2000000,
      "logs_bytes": 400000,
      "mirror_bytes": 20000000,
      "total_bytes": 122400000,
      "deploys": 12,
      "evicted_deploys": 3
    }
  ]
}
```

`deploys` counts a repo's non-evicted deploys; `evicted_deploys` the rows
kept as history after their artifacts were reclaimed. `db_bytes` is `0` for
`--in-memory` instances.

`artifacts_bytes` measures artifacts **resident on local disk**. With the
[artifact tier](/guide/configuration#artifact-tier-s3) configured,
`durable_tier_configured` is `true` and local disk is a cache: artifacts are
swept off it above the resident cap and re-hydrated on the next request, so a
falling `artifacts_bytes` is cache eviction, not data loss. `durable_bytes` is
the tier's total footprint (`0` if unknown or no tier is configured) and is
**not** included in `total_bytes`, which measures only local disk.

### `GET /api/retention`

Returns the [retention policy](/guide/configuration#retention-garbage-collection).
Both limits default to `0` (unlimited — automatic eviction disabled).

```json
{ "max_deploys_per_repo": 10, "max_age_days": 30 }
```

### `PUT /api/retention`

Replaces the retention policy. `max_deploys_per_repo` keeps at most N
non-evicted deploys per repo, newest first; `max_age_days` evicts deploys
created more than N days ago; `0` disables a limit. The policy takes effect
on the next sweep — hourly, or immediately via `POST /api/gc` — so saving
never evicts by itself.

Response: `200 OK` with the saved policy. `400` for a negative limit or
invalid JSON.

### `GET /api/warm`

Returns the [warm-process policy](/guide/configuration#warm-previews):
`max_warm` is the soft warm *target* per serving node — bursts above it are
served in full; only genuinely idle least-recently-used processes are pruned
back (`0` = unlimited; a process-mode deploy counts as two — frontend +
backend). `min_warm` is the floor: that many most-recent processes never
idle out — **fleet-wide** on a control node (the heartbeat loop ranks the
fleet's processes by recency and hands each worker its share, so `12`
protects 12 processes total however many workers exist), and simply local
on a single node. A `--min-warm-window` gates the floor by wall clock:
outside active hours it is `0` and the fleet can drain to zero.
`idle_timeout_seconds` overrides every manifest's `idle_timeout` when `> 0`
(`0` = per-manifest values, default 30m).

```json
{ "max_warm": 12, "min_warm": 0, "idle_timeout_seconds": 0 }
```

### `PUT /api/warm`

Replaces the warm policy. Applies without a restart: the local reaper
enforces both values on its next tick — the idle override governs
already-running processes too — and on a control node the fleet's heartbeat
loop pushes them to every worker (re-pushing after worker reboots). Saved
values override the `--max-warm` flag and manifest `idle_timeout`s from then
on.

Response: `200 OK` with the saved policy. `400` for a negative value or a
`min_warm` above a non-zero `max_warm`.

### `POST /api/gc`

Runs one retention sweep immediately and reports what it evicted. Evicting
marks the deploy `evicted` (the row survives as history; its subdomain
answers with a "redeploy to rebuild" page) and reclaims the artifacts,
backend state, and logs that no surviving deploy shares. Stale `tmp/`
staging leftovers are collected on every run, so the endpoint is useful even
with retention disabled.

Never evicted: queued/building deploys, each repo's newest ready deploy, and
deploys a branch alias routes to.

```json
{
  "policy": { "max_deploys_per_repo": 10, "max_age_days": 30 },
  "evicted": [
    { "id": 7, "repo": "myapp", "short_sha": "a1b2c3d", "branch": "main" }
  ],
  "freed_bytes": 52428800
}
```

## Statistics

### `GET /api/stats`

Instance-wide statistics for the dashboard's **Statistics** view.

- **`startup`** — cold-start latency, in seconds, over the last 30 days,
  derived from the `start_attempt`→`healthy` process-event trail (each
  successful start is one sample). Percentile fields are omitted when `count`
  is `0`.
- **`hits`** — cumulative warm-vs-cold outcomes (per-worker counters, summed
  across the fleet on a control node). A warm hit is a preview request served
  by an already-*healthy* process; a request that launches a process — or
  joins an in-flight start and waits it out — counts cold, so a page load
  firing many asset requests during one cold start doesn't inflate the ratio.
  `warm_ratio` is `warm / (warm + cold)`, omitted until there has been at
  least one request. Worker restarts don't reset fleet totals (the control
  node banks each worker's history), but a control-node restart does.
- **`runtime`** — the live serving footprint: `workers` fresh serving nodes
  (`1` on a single node), `running` warm processes right now, and `capacity`
  the total bounded warm-process capacity (`0` = unlimited).

```json
{
  "startup": { "count": 42, "p50_seconds": 1.8, "p90_seconds": 4.2, "p99_seconds": 9.1 },
  "hits": { "warm": 152, "cold": 20, "warm_ratio": 0.883 },
  "runtime": { "workers": 3, "running": 7, "capacity": 30 }
}
```

## Webhooks

### `POST /api/webhooks/github`

Receives GitHub webhook deliveries and deploys pushed commits. Enabled only
when the server is started with a webhook secret; see
[GitHub webhooks](/guide/triggers#github-webhooks) for setup.

Every delivery must carry a valid `X-Hub-Signature-256` (HMAC-SHA256 of the
raw body with the shared secret); anything else is `403`. The payload's
repository URLs are matched against registered repo sources — https, ssh,
and git forms of the same repository compare equal.

Responses:

- `202 Accepted` with the deploy — a branch push that matched a registered
  repo. The pushed head sha is deployed (exactly what the event named, even
  if the branch has moved since).
- `200 OK` `{"status": "ignored", "reason": "…"}` — deliveries that are
  deliberately skipped: tag pushes, branch deletions, non-push events, and
  pushes to a branch the repo's `watch_branches` filter excludes. `ping`
  answers `{"status": "pong"}`.
- `404` — no registered repo matches the payload's repository.
- `503` — the server has no webhook secret configured.
