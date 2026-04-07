# Onyx Dev Container

A containerized development environment for working on Onyx.

## What's included

- Ubuntu 25.10 base image
- Node.js 20
- Claude Code
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
