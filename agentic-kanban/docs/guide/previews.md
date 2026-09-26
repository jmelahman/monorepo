# Previews

Kanban can build any ticket's branch and serve it at its own address, so you can try the change in a browser:

```
http://a1b2c3d.my-board.preview.localhost:7474/
```

Previews are built by an embedded copy of [local-preview](https://jmelahman.github.io/local-preview/). It only rebuilds the frontend or backend when that part changed. It starts backends on demand. A new backend starts with a copy of the data from the nearest ancestor commit that has a preview, so data on one branch never leaks into another. The local-preview [concepts](https://jmelahman.github.io/local-preview/guide/concepts) page explains the details.

## Setting up a repo

Add a [`preview.toml`](https://jmelahman.github.io/local-preview/reference/preview-toml) to the repo root that describes how to build and run the app. You can put the same content under a `[previews]` table in `.kanban.toml` instead. If both exist, `preview.toml` wins. Kanban's own `.kanban.toml` is a working example.

Kanban reads the manifest from the commit being built, so it only applies to commits made after you add it.

To preview an app that needs a database or other services, see local-preview's [external dependencies](https://jmelahman.github.io/local-preview/guide/external-dependencies) guide.

### Repos you can't change

If you can't add a manifest to the repo, put one on the kanban server instead, named after the board's slug:

```
~/.config/preview/manifests/<board-slug>.toml
```

It uses the plain `preview.toml` format and is shared with the standalone `preview` CLI. Kanban only uses it when the commit has no manifest of its own. Since it isn't versioned with the code, update it by hand if the build changes. Set `KANBAN_PREVIEW_MANIFESTS` to use a different directory.

## Deploying

Open a ticket's **previews** tab and click **deploy tip**. Kanban builds the branch's latest commit (not uncommitted changes) and shows the status, build logs, and a link when it's ready. `*.preview.localhost` works in modern browsers with no DNS setup.

Kanban also deploys automatically whenever an agent finishes working, if the board has a manifest. Deploying a commit that's already built does nothing. Set `KANBAN_PREVIEW_AUTO_DEPLOY=0` to turn this off.

### Build environment

Builds run in the repo's devcontainer, using the devcontainer config from the commit being built. They share the session's cache volumes, so dependency downloads are reused. Repos without a devcontainer build in the bundled image. A manifest that names its own `image` uses that instead.

Set `KANBAN_PREVIEW_BUILDS=host` to build directly on the kanban host. Kanban also builds on the host if Docker isn't available.

## The previews dashboard

Click **preview** in the header to see every deploy across all boards. Each row shows the board, commit, branch, author, and status:

| Status                | Meaning                                                  |
| --------------------- | -------------------------------------------------------- |
| `queued` / `building` | Waiting to build, or building.                           |
| `ready`               | Built. A static site is served right away.               |
| `idle`                | Built. The backend starts on the first request.          |
| `starting`            | The backend is starting.                                 |
| `running`             | The backend is running.                                  |
| `failed`              | The build failed. Open the logs for details.             |
| `evicted`             | Removed to save disk. Deploy again to rebuild.           |

From a row you can:

- Open the preview, its build logs, or its [downloads](#downloads).
- **stop** its backend. It returns to `idle` and starts again on the next request. Deploys built from identical output share a backend, so those stop too.
- Delete it. Deploying the same commit later rebuilds it.

**deploy** in the dashboard builds any branch, tag, or commit on any board. Leave the ref empty to build the base branch, which is handy for comparing tickets against `main`.

## Disk usage

Every preview leaves build output, backend data, and logs behind. The database icon in the dashboard header opens **Storage & retention**, which shows disk usage by category and by board.

You can set two limits there:

- **Deploys per board** keeps the newest N.
- **Max age (days)** removes deploys older than N days.

Both are empty by default, so nothing is removed. Cleanup runs hourly, or immediately with **sweep now**. Removed deploys stay in the list as `evicted`. Kanban never removes a board's newest working preview, or anything still building.

## Downloads

A manifest can also publish files for download instead of running them, such as a CLI binary for each commit:

```toml
[artifacts.cli]
path  = "."
image = "golang:1.26-alpine"
build = [["go", "build", "-o", "bin/mytool-linux-amd64", "."]]
files = ["bin/mytool-linux-amd64"]
```

Ready deploys list these as download links, each named after the file. File names must be unique within one artifact. This needs local-preview v0.1.2 or newer. Older versions reject any manifest that contains `[artifacts]`.

## Settings

| Variable                     | Default                       | Description                                                                  |
| ---------------------------- | ----------------------------- | ---------------------------------------------------------------------------- |
| `KANBAN_PREVIEW_DOMAIN`      | `preview.localhost`           | Domain previews are served under. Point a wildcard DNS record at kanban to use your own. |
| `KANBAN_PREVIEW_BUILDS`      | `devcontainer`                | `host` builds on the kanban host.                                            |
| `KANBAN_PREVIEW_AUTO_DEPLOY` | on                            | `0` turns off automatic deploys.                                             |
| `KANBAN_PREVIEW_MANIFESTS`   | `~/.config/preview/manifests` | Directory for server-side manifests. Respects `$PREVIEW_CONFIG_DIR`.         |

Preview data lives in `<data-dir>/previews/`. Deleting a board deletes its previews. If previews can't start, for example because `git` is missing, the rest of kanban still works.
