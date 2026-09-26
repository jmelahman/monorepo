# Dev Container

A containerized development environment.

## What's included

- Ubuntu base image
- Bun (`node` falls back to Bun)
- Go (from the `golang` image, with `wgo` for live reload and `govulncheck`)
- Docker CLI + Compose plugin (talks to the host daemon via mounted socket)
- Claude Code, pi
- prek and uv
- Playwright's Chromium system libraries. Browsers are installed per project into `~/.cache/ms-playwright`, which persists across rebuilds
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
bun add -g @devcontainers/cli

# Start the container
devcontainer up --workspace-folder .

# Run a shell
devcontainer exec --workspace-folder . bash
```

## Rebuilding the image

The image is published as `ghcr.io/jmelahman/devcontainer:latest` and
referenced by `devcontainer.json` and the `[devcontainer] image` of each
fullstack project's `.kanban.toml` in the monorepo (which replaces a
per-project `.devcontainer/`). The `release.yml` audit job pins it by digest;
advance that pin by hand.

The monorepo's `Devcontainer` workflow (`.github/workflows/devcontainer.yml`,
driven by the root `docker-bake.hcl`) rebuilds and pushes it — automatically
when anything under `templates/fullstack-template/.devcontainer/` changes on
`master`, or on demand:

```bash
gh workflow run devcontainer.yml
```

Dependabot bumps the `FROM` base image in the `Dockerfile` weekly.

## Host integration

The container bind-mounts a few things from the host so it feels like a normal shell session:

- `~/.claude` and `~/.claude.json` — Claude Code config and session history persist across rebuilds
- `~/.gitconfig` (read-only) — your git identity
- `$SSH_AUTH_SOCK` — SSH agent forwarding for git-over-SSH
- The host Docker socket — `docker` commands inside the container act on the host daemon
- `$GH_TOKEN` is forwarded for the `gh` CLI

Named volumes cache the Go module/build directories and Bun's package cache (`~/.bun/install/cache`) so reinstalls are fast across container rebuilds.

### Remote user

The container runs as `dev` by default. Two env vars on the host let you flip
to a different in-container user — they're consumed by `${localEnv:...}`
substitutions in `devcontainer.json`, so set them in the shell you launch
VS Code or `devcontainer up` from:

- `DEVCONTAINER_REMOTE_USER` (default `dev`) — value of `remoteUser`.
- `DEVCONTAINER_REMOTE_HOME` (default `/home/dev`) — prefix used as the target
  for every host-home bind (`~/.claude`, `~/.zshrc`, the `~/.cache` /
  `~/.local` / `~/.bun/install/cache` named volumes, etc.).

To run as root instead, export both:

```bash
export DEVCONTAINER_REMOTE_USER=root
export DEVCONTAINER_REMOTE_HOME=/root
```

Both vars need to agree — devcontainer.json substitution is string-only and
can't derive one from the other.

The container attaches to the `kanban-net` Docker network so it can reach
sibling containers (e.g. an [agentic-kanban](https://github.com/jmelahman/agentic-kanban)
server) by name. Drop the `--network` entry from `runArgs` in
`devcontainer.json` if you don't run one.

## Firewall

The container ships with an opt-in default-deny firewall (`init-firewall.sh`). It's off by default; set `DEVCONTAINER_FIREWALL=true` in your host shell before launching the container to enable it. When enabled, it only allows outbound traffic to:

- npm registry
- GitHub (API IP ranges fetched from `api.github.com/meta`, plus `github.com`)
- Anthropic API (prod, staging, files)
- Sentry
- VS Code update servers
- Go module proxy (`proxy.golang.org`, `sum.golang.org`, `storage.googleapis.com`)
- Rust toolchain + crates registry (`static.rust-lang.org`, `index.crates.io`, `static.crates.io`) — needed for prek to build ripsecrets
- Playwright browser downloads (`cdn.playwright.dev`, `playwright.download.prss.microsoft.com`)
- Subnets of attached Docker networks (so sibling containers are reachable)

This requires the `NET_ADMIN` and `NET_RAW` capabilities, which are added via `runArgs` in `devcontainer.json`.

Any value other than `true` (including unset) leaves the firewall off and the container runs with no outbound filtering.

Inbound traffic on the loopback interface is always allowed, which is what
enables `docker exec ... socat - TCP:127.0.0.1:<port>` tunneling to publish
container ports to the host without poking holes in the firewall.
