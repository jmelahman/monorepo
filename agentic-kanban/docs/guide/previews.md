# Previews

Every ticket branch can be deployed as a live preview: a real build of the
branch's latest commit, served at its own subdomain like

```
http://a1b2c3d.my-board.preview.localhost:7474/
```

Previews are powered by an embedded
[local-preview](https://jmelahman.github.io/local-preview/) orchestrator:
frontend and backend are content-addressed separately (commits that don't
touch a side reuse the existing artifact and even the running backend
process), backend processes start on demand, and backend state follows git
lineage — a new backend version forks its data from the nearest deployed
ancestor commit, so previews on one branch feel continuous while divergent
branches can never corrupt each other. See the local-preview
[concepts](https://jmelahman.github.io/local-preview/guide/concepts) page
for how that works.

## Onboarding a repo

The board's repo needs a preview manifest describing how to build and run
it — either a dedicated
[`preview.toml`](https://jmelahman.github.io/local-preview/reference/preview-toml)
at the repo root, or the same schema under a `[previews]` table in the
`.kanban.toml` the repo may already carry (`[previews.frontend]` /
`[previews.backend]`; `preview.toml` wins when both exist — this repo's own
`.kanban.toml` is a working example). That's the only setup — kanban
registers the repo with the orchestrator automatically on the first deploy,
and worktree branches are deployable as-is (they share the repo's object
store). The manifest is always read from the deployed commit, so onboarding
applies to commits made after it landed.

### Repos you can't change

Some boards track a repo whose upstream won't take a `preview.toml`. Those
are onboarded from the **server side** instead: drop a manifest named for
the board's slug in kanban's manifest directory, in the plain `preview.toml`
schema.

```bash
# Board "Onyx" (slug: onyx) → the manifest kanban looks for
~/.config/preview/manifests/onyx.toml
```

That's local-preview's own manifest directory, so a manifest written for the
`preview` CLI works in kanban unchanged, and vice versa. It's read from the
server's disk at build time rather than from the deployed commit — the
tradeoff for onboarding a repo that can't carry its own contract: the
manifest doesn't version with the code, so a commit that moves the build
needs the manifest updated by hand.

In-repo sources win: kanban only falls back to this directory when the
deployed commit has neither a `preview.toml` nor a `[previews]` table.
Automatic deploy-on-idle honors it too — a board onboarded this way deploys
on agent idle like any other.

Set `KANBAN_PREVIEW_MANIFESTS` to point elsewhere (a containerized kanban
wants a mounted path, not the server user's home).

::: tip
Previewing an app with real infrastructure — a database, a search engine, a
model server — is what the manifest's `run_image`, `networks`, and `env`
keys are for: you run the dependency stack once and every preview's
processes join its docker network. local-preview's
[external dependencies](https://jmelahman.github.io/local-preview/guide/external-dependencies)
guide walks through exactly that.
:::

## Using it

Open a ticket's session pane → **previews** tab → **deploy tip**. The deploy
builds the branch's current commit (from the committed tree, never the
working directory) and the row shows status, build logs, and the preview
link once ready. `*.preview.localhost` resolves in every modern browser
with no DNS setup.

Deploys are idempotent per commit — "deploy tip" after new agent commits
builds only what changed.

## The previews dashboard

The **preview** item in the header opens a dashboard over every deploy on
the server, across every board — the fleet view the session tab's
single-branch list can't give you.

Each row carries the board it belongs to, the commit, its branch and author,
and a status badge that reads the build status until the deploy is ready and
the live process state after that:

| Badge | Meaning |
| --- | --- |
| `queued` / `building` | Waiting for a build slot, or building now. |
| `ready` | Built. Static preview — served instantly. |
| `idle` | Built, backend not running — it starts on the first request. |
| `starting` | A supervised process is warming up. |
| `running` | Every side is warm. |
| `failed` | The build failed; the error is on the row, the detail in the logs. |
| `evicted` | Artifacts were cleaned up. Redeploy to rebuild. |

Rows link out to the live preview, open the build log, and offer any
[artifacts](#downloadable-artifacts) as downloads. Filter to one board with
the board picker; the list polls once a second while anything is building or
starting and every five seconds otherwise.

Two lifecycle controls sit on each row:

- **stop** — shown while a deploy is `starting` or `running`. It stops the
  supervised processes and leaves the deploy in place, so it drops back to
  `idle` and cold-starts on the next request. Processes are shared per
  artifact hash, so a sibling deploy built to the same output stops too.
- The trash icon **deletes** the deploy, reclaiming any build output no
  surviving deploy still references. Sides are content-addressed, so a half
  another deploy shares is kept. Redeploying the commit rebuilds it.

**deploy** opens a dialog that deploys any ref of any git-linked board — a
branch, a tag, a bare sha. Leave the ref empty to deploy the board's base
branch, which is the usual way to get a preview of `main` alongside the
ticket branches diverging from it.

## Storage and retention

Every commit previewed leaves build output, backend state, and logs behind,
so previews only ever grow until something reclaims them. The database icon
in the dashboard header opens **Storage & retention**.

The top half reports where the disk went — artifacts, backend state, logs,
git mirrors, staging, and the orchestrator's own database — then breaks the
total down per board, with each board's deploy count. Sizes are measured by
walking the data dir, so they're read when you open the dialog rather than
polled.

The bottom half is the policy that bounds it, with two independent limits:

- **Deploys per board** keeps at most N deploys, newest first.
- **Max age (days)** evicts deploys older than N days.

Leave a field empty for no limit; empty both and nothing is ever evicted,
which is the default. A sweep runs hourly, and **sweep now** runs one
immediately and reports what it freed.

Eviction reclaims a deploy's bytes but keeps its row, which is what the
`evicted` state in the table above means — the deploy stays visible as
history and redeploying the same commit rebuilds it. A board's newest ready
deploy is never evicted, so tight limits can't leave a board with no working
preview, and neither are deploys still queued or building. Even with
retention off, a sweep clears stale staging leftovers from interrupted
builds.

## Downloadable artifacts

A manifest can also declare build outputs that are published for download
rather than run — a CLI binary per commit, say — as
`[artifacts.<name>]` sections:

```toml
[artifacts.cli]
path  = "."
image = "golang:1.26-alpine"
build = [["go", "build", "-o", "bin/mytool-linux-amd64", "."]]
files = ["bin/mytool-linux-amd64"]
```

They're hashed and cached like the other sides, so a commit that doesn't
touch the artifact's partition reuses the existing build. Ready deploys list
them in the previews tab as download links; files are addressed by base
name, so base names must be unique within one artifact. Each artifact's
build output also gets its own section in the deploy's build logs.

::: tip
Artifacts need local-preview v0.1.2 or newer. Earlier versions reject the
whole manifest — `unknown keys: artifacts.<name>` — failing every deploy of
a repo that declares them.
:::

## Automatic deploys

When an agent finishes a burst of work (its session transitions to idle),
kanban automatically deploys the branch tip. Deploys are idempotent per
commit, so an unchanged tip is a no-op — and the trigger only fires for
boards that are onboarded (a manifest in the worktree, or a server-side one
named for the board slug), so boards that haven't onboarded never accumulate
failed deploys. Disable with `KANBAN_PREVIEW_AUTO_DEPLOY=0`.

## Reproducible builds

Build steps run inside the board repo's **devcontainer** by default: the
devcontainer config is read from the deployed commit's tree (old commits
build with the environment they shipped with), resolved through kanban's
content-addressed image cache (unchanged configs reuse the image), and each
step runs in a one-shot container with the extracted tree mounted. The
devcontainer's named cache volumes are mounted alongside it and `HOME` points
at the remote user's home, so Go and npm resolve their caches onto the same
volumes the interactive session container uses and a repeat build starts
warm. Repos without a devcontainer build in the builtin session image. Set
`KANBAN_PREVIEW_BUILDS=host` to run build steps directly on the kanban host
instead (or previews fall back to host builds automatically when Docker is
unavailable). A manifest-declared `image` on a side beats devcontainer
discovery — the repo's explicit contract wins.

## Configuration

| Env var | Default | Description |
| --- | --- | --- |
| `KANBAN_PREVIEW_DOMAIN` | `preview.localhost` | Base domain previews are served under. Point a wildcard DNS record at the kanban server to use a real domain. |
| `KANBAN_PREVIEW_BUILDS` | `devcontainer` | Set to `host` to run build steps on the kanban host instead of in the repo's devcontainer. |
| `KANBAN_PREVIEW_AUTO_DEPLOY` | on | Set to `0` to disable deploy-on-idle. |
| `KANBAN_PREVIEW_MANIFESTS` | `$PREVIEW_CONFIG_DIR/manifests`, else `~/.config/preview/manifests` | Directory searched for out-of-repo manifests (`<board-slug>.toml`) for repos that can't carry their own. |

Preview state (builds, artifacts, backend state, its own SQLite) lives under
`<data-dir>/previews/`. With `--in-memory` it moves to a temp dir and an
ephemeral DB. Deleting a board tears its previews down with it — running
backends are stopped and the board's mirror clone, artifacts, state dirs,
and build logs are removed. If the orchestrator can't start (e.g. `git` missing), kanban
runs normally and the preview endpoints report unavailable.
