# CLI

The binary is both the server and a client for it. Client subcommands talk to
a running `preview serve` over HTTP; point them at a non-default server with
`--server`, `$PREVIEW_URL`, or — persistently — [`preview
configure`](#preview-configure).

## `preview serve`

Start the orchestrator.

| Flag | Default | Description |
| --- | --- | --- |
| `--addr` | `:8080` | HTTP listen address (`$PREVIEW_ADDR` sets the default; prefer the env in the container image — its healthcheck derives the probe port from it) |
| `--data-dir` | (XDG) | Override the data directory |
| `--in-memory` | `false` | Ephemeral in-memory SQLite |
| `--preview-domain` | `preview.localhost` | Base domain previews are served under |
| `--preview-base-url` | (derived) | Public base URL of previews, e.g. `https://preview.example.com` — sets the scheme, domain, and port of generated preview URLs when the server sits behind a proxy |
| `--build-concurrency` | `2` | Number of deploys built in parallel |
| `--poll-interval` | `1m` | How often watched repos are fetched for new commits (`0` disables watching) |
| `--min-warm-window` | (always active) | Active hours for the [min-warm floor](/reference/api#get-apiwarm), e.g. `"Mon-Fri 08:00-18:00 America/Chicago"` (tz optional, default UTC). Outside the window the floor is `0`, so idle previews drain and the worker fleet can scale to zero; demand-driven scale-out is unaffected (default: `$PREVIEW_MIN_WARM_WINDOW`) |
| `--github-webhook-secret` | `$PREVIEW_GITHUB_WEBHOOK_SECRET` | Shared secret validating GitHub webhook deliveries (empty disables the endpoint) |
| `--max-upload-bytes` | `$PREVIEW_MAX_UPLOAD_BYTES` (`2147483648`, 2 GiB) | Maximum bytes a CI [upload](/reference/api#uploads) may stream: the compressed request body is rejected with `413` above it, and extraction aborts if the decompressed tar exceeds it (guards against a gzip bomb filling the disk). Raise it for larger legitimate artifacts; `0` disables both caps |
| `--sso-github-client-id` | `$PREVIEW_SSO_GITHUB_CLIENT_ID` | GitHub OAuth App client ID; setting it turns on [SSO login](/guide/sso) for the dashboard, API, and previews |
| `--sso-github-client-secret` | `$PREVIEW_SSO_GITHUB_CLIENT_SECRET` | GitHub OAuth App client secret |
| `--sso-callback-url` | `$PREVIEW_SSO_CALLBACK_URL` | Public OAuth callback URL; must match the OAuth App exactly |
| `--sso-allowed-org` / `--sso-allowed-team` | `$PREVIEW_SSO_ALLOWED_ORG` / `_TEAM` | Allow a GitHub org (optionally one team within it) |
| `--sso-allowed-logins` / `--sso-allowed-emails` | `$PREVIEW_SSO_ALLOWED_LOGINS` / `_EMAILS` | Comma-separated usernames / verified emails |
| `--s3-endpoint` / `--s3-bucket` | `$PREVIEW_S3_ENDPOINT` / `$PREVIEW_S3_BUCKET` | S3/MinIO endpoint and bucket; both enable the [artifact tier](/guide/configuration#artifact-tier-s3) (built artifacts persist and hydrate instead of rebuilding after eviction) |
| `--s3-prefix` / `--s3-region` | `$PREVIEW_S3_PREFIX` / `$PREVIEW_S3_REGION` | Optional key prefix and bucket region |
| `--s3-access-key` / `--s3-secret-key` | `$PREVIEW_S3_ACCESS_KEY` / `$PREVIEW_S3_SECRET_KEY` | Static artifact-tier keypair, for an endpoint with no ambient identity. Set both or neither; unset resolves the AWS environment or instance role |
| `--s3-use-ssl` | `true` | Use TLS for the endpoint (set `false` for local MinIO over http) |
| `--cache-max-artifact-bytes` | `$PREVIEW_CACHE_MAX_ARTIFACT_BYTES` (`0`) | Soft cap on resident (local-disk) artifact bytes; above it the coldest artifacts are swept back to the durable tier and re-hydrated on serve. Requires the artifact tier; `0` disables cache eviction and keeps every artifact resident — except under `--role worker`, where `0` defaults the cap to half the filesystem holding the data dir (a worker's disk is only a cache) |
| `--reserved-upstream` | `$PREVIEW_RESERVED_UPSTREAMS` (comma-separated) | Route a fixed host under the preview domain to a plain upstream, as `<label>=host:port` (e.g. `app=127.0.0.1:3100` serves `app.<preview-domain>`). Repeatable. The host is reverse-proxied wholesale — behind the same [SSO gate](/guide/sso) as previews, but outside the deploy/build machinery — for an always-on companion service that must live under the preview domain (wildcard cert, login gate) yet isn't a per-commit preview. A reserved label shadows any preview of the same label |
| `--onyx-auth-upstream` | `$PREVIEW_ONYX_AUTH_UPSTREAM` | Enable onyx single-sign-on for previews. The value is the `--reserved-upstream` label of the one canonical onyx host that owns the Google OAuth client (e.g. `app`). A preview browser without an onyx session is bounced there to log in; the session — a stateless JWT signed by the shared `USER_AUTH_SECRET` — is widened to the whole preview domain so every preview validates it locally. Must name an existing reserved upstream. Empty disables it |
| `--onyx-auth-cookie` | `$PREVIEW_ONYX_AUTH_COOKIE` (`fastapiusersauth`) | onyx session cookie the proxy watches for (presence only — the JWT is validated by each onyx backend, not the proxy). Matches onyx's `AUTH_COOKIE_NAME` when overridden |
| `--role` | `all` | Serving role: `all` (single node), `control` (route previews to a worker tier), or `worker` (supervise processes for a control node) |
| `--worker-secret` | `$PREVIEW_WORKER_SECRET` | Shared secret authenticating the internal worker API in both directions |
| `--worker-listen` | — | Private address to expose the internal worker API on, e.g. `:9100` (roles `worker`/`all`). **Must not be internet/ALB-reachable.** Requires `--worker-secret` |
| `--worker-endpoint` | `$PREVIEW_WORKER_ENDPOINT` | Control node only: a worker's private worker-API base URL, e.g. `http://10.0.1.5:9100` |
| `--worker-endpoints` | `$PREVIEW_WORKER_ENDPOINTS` | Control node only: comma-separated worker-API base URLs forming the fleet; combined with `--worker-endpoint` |
| `--worker-host` | (the endpoint host) | Control node only: the routable host a single worker's preview processes are reached on (per-endpoint host is derived from the URL when several are given) |
| `--control-listen` | `$PREVIEW_CONTROL_LISTEN` | Control node only: private address to expose the worker-registration API on, e.g. `:9101`. Lets workers self-register instead of being hand-listed via `--worker-endpoint(s)`, so an autoscaled worker joins the fleet on boot (the fleet may even start empty and fill on demand). **Must not be internet/ALB-reachable.** Requires `--worker-secret` |
| `--control-endpoint` | `$PREVIEW_CONTROL_ENDPOINT` | Worker node only: the control node's `--control-listen` base URL, e.g. `http://10.0.1.1:9101`. The worker registers itself there on boot and every ~20s, and deregisters on shutdown; empty disables self-registration |
| `--worker-advertise` | `$PREVIEW_WORKER_ADVERTISE` | Worker node only: this worker's own worker-API base URL the control node should dial back, e.g. `http://10.0.1.5:9100`. Required with `--control-endpoint` unless it can be derived from a host-qualified `--worker-listen` |
| `--worker-instance-id` | `$PREVIEW_WORKER_INSTANCE_ID` | Worker node only: this worker's cloud instance-id (EC2 instance-id), sent with self-registration so the control node can scale-in-protect the node while it serves previews. Empty for a non-cloud worker |
| `--worker-asg-name` | `$PREVIEW_WORKER_ASG_NAME` | Control node only: the worker tier's EC2 Auto Scaling group name. Set it to publish fleet autoscaling metrics (`UnservedDemand`, `FleetLoad`) to CloudWatch and reconcile scale-in protection on busy workers; empty disables the autoscaling integration |
| `--aws-region` | `$AWS_REGION` | Control node only: AWS region for the CloudWatch/Auto Scaling API. Defaults to the ambient SDK region |
| `--metrics-namespace` | `LocalPreview` | Control node only: CloudWatch namespace for the published fleet metrics |

Two ways to assemble a worker tier: a **static** fleet, where the control node
is handed every worker's URL via `--worker-endpoint(s)`; or a **self-registering**
fleet, where the control node opens `--control-listen` and each worker announces
itself via `--control-endpoint`. The latter is what lets a worker autoscaling
group scale from zero — the control node needs no prior knowledge of a worker's
address. Both paths join a worker to the same fleet by the same internal
transport, and both are shared-secret authenticated on private listeners only.

With `--worker-asg-name` set, the control node closes the autoscaling loop: it
publishes `UnservedDemand` (the count of distinct previews that found no worker
— the scale-from-zero signal, deduplicated so retries of one preview read as
one) and `FleetLoad` (utilization, published only once a worker exists) to
CloudWatch, and reconciles EC2 scale-in protection so an ASG scale-in drains an
idle node instead of terminating one mid-preview. A worker passes its
`--worker-instance-id` so the control node knows which instance to protect.
A freshly built deploy's pre-warm retries while the fleet is at zero — the
demand it registers is what launches the worker it then lands on. A browser
hitting a preview while the fleet is at zero gets a "Waking the preview
fleet…" waiting page (not an error): its polls keep the demand signal alive
while the node boots, hand off to the usual "Starting…" screen once a worker
registers, and load the preview when it's up; API callers get a retryable
`503` with `Retry-After`. The
scaling policies and alarms themselves live in your infrastructure (see the
[Terraform example](https://github.com/jmelahman/local-preview/tree/master/examples/terraform)).

## `preview repo`

| Command | Description |
| --- | --- |
| `preview repo create <name> --source <path-or-url>` | Register a repository (mirror clone); `--watch` and `--branches` enable [watching](/guide/triggers#watched-repos) from the start. The server clones in the background; the command reports clone progress and waits until the repo is ready (non-zero exit if the clone fails) |
| `preview repo list` | List registered repositories with their clone status |
| `preview repo watch <name>` | Poll the repo and deploy branch tips as they move (`--branches` narrows which, as comma-separated globs; `**` spans `/`, and a `!` prefix excludes, e.g. `!main` or `!gh-readonly-queue/**`). Starts from the repo's current state; `--backfill` also deploys the tips it already has |
| `preview repo unwatch <name>` | Stop watching |
| `preview repo delete <name>` | Unregister a repository and delete its previews (deploys, artifacts, state, logs, mirror clone) |

## `preview deploy`

| Command | Description |
| --- | --- |
| `preview deploy [ref]` | Deploy a commit (default: HEAD of the current repo) and print its preview URL, plus a download URL per [artifact](/reference/preview-toml#artifacts-name) file |
| `preview deploy list [query]` | List deploys. The optional query is a free-text search — a commit-sha prefix, or a substring of the repo, branch, ref, or author (case-insensitive); `--repo`, `--branch`, `--author`, `--status` each narrow by one field (`--status crashed` lists the ready deploys whose process died, which `--status ready` leaves out) |
| `preview deploy show <id>` | Print one deploy as JSON |
| `preview deploy logs <id>` | Print a deploy's build logs |
| `preview deploy logs <id> --run` | Print the process run log — the preview server's stdout+stderr, init output included. `--side fe` selects a process-mode frontend; `-f`/`--follow` keeps polling for new output until interrupted |
| `preview deploy stats <id>` | Show live CPU/memory of the deploy's processes (docker-stats-like; samples twice a second apart to compute the CPU percentage) |
| `preview deploy stop <id>` | Stop the deploy's processes; they cold-start again on the next request. Processes are shared per artifact hash, so any sibling deploy on the same hash stops too |
| `preview deploy delete <id>` | Delete a deploy and reclaim any build artifacts, backend state, and process history no surviving deploy still references; its short-sha subdomain is freed |

Flags on `preview deploy`:

| Flag | Description |
| --- | --- |
| `--repo` | Registered repo name; by default the current directory is matched against registered repos: a source path equal to the worktree root, an origin URL naming the same remote as the source (ssh/https/`.git` spellings all match), or a worktree root directory named like a registered repo |
| `--rebuild` | Rebuild artifacts even if cached (a live backend keeps its old binary until restarted) |
| `--no-wait` | Return immediately instead of waiting for the build |
| `--json` | Print the deploy as JSON |

The command exits non-zero if the deploy fails, so it can gate scripts.

## `preview upload`

Publish a CI-built side into the server's content-addressed store so a deploy
of the commit serves it without rebuilding — see
[uploading prebuilt artifacts](/guide/uploads). The server resolves the ref
and computes the same hash a build would target, then lands the upload in that
slot. Each `<path>` may be a directory (its **contents** are tarred, so the tar
root maps to the published root) or an existing `.tar`/`.tar.gz` file. The ref
defaults to HEAD of the current repo.

| Command | Description |
| --- | --- |
| `preview upload frontend <path> [ref]` | Upload a prebuilt frontend — the `dist` tree for a static bundle, or the built `path` tree for a [process-mode frontend](/reference/preview-toml#process-mode-frontends) |
| `preview upload backend <path> [ref]` | Upload a prebuilt backend tree (the built `backend.path` tree, run as-is) |
| `preview upload artifact <name> <path> [ref]` | Upload a named [downloadable artifact](/reference/preview-toml#artifacts-name); the tree must contain the artifact's declared `files` at their `path`-relative locations |

Flags (shared by every subcommand):

| Flag | Description |
| --- | --- |
| `--repo` | Registered repo name; by default the current directory is matched against registered repos: a source path equal to the worktree root, an origin URL naming the same remote as the source (ssh/https/`.git` spellings all match), or a worktree root directory named like a registered repo |
| `--overwrite` | Replace the artifact if it is already present (an upload of an already-present hash is otherwise a no-op) |
| `--deploy` | Deploy the commit after uploading and wait for its preview URL — the one-command CI flow |
| `--oidc` | Authenticate with a [GitHub Actions OIDC](/guide/uploads#authenticating-with-github-actions-oidc) token minted from the runner (needs `permissions: id-token: write`) |
| `--oidc-audience` | Audience to request for the OIDC token (default: `$PREVIEW_GITHUB_OIDC_AUDIENCE`, else the server URL) — must match the server's `--github-oidc-audience` |

Setting `$PREVIEW_UPLOAD_TOKEN` sends that token as-is (a bearer `Authorization`
header), taking precedence over `--oidc` and requiring no runner — an escape
hatch for non-Actions OIDC or local testing.

An upload primes the store independently of any deploy: order doesn't matter,
and an uploaded side is shared by every commit with the same hash. Exits
non-zero on an unresolvable ref, a manifest error, or a tar missing a declared
artifact file.

## `preview open`

Jump from a checkout to its preview: `preview open` finds the deploy of the
current repo's HEAD (falling back to the checked-out branch's newest deploy
when that exact commit was never deployed) and opens its preview URL in the
browser. `preview open <ref>` accepts a branch — its newest deploy wins — a
tag, or a full or abbreviated commit sha; anything else is tried as a
free-text search of the repo's deploys. The URL is always printed, so the
command composes with scripts even when no browser is around.

| Flag | Description |
| --- | --- |
| `--repo` | Registered repo name; by default the current directory is matched against registered repos: a source path equal to the worktree root, an origin URL naming the same remote as the source (ssh/https/`.git` spellings all match), or a worktree root directory named like a registered repo |
| `--print` | Print the preview URL only, without opening a browser |

Exits non-zero when the matched deploy isn't ready — still building, failed,
or evicted — with a hint at the follow-up command (`preview deploy`,
`preview deploy logs`).

## `preview logs`

Stream a preview's run log — the preview server's own stdout and stderr, init
output included — addressed by ref instead of a numeric deploy id.
`preview logs` resolves the ref exactly like `preview open` (HEAD of the
current repo by default; a branch, tag, or sha otherwise), then tails the
run log of the deploy it finds. For a deploy's _build_ logs, use
`preview deploy logs <id>`.

| Flag | Description |
| --- | --- |
| `--repo` | Registered repo name; by default the current directory is matched against registered repos: a source path equal to the worktree root, an origin URL naming the same remote as the source (ssh/https/`.git` spellings all match), or a worktree root directory named like a registered repo |
| `--side` | Which process: `be` (backend, default) or `fe` (process-mode frontend) |
| `-f`, `--follow` | Keep polling for new output until interrupted |

## `preview stats`

Show live CPU/memory of a preview's processes (docker-stats-like; samples
twice a second apart to compute the CPU percentage), addressed by ref. Like
`preview logs`, it resolves the ref the way `preview open` does.

| Flag | Description |
| --- | --- |
| `--repo` | Registered repo name; by default the current directory is matched against registered repos: a source path equal to the worktree root, an origin URL naming the same remote as the source (ssh/https/`.git` spellings all match), or a worktree root directory named like a registered repo |

Both `preview logs` and `preview stats` exit non-zero when the matched deploy
has no live process to inspect — still building, failed, or evicted — with a
hint at the follow-up command.

## `preview exec`

Run a command inside the container backing a preview, docker-exec style,
addressed by ref like `preview open`. The command goes after `--`; without
one you get `/bin/sh`, and when run from a terminal that bare form defaults
to an interactive TTY session (`-it`). The exec's exit status becomes
`preview exec`'s own, so it scripts like `docker exec`.

```bash
preview exec                       # interactive shell in HEAD's preview
preview exec my-branch -- ls -la   # one-shot command, output streamed
preview exec -it my-branch -- bash # interactive shell in a specific deploy
```

| Flag | Description |
| --- | --- |
| `--repo` | Registered repo name; by default the current directory is matched against registered repos (same matching as `preview logs`) |
| `--side` | Which process: `be` (backend, default) or `fe` (process-mode frontend) |
| `-i`, `--interactive` | Attach this terminal's stdin to the command |
| `-t`, `--tty` | Allocate a pseudo-terminal (for shells and TUIs; requires stdin to be a terminal) |

The preview's process must be running — open the preview (or `curl` it) to
start it, then retry. It must also be containered: a manifest `run_image` or
a devcontainer runtime. Host-process previews have no container to exec
into and report exactly that. The session works against a remote server and
a worker fleet alike — it rides a WebSocket from the CLI through the apex
API to whichever node runs the process.

## `preview configure`

Store which server the client subcommands talk to, so `preview open`,
`preview deploy`, and friends reach a remote instance without `--server` on
every invocation:

```bash
preview configure https://preview.example.com
```

The setting is written to the [CLI config
file](/guide/configuration#cli-configuration) and applies to every shell —
including the git hooks installed by `preview install-hook`, which run
`preview deploy` with no flags of their own. With no URL argument the
command prompts for one, offering the current value as the default. After
saving, it pings the server and reports its version and preview domain; an
unreachable server is a warning, not a failure, so you can configure the CLI
before the server is up.

| Flag | Description |
| --- | --- |
| `--show` | Print the config file's path, the stored server, whether a token is set, and which source the effective server comes from |
| `--unset` | Remove the stored server, falling back to `http://localhost:8080` |
| `--token <pat>` | Store a bearer token — a GitHub personal-access token — sent to a server that requires [SSO](/guide/sso); an empty value clears it. Distinct from `preview upload`'s `--oidc`/`$PREVIEW_UPLOAD_TOKEN`, which authenticate only uploads |

Resolution order for the server URL, first match winning:

1. `--server`
2. `$PREVIEW_URL`
3. the config file's `server`
4. `http://localhost:8080`

The bearer token is resolved similarly: `$PREVIEW_TOKEN`, then the config
file's `token`. It is sent only when set, so it's harmless against a server
without SSO.

## `preview install-hook`

Run from inside a target repository. Installs a git post-commit hook that
requests a preview deploy of every new commit (`--dry-run` to preview). If
the repo uses the pre-commit framework, no hook file is written; instead the
command prints a `.pre-commit-config.yaml` stanza subscribing to the hook
this repository publishes in its `.pre-commit-hooks.yaml`, for use with
`pre-commit install --hook-type post-commit`:

```yaml
  - repo: https://github.com/jmelahman/local-preview
    rev: v0.1.8  # pre-filled with the CLI's own release when it is one
    hooks:
      - id: local-preview-deploy
```

See [deployment triggers](/guide/triggers#post-commit-hook) for how the
published hook works and how to keep `rev` current.

Existing hand-written post-commit hooks are never overwritten.

## `preview version`

`preview --version` prints the build version (populated from `git describe`
at release time).
