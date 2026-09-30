-- One row per finished run. `payload` is the source of truth, byte for byte as
-- sent; every other column is copied out of it so the obvious questions can be
-- asked in SQL without a replay. `day` is the only thing the server adds, and
-- it is deliberately a day rather than a timestamp.
CREATE TABLE IF NOT EXISTS runs (
  id INTEGER PRIMARY KEY,
  day TEXT NOT NULL,
  v INTEGER NOT NULL,
  content INTEGER NOT NULL,
  build TEXT NOT NULL,
  build_commit TEXT NOT NULL,
  words TEXT NOT NULL,
  ascension INTEGER NOT NULL,
  nth INTEGER NOT NULL,
  ended TEXT NOT NULL,
  won INTEGER NOT NULL,
  stage INTEGER NOT NULL,
  round INTEGER NOT NULL,
  payload TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS runs_content ON runs (content);

-- One row per run, however many times it arrives. A run is its seed, which is
-- 31 random bits drawn at `startRun`, its place in the device's count, and the
-- build that played it; the payload keeps the seed, so the index reads it from
-- there rather than growing a column. The worker inserts OR IGNORE against it.
--
-- Two players' first runs on one build that draw the same seed collide here,
-- and the second is dropped without a trace. That is accepted: the fix would
-- be an id that ties runs to a device, which telemetry never carries (see the
-- Storage section of CLAUDE.md).
--
-- The DELETE is for a table from before the index, which the resend loop fixed
-- beside it had already filled with copies (ten of one run by 2026-09-30); the
-- index cannot be built over them. It keeps the first copy and is a no-op on
-- any table the index already guards.
DELETE FROM runs WHERE id NOT IN (
  SELECT MIN(id) FROM runs GROUP BY json_extract(payload, '$.seed'), nth, build_commit
);
CREATE UNIQUE INDEX IF NOT EXISTS runs_once
  ON runs (json_extract(payload, '$.seed'), nth, build_commit);
