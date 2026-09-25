// Package db wraps the SQLite store. The schema lives in migrations/*.sql and
// is applied in order on every Open, tracked with PRAGMA user_version.
package db

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// ErrNotFound is returned when a row does not exist.
var ErrNotFound = errors.New("not found")

// ErrInvalid wraps validation failures so the API can map them to 400s.
var ErrInvalid = errors.New("invalid")

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

// querier is the subset of *sql.DB and *sql.Tx the store methods use, so the
// same methods run inside or outside a transaction.
type querier interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// Store is a handle to the SQLite database.
type Store struct {
	db querier
	// sqlDB is nil for a Store bound to a transaction (see Tx).
	sqlDB *sql.DB
}

// Tx runs fn against a Store bound to one transaction, committing when fn
// returns nil. Nested calls reuse the outer transaction.
func (s *Store) Tx(fn func(*Store) error) error {
	if s.sqlDB == nil {
		return fn(s)
	}
	tx, err := s.sqlDB.Begin()
	if err != nil {
		return err
	}
	if err := fn(&Store{db: tx}); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

var memCounter atomic.Int64

// Open opens (creating if necessary) the SQLite database at path and applies
// pending migrations. Pass ":memory:" for an ephemeral in-memory database.
func Open(path string) (*Store, error) {
	pragmas := "_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	dsn := "file:" + path + "?" + pragmas + "&_pragma=journal_mode(WAL)"
	if path == ":memory:" {
		// Each Open gets its own named shared-cache DB so parallel tests don't
		// see each other's rows; combined with MaxOpenConns(1) below it
		// behaves like a single persistent connection.
		dsn = fmt.Sprintf("file:memdb%d?mode=memory&cache=shared&%s", memCounter.Add(1), pragmas)
	}
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// modernc.org/sqlite serializes writes; a single connection avoids
	// SQLITE_BUSY under concurrent handlers at the cost of throughput a
	// single-user app doesn't need.
	sqlDB.SetMaxOpenConns(1)
	if err := migrate(sqlDB); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return &Store{db: sqlDB, sqlDB: sqlDB}, nil
}

// migrate applies every migrations/NNNN_*.sql file whose number is greater
// than the database's user_version, each in its own transaction.
func migrate(sqlDB *sql.DB) error {
	names, err := fs.Glob(migrationsFS, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	var current int
	if err := sqlDB.QueryRow(`PRAGMA user_version`).Scan(&current); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}
	for i, name := range names {
		version := i + 1
		if version <= current {
			continue
		}
		body, err := migrationsFS.ReadFile(name)
		if err != nil {
			return err
		}
		tx, err := sqlDB.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, version)); err != nil {
			tx.Rollback()
			return fmt.Errorf("bump user_version: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit %s: %w", name, err)
		}
	}
	return nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	return s.sqlDB.Close()
}

// nowUTC formats the current time the same way the schema's column defaults do.
func nowUTC() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

// rowsAffected turns a zero-row UPDATE/DELETE into ErrNotFound.
func rowsAffected(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// notFound maps sql.ErrNoRows to ErrNotFound.
func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// queryAll runs q and scans each row with scan, returning a non-nil slice.
func queryAll[T any](s *Store, scan func(rowScanner) (T, error), q string, args ...any) ([]T, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

// IsEmpty reports whether the database holds no user data (settings aside).
func (s *Store) IsEmpty() (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT
		(SELECT COUNT(*) FROM life_values) + (SELECT COUNT(*) FROM goals) +
		(SELECT COUNT(*) FROM steps) + (SELECT COUNT(*) FROM checkins) +
		(SELECT COUNT(*) FROM thought_records) + (SELECT COUNT(*) FROM retros) +
		(SELECT COUNT(*) FROM curator_notes)`).Scan(&n)
	return n == 0, err
}
