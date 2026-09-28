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
