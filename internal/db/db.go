// Package db wraps the SQLite store. The schema lives in schema.sql and is
// applied idempotently on every Open.
package db

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

// ErrNotFound is returned when a row does not exist.
var ErrNotFound = errors.New("not found")

// Store is a handle to the SQLite database.
type Store struct {
	db *sql.DB
}

// Open opens (creating if necessary) the SQLite database at path and applies
// the schema. Pass ":memory:" for an ephemeral in-memory database.
func Open(path string) (*Store, error) {
	dsn := path
	if path == ":memory:" {
		// Shared cache so the in-memory DB survives across pooled
		// connections; combined with MaxOpenConns(1) below it behaves like a
		// single persistent connection.
		dsn = "file::memory:?cache=shared"
	}
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// modernc.org/sqlite serializes writes; a single connection avoids
	// SQLITE_BUSY under concurrent handlers at the cost of throughput this
	// application doesn't need.
	sqlDB.SetMaxOpenConns(1)
	if _, err := sqlDB.Exec(schema); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	if err := migrate(sqlDB); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}
	return &Store{db: sqlDB}, nil
}

// migrate adds columns to tables that predate them — CREATE TABLE IF NOT
// EXISTS in schema.sql only shapes fresh databases.
func migrate(sqlDB *sql.DB) error {
	const text = `TEXT NOT NULL DEFAULT ''`
	added := []struct{ table, column, ddl string }{
		{"deploys", "branch", text},
		{"deploys", "author_name", text},
		{"deploys", "author_email", text},
		{"deploys", "artifacts", text},
		{"deploys", "created_by", text},
		{"backend_artifacts", "init_done_at", text},
		{"repos", "watch", `INTEGER NOT NULL DEFAULT 0`},
		{"repos", "watch_branches", text},
		// 1, not 0: a repo watched before the column existed has already
		// deployed every tip it was going to.
		{"repos", "watch_baselined", `INTEGER NOT NULL DEFAULT 1`},
		// 'ready', not 'cloning': rows that predate the column were created
		// by the synchronous flow, which only inserted after a finished clone.
		{"repos", "status", `TEXT NOT NULL DEFAULT 'ready'`},
		{"repos", "error", text},
	}
	for _, a := range added {
		var n int
		err := sqlDB.QueryRow(
			`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`,
			a.table, a.column).Scan(&n)
		if err != nil {
			return err
		}
		if n == 0 {
			if _, err := sqlDB.Exec(fmt.Sprintf(
				`ALTER TABLE %s ADD COLUMN %s %s`,
				a.table, a.column, a.ddl)); err != nil {
				return err
			}
		}
	}
	return nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	return s.db.Close()
}
