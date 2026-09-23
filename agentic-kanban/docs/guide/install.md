# Install

Pick whichever path matches your environment. All three give you the same `kanban` binary; the Docker image bakes it into a long-running container.

## With uv (recommended)

```sh
uv tool install agentic-kanban
```

This installs the binary to `~/.local/bin/kanban`. Make sure that's on your `PATH`.

## With Docker

The Docker image is the most "batteries-included" option — it ships with a Docker socket mount so kanban can spawn session containers, and it pre-mounts agent config dirs (`~/.claude` for Claude Code, `~/.pi/agent` for pi) so the harness picks up your existing credentials.

The image entrypoint runs as the unprivileged `nonroot` user (UID/GID `65532`), not root. That means two things you need to plan for at run time:

1. **Bind-mounted host paths must be writable by UID 65532.** Either `chown` the host directories (`$HOME/.local/share/kanban`, `$HOME/.config/kanban`, `$HOME/.claude`) to that UID, or pass `--user "$(id -u):$(id -g)"` to override the image's UID with your own (in which case the host paths can keep their normal ownership).
2. **The bind-mounted docker socket must be group-accessible to the container user.** Add `--group-add "$(stat -c '%g' "${DOCKER_SOCK_PATH:-/var/run/docker.sock}")"` so the container process inherits the host group that owns the socket. Without it, `nonroot` cannot talk to `/var/run/docker.sock` and session spawning fails with `permission denied`.

```sh
SOURCE=$HOME/code
DOCKER_SOCK_PATH=${DOCKER_SOCK_PATH:-/var/run/docker.sock}
docker run -d --name kanban \
  --restart unless-stopped \
  --user "$(id -u):$(id -g)" \
  --group-add "$(stat -c '%g' "$DOCKER_SOCK_PATH")" \
  -p 127.0.0.1:7474:7474 \
  -p 13000-13099:13000-13099 \
  -v $DOCKER_SOCK_PATH:/var/run/docker.sock \
  -v $HOME/.claude:$HOME/.claude \
  -v $HOME/.claude.json:$HOME/.claude.json \
  -v $HOME/.pi/agent:$HOME/.pi/agent \
  -v $HOME/.local/share/kanban:$HOME/.local/share/kanban \
  -v $HOME/.gitconfig:$HOME/.gitconfig:ro \
  -v $HOME/.config/git:$HOME/.config/git:ro \
  -v $SOURCE:$SOURCE \
  -e HOME=$HOME \
  -e XDG_RUNTIME_DIR=$XDG_RUNTIME_DIR \
  -e KANBAN_DATA_DIR=$HOME/.local/share/kanban \
  -e KANBAN_HOST_DOCKER_SOCK=$DOCKER_SOCK_PATH \
  -e GH_TOKEN=$(gh auth token) \
  lahmanja/kanban:latest
```

::: tip Hiding files from the diff view
The session **diff** tab honors git's full ignore chain, including your **global** excludes — the file named by `core.excludesFile` in your `~/.gitconfig` (default `~/.config/git/ignore`). Both are mounted read-only above, so adding a pattern like `.claude/settings.local.json` to your global excludes keeps it out of every board's diff without editing each repo. If your `core.excludesFile` points at a non-default path such as `~/.gitignore`, mount that file too: `-v $HOME/.gitignore:$HOME/.gitignore:ro`.

Your signing key is never mounted into the container, and kanban makes its own merge/squash commits with **signing disabled by default** — so a `commit.gpgsign = true` in the mounted gitconfig won't break merges. If you've mounted your signing key + agent and want kanban's commits signed, enable **Sign commits** in App Settings (or `[git] sign_commits = true` in `~/.config/kanban/config.toml`).
:::

::: tip Rootless Docker
Set `DOCKER_SOCK_PATH` in your shell to point at your rootless socket — see the [rootless Docker docs](https://docs.docker.com/engine/security/rootless/). The same value flows into `KANBAN_HOST_DOCKER_SOCK` so the host path is used as the bind source when kanban spawns session containers. On rootless setups the socket is typically owned by your own UID/GID, so `--user "$(id -u):$(id -g)" --group-add "$(stat -c '%g' "$DOCKER_SOCK_PATH")"` is usually enough — no `chown` needed.
:::

## From GitHub Releases

Prebuilt Linux/macOS/Windows binaries are attached to every tag on the [Releases page](https://github.com/jmelahman/agentic-kanban/releases). Drop the binary somewhere on your `PATH`.

## Build from source

You'll need Go 1.22+ and Node 24+.

```sh
git clone https://github.com/jmelahman/agentic-kanban
cd agentic-kanban
docker bake          # builds the multi-arch container image
# ...or, for a local Go build:
npm --prefix web ci && npm --prefix web run build
go build -tags embed -o kanban
```

## Verify

```sh
kanban --version
kanban serve
```

The server starts listening on `:7474`. Open <http://localhost:7474/>.
