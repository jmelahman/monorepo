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

// Item is a row in the items table.
type Item struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
}

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
	// template doesn't need.
	sqlDB.SetMaxOpenConns(1)
	if _, err := sqlDB.Exec(schema); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Store{db: sqlDB}, nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	return s.db.Close()
}

// ListItems returns all items, newest first.
func (s *Store) ListItems() ([]Item, error) {
	rows, err := s.db.Query(`SELECT id, title, created_at FROM items ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Item{}
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.Title, &it.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// CreateItem inserts a new item and returns it.
func (s *Store) CreateItem(title string) (Item, error) {
	var it Item
	err := s.db.QueryRow(
		`INSERT INTO items (title) VALUES (?) RETURNING id, title, created_at`, title,
	).Scan(&it.ID, &it.Title, &it.CreatedAt)
	if err != nil {
		return Item{}, err
	}
	return it, nil
}

// DeleteItem removes an item by id. Returns ErrNotFound if no row matched.
func (s *Store) DeleteItem(id int64) error {
	res, err := s.db.Exec(`DELETE FROM items WHERE id = ?`, id)
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
