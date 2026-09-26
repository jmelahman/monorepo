# Install

Every option below installs the same `kanban` binary.

## With uv (recommended)

```sh
uv tool install agentic-kanban
```

This puts `kanban` in `~/.local/bin`. Make sure that directory is on your `PATH`.

## With Docker

The image runs kanban as a long-lived service. It needs your Docker socket so it can start session containers. It also needs your agent config directories (`~/.claude` for Claude Code, `~/.pi/agent` for pi) so the agent can use your existing login.

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

Set `SOURCE` to the directory holding your repositories. It's mounted at the same path inside the container, so board paths match on both sides.

The image runs as an unprivileged user (UID 65532) by default. The command above changes two things so it can reach your files and Docker:

- `--user "$(id -u):$(id -g)"` runs the container as you, so the mounted directories stay writable without a `chown`.
- `--group-add` adds the group that owns the Docker socket. Without it, starting a session fails with `permission denied`.

::: tip Rootless Docker
Point `DOCKER_SOCK_PATH` at your rootless socket before running the command. See the [rootless Docker docs](https://docs.docker.com/engine/security/rootless/). The same flags work unchanged.
:::

### Git config in the container

The command mounts your `~/.gitconfig` and `~/.config/git` read-only. This does two things:

- Your global ignore file (`core.excludesFile`) applies to the ticket diff. Add a pattern like `.claude/settings.local.json` there to hide it on every board. If `core.excludesFile` points somewhere else, such as `~/.gitignore`, mount that file too.
- Kanban's merge commits use your name and email.

Kanban doesn't sign its own commits by default, so `commit.gpgsign = true` in your config won't break merges. To sign them, mount your signing key and agent, then turn on **Sign commits** in the app settings.

## From GitHub Releases

Download a Linux, macOS, or Windows binary from the [Releases page](https://github.com/jmelahman/agentic-kanban/releases) and put it on your `PATH`.

## From source

You need Go and [Bun](https://bun.sh).

```sh
git clone https://github.com/jmelahman/agentic-kanban
cd agentic-kanban
bun install --cwd web --frozen-lockfile && bun run --cwd web build
go build -tags embed -o kanban
```

To build the container image instead, run `docker bake`.

## Check that it works

```sh
kanban --version
kanban serve
```

Then open <http://localhost:7474/>.
