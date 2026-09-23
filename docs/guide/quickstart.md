# Quickstart

This walkthrough registers a repository, deploys a commit, and opens the
preview in a browser. It assumes the [`preview` binary is
installed](/guide/install) and that you have a web app in a local git repo to
try it on.

## 1. Start the server

```sh
preview serve
```

The server listens on `:8080`. Open <http://localhost:8080/> for the
dashboard. There's nothing registered yet, so it's empty:

<img class="light-only" src="/quickstart-01-empty-light.png" alt="Empty dashboard" />
<img class="dark-only" src="/quickstart-01-empty-dark.png" alt="Empty dashboard" />

## 2. Add a preview.toml to your app

The target repo declares how to build and run itself in a
[`preview.toml`](/reference/preview-toml) at its root. Here's the one for
`myapp`, a static site in `site/` with a Go API next to it:

```toml
[frontend]
path  = "site"
build = [["sh", "-c", "mkdir -p dist && cp index.html dist/"]]
dist  = "dist"

[backend]
path        = "."
build       = [["go", "build", "-o", "bin/server", "."]]
run         = ["./bin/server", "-addr", "127.0.0.1:{port}", "-data", "{state_dir}"]
health_path = "/api/health"
```

Commit it. Previews always build from committed trees, so an uncommitted
`preview.toml` doesn't exist as far as the server is concerned.

## 3. Register the repository

Click **Register repo** in the header, then pick a name and point the source
at the repo. The name becomes the preview subdomain, so it must be a
lowercase DNS label. The source can be a local path or any clone URL.

<img class="light-only" src="/quickstart-02-register-light.png" alt="Register a repository" />
<img class="dark-only" src="/quickstart-02-register-dark.png" alt="Register a repository" />

Registering makes the server keep its own mirror clone of the repo. Builds
read from that mirror, never from your working directory. The clone runs in
the background — large repositories show a **cloning** state (with live
progress when the source reports it) and become deployable once it
finishes.

The CLI equivalent:

```sh
preview repo create myapp --source ~/code/myapp
```

## 4. Deploy a commit

Click **Deploy** above the deployments list. The repository you just
registered is already selected; enter a ref — a branch, a tag, or a sha —
and click **Deploy**.

<img class="light-only" src="/quickstart-03-deploy-light.png" alt="Deploy a commit" />
<img class="dark-only" src="/quickstart-03-deploy-dark.png" alt="Deploy a commit" />

The dialog stays open and turns into a progress view: a checklist of the
build's phases and a live tail of the build log. It finishes with an **Open
preview** button. Closing the dialog doesn't interrupt anything — the
deployment also appears in the list below, moving from `queued` through
`building` to `idle` (built and served on demand). The dashboard polls while
a build is running, so there's nothing to refresh.

From inside the target repo, the CLI can do the same and waits for the
result:

```sh
preview deploy            # deploys HEAD
preview deploy main       # or any branch, tag, or sha
```

```
ready: http://d9ebf14.myapp.preview.localhost:8080/
```

## 5. Open the preview

Once the build finishes, the row grows an **open** link:

<img class="light-only" src="/quickstart-04-ready-light.png" alt="A ready deployment" />
<img class="dark-only" src="/quickstart-04-ready-dark.png" alt="A ready deployment" />

It leads to `http://d9ebf14-myapp.preview.localhost:8080/` — every commit
gets a subdomain of the form `<sha>-<repo>`, and browsers resolve
`*.localhost` names without any DNS setup.

The frontend is served statically. The backend process is why the badge
reads `idle` rather than `running`: it isn't started by the build, but by
the first request that hits the preview's `/api/` — expect a brief warm-up
on that first visit. Open the preview, and the badge flips to `running`;
from then on requests are served instantly. The row also shows the two
artifact hashes the commit resolved to — deploy a docs-only commit and
you'll see both hashes stay the same and the build finish instantly.

## 6. Deploy every commit

Run this once inside the target repo:

```sh
preview install-hook
```

It installs a git post-commit hook that requests a deploy of each new commit
(or, if the repo uses the pre-commit framework, prints the stanza to add to
`.pre-commit-config.yaml` instead). From then on, committing is deploying.

## What's next

- [Concepts](./concepts) explains what makes previews cheap: content
  addressing, shared backends, and lineage-forked state.
- [Configuration](./configuration) covers the data directory, flags, and
  serving under a real domain.
- The [preview.toml reference](/reference/preview-toml) has the full manifest
  schema.
- Drive deploys from scripts with the [REST API](/reference/api) and
  [CLI](/reference/cli).
