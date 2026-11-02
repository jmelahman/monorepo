---
name: kanban-init
description: Create an agentic-kanban board for a project and set up its session container image, tasks, and port mappings. Use when adding a project to kanban or configuring the dev environment its ticket sessions run in.
disable-model-invocation: true
---

# Set up a project on agentic-kanban

Create a board for a project and configure the environment its ticket sessions
run in. Treat anything the user passed when invoking this skill, such as
`/kanban-init services/api ghcr.io/me/devbox:latest`, as answers to step 2.

Use the `kanban` CLI. If only the `mcp__kanban__*` tools are available, use
`list_boards`, `create_board`, and `update_board`, passing `repo_path` and
`project_dir` yourself. MCP can't run tasks, so ask the user to run step 6.

## 1. Look at what's there

1. Run `kanban board list`. If it can't connect, ask the user to start
   `kanban serve`, or to give you the server's URL for `$KANBAN_URL`. Don't start
   a server yourself.
2. Work out the repo root and, in a monorepo, the project directory: the
   subdirectory the agent should work from. Run `kanban board get <id> --json` on
   any board that looks related. If one already has this repo and project
   directory, update it with `kanban board update` rather than creating another.
   If the project isn't in a git repo, stop and ask: the rest of this skill
   relies on committed config, which a `--mount-path` board doesn't have.
3. Read what the project already has:
   - `.devcontainer/devcontainer.json` or `.devcontainer.json`, in the project
     directory and the repo root
   - `.kanban.toml` or `.config/kanban/config.toml`, in both places
   - `.vscode/tasks.json` in the project directory
   - the project's README and build files, to find its dev server commands
4. Check which keys the user's config file sets, since they override the
   project's. List only the names, because values can include secrets:

   ```sh
   kanban config list --global --json | jq -r '.entries[].key'
   ```

   Tell the user about any key that will override a value you're about to set.

## 2. Settle the plan

Infer what you can. Ask the user only for what you can't, and in one go:

- the container image
- which tasks to define, if the project has any, and the port each dev server
  listens on
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

Kanban reads `.kanban.toml` and `.config/kanban/config.toml`. If the user named one
of them, use it. Otherwise use the existing file in the directory you're
configuring. If there's none, create `.config/kanban/config.toml` if that directory
has a `.config/`, and `.kanban.toml` otherwise.

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

- **If there is one**, kanban ignores `[devcontainer].image`, `docker_socket`,
  `claude_config`, and `post_start_command`. Change `image` in the `devcontainer.json` instead, or ask the
  user if it uses a `build` section. To use `[devcontainer].image`, the file has to
  go: see below.

The image needs the CLI of the user's agent, such as `claude`, and a `dev` or `root`
user. A session still starts without either the CLI or a login that works, so step
6 checks both. Under rootless Docker, kanban runs as `root`, which maps to the host
user. Otherwise it runs as `root` if that owns `~/.claude`, and as `dev` if not,
which only works if the image's `dev` has the UID that owns the credentials. If it
doesn't, the user can set `DEVCONTAINER_REMOTE_USER=root` on the server, which
applies to every board.

`mounts`, `run_args`, and `[devcontainer.container_env]` work in both cases. Leave
`docker_socket` off unless the user asks for it: it gives the agent root-level
access to their machine.

### Replacing a devcontainer.json with an image

Removing the file changes more than the image, and nothing warns about what stops
working. Before removing it, compare it with this table and tell the user what
they'd lose. Treat a lost security control, such as a firewall whose
`postStartCommand` relies on the old image or user, as a decision for them, not a
detail.

| In `devcontainer.json`                       | Without it |
| -------------------------------------------- | ---------- |
| `runArgs`, `mounts`, `containerEnv`          | Move to `run_args`, `mounts`, and `[devcontainer.container_env]`. |
| `postStartCommand`                           | Move to `post_start_command`, which runs the same way, in the same place. It needs the same `run_args`, and `sudo` if the session's user won't be root. |
| `build`                                      | Lost. Build and publish the image yourself. |
| `remoteUser`, `containerUser`                | Lost. See the user rules above. |
| `workspaceMount`, `workspaceFolder`          | Lost. Kanban picks the mount. |
| `postCreateCommand` and other lifecycle hooks, `features` | Kanban never ran these, so nothing changes. They only ran in other tools, such as VS Code. |

Also tell them that `claude_config` is on by default for an image, so sessions start
mounting their `~/.claude` read-write unless it's set to `false`.

Other files can depend on the directory too, so check before deleting it, and
resolve what you find:

```sh
git grep -n devcontainer -- ':!.devcontainer' ':!.devcontainer.json'
```

### Tasks

Tasks are optional. Skip this section if the project has no dev servers or other
commands worth running from kanban, as with a desktop game.

Kanban runs tasks from `.vscode/tasks.json` in the project directory. Add any
that are missing, and keep what's already in the file:

```json
{
  "version": "2.0.0",
  "tasks": [
    { "label": "Start Frontend", "type": "shell", "command": "bun run dev --host 0.0.0.0" }
  ]
}
```

Servers must listen on `0.0.0.0` to be reachable from outside the container.

To give a task a URL on the host, map its label to the port it listens on in the
container. A label that doesn't match exactly leaves the task with no URL and no
error, so give every mapped task an explicit `label`. Generated names are hard to
guess: an `npm` task with a `path` is named `npm: dev - .`, not `npm: dev`.

```toml
[[task]]
label = "Start Frontend"
container_port = 3000
```

### Secrets

Keep secrets out of the config, which gets committed. Have the user run this
themselves, so the value stays out of this conversation too:

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

Offer to test it with a throwaway ticket. Ask first, because this pulls the image
and starts a container:

```sh
kanban ticket create --board <slug> --title "Check the dev environment" --json
kanban ticket tasks <id> --run "<label>" --detach
kanban ticket tasks <id> --json
kanban ticket info <id> --json
docker exec <container_name> sh -c 'command -v <agent>; id; test -r "$HOME/.claude/.credentials.json" && echo readable'
```

If the project has no tasks, start the session through the API instead of
`--run`, and skip `ticket tasks`. The first call only creates the session, whose
ID it returns:

```sh
curl -fsS -X POST "$KANBAN_URL/api/tickets/<id>/session"
curl -fsS -X POST "$KANBAN_URL/api/sessions/<session id>/start"
```

`$KANBAN_URL` defaults to `http://localhost:7474`. Check each of these:

- `tasks` lists every task you expect, and `warnings` is empty.
- Each mapped task has `has_port: true`, and `last_run.status` is `running`.
- `info` shows `proxy_active: true` for each running task's port. If not, the `--run`
  output says why.
- The `docker exec` output shows the agent's CLI and the expected user.
- It also prints `readable`, if the host has `~/.claude/.credentials.json` and
  `claude_config` is on. If not, every session will ask the agent to log in: see the
  user rules in step 4. A correct `id` doesn't rule this out.

Clean up with `kanban ticket archive <id> --delete`, which also removes the
ticket's container, worktree, and branch.

Finish by telling the user the board's slug, the image, each task with its port,
what you committed, what the test showed or that you skipped it, anything that
stopped working because you removed a `devcontainer.json`, and anything still left
for them to do.
