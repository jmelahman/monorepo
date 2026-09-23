-- Schema for the local-preview orchestrator. Applied idempotently on every
-- Open; add new tables as CREATE TABLE IF NOT EXISTS statements. New columns
-- on existing tables also need an ALTER TABLE entry in db.go's migrate —
-- CREATE TABLE IF NOT EXISTS is a no-op on databases that predate them.
--
-- Log/artifact CONTENT never lives here (single-connection SQLite) — only
-- paths. The supervisor's live process state is in-memory; process_records
-- exists purely for crash recovery and process_events for observability.

CREATE TABLE IF NOT EXISTS repos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    -- A lowercase DNS label: it is the <repo> segment of preview hosts.
    name TEXT NOT NULL UNIQUE,
    source TEXT NOT NULL,
    bare_path TEXT NOT NULL,
    -- Watched repos are polled for new commits; watch_branches narrows which
    -- branch tips deploy (comma-separated globs, empty = all branches).
    watch INTEGER NOT NULL DEFAULT 0,
    watch_branches TEXT NOT NULL DEFAULT '',
    -- Whether watch_baselines holds this repo's pre-watch tips yet. Cleared
    -- when watching is switched on, set by the poll that captures them. The
    -- default suits rows that predate the column: their backfill already
    -- happened, so there is nothing left to hold back.
    watch_baselined INTEGER NOT NULL DEFAULT 1,
    -- Mirror-clone outcome: registration returns while the clone runs in the
    -- background, so a repo is deployable only once 'ready'. The default is
    -- 'ready' so rows migrated from before the column (always fully cloned)
    -- stay usable; creates insert 'cloning' explicitly.
    status TEXT NOT NULL DEFAULT 'ready'
        CHECK (status IN ('cloning', 'ready', 'failed')),
    error TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE TABLE IF NOT EXISTS deploys (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id INTEGER NOT NULL REFERENCES repos(id),
    sha TEXT NOT NULL,
    -- Shortest prefix (>=7 chars) unique among this repo's deploys at insert
    -- time; preview hosts route on sha prefixes.
    short_sha TEXT NOT NULL,
    ref TEXT NOT NULL DEFAULT '',
    -- Commit metadata captured at deploy creation. ref is what the caller
    -- asked for (possibly a tag or sha); branch is the branch the commit was
    -- attributed to, best-effort.
    branch TEXT NOT NULL DEFAULT '',
    author_name TEXT NOT NULL DEFAULT '',
    author_email TEXT NOT NULL DEFAULT '',
    fe_hash TEXT NOT NULL DEFAULT '',
    be_hash TEXT NOT NULL DEFAULT '',
    -- Named downloadable artifacts ([artifacts.<name>]) as a JSON object:
    -- {"<name>": {"hash": "…", "log_path": "…"}}. Empty when the manifest
    -- declares none.
    artifacts TEXT NOT NULL DEFAULT '',
    -- Build outcome only. Process runtime status is the supervisor's,
    -- merged into API responses at read time.
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'building', 'ready', 'failed', 'evicted')),
    error TEXT NOT NULL DEFAULT '',
    attempt_count INTEGER NOT NULL DEFAULT 0,
    fe_build_log_path TEXT NOT NULL DEFAULT '',
    be_build_log_path TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    -- Identity that triggered the deploy (SSO login, GitHub Actions actor for
    -- CI/OIDC, or empty for the automatic poller). Audit trail only; distinct
    -- from author_name/author_email, which are the commit's git author.
    created_by TEXT NOT NULL DEFAULT '',
    UNIQUE (repo_id, sha),
    UNIQUE (repo_id, short_sha)
);

-- The branch tips a repo already had when watching was enabled. The watcher
-- deploys a tip that has no deploy row, so without this the first poll would
-- deploy every branch at once; a tip listed here is skipped until it moves.
-- Rows are captured on the first poll (repos.watch_baselined) and dropped
-- when watching is turned off.
CREATE TABLE IF NOT EXISTS watch_baselines (
    repo_id INTEGER NOT NULL REFERENCES repos(id),
    sha TEXT NOT NULL,
    PRIMARY KEY (repo_id, sha)
);

-- Used from M2 (branch alias routing); created now so the schema stays a
-- single idempotent file.
CREATE TABLE IF NOT EXISTS branch_aliases (
    repo_id INTEGER NOT NULL REFERENCES repos(id),
    branch TEXT NOT NULL,
    deploy_id INTEGER NOT NULL REFERENCES deploys(id),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    PRIMARY KEY (repo_id, branch)
);

-- Row existence means the state dir is fully provisioned (forked or fresh).
-- run_config is the backend section of preview.toml as JSON: be_hash
-- uniquely determines the run contract, so the supervisor never re-parses
-- the manifest.
CREATE TABLE IF NOT EXISTS backend_artifacts (
    repo_id INTEGER NOT NULL REFERENCES repos(id),
    be_hash TEXT NOT NULL,
    forked_from TEXT NOT NULL DEFAULT '',
    state_dir TEXT NOT NULL,
    run_config TEXT NOT NULL,
    -- Set once the manifest's init steps have succeeded against this
    -- artifact's state dir; empty means init is still owed (or none declared).
    init_done_at TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    PRIMARY KEY (repo_id, be_hash)
);

-- Process-mode frontends (frontend.run set): run_config is the frontend
-- section of preview.toml as JSON, mirroring backend_artifacts. Row
-- existence means "this frontend artifact is a server process, not a
-- static bundle" — the proxy branches on it. No state dir: frontends
-- carry no forked state.
CREATE TABLE IF NOT EXISTS frontend_artifacts (
    repo_id INTEGER NOT NULL REFERENCES repos(id),
    fe_hash TEXT NOT NULL,
    run_config TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    PRIMARY KEY (repo_id, fe_hash)
);

-- Crash-recovery bookkeeping only: upserted on process start, deleted on
-- clean stop. Leftover rows at startup mean an unclean exit.
CREATE TABLE IF NOT EXISTS process_records (
    repo_id INTEGER NOT NULL REFERENCES repos(id),
    be_hash TEXT NOT NULL,
    pid INTEGER NOT NULL,
    pgid INTEGER NOT NULL,
    port INTEGER NOT NULL,
    started_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    PRIMARY KEY (repo_id, be_hash)
);

-- Instance-level settings the dashboard can change at runtime (retention
-- policy, etc.), as opposed to process-lifetime flags. Values are JSON or
-- plain strings; consumers own their key's encoding.
CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE TABLE IF NOT EXISTS process_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id INTEGER NOT NULL REFERENCES repos(id),
    be_hash TEXT NOT NULL,
    event TEXT NOT NULL,
    detail TEXT NOT NULL DEFAULT '',
    occurred_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Interactive "Sign in with GitHub" sessions. scope cuts the trust boundary:
-- 'apex' authenticates the dashboard and /api/*, 'preview' authenticates
-- preview subdomains only and is never accepted by the apex auth middleware —
-- so a preview-scoped cookie stolen by untrusted previewed code can't drive
-- the control plane. Only the sha256 hash of the opaque token is stored, never
-- the token itself, so a database leak can't be replayed as a live cookie.
CREATE TABLE IF NOT EXISTS sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token_hash TEXT NOT NULL UNIQUE,
    scope TEXT NOT NULL DEFAULT 'apex'
        CHECK (scope IN ('apex', 'preview')),
    github_login TEXT NOT NULL,
    github_user_id INTEGER NOT NULL,
    email TEXT NOT NULL DEFAULT '',
    avatar_url TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    expires_at TEXT NOT NULL
);

-- Single-use handoff codes for the apex→preview grant handshake: an apex
-- session mints one, the proxy redeems it exactly once to establish a
-- preview-scoped session on the preview origin (see internal/proxy). Codes are
-- stored hashed and are short-lived.
CREATE TABLE IF NOT EXISTS preview_grants (
    code_hash TEXT PRIMARY KEY,
    apex_session_id INTEGER NOT NULL REFERENCES sessions(id),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    expires_at TEXT NOT NULL
);
