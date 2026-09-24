# Configuration

Kanban reads two TOML files and merges them, with **user values overriding project values per key**:

- **Project**: `<repo>/.kanban.toml` — checked into the target repo, applies to every worktree of that repo.
- **User**: `$XDG_CONFIG_HOME/kanban/config.toml` (falling back to `~/.config/kanban/config.toml`) — your personal overrides across all repos.

Either file may be absent. Both accept the same schema.

A board scoped to a monorepo subdirectory adds a third layer in the middle,
so precedence runs **repo root → subproject → user**: `<repo>/<project_dir>/.kanban.toml`
overrides the repo-wide file key by key and inherits everything it doesn't
set. See [Monorepos](/guide/monorepos).

## Schema

```toml
[harness]
id = "claude"                 # default agent harness: "claude" or "pi"

[worktrees]
root = "/path/to/worktrees"   # parent dir for new worktrees (overrides --worktrees-dir)

# How kanban makes its own merge/squash commits. sign_commits is off by
# default: kanban forces signing off (`-c commit.gpgsign=false`) so merges
# never fail when the container has no signing key. Turn it on — also via the
# App Settings dialog — only when you've mounted your signing key/agent into
# the container and want kanban's commits signed; kanban then defers to your
# gitconfig's commit.gpgsign.
[git]
sign_commits = false

# Directory the per-ticket "plans" tab reads. The tab only appears when the
# directory has at least one `.md` file. An absolute path is shared across
# every ticket. A relative path (e.g. "./plans") is resolved against each
# session's worktree, so each ticket sees its own plans — Claude Code's
# `/plan` mode writes there by default.
[plans]
dir = "~/.claude/plans"        # default; expand-home is applied

[branches]
prefix = "kanban"              # branch name = "<prefix>/<ticket-slug>"

[sync]
allow_rebase = true            # offer "rebase onto base" in the sync menu
allow_merge  = true            # offer "merge base into branch"

[merge]
allow_merge_commit = true      # which strategies appear in the merge menu
allow_squash       = true
allow_rebase       = false
ai_commit_message  = false     # opt in to harness-generated messages for the auto-commit (default off — uses ticket title)
default_strategy   = "squash"  # used when a merge names no strategy (unset by default)

[github]
auto_move     = true           # move tickets when the linked PR/issue changes state
draft_column  = "In Progress"
review_column = "In Review"
done_column   = "Done"
closed_column = "Done"

# Toggles the in-process error-to-ticket reporter. Off by default — typically
# only useful to developers maintaining kanban itself.
[errors]
enabled    = false
board_name = "kanban-errors"

# Opt in to the in-app developer toolbar. Off by default. When enabled, a
# "Developer" tab appears in App Settings with a "Show developer toolbar"
# toggle for a small corner widget showing live frontend health: FPS / worst
# frame time, JS heap usage (Chromium only) plus a cross-browser loaded-asset
# size estimate, DOM node and React Query cache counts (with growth deltas that
# flag possible leaks), fetch/mutation activity, SSE connection status, and
# in-flight request count.
# Which sections show and where the widget pins are per-browser preferences
# (localStorage), so this flag only controls whether the feature is available at
# all.
[dev_toolbar]
enabled = false

# Build Cop polls GitHub Actions on a schedule and files tickets when a job's
# failure rate over a rolling window exceeds the threshold, or when the same
# job passes on retry too often (a flake signal — see below). Off by default.
# Each [[buildcop.boards]] entry produces one auto-managed board scoped to
# the given branch filter; columns are "Failing" / "Investigating" / "Fixed" /
# "Won't fix". A job auto-moves to "Fixed" once it has `green_streak_required`
# consecutive successful runs — flake events count as non-green for that
# purpose. Drag a ticket to "Won't fix" to silence a job you've decided not to
# address (a known-flaky test, an infra failure): the poller never touches a
# ticket parked there — it won't re-open it on continued failure or auto-move
# it to "Fixed" on recovery — until you move it out yourself. ("Won't fix" is
# backfilled onto boards created before it existed on the next poll.)
[buildcop]
enabled  = false
interval = "2m"  # poll cadence; default 2m. Each tick lists completed runs
                 # in `window_days`, then fetches jobs for any uncached run
                 # (a `/repos/.../actions/runs/{id}/jobs` call per new run).
                 # Bump this if you're hitting GitHub's 5000/hr REST rate
                 # limit — observability metrics live at `/metrics`.

[[buildcop.boards]]
name                  = "Build Cop: master"  # optional — defaults to "Build Cop: <branch>"
repo_path             = "/workspace"         # required: absolute path to a checkout with a GitHub origin
branch                = "master"             # "" or "*" matches every branch
failure_threshold     = 0.10                 # rate (0..1) above which a ticket is filed
min_runs              = 5                    # minimum runs in the window before evaluating
green_streak_required = 10                   # consecutive green runs to auto-move to Fixed
window_days           = 7                    # rolling window in days
flaky_threshold       = 3                    # passes-on-retry in the window that trigger a flake ticket

# Extra knobs layered onto the worktree's devcontainer.json at session spawn.
# `mounts` and `run_args` append to whatever the devcontainer.json declares;
# `container_env` merges with kanban values winning. `docker_socket` and
# `claude_config` only affect the built-in fallback devcontainer —
# hand-written devcontainer.json files manage their own mounts.
# `docker_socket` defaults to false because forwarding the host daemon
# grants the session agent root-equivalent authority on the host; opt in
# only when a session legitimately needs to drive Docker. `claude_config`
# defaults to true and can also be forced from the CLI via
# `--claude-config` or `$KANBAN_CLAUDE_CONFIG`, which take precedence over
# the toml value (useful when running kanban inside a devcontainer that
# already mounts `~/.claude` at a path the host's docker daemon can't see).
[devcontainer]
mounts        = ["type=bind,source=/tmp/ssh-agent.sock,target=/tmp/ssh-agent.sock"]
run_args      = ["--cap-add=SYS_PTRACE"]
docker_socket = false          # built-in only: bind /var/run/docker.sock into the container
claude_config = true           # built-in only: bind ~/.claude into the container

[devcontainer.container_env]
SSH_AUTH_SOCK = "/tmp/ssh-agent.sock"

# Per-task ports: associate .vscode/tasks.json labels with container ports.
# When such a task runs, kanban allocates a host port from 13000-13099 and
# runs a TCP proxy.
[[task]]
label = "Start Frontend"
container_port = 3000

[[task]]
label = "Start Backend"
container_port = 8080
```

### Build Cop flake detection

The GitHub Actions runs API only returns the *latest* attempt of each workflow run, so a job that fails on attempt 1 and passes on attempt 2 looks green to a naive poller. Build Cop closes that gap: for every workflow run in the window with `run_attempt > 1` ending in `success`, it fetches each prior attempt's jobs and records a *pass-on-retry* for every job that failed earlier and still exists in the successful attempt. Because all attempts of a single run share `head_sha`, the code didn't change between attempts — the failure is a flake.

The counter (`flaky_threshold` compares against this) is **one per commit**, not one per failed attempt: a single run whose `lint` job flaked twice before turning green on attempt 3 still contributes `1`, so the count reflects the true rate of flaky commits over the window. When it reaches `flaky_threshold` for a `(workflow, job)` pair, a ticket is filed in the `Failing` column with a title like `CI / lint flaky (3 passes-on-retry over 40 runs)`. If the same job also fails outright often enough to trip `failure_threshold`, the ticket title becomes `failing+flaky` and lists both counts. Flake events also break the green streak: a flaky job can only reach `Fixed` after `green_streak_required` consecutive clean, non-flaky runs.

### Build Cop authentication

The poller calls the GitHub REST API the same way the PR-state poller does: it reads `$GH_TOKEN`, falls back to `$GITHUB_TOKEN`, then shells out to `gh auth token`. Without any of those, requests go out unauthenticated and hit the 60-requests-per-hour-per-IP limit almost immediately. For a typical board (one workflow, ~50 runs in 7 days) the poller needs a few hundred requests per hour, so use an authenticated token. `$GITHUB_API_URL` is honored for GitHub Enterprise Server hosts.

## Merge semantics

Most sections are object-merged: a key set in the user file wins; keys only set in the project file remain. Three sections behave specially:

- `[devcontainer].mounts` and `[devcontainer].run_args` are **appended** to whatever the worktree's `devcontainer.json` already declares. They aren't overrides.
- `[[task]]` entries merge by `label`: a user entry with the same `label` replaces the project entry, and user-only labels are appended.
- `[[buildcop.boards]]` is **replaced wholesale** when the user file sets any entries — board names can change and there's no stable identity to merge by, so the rule is "if the user declared boards, those are the boards."

## Choosing a harness per session

`[harness].id` is only the default. One ticket's session can run a different harness: pick it on the **Harness** row of `kanban ticket create` or `kanban ticket attach` (`←`/`→`), pass `--harness <id>` to either, or call `PUT /api/sessions/{id}/harness`. The choice is stored on the session and wins over both config files; clearing it (`""` through the API) puts the session back on the default. Switching stops an agent that is running another harness — see [`ticket attach`](/reference/cli#ticket-attach-id).

## Managing config from the API / CLI / MCP

Besides editing the TOML files by hand, every key above is readable and writable
through a `git config`-style surface — REST, CLI, and MCP — backed by a running
`kanban serve`. `--global` targets the user config; `--local --board <id-or-slug>`
targets that board's project `.kanban.toml`. Reads default to the merged
**effective** view, which also surfaces read-only runtime values (data dir,
ports).

```sh
kanban config list                                   # effective view
kanban config set sync.allow_rebase true --global
kanban config set branches.prefix feat --local --board playground
kanban config set devcontainer.run_args '["--cap-add=NET_ADMIN"]' --global
kanban config unset github.draft_column --global
```

Two caveats: writes rewrite the whole file, so **comments and key ordering are
not preserved**; and because `[devcontainer].mounts`/`run_args` *append* across
layers (see [merge semantics](#merge-semantics)), the effective view shows the
combined value. See the [CLI](/reference/cli#config), [REST](/reference/api#config),
and [MCP](/reference/mcp#config) references for the full surface.

## Per-board environment variables

Boards can carry environment variables that kanban injects into every session
container it launches for that board — the place for secrets an agent's
tooling needs at runtime, such as an API key consumed by an MCP server inside
the session. Unlike `[devcontainer].container_env` (plaintext in a
`.kanban.toml` that can end up committed to the repo), board env vars live in
kanban's local database and are treated as secrets:

- **Write-only.** No API, MCP tool, CLI subcommand, or UI view returns a
  stored value — only key names. Editing means overwriting or removing a key.
- **Encrypted at rest.** Values are sealed with AES-256-GCM before they touch
  the database. The key is generated on first use at `<data-dir>/secrets.key`
  (mode `0600`). Back that file up together with `kanban.db`; if it's lost,
  the stored values are unrecoverable and must be re-entered. With
  `--in-memory` the key is ephemeral and never written to disk.
- **Applied at container creation.** Changes take effect the next time a
  session is started or restarted — never inside a running container.

Manage them from the board settings **env** tab in the web UI, the CLI, or
MCP (`list_board_env` / `set_board_env` / `unset_board_env`):

```sh
kanban env set playground MY_API_KEY=sk-abc123
kanban env list playground     # → MY_API_KEY (never the value)
kanban env unset playground MY_API_KEY
```

Precedence when the same key is defined in multiple places (lowest to
highest): `devcontainer.json` `containerEnv` / `[devcontainer].container_env`
→ board env vars → the server-injected `KANBAN_*` variables. Keys must match
`[A-Za-z_][A-Za-z0-9_]*`, and the `KANBAN_` prefix is rejected as reserved.

Two limits worth knowing: encryption protects the database file, not the
running container — anyone who can run `docker inspect` on a session container
(or read `/proc/<pid>/environ` inside it) can see the values, which is
inherent to environment-variable delivery. And the agent inside a session can
of course read its own environment; only give a board secrets its sessions are
meant to use.

## Overriding the user-config path

```sh
kanban serve --config /path/to/config.toml
# or
KANBAN_CONFIG=/path/to/config.toml kanban serve
```

The `--config` flag wins over `$KANBAN_CONFIG`.

## Per-task ports

Kanban understands `.vscode/tasks.json` in the target repo. When a task whose `label` matches a `[[task]]` entry starts inside a session, kanban allocates the next free host port in the range (default `13000–13099`) and runs a TCP proxy from that host port to the declared `container_port`. The host port is shown on the ticket so you can open the WIP service in your browser.

Adjust the proxy range with `--port-range-start` and `--port-range-end` on `kanban serve`.

On a board with a `project_dir`, `.vscode/tasks.json` is read from the
subproject rather than the repo root, so the `[[tasks]]` entries that pair
with it belong in the subproject's `.kanban.toml`.

## Resuming Claude Code sessions across restarts

When a session container is restarted (or kanban itself is restarted and the container has to be recreated), the next `claude` launch automatically resumes the prior conversation for that ticket. There's nothing to configure — the mechanism is on by default whenever `~/.claude` is bind-mounted into the session container (the built-in devcontainer does this; see `claude_config = true` above).

How it works:

- On every `claude` startup, the bundled `.claude/settings.local.json` `SessionStart` hook PATCHes `/api/sessions/{id}/claude-session` with the conversation's UUID.
- Kanban stores the UUID on the session row.
- The next time the agent terminal is attached, kanban launches `claude --resume <uuid>` instead of a bare `claude`, so the transcript at `~/.claude/projects/<cwd-hash>/<uuid>.jsonl` is reopened.

To force a fresh conversation for a ticket, clear the stored UUID:

```sh
sqlite3 "$KANBAN_DATA_DIR/kanban.db" \
  "UPDATE sessions SET claude_session_id = NULL WHERE id = <session_id>"
```

The next attach will start a brand-new Claude Code session.

Caveats:

- Kanban only writes `.claude/settings.local.json` when no file is present. If one already exists — either hand-authored or shipped by an older kanban that didn't install the `SessionStart` hook — it's left untouched, no UUID is captured, and resume is silently off. To opt in, delete the file and re-ensure the session.
- Only the `claude` harness is wired up; other harnesses (`pi`) have no equivalent flag and launch fresh every time.

## Commit identity for merges

When a session is merged or squash-merged, kanban shells out to `git commit` inside its own container. If that container has no `user.name` / `user.email` configured, git aborts with `Author identity unknown` — your host's `~/.gitconfig` isn't visible inside the container.

Two ways to fix it, in order of preference:

1. **Set it on the board.** In the board settings UI, fill in *Commit identity → Author name / Author email*. Both fields must be set; leaving either blank falls back to whatever `git config` resolves inside the kanban container. Stored per-board (so different repos can use different identities), persisted as `git_author_name` / `git_author_email` on the board, and passed through as `-c user.name=… -c user.email=…` on every commit kanban creates.
2. **Mount your host gitconfig into the kanban container.** In `compose.yaml`, add `${HOME}/.gitconfig:/root/.gitconfig:ro` alongside the existing volumes. Useful if you want one identity across every board and don't mind the volume.

The board-level setting wins when both are present (it's an explicit `-c` flag on the git invocation).

## Default merge strategy

`[merge].default_strategy` is the strategy used when a merge request names
none — `kanban ticket merge` with no `--strategy`, the `merge_ticket` MCP tool
with no `strategy`, or a `POST /api/tickets/{id}/merge` with an empty body. It
also leads the merge menu in the session pane, marked `(default)`.

Resolution order for an unnamed strategy:

1. `merge.default_strategy`, if set and enabled on this board.
2. The board's only enabled strategy, if exactly one is enabled — there is
   nothing to choose.
3. Otherwise the request is rejected with `strategy is required`, listing the
   enabled strategies.

Setting it to a strategy the same config disables is a misconfiguration: it is
reported as `default strategy <x> is disabled for this board` rather than
silently falling through. `[sync]` has no equivalent key — sync defaults to
`rebase`, falling back to `merge` when `allow_rebase = false`.

## AI-generated commit messages

When kanban auto-commits a session's pending changes at merge time, it uses the ticket title as the message. Set `[merge].ai_commit_message = true` to instead invoke the session's harness (e.g. `claude --model haiku`) to generate a one-line message from the staged diff. The call is gated by a 90-second timeout and falls back to the ticket title on any error. Off by default because not every harness ships a working template, and the round-trip can be slow.

## Running kanban inside a container

When kanban itself runs inside a container (e.g. a devcontainer) but spawns
session containers via the host's docker daemon, every bind-mount source
kanban hands to dockerd needs to be a **host** path. Paths kanban sees as
`/workspace/...` or `/root/.claude` mean nothing to the host's docker.

Set these env vars on the kanban process to translate prefixes before they
reach dockerd:

| Env var | What it rewrites |
| --- | --- |
| `$KANBAN_HOST_WORKSPACE` | Host path of `/workspace` inside the kanban container. Covers board `repo_path`, `worktree_root`, and any `[devcontainer].mounts` whose source lives under `/workspace`. |
| `$KANBAN_HOST_HOME` | Host path of the kanban container's `$HOME`. Covers `~/.claude` / `~/.claude.json` forwarding. |
| `$KANBAN_HOST_DOCKER_SOCK` | Host path of the docker socket. Needed when kanban's own `/var/run/docker.sock` is a bind of the host's rootless socket (e.g. `/var/run/user/1000/docker.sock`) — the in-container path is invalid as a bind source on the host. When unset, kanban probes `/var/run/docker.sock` then `$XDG_RUNTIME_DIR/docker.sock` directly. |

Example for a kanban devcontainer running as root with the project bind-mounted
from `/home/jamison/code/kanban`:

```sh
export KANBAN_HOST_WORKSPACE=/home/jamison/code/kanban
export KANBAN_HOST_HOME=/home/jamison
# Set on rootless-docker hosts where the socket isn't at /var/run/docker.sock:
export KANBAN_HOST_DOCKER_SOCK=/var/run/user/1000/docker.sock
```

Both are unset (and the translation is a no-op) on the default host install.
Paths that don't start with either prefix pass through untouched.

For full end-to-end terminal access, the worktree root also needs to be
reachable on the host at the translated path — i.e. `KANBAN_HOST_WORKSPACE`
must point at a directory the host's docker daemon can stat.

### Built-in session container user

When kanban spawns a session in the built-in devcontainer image, it picks
the in-container user by stat-ing the host's `~/.claude` and matching the
owner UID against the accounts the image ships: UID `0` → `root` (home
`/root`), UID `1000` → `dev` (home `/home/dev`). Other UIDs fall back to
`dev`.

The reason it matters: bind mounts preserve host UIDs verbatim, so if the
session ran as `dev` but `~/.claude` is root-owned (mode 0700 directory,
0600 credentials), `dev` couldn't read the credentials and Claude would
re-prompt `/login` every new session — and couldn't write the new
credentials back either, so the loop never broke.

Two env vars override the auto-pick when set. They resolve together, so
a session can't end up as `root` with `/home/dev` (or vice versa):

| Env var | Effect |
| --- | --- |
| `$DEVCONTAINER_REMOTE_USER` | In-container user (`dev` or `root`). If only this is set, the home is derived from a known-user table. |
| `$DEVCONTAINER_REMOTE_HOME` | In-container home prefix. If only this is set, the user still comes from the `~/.claude` auto-pick. |

For host UIDs the image doesn't ship an account for (e.g. macOS users
with UID 501), the auto-pick falls back to `dev`. Set the env vars
explicitly or chown the bind source to avoid the re-auth loop on those
hosts.

## See also

- [CLI reference](/reference/cli) — every flag for `serve`, `mcp`, `board list`, `ticket create`, `config set`.
- [REST API](/reference/api) — endpoints exposed by the running server, including `/api/config`.
- [MCP reference](/reference/mcp) — the `*_config` tools and the rest of the agent-facing surface.
- [Previews](/guide/previews) — per-branch preview deployments and `$KANBAN_PREVIEW_DOMAIN`.
- [Monorepos](/guide/monorepos) — scoping a board to a subdirectory of the repo.
