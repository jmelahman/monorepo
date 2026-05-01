# kanban

A kanban board for managing Claude Code sessions. Each ticket is bound to a Claude Code session running inside its own git worktree, executed in the target repository's existing devcontainer.

## Run

```bash
SOURCE=$HOME/code
docker run -d --name kanban \
  --restart unless-stopped \
  -p 127.0.0.1:7474:7474 \
  -p 13000-13099:13000-13099 \
  -v $XDG_RUNTIME_DIR/docker.sock:/var/run/docker.sock \
  -v $HOME/.claude:$HOME/.claude \
  -v $SOURCE:$SOURCE \
  -v $HOME/.local/share/kanban:$HOME/.local/share/kanban \
  -e HOME=$HOME \
  -e XDG_RUNTIME_DIR=$XDG_RUNTIME_DIR \
  -e KANBAN_DATA_DIR=$HOME/.local/share/kanban \
  -e GH_TOKEN=$(gh auth token) \
  lahmanja/kanban:latest
```

Open `http://localhost:7474`.

## Build

```bash
cd kanban
docker build -t kanban:dev .
```

## Per-task ports

Optional `<worktree>/.kanban.toml` associates `.vscode/tasks.json` labels with container ports:

```toml
[[task]]
label = "Start Frontend"
container_port = 3000

[[task]]
label = "Start Backend"
container_port = 8080
```

When a task with an associated port runs, kanban allocates a host port from `13000-13099` and runs a TCP proxy.
