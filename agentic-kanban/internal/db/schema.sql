CREATE TABLE IF NOT EXISTS boards (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  slug TEXT NOT NULL UNIQUE,
  repo_path TEXT,
  mount_path TEXT,
  -- project_dir scopes the board to a subdirectory of repo_path (a monorepo
  -- subproject). Repo-relative and slash-separated; the whole repo is still
  -- checked out and mounted, but the agent's working directory is this
  -- subdirectory. Mutually exclusive with mount_path, which names an absolute
  -- host path and is meant for mounting a *parent* of the repo. Added by
  -- migrate() on older databases.
  project_dir TEXT,
  worktree_root TEXT,
  base_branch TEXT NOT NULL DEFAULT 'main',
  branch_prefix TEXT,
  git_author_name TEXT,
  git_author_email TEXT,
  created_at INTEGER NOT NULL DEFAULT (unixepoch()),
  position INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS columns (
  id INTEGER PRIMARY KEY,
  board_id INTEGER NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  position INTEGER NOT NULL,
  UNIQUE(board_id, position)
);

CREATE TABLE IF NOT EXISTS tickets (
  id INTEGER PRIMARY KEY,
  board_id INTEGER NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
  column_id INTEGER NOT NULL REFERENCES columns(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  slug TEXT NOT NULL,
  body TEXT NOT NULL DEFAULT '',
  position INTEGER NOT NULL,
  created_at INTEGER NOT NULL DEFAULT (unixepoch()),
  archived_at INTEGER,
  fingerprint TEXT,
  UNIQUE(board_id, slug)
);

CREATE TABLE IF NOT EXISTS sessions (
  id INTEGER PRIMARY KEY,
  ticket_id INTEGER NOT NULL UNIQUE REFERENCES tickets(id) ON DELETE CASCADE,
  worktree_path TEXT NOT NULL,
  branch_name TEXT NOT NULL,
  container_id TEXT,
  container_name TEXT,
  status TEXT NOT NULL DEFAULT 'stopped'
    CHECK (status IN ('stopped','starting','idle','working','awaiting_perm','error')),
  started_at INTEGER,
  stopped_at INTEGER,
  -- pr_state is NULL until the session has an associated PR.
  pr_state TEXT
    CHECK (pr_state IS NULL OR pr_state IN ('draft','open','merged','closed')),
  pr_number INTEGER,
  pr_url TEXT,
  pr_title TEXT,
  -- mount_path and repo_path are per-session overrides of the board's paths,
  -- read by session.ResolvePaths. Nothing assigns them today; they exist so a
  -- session can be pinned to paths that outlive a board edit.
  mount_path TEXT,
  repo_path TEXT,
  claude_session_id TEXT,
  -- harness is the agent harness ID chosen for this session; NULL means the
  -- user/project default. Added by migrate() on older databases.
  harness TEXT,
  -- workspace_folder is the container path the agent's working directory was
  -- created at, snapshotted when the container was spawned: the devcontainer's
  -- workspaceFolder joined with the board's project_dir. NULL on rows predating
  -- this column, which fall back to /workspace. Added by migrate().
  workspace_folder TEXT
);

CREATE TABLE IF NOT EXISTS port_allocations (
  id INTEGER PRIMARY KEY,
  session_id INTEGER NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
  label TEXT NOT NULL,
  container_port INTEGER NOT NULL,
  host_port INTEGER NOT NULL UNIQUE,
  proxy_active INTEGER NOT NULL DEFAULT 0,
  UNIQUE(session_id, label)
);

CREATE TABLE IF NOT EXISTS task_runs (
  id INTEGER PRIMARY KEY,
  session_id INTEGER NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
  task_label TEXT NOT NULL,
  command TEXT NOT NULL,
  exec_id TEXT,
  status TEXT NOT NULL DEFAULT 'running'
    CHECK (status IN ('running','exited','stopped')),
  exit_code INTEGER,
  started_at INTEGER NOT NULL DEFAULT (unixepoch()),
  stopped_at INTEGER
);

CREATE TABLE IF NOT EXISTS board_env_vars (
  id INTEGER PRIMARY KEY,
  board_id INTEGER NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
  key TEXT NOT NULL,
  -- value holds AES-GCM ciphertext (see internal/secrets), never plaintext.
  value TEXT NOT NULL,
  UNIQUE(board_id, key)
);

CREATE TABLE IF NOT EXISTS hook_configs (
  id INTEGER PRIMARY KEY,
  -- board_id NULL means the hook applies to all boards (global).
  board_id INTEGER REFERENCES boards(id) ON DELETE CASCADE,
  event TEXT NOT NULL,
  command TEXT NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 1
);

CREATE INDEX IF NOT EXISTS idx_tickets_column_pos
  ON tickets(column_id, position) WHERE archived_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_tickets_board_archived
  ON tickets(board_id, archived_at);
CREATE INDEX IF NOT EXISTS idx_task_runs_session
  ON task_runs(session_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_hook_configs_event
  ON hook_configs(event, enabled, board_id);
CREATE INDEX IF NOT EXISTS idx_port_alloc_session
  ON port_allocations(session_id);
-- COALESCE collapses NULL (global) and concrete board_ids into one key so
-- duplicate hooks are rejected whether or not board_id is set.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_hook_configs
  ON hook_configs(event, command, COALESCE(board_id, 0));
CREATE INDEX IF NOT EXISTS idx_tickets_fingerprint
  ON tickets(board_id, fingerprint)
  WHERE fingerprint IS NOT NULL AND archived_at IS NULL;
