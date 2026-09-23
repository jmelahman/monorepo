# Dev Container

A containerized development environment.

## What's included

- Ubuntu 25.10 base image
- Node.js 20 (with `vite`, `typescript`, `@typescript/native-preview`)
- Go 1.25 (with `wgo` for live reload)
- Docker CLI + Compose plugin (talks to the host daemon via mounted socket)
- Claude Code
- Playwright + headless Chromium (shared at `/ms-playwright`) for browser-based UI testing
- Shell tools: zsh, fzf, ripgrep, fd, neovim, less, jq
- `socat`, `openssh-client`, `gh` CLI
- Optional network firewall (off by default; set `DEVCONTAINER_FIREWALL=true` to opt in to a default-deny allowlist of npm, GitHub, Anthropic, Sentry, Go module proxy, and VS Code update servers)

## Usage

### VS Code

1. Install the [Dev Containers extension](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers)
2. Open this repo in VS Code
3. "Reopen in Container" when prompted

### CLI

```bash
npm install -g @devcontainers/cli

# Start the container
devcontainer up --workspace-folder .

# Run a shell
devcontainer exec --workspace-folder . bash
```

## Host integration

The container bind-mounts a few things from the host so it feels like a normal shell session:

- `~/.claude` and `~/.claude.json` — Claude Code config and session history persist across rebuilds
- `~/.gitconfig` (read-only) — your git identity
- `$SSH_AUTH_SOCK` — SSH agent forwarding for git-over-SSH
- The host Docker socket — `docker` commands inside the container act on the host daemon
- `$GH_TOKEN` is forwarded for the `gh` CLI

Named volumes cache the Go module/build directories and `~/.npm` so reinstalls are fast across container rebuilds.

### Remote user

The container runs as `dev` by default. Two env vars on the host let you flip
to a different in-container user — they're consumed by `${localEnv:...}`
substitutions in `devcontainer.json`, so set them in the shell you launch
VS Code or `devcontainer up` from:

- `DEVCONTAINER_REMOTE_USER` (default `dev`) — value of `remoteUser`.
- `DEVCONTAINER_REMOTE_HOME` (default `/home/dev`) — prefix used as the target
  for every host-home bind (`~/.claude`, `~/.zshrc`, the `~/.cache` /
  `~/.local` / `~/.npm` named volumes, etc.).

To run as root instead, export both:

```bash
export DEVCONTAINER_REMOTE_USER=root
export DEVCONTAINER_REMOTE_HOME=/root
```

Both vars need to agree — devcontainer.json substitution is string-only and
can't derive one from the other.

The container attaches to the `kanban-net` Docker network so it can reach sibling containers (e.g. the `kanban/` services) by name.

## Firewall

The container ships with an opt-in default-deny firewall (`init-firewall.sh`). It's off by default; set `DEVCONTAINER_FIREWALL=true` in your host shell before launching the container to enable it. When enabled, it only allows outbound traffic to:

- npm registry
- GitHub (API IP ranges fetched from `api.github.com/meta`, plus `github.com`)
- Anthropic API (prod, staging, files)
- Sentry
- VS Code update servers
- Go module proxy (`proxy.golang.org`, `sum.golang.org`, `storage.googleapis.com`)
- Rust toolchain + crates registry (`static.rust-lang.org`, `index.crates.io`, `static.crates.io`) — needed for prek to build ripsecrets
- Subnets of attached Docker networks (so sibling containers are reachable)

This requires the `NET_ADMIN` and `NET_RAW` capabilities, which are added via `runArgs` in `devcontainer.json`.

Any value other than `true` (including unset) leaves the firewall off and the container runs with no outbound filtering. When kanban itself spawns this devcontainer as a session, the value is read from the kanban server's environment via `${localEnv:DEVCONTAINER_FIREWALL}` — set it on the kanban process (e.g. in `compose.yaml`'s `environment:` block) for it to propagate into sessions.

Inbound traffic on the loopback interface is always allowed, which is what enables the `docker exec ... socat - TCP:127.0.0.1:<port>` tunneling pattern (see `kanban/`) to publish container ports to the host without poking holes in the firewall.
