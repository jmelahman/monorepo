# Configuration

Kanban reads settings from two TOML files that share the same schema:

- **Project**: `<repo>/.kanban.toml`, checked into the repo. It applies to every ticket on that repo's boards.
- **User**: `~/.config/kanban/config.toml` (or `$XDG_CONFIG_HOME/kanban/config.toml`). It holds your personal settings across all repos.

Both are optional. When both set a key, the user file wins. On a [monorepo](./monorepos) board, a `.kanban.toml` in the subproject sits between the two. It overrides the repo root's file and is overridden by yours.

To use a different user file, pass `kanban serve --config <path>` or set `$KANBAN_CONFIG`.

## Example

Every key is shown below with its default unless noted.

```toml
[harness]
id = "claude"                 # default agent: "claude" or "pi"

[worktrees]
root = "/path/to/worktrees"   # where new worktrees go (unset by default)

[branches]
prefix = "kanban"             # ticket branches are named <prefix>/<ticket-slug>

[plans]
dir = "~/.claude/plans"       # directory shown in the ticket's "plans" tab

[sync]
allow_rebase = true
allow_merge  = true

[merge]
allow_merge_commit = true
allow_squash       = true
allow_rebase       = false
default_strategy   = "squash" # unset by default
ai_commit_message  = false

[git]
sign_commits = false

[github]
auto_move     = true
draft_column  = "In Progress"
review_column = "Review"
done_column   = "Done"
closed_column = ""            # empty: closed PRs don't move the ticket

[devcontainer]
image         = "ghcr.io/me/devbox:latest"  # bundled image only
docker_socket = false                       # bundled image only
claude_config = true                        # bundled image only
mounts        = ["type=bind,source=/tmp/ssh-agent.sock,target=/tmp/ssh-agent.sock"]
run_args      = ["--cap-add=SYS_PTRACE"]

[devcontainer.container_env]
SSH_AUTH_SOCK = "/tmp/ssh-agent.sock"

[[task]]
label          = "Start Frontend"
container_port = 3000

[buildcop]
enabled  = false
interval = "2m"

[[buildcop.boards]]
repo_path = "/workspace"
branch    = "master"
```

Two more sections are mainly for people working on kanban itself: `[errors]` files server errors as tickets on a board, and `[dev_toolbar]` adds a frontend performance widget to the app settings. Both are off by default.

## How the files merge

Most keys merge one at a time: a key in the user file replaces the same key in the project file, and everything else is kept. Three keys work differently:

- `[devcontainer].mounts` and `run_args` are **appended** to each other and to whatever `devcontainer.json` declares.
- `[[task]]` entries merge by `label`. A user entry replaces a project entry with the same label.
- `[[buildcop.boards]]` in the user file replaces the project's list entirely.

## Editing config from the CLI

You can edit either file by hand, or use `kanban config`, which works like `git config`. It needs a running server.

```sh
kanban config list                                  # merged view, with where each value comes from
kanban config set sync.allow_rebase true --global   # your user file
kanban config set branches.prefix feat --local --board playground  # the board's .kanban.toml
kanban config unset github.draft_column --global
```

Writes rewrite the whole file, so comments and key order are lost. The same operations are available over [REST](/reference/api#config) and [MCP](/reference/mcp#config).

## Agent harness

`[harness].id` sets the default agent. To use a different one on a single ticket, pick it on the **Harness** row of `kanban ticket create` or `kanban ticket attach`, or pass `--harness <id>`. The choice is saved on the ticket's session and overrides both config files. Switching stops an agent that's already running. See [`ticket attach`](/reference/cli#ticket-attach-id).

## Sync and merge

`[sync]` and `[merge]` control which options appear in the ticket's **Sync** and **Merge** menus. Sync uses rebase unless `allow_rebase = false`, in which case it merges.

### Default merge strategy

`[merge].default_strategy` is used when a merge doesn't name a strategy, for example `kanban ticket merge` without `--strategy`. It's also listed first in the merge menu.

If it's unset and the board allows only one strategy, that one is used. Otherwise the merge is rejected and the error lists the allowed strategies. Setting a default that the same config disables is an error.

### Commit messages

When kanban merges a ticket that has uncommitted changes, it commits them first using the ticket title as the message. Set `ai_commit_message = true` to have the agent write a one-line message from the diff instead. If that fails or takes longer than 90 seconds, kanban falls back to the title.

### Commit identity

Kanban makes merge and squash commits itself, so it needs a git name and email. If none is configured where kanban runs, the merge fails with `Author identity unknown`. Either:

- Fill in **Commit identity** in the board settings. This applies only to that board and takes priority.
- Or make your `~/.gitconfig` visible to kanban. The [Docker install](./install#with-docker) mounts it already.

### Signing

Kanban turns off commit signing for its own commits so merges don't fail when no signing key is available. To sign them, mount your signing key and agent into the container, then set `[git] sign_commits = true` or turn on **Sign commits** in the app settings. Kanban then follows your gitconfig's `commit.gpgsign`.

## GitHub

When `[github].auto_move` is on, kanban moves a ticket when its pull request changes state:

| PR state | Column key      | Default       |
| -------- | --------------- | ------------- |
| Draft    | `draft_column`  | `In Progress` |
| Open     | `review_column` | `Review`      |
| Merged   | `done_column`   | `Done`        |
| Closed   | `closed_column` | (no move)     |

Kanban reads a GitHub token from `$GH_TOKEN`, then `$GITHUB_TOKEN`, then `gh auth token`. Set `$GITHUB_API_URL` for GitHub Enterprise Server.

## Devcontainer

Kanban uses the first `devcontainer.json` it finds in the repo's `.devcontainer/devcontainer.json`, the repo's `.devcontainer.json`, or `~/.config/kanban`. If there isn't one, it uses a bundled Ubuntu image.

`[devcontainer]` adds to whichever one is used:

- `mounts` and `run_args` add bind mounts and `docker run` arguments.
- `container_env` adds environment variables. Kanban's own variables win on conflict.

Three keys only affect the bundled image, since a `devcontainer.json` you write manages these itself:

- `image` replaces the bundled image. It should include the same `dev` (UID 1000) or `root` user; see [Session container user](#session-container-user).
- `docker_socket` mounts the host's Docker socket into the session. It's off by default because it gives the agent root-level access to your machine.
- `claude_config` mounts your `~/.claude` so Claude Code is already logged in. It's on by default. `kanban serve --claude-config=false` or `$KANBAN_CLAUDE_CONFIG` overrides it.

## Task ports

Kanban runs the tasks in the repo's `.vscode/tasks.json`. To reach a task's server from your browser, map its label to a container port:

```toml
[[task]]
label          = "Start Frontend"
container_port = 3000
```

When that task runs, kanban picks a free host port between 13000 and 13099 and forwards it to port 3000 in the container. The ticket shows the URL. Change the range with `kanban serve --port-range-start` and `--port-range-end`.

On a monorepo board, tasks come from the subproject's `.vscode/tasks.json`, so put the matching `[[task]]` entries in the subproject's `.kanban.toml`.

## Plans

The ticket's **plans** tab lists the markdown files in `[plans].dir`. It only appears when that directory has at least one `.md` file. An absolute path is shared by every ticket. A relative path such as `./plans` is resolved inside each ticket's worktree, so each ticket sees its own plans.

## Board environment variables

Board environment variables are set in every session container on a board. Use them for secrets the agent's tools need, such as an API key for an MCP server. Unlike `[devcontainer].container_env`, they're kept out of `.kanban.toml`, so they can't be committed by accident.

```sh
kanban env set playground MY_API_KEY=sk-abc123
kanban env list playground     # prints key names only
kanban env unset playground MY_API_KEY
```

You can also manage them from the **env** tab in the board settings.

- **Values can't be read back.** Nothing in kanban displays a stored value. To change one, set it again.
- **Values are encrypted on disk** with a key stored at `<data-dir>/secrets.key`. Back up that file along with `kanban.db`. If you lose it, you'll need to set the values again.
- **Changes apply the next time a session starts or restarts.**

Anyone who can `docker inspect` the session container can see the values, and so can the agent. Only give a board the secrets its sessions need.

If a key is set in more than one place, board variables override `container_env` and `devcontainer.json`. Kanban's own `KANBAN_*` variables override everything, and you can't set keys with that prefix.

## Build Cop

Build Cop watches GitHub Actions and files a ticket when a job fails too often. Each `[[buildcop.boards]]` entry creates a board that Build Cop manages, with the columns `Failing`, `Investigating`, `Fixed`, and `Won't fix`.

```toml
[buildcop]
enabled  = true
interval = "2m"                  # how often to check GitHub

[[buildcop.boards]]
name                  = "Build Cop: master"  # default: "Build Cop: <branch>"
repo_path             = "/workspace"         # required; a checkout with a GitHub remote
branch                = "master"             # "" or "*" for every branch
failure_threshold     = 0.10                 # file a ticket above this failure rate
min_runs              = 5                    # runs needed before judging a job
window_days           = 7                    # how far back to look
flaky_threshold       = 3                    # pass-on-retry count that files a flake ticket
green_streak_required = 10                   # clean runs before moving to Fixed
```

A ticket moves to `Fixed` on its own once the job passes `green_streak_required` times in a row. To stop tracking a job, drag its ticket to `Won't fix`. Build Cop won't touch it again until you move it out.

**Flaky jobs.** A job that fails and then passes when re-run on the same commit counts as one pass-on-retry. Several retries of one run still count once. When a job reaches `flaky_threshold` in the window, Build Cop files a ticket for it, such as `CI / lint flaky (3 passes-on-retry over 40 runs)`. A flaky run also resets the job's green streak.

**Rate limits.** Build Cop needs a GitHub token (see [GitHub](#github)). Without one, GitHub allows 60 requests an hour, which isn't enough for even a small repo. If you hit the limit of 5,000 an hour, raise `interval`. [Observability](./observability#github-rate-limits) shows how to check usage.

## Claude Code resume

When a session container restarts, Claude Code picks up the previous conversation. This works whenever `~/.claude` is mounted into the container, which the bundled image does by default.

To start a fresh conversation instead, clear the saved conversation ID:

```sh
sqlite3 "$KANBAN_DATA_DIR/kanban.db" \
  "UPDATE sessions SET claude_session_id = NULL WHERE id = <session_id>"
```

Resume relies on a hook in `.claude/settings.local.json`, which kanban writes into the worktree. Kanban won't overwrite an existing file, so if your repo already has one, resume is off. Delete the file and restart the session to turn it on. Only Claude Code supports resume. Other harnesses start fresh each time.

## Running kanban inside a container

If kanban runs in a container but starts sessions through the host's Docker, the paths it passes to Docker must be host paths. These variables tell kanban how to translate them:

| Variable                  | Host path of                                                                 |
| ------------------------- | ---------------------------------------------------------------------------- |
| `KANBAN_HOST_WORKSPACE`   | `/workspace` in kanban's container. Covers board repos, worktrees, and mounts under it. |
| `KANBAN_HOST_HOME`        | `$HOME` in kanban's container. Covers `~/.claude` and `~/.claude.json`.       |
| `KANBAN_HOST_DOCKER_SOCK` | The Docker socket. Needed when it isn't at `/var/run/docker.sock` on the host, as with rootless Docker. |

For example:

```sh
export KANBAN_HOST_WORKSPACE=/home/me/code/kanban
export KANBAN_HOST_HOME=/home/me
export KANBAN_HOST_DOCKER_SOCK=/run/user/1000/docker.sock
```

On a normal install none of these are needed.

### Session container user

The bundled image has two users, `root` (UID 0) and `dev` (UID 1000). Kanban runs the session as whichever one owns your `~/.claude`, so Claude Code can read and write its credentials. If neither matches, as with UID 501 on macOS, kanban uses `dev`. Claude Code then asks you to log in on every new session.

To fix that, `chown` your `~/.claude` to UID 1000, or set the user yourself:

| Variable                   | Effect                                                         |
| -------------------------- | -------------------------------------------------------------- |
| `DEVCONTAINER_REMOTE_USER` | User to run as (`dev` or `root`). The home directory follows.  |
| `DEVCONTAINER_REMOTE_HOME` | Home directory to use. The user is still picked automatically. |
