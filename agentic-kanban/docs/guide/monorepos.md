# Monorepos

By default, a board's agents work from the repository root. In a monorepo you usually want one board per project, with each agent working from a subdirectory such as `services/api`. A board's **project directory** does this.

The whole repository is still checked out and mounted. Only the agent's working directory changes.

## Creating a project board

Run `board create` from the subdirectory. Kanban works out the repo, the project directory, and the board name:

```sh
cd ~/code/monorepo/services/api
kanban board create
# repo_path=~/code/monorepo  project_dir=services/api  name=api
```

Or pass them yourself:

```sh
kanban board create \
  --repo-path ~/code/monorepo \
  --project-dir services/api \
  --name api
```

In the web UI, the field is on the **Git** tab of the board settings. Clear it to go back to a whole-repo board.

The project directory must be a relative path inside the repo. A board with a project directory needs a `repo_path` and can't also have a `mount_path`.

## What changes

Each ticket still gets a worktree of the whole repo, mounted at `/workspace` (or your devcontainer's `workspaceFolder`). The agent just starts in `/workspace/services/api`. Because the full checkout is there, a project that imports a sibling package or relies on a root `go.work` keeps working.

Git operations still cover the whole repo. Diffs, sync, and merge include changes outside the project directory, such as edits to shared CI config.

Kanban looks for config in the project directory first:

| File                          | Where kanban looks                                                                                  |
| ----------------------------- | --------------------------------------------------------------------------------------------------- |
| `devcontainer.json`           | The project directory, then the repo root, then `~/.config/kanban`, then the bundled image.          |
| Dockerfile                    | Next to whichever `devcontainer.json` was used.                                                     |
| `.kanban.toml`                | Both. The project's file overrides the root's, key by key.                                           |
| `.vscode/tasks.json`          | The project directory only. Put the matching `[[task]]` port entries in the project's `.kanban.toml`. |
| Plans directory               | A relative `plans.dir` resolves inside the project directory.                                        |

## Picking a board from the current directory

CLI commands that take an optional board, such as `ticket create` and `ticket attach`, find it from your current directory. When several boards share a repo, kanban picks the board whose project directory contains your current directory most closely:

```
~/code/monorepo                  -> the board with no project directory
~/code/monorepo/services/api     -> the services/api board
~/code/monorepo/services/api/db  -> the services/api board
```

If no board matches, or two boards have the same project directory, pass `--board`.

## Boards from older versions

Older versions had no project directory, so some boards pointed `mount_path` at a subdirectory of the repo instead. That setup didn't work properly: the agent edited your main checkout rather than the ticket's worktree.

Kanban converts these boards when it starts: a `mount_path` inside `repo_path` becomes the project directory. Restart any sessions that were running before the upgrade so they pick up the new mount.
