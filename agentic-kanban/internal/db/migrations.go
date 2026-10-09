package db

import (
	"database/sql"
	"fmt"
	"log"
)

// baselineVersion is the schema version produced by schema.sql plus
// legacyMigrate: everything the database looked like before migrations were
// numbered. Both are frozen; a schema change is a new entry in migrations.
const baselineVersion = 1

// migration is one schema change, applied exactly once per database.
type migration struct {
	// name is logged and used in error messages.
	name string
	// apply runs inside a transaction that also bumps user_version, so a
	// failure leaves the database at the previous version.
	apply func(tx *sql.Tx) error
}

// migrations[i] takes the schema from version baselineVersion+i to
// baselineVersion+i+1. Append only: never reorder, edit, or delete an entry
// that has shipped, because databases record how many entries they have run,
// not which ones.
// See REGRESSIONS.md: "Schema changes go in the numbered migration list".
var migrations = []migration{
	{name: "drop sessions path overrides", apply: dropSessionPathOverrides},
}

// dropSessionPathOverrides removes sessions.mount_path and sessions.repo_path,
// per-session overrides of the board's paths that nothing ever assigned.
func dropSessionPathOverrides(tx *sql.Tx) error {
	for _, col := range []string{"mount_path", "repo_path"} {
		if _, err := tx.Exec(`ALTER TABLE sessions DROP COLUMN ` + col); err != nil {
			return fmt.Errorf("drop sessions.%s: %w", col, err)
		}
	}
	return nil
}

// schemaVersion is the version a fully migrated database reports.
func schemaVersion() int { return baselineVersion + len(migrations) }

// migrate brings the database to schemaVersion, using PRAGMA user_version to
// record how far it has got.
func migrate(db *sql.DB) error {
	var v int
	if err := db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	// Checked before any write: an older binary must not run its idea of the
	// schema over a database a newer one has already changed.
	if v > schemaVersion() {
		return fmt.Errorf("database schema version %d is newer than this binary supports (%d); upgrade kanban", v, schemaVersion())
	}
	if v == 0 {
		// Version 0 is either a brand-new file or a database from before
		// migrations were numbered; the same two steps bring both to the
		// baseline. They are idempotent, so they are not wrapped in a
		// transaction: a crash between them leaves the version at 0 and the
		// next start repeats them safely. The one step that does not repeat
		// is backfillProjectDir, which legacyMigrate runs only when it has
		// just added the column, so a crash during that backfill leaves it
		// unfinished, as it did before migrations were numbered.
		if _, err := db.Exec(schemaSQL); err != nil {
			return fmt.Errorf("apply schema: %w", err)
		}
		if err := legacyMigrate(db); err != nil {
			return err
		}
		var err error
		if v, err = stampBaseline(db); err != nil {
			return fmt.Errorf("stamp baseline: %w", err)
		}
	}
	for i := v - baselineVersion; i < len(migrations); i++ {
		m := migrations[i]
		version := baselineVersion + i + 1
		applied, err := applyMigration(db, m, version)
		if err != nil {
			return fmt.Errorf("migration %d (%s): %w", version, m.name, err)
		}
		if applied {
			log.Printf("migrate: applied %d (%s)", version, m.name)
		}
	}
	return nil
}

// beginLocked starts a transaction that already holds SQLite's write lock
// and returns the version read under it. The version migrate read at startup
// is stale if a second kanban process started against the same file, so every
// version write re-reads it this way first.
//
// The driver only offers BEGIN IMMEDIATE pool-wide (the _txlock DSN option),
// so the lock comes from making the transaction's first statement a write: an
// UPDATE reserves the database even when it matches no rows. It has to come
// before the read, because a transaction that has already read cannot wait
// for a writer and fails as busy.
func beginLocked(db *sql.DB) (*sql.Tx, int, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, 0, fmt.Errorf("begin: %w", err)
	}
	if _, err := tx.Exec(`UPDATE boards SET id = id WHERE 0`); err != nil {
		_ = tx.Rollback()
		return nil, 0, fmt.Errorf("lock database: %w", err)
	}
	var current int
	if err := tx.QueryRow("PRAGMA user_version").Scan(&current); err != nil {
		_ = tx.Rollback()
		return nil, 0, fmt.Errorf("read schema version: %w", err)
	}
	return tx, current, nil
}

// stampBaseline records that the baseline schema is in place and returns the
// version the database is now at. That is not always baselineVersion: another
// process may have stamped it and gone on to apply migrations, and writing
// the baseline over that would make them run a second time.
func stampBaseline(db *sql.DB) (int, error) {
	tx, current, err := beginLocked(db)
	if err != nil {
		return 0, err
	}
	if current != 0 {
		_ = tx.Rollback()
		return current, nil
	}
	if err := setUserVersion(tx, baselineVersion); err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return baselineVersion, nil
}

// applyMigration runs m and records version in one transaction, so a failed
// migration leaves neither a half-applied schema nor a bumped version. It
// reports false when another process sharing the file applied m first.
func applyMigration(db *sql.DB, m migration, version int) (bool, error) {
	tx, current, err := beginLocked(db)
	if err != nil {
		return false, err
	}
	if current >= version {
		_ = tx.Rollback()
		return false, nil
	}
	if err := m.apply(tx); err != nil {
		_ = tx.Rollback()
		return false, err
	}
	if err := setUserVersion(tx, version); err != nil {
		_ = tx.Rollback()
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit: %w", err)
	}
	return true, nil
}

// setUserVersion writes PRAGMA user_version. The pragma does not accept a
// bound parameter, so the statement is built from the int.
func setUserVersion(tx *sql.Tx, version int) error {
	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", version)); err != nil {
		return fmt.Errorf("set schema version %d: %w", version, err)
	}
	return nil
}
