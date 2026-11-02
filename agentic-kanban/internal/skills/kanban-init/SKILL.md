---
name: kanban-init
description: Create an agentic-kanban board for a project and set up its session container image, tasks, and port mappings.
disable-model-invocation: true
---

# Set up a project on agentic-kanban

Create a board for a project and configure the environment its ticket sessions
run in. Treat anything the user passed when invoking this skill as the answers
to step 2.

Use the `kanban` CLI. If it isn't available but the `mcp__kanban__*` tools are,
use their equivalents: `list_boards`, `create_board`, `update_board`,
`set_board_env`.

## 1. Look at what's there

1. Run `kanban board list`. If it can't connect, ask the user to start
   `kanban serve`, or to give you the server's URL for `$KANBAN_URL`. Don't start
   a server yourself.
2. Work out the repo root and, in a monorepo, the project directory: the
   subdirectory the agent should work from. Run `kanban board get <id> --json` on
   any board that looks related. If one already has this repo and project
   directory, update it with `kanban board update` rather than creating another.
3. Read what the project already has:
   - `.devcontainer/devcontainer.json` or `.devcontainer.json`, in the project
     directory and the repo root
   - `.kanban.toml` or `.config/kanban/config.toml`, in both places
   - `.vscode/tasks.json` in the project directory
   - the project's README and build files, to find its dev server commands
4. Run `kanban config list --global`. The user's config file overrides the
   project's, so tell the user about any key there that will override a value
   you're about to set.

## 2. Settle the plan

Infer what you can. Ask the user only for what you can't, and in one go:

- the container image
- which tasks to define, and the port each dev server listens on
- any secrets the agent's tools need, by name

Show the plan before changing anything if you had to guess at any of it.

## 3. Create the board

Run this from the project directory, which sets the repo, project directory, and
name:

```sh
kanban board create --json
```

Pass `--name`, `--base-branch`, or `--branch-prefix` if the user asked for them.
Pass `--repo-path` and `--project-dir` if the server runs in Docker and sees
different paths from yours. Note the slug and `base_branch` in the output.

## 4. Write the config

Edit the project file directly instead of using `kanban config set --local`,
which only writes to the repo root's file and drops its comments.

Use the existing file in the directory you're configuring. If there's none, create
`.config/kanban/config.toml` if that directory has a `.config/`, and `.kanban.toml`
otherwise.

Kanban reads each setting from a specific place:

| Setting                                        | Where it goes |
| ---------------------------------------------- | ------------- |
| `[devcontainer]`                               | The project directory, or the repo root to share it across subprojects. |
| `[[task]]`                                     | The same directory as the `.vscode/tasks.json` it pairs with. |
| `[sync]`, `[merge]`, `[github]`, `[branches]`  | The repo root. |

### Container image

Which file sets the image depends on whether a `devcontainer.json` exists.
Kanban looks in the project directory, then the repo root, then
`~/.config/kanban`.

- **If there's none**, set the image in the project file:

  ```toml
  [devcontainer]
  image = "ghcr.io/example/devbox:latest"
  ```

- **If there is one**, kanban ignores `[devcontainer].image`, `docker_socket`, and
  `claude_config`. Change `image` in the `devcontainer.json` instead, or ask the
  user if it uses a `build` section.

The image needs the CLI of the user's agent installed, and a `dev` (UID 1000) or
`root` user.

`mounts`, `run_args`, and `[devcontainer.container_env]` work in both cases. Leave
`docker_socket` off unless the user asks for it: it gives the agent root-level
access to their machine.

### Tasks

Kanban runs tasks from `.vscode/tasks.json` in the project directory. Add any
that are missing:

```json
{
  "version": "2.0.0",
  "tasks": [
    { "label": "Start Frontend", "type": "shell", "command": "bun run dev --host 0.0.0.0" }
  ]
}
```

Servers must listen on `0.0.0.0` to be reachable from outside the container.

To give a task a URL on the host, map its exact label to the port it listens on
in the container:

```toml
[[task]]
label = "Start Frontend"
container_port = 3000
```

### Secrets

Never put a secret in a config file. Have the user run this themselves, so the
value stays out of this conversation:

```sh
kanban env set <board> KEY=VALUE
```

## 5. Commit it

Sessions read the config and tasks from the ticket's worktree, which forks from
the base branch. Until these files are on that branch, new tickets won't see them.

Show the user the changed files and ask before committing, unless they've already
told you to. Commit only those files. If the base branch isn't checked out, tell
the user rather than switching branches.

## 6. Check it and report

Offer to start a throwaway ticket, because this pulls the image and starts a
container:

```sh
kanban ticket create --board <slug> --title "Check the dev environment"
kanban ticket tasks <id> --run "<label>" --detach
kanban ticket tasks <id> --json
```

Clean up with `kanban ticket archive <id> --delete`.

Finish by telling the user the board's slug, the image, each task with its port,
what you committed, and anything still left for them to do.
