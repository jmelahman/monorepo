-- Life areas the goals serve (Health, Connection, Creativity, ...).
CREATE TABLE life_values (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    color TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Roadmap epics.
CREATE TABLE goals (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    value_id INTEGER REFERENCES life_values(id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    why TEXT NOT NULL DEFAULT '',
    horizon TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'resting', 'done')),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Small, concrete steps (behavioral-activation activities).
CREATE TABLE steps (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    goal_id INTEGER REFERENCES goals(id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    notes TEXT NOT NULL DEFAULT '',
    energy_cost INTEGER NOT NULL DEFAULT 1 CHECK (energy_cost BETWEEN 1 AND 3),
    lane TEXT NOT NULL DEFAULT 'someday' CHECK (lane IN ('someday', 'week', 'today', 'done', 'let_go')),
    sort_order INTEGER NOT NULL DEFAULT 0,
    lane_changed_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    completed_at TEXT,
    predicted_pleasure INTEGER CHECK (predicted_pleasure BETWEEN 0 AND 10),
    mastery INTEGER CHECK (mastery BETWEEN 0 AND 10),
    pleasure INTEGER CHECK (pleasure BETWEEN 0 AND 10),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
CREATE INDEX steps_lane ON steps (lane, sort_order);

-- Sprints: one row per ISO week, keyed by its Monday.
CREATE TABLE weeks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    start_date TEXT NOT NULL UNIQUE,
    intention TEXT NOT NULL DEFAULT ''
);

-- Daily standups.
CREATE TABLE checkins (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date TEXT NOT NULL,
    kind TEXT NOT NULL DEFAULT 'morning' CHECK (kind IN ('morning', 'evening', 'adhoc')),
    mood INTEGER CHECK (mood BETWEEN 0 AND 10),
    energy INTEGER CHECK (energy BETWEEN 0 AND 10),
    anxiety INTEGER CHECK (anxiety BETWEEN 0 AND 10),
    note TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    llm_session TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
CREATE INDEX checkins_date ON checkins (date);

-- Curator conversation, append-only. content_json holds backend-native
-- message payloads (e.g. tool calls) so turns can be replayed verbatim.
CREATE TABLE checkin_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    checkin_id INTEGER NOT NULL REFERENCES checkins(id) ON DELETE CASCADE,
    seq INTEGER NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('user', 'assistant', 'tool')),
    text TEXT NOT NULL DEFAULT '',
    content_json TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE (checkin_id, seq)
);

CREATE TABLE thought_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    situation TEXT NOT NULL DEFAULT '',
    emotions_json TEXT NOT NULL DEFAULT '[]',
    automatic_thought TEXT NOT NULL DEFAULT '',
    distortions_json TEXT NOT NULL DEFAULT '[]',
    evidence_for TEXT NOT NULL DEFAULT '',
    evidence_against TEXT NOT NULL DEFAULT '',
    balanced_thought TEXT NOT NULL DEFAULT '',
    rerated_emotions_json TEXT NOT NULL DEFAULT '[]',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE TABLE retros (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    week_id INTEGER NOT NULL UNIQUE REFERENCES weeks(id) ON DELETE CASCADE,
    went_well TEXT NOT NULL DEFAULT '',
    was_hard TEXT NOT NULL DEFAULT '',
    try_next TEXT NOT NULL DEFAULT '',
    ai_draft TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Facts the AI curator keeps about the user. Always user-visible/editable.
CREATE TABLE curator_notes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    text TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Log of every AI write, with enough state to undo it.
CREATE TABLE ai_actions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    checkin_id INTEGER REFERENCES checkins(id) ON DELETE SET NULL,
    source TEXT NOT NULL,
    tool TEXT NOT NULL,
    summary TEXT NOT NULL,
    entity TEXT NOT NULL,
    entity_id INTEGER NOT NULL,
    before_json TEXT,
    after_json TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    undone_at TEXT
);
CREATE INDEX ai_actions_checkin ON ai_actions (checkin_id);

-- Key/value app settings (crisis resources text, preferences).
CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
