# Dev Container

A containerized development environment.

## What's included

- Ubuntu 25.10 base image
- Node.js 20, Go 1.25
- Docker CLI + Compose plugin
- Claude Code
- Shell tools: zsh, fzf, ripgrep, fd, neovim, less, jq
- `socat`, `openssh-client`, `gh` CLI
- Network firewall (default-deny, whitelists only npm, GitHub, and Anthropic APIs)

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

## Firewall

The container starts with a default-deny firewall (`init-firewall.sh`) that only allows outbound traffic to:

- npm registry
- GitHub
- Anthropic API
- Sentry
- VS Code update servers

This requires the `NET_ADMIN` and `NET_RAW` capabilities, which are added via `runArgs` in `devcontainer.json`.

Inbound traffic on the loopback interface is always allowed, which is what enables the `docker exec ... socat - TCP:127.0.0.1:<port>` tunneling pattern (see `kanban/`) to publish container ports to the host without poking holes in the firewall.
