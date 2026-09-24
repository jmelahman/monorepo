package db

import (
	_ "embed"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync/atomic"

	"database/sql"

	"github.com/jmelahman/kanban/internal/config"
	"github.com/jmelahman/kanban/internal/metrics"
	"github.com/jmelahman/kanban/internal/secrets"
)

//go:embed schema.sql
var schemaSQL string

// pragmas applied to on-disk databases. WAL + synchronous=NORMAL is the
// recommended pairing for local SQLite apps: durable on crash, ~3× fewer
// fsyncs than FULL. temp_store=MEMORY avoids disk spills for the position-
// shifting transactions in MoveTicket. cache_size is in KiB when negative
// (~20 MB here); mmap_size is in bytes (128 MB).
const onDiskPragmas = "_pragma=journal_mode(WAL)" +
	"&_pragma=foreign_keys(1)" +
	"&_pragma=busy_timeout(5000)" +
	"&_pragma=synchronous(NORMAL)" +
	"&_pragma=temp_store(MEMORY)" +
	"&_pragma=cache_size(-20000)" +
	"&_pragma=mmap_size(134217728)"

// memSeq names each ":memory:" Open's database. A shared-cache in-memory DB
// is keyed by name process-wide, so an unnamed one would be the same database
// for every concurrently open Store — tests' "fresh" stores saw each other's
// rows.
var memSeq atomic.Int64

type Store struct {
	db *sql.DB
	// envCipher encrypts/decrypts board env var values inside the Store so
	// plaintext never reaches disk. Set via SetEnvCipher; the env var methods
	// refuse to run without it (no silent-plaintext fallback).
	envCipher *secrets.Box
}

// SetEnvCipher configures the cipher used for board env var values. Must be
// called before any board env var method is used.
func (s *Store) SetEnvCipher(box *secrets.Box) { s.envCipher = box }

// Open opens a Store backed by a SQLite database at path, applies the
// embedded schema, and runs migrations. The sentinel path ":memory:" opens
// a process-local in-memory database (shared cache so the connection pool
// sees one DB, uniquely named per Open) and skips on-disk file setup; data is discarded when Close
// is called.
func Open(path string) (*Store, error) {
	var dsn string
	if path == ":memory:" {
		dsn = fmt.Sprintf("file:memdb%d?mode=memory&cache=shared&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", memSeq.Add(1))
	} else {
		if err := config.MakeFileAll(path); err != nil {
			return nil, fmt.Errorf("ensure db file: %w", err)
		}
		dsn = fmt.Sprintf("file:%s?%s", path, onDiskPragmas)
	}
	db, err := sql.Open(metrics.InstrumentedDriverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if path == ":memory:" {
		// Pin to a single connection so the shared in-memory DB isn't dropped
		// when the pool churns; modernc/sqlite tears the DB down once the last
		// connection closes.
		db.SetMaxOpenConns(1)
	} else {
		// Cap the pool to keep connection churn (and "database is locked"
		// retries under busy_timeout) bounded. WAL mode supports concurrent
		// readers + one writer, so 8 is plenty for a local desktop app.
		db.SetMaxOpenConns(8)
		db.SetMaxIdleConns(4)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Store{db: db}, nil
}

// migrate applies idempotent schema changes to existing databases that
// CREATE TABLE IF NOT EXISTS in schema.sql cannot reach. Each step must
// be safe to re-run on an already-migrated DB.
func migrate(db *sql.DB) error {
	hasColumn, err := tableHasColumn(db, "boards", "position")
	if err != nil {
		return fmt.Errorf("inspect boards: %w", err)
	}
	if !hasColumn {
		if _, err := db.Exec(`ALTER TABLE boards ADD COLUMN position INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("add boards.position: %w", err)
		}
		// Seed deterministic order matching the previous ORDER BY id behavior.
		if _, err := db.Exec(`UPDATE boards SET position = id WHERE position = 0`); err != nil {
			return fmt.Errorf("backfill boards.position: %w", err)
		}
	}
	hasColumn, err = tableHasColumn(db, "sessions", "harness")
	if err != nil {
		return fmt.Errorf("inspect sessions: %w", err)
	}
	if !hasColumn {
		if _, err := db.Exec(`ALTER TABLE sessions ADD COLUMN harness TEXT`); err != nil {
			return fmt.Errorf("add sessions.harness: %w", err)
		}
	}
	hasColumn, err = tableHasColumn(db, "boards", "project_dir")
	if err != nil {
		return fmt.Errorf("inspect boards: %w", err)
	}
	if !hasColumn {
		if _, err := db.Exec(`ALTER TABLE boards ADD COLUMN project_dir TEXT`); err != nil {
			return fmt.Errorf("add boards.project_dir: %w", err)
		}
		// Convert the pre-project_dir way of expressing "this board is one
		// subproject": a mount_path pointing inside repo_path. That shape is
		// broken (it binds the main checkout, not the ticket's worktree, and
		// mounts no .git above it), so nothing is lost by rewriting it.
		if err := backfillProjectDir(db); err != nil {
			return err
		}
	}
	hasColumn, err = tableHasColumn(db, "sessions", "workspace_folder")
	if err != nil {
		return fmt.Errorf("inspect sessions: %w", err)
	}
	if !hasColumn {
		if _, err := db.Exec(`ALTER TABLE sessions ADD COLUMN workspace_folder TEXT`); err != nil {
			return fmt.Errorf("add sessions.workspace_folder: %w", err)
		}
	}
	return nil
}

// backfillProjectDir rewrites boards whose mount_path is a strict descendant
// of their repo_path into the equivalent project_dir, clearing mount_path.
//
// Only strict descendants convert. mount_path == repo_path is a deliberate
// configuration ("mount the main checkout rather than the worktree") and
// clearing it would silently move those boards onto branch-isolated
// worktrees; ancestors are the parent-of-many-repos pattern mount_path was
// built for. Called only when project_dir was just added, so it never re-runs
// against a mount_path a user set on purpose afterwards.
func backfillProjectDir(db *sql.DB) error {
	rows, err := db.Query(`SELECT id, repo_path, mount_path FROM boards
		WHERE repo_path IS NOT NULL AND repo_path != ''
		  AND mount_path IS NOT NULL AND mount_path != ''`)
	if err != nil {
		return fmt.Errorf("scan boards for project_dir backfill: %w", err)
	}
	type conversion struct {
		id         int64
		projectDir string
	}
	var converted []conversion
	for rows.Next() {
		var id int64
		var repo, mount string
		if err := rows.Scan(&id, &repo, &mount); err != nil {
			rows.Close()
			return fmt.Errorf("scan board for project_dir backfill: %w", err)
		}
		rel, err := filepath.Rel(filepath.Clean(repo), filepath.Clean(mount))
		if err != nil || rel == "." || rel == "" || strings.HasPrefix(rel, "..") {
			continue
		}
		converted = append(converted, conversion{id: id, projectDir: filepath.ToSlash(rel)})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("scan boards for project_dir backfill: %w", err)
	}
	rows.Close()

	for _, c := range converted {
		if _, err := db.Exec(`UPDATE boards SET project_dir = ?, mount_path = NULL WHERE id = ?`, c.projectDir, c.id); err != nil {
			return fmt.Errorf("backfill boards.project_dir: %w", err)
		}
		log.Printf("migrate: board %d mount_path was inside repo_path; converted to project_dir=%s", c.id, c.projectDir)
	}
	return nil
}

// tableHasColumn reports whether table has a column named column. table is
// interpolated into the PRAGMA, so only pass constants.
func tableHasColumn(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) DB() *sql.DB { return s.db }
