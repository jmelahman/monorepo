# Install

Every option below installs the same `kanban` binary.

## With uv (recommended)

```sh
uv tool install agentic-kanban
```

This puts `kanban` in `~/.local/bin`. Make sure that directory is on your `PATH`.

## With Docker

The image runs kanban as a long-lived service. It needs your Docker socket so it can start session containers. It also needs your agent config directories (`~/.claude` for Claude Code, `~/.pi/agent` for pi, `~/.config/opencode` and `~/.local/share/opencode` for opencode) so the agent can use your existing login.

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
  -v $HOME/.config/opencode:$HOME/.config/opencode \
  -v $HOME/.local/share/opencode:$HOME/.local/share/opencode \
  -v $HOME/.local/share/kanban:$HOME/.local/share/kanban \
  -v $HOME/.gitconfig:$HOME/.gitconfig:ro \
  -v $HOME/.config/git:$HOME/.config/git:ro \
  -v ${SSH_AUTH_SOCK:-/dev/null}:${SSH_AUTH_SOCK:-/dev/null} \
  -v $SOURCE:$SOURCE \
  -e HOME=$HOME \
  -e XDG_RUNTIME_DIR=$XDG_RUNTIME_DIR \
  -e SSH_AUTH_SOCK=${SSH_AUTH_SOCK:-} \
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

Kanban doesn't sign its own commits by default, so `commit.gpgsign = true` in your config won't break merges. To sign them, turn on **Sign commits** in the app settings. With SSH signing the command above has everything needed: the image has no `ssh-keygen`, and kanban signs through your forwarded agent itself. See [Signing](./configuration#signing).

The `SSH_AUTH_SOCK` lines forward your ssh-agent. Without one, they mount `/dev/null` over itself, which does nothing.

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

## Man pages

`pip install agentic-kanban` installs the man pages alongside the binary, so `man kanban` works when the environment's `bin` directory is on your `PATH`.

`uv tool install` keeps them in the tool's private directory, where `man` can't find them. For that and every other install method, generate them with the binary. On Linux and macOS, run:

```sh
kanban man -o ~/.local/share/man/man1
man kanban
```

Each command has its own page, such as `man kanban-ticket-attach`. Run the first command again after upgrading.

`man` finds this directory when `~/.local/bin` is on your `PATH`. If it reports `No manual entry`, add `~/.local/share/man` to `MANPATH`.

GitHub Releases archives also include the pages in `man/man1`.

## Check that it works

```sh
kanban --version
kanban serve
```

Then open <http://localhost:7474/>.

## Releases

The [Releases](/guide/releases) tab runs `git subtree`, and `gh` to watch what a release publishes. The Docker image includes both. On a bare-metal install, without `git subtree` it can't work, and without `gh` a release is tagged but not watched.
