---
layout: home

hero:
  name: Agentic Kanban
  text: A kanban board for AI agent sessions.
  tagline: Each ticket gets its own git worktree and an agent running in your repo's devcontainer.
  image:
    light: /demo_light.png
    dark: /demo_dark.png
    alt: Agentic Kanban screenshot
  actions:
    - theme: brand
      text: Get Started
      link: /guide/install
    - theme: alt
      text: Quickstart
      link: /guide/quickstart
    - theme: alt
      text: View on GitHub
      link: https://github.com/jmelahman/agentic-kanban

features:
  - title: One worktree per ticket
    details: Agents work in parallel on separate branches, each in its own container, so they never step on each other.
  - title: Terminal in the browser
    details: Drive the agent and a plain shell from the ticket. Claude Code conversations resume after container and server restarts.
  - title: Works without a devcontainer
    details: Repos with no `.devcontainer/devcontainer.json` run in a bundled Ubuntu image with git, gh, Node, and Go.
  - title: Your choice of agent
    details: Run Claude Code or pi.dev, and switch harnesses per ticket.
  - title: Diffs, tasks, and ports
    details: Review the branch diff, run `.vscode/tasks.json` tasks, and open forwarded dev-server ports from the ticket.
  - title: GitHub integration
    details: Tickets move between columns as their pull requests change state, with review and CI status on the ticket.
  - title: Build Cop
    details: Watches GitHub Actions and files a ticket when a job starts failing or flaking too often.
  - title: REST, CLI, and MCP
    details: Script the board over HTTP, from the `kanban` CLI, or let an AI tool drive it through MCP.
  - title: A single binary
    details: The server, web UI, CLI, and MCP server ship as one Go binary. Also available as a Docker image and on PyPI.
---
