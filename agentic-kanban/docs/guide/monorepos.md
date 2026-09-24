# Monorepos

A board normally treats the repository root as the project. For a monorepo
you usually want the opposite: one board per subproject, each agent working
from `services/api` or `web` rather than from the top of the tree.

That is what a board's **project directory** does. Set `project_dir` to a
repo-relative subdirectory and the whole repository is still checked out and
mounted — the agent just starts, and stays, one level down.

## Setting it

Run `board create` from inside the subproject and both the repo and the
project directory are inferred, along with the board name:

```sh
cd ~/code/monorepo/services/api
kanban board create
# repo_path=~/code/monorepo  project_dir=services/api  name=api
```

Or spell it out from anywhere:

```sh
kanban board create \
  --repo-path ~/code/monorepo \
  --project-dir services/api \
  --name api
```

In the UI the field lives on the **Git** tab of board settings, under
"Repository path". Clearing it turns the board back into a whole-repo board.

Two rules are enforced everywhere boards are written — HTTP, MCP and CLI:

- `project_dir` needs a `repo_path`, since it names a subdirectory of one.
- `project_dir` and `mount_path` are mutually exclusive. `mount_path` makes
  the bind source the **main checkout**, so descending into a subdirectory
  from there would land outside the per-ticket worktree and silently drop
  branch isolation.

Absolute paths and anything escaping the repo with `..` are rejected.

## What actually changes

Each ticket still gets its own git worktree of the **whole** repository, and
that worktree is still bind-mounted at the devcontainer's `workspaceFolder`
(`/workspace` unless your `devcontainer.json` says otherwise). Only the
working directory moves:

| | Whole-repo board | `project_dir = services/api` |
| --- | --- | --- |
| Bind source | `<worktree>` | `<worktree>` |
| Mounted at | `/workspace` | `/workspace` |
| Agent's cwd | `/workspace` | `/workspace/services/api` |

So `git rev-parse --show-toplevel` from the agent's shell still resolves —
to `/workspace` — and `--show-prefix` reports `services/api/`. Nothing about
the checkout is trimmed; a subproject that imports a sibling, or a `go.work`
at the root, keeps working.

### Configuration is looked up in the subproject first

| What | Where kanban looks |
| --- | --- |
| `devcontainer.json` | The subproject's `.devcontainer/devcontainer.json`, then its `.devcontainer.json`, then the same pair at the repo root, then `~/.config/kanban`, then the built-in image. |
| `.kanban.toml` | Merged root → subproject → user file, so a subproject overrides repo-wide keys and inherits the ones it doesn't set. |
| `.vscode/tasks.json` | The subproject only. Tasks run from the agent's cwd, so discovering them anywhere else would resolve every `${workspaceFolder}`-relative `cwd` against the wrong tree. |
| Dockerfile / build context | The subproject, so a `Dockerfile` beside a subproject's `devcontainer.json` wins over the monorepo root's. |
| Plans directory | A relative `plans.dir` (e.g. `./plans`) resolves under the subproject. An absolute one — including the `~/.claude/plans` default — stays global. |
| `.claude/settings.local.json` | Written into the subproject, because that is the agent's cwd. Without this the session-status hooks never fire. |

A board-level task list lives in the subproject's `.vscode/tasks.json`, so
the matching `[[tasks]]` port mappings should live in the subproject's
`.kanban.toml` too.

### Git stays repo-wide

Diffs, `git add -A`, sync and merge all operate on the whole worktree, not
just the subdirectory. An agent working on `services/api` legitimately edits
the root `go.work`, shared CI config or a sibling library, and scoping git to
a pathspec would silently drop those changes from the commit.

## Board auto-detection

Subcommands that take an optional `[id]` — `ticket create`, `attach`,
`session`, `board get`/`state`/`archived` — infer the board from the current
directory. With several boards on one monorepo the repo path alone is
ambiguous, so kanban also compares the current directory against each
board's `project_dir` and keeps the **longest match**:

```
~/code/monorepo                  -> the board with no project_dir
~/code/monorepo/services/api     -> the services/api board
~/code/monorepo/services/api/db  -> the services/api board
```

A board with an empty `project_dir` is the repo-wide catch-all, so
single-board repos behave exactly as before. If no board covers the current
directory — or two boards share the same `project_dir` — the command asks
for an explicit id or slug rather than guessing.

## Upgrading an existing board

Before `project_dir` existed, the only way to point a board at a
subdirectory was to set `mount_path` to a child of `repo_path`. That
configuration is broken: it binds the *main checkout*, so the agent edits
your checked-out branch instead of the ticket's worktree, `/workspace` is
not a git repository, and the status hooks never fire.

Those boards convert themselves the first time the upgraded server
starts: a
`mount_path` that is a strict descendant of `repo_path` becomes
`project_dir`, and `mount_path` is cleared. A `mount_path` equal to,
above, or unrelated to `repo_path` is left exactly as it was — including
the "parent of several repos" pattern, which `project_dir` does not
replace.

Restart any session that was created before the conversion; mounts and the
working directory are fixed when the container is created, so a running
container keeps the directory it started with.

## See also

- [Configuration](/guide/configuration) — the full `.kanban.toml` schema.
- [CLI reference](/reference/cli) — `--project-dir` and board inference.
- [REST API](/reference/api) — the `project_dir` board field.
