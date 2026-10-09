package db_test

import (
	"fmt"
	"testing"

	"github.com/jmelahman/kanban/internal/db"
)

func TestCreateTicket_SlugCollisionGetsSuffix(t *testing.T) {
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open(:memory:): %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	ctx := t.Context()

	b := &db.Board{Name: "Slugs", Slug: "slugs", BaseBranch: "main", RepoPath: "/tmp/x"}
	if err := store.CreateBoard(ctx, b); err != nil {
		t.Fatalf("CreateBoard: %v", err)
	}
	cols, err := store.ListColumns(ctx, b.ID)
	if err != nil {
		t.Fatalf("ListColumns: %v", err)
	}
	if len(cols) == 0 {
		t.Fatal("board has no columns")
	}

	for _, want := range []string{"dup", "dup-2", "dup-3"} {
		tk := &db.Ticket{BoardID: b.ID, ColumnID: cols[0].ID, Title: "Dup", Slug: "dup"}
		if err := store.CreateTicket(ctx, tk); err != nil {
			t.Fatalf("CreateTicket (want slug %q): %v", want, err)
		}
		if tk.Slug != want {
			t.Errorf("slug = %q; want %q", tk.Slug, want)
		}
	}
}

func TestIsUniqueViolation(t *testing.T) {
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open(:memory:): %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	ctx := t.Context()

	first := &db.Board{Name: "One", Slug: "same", BaseBranch: "main", RepoPath: "/tmp/x"}
	if err := store.CreateBoard(ctx, first); err != nil {
		t.Fatalf("CreateBoard: %v", err)
	}
	second := &db.Board{Name: "Two", Slug: "same", BaseBranch: "main", RepoPath: "/tmp/y"}
	dupErr := store.CreateBoard(ctx, second)
	if dupErr == nil {
		t.Fatal("CreateBoard with a duplicate slug succeeded; want an error")
	}
	if !db.IsUniqueViolation(dupErr) {
		t.Errorf("IsUniqueViolation(%v) = false; want true", dupErr)
	}
	if wrapped := fmt.Errorf("create board: %w", dupErr); !db.IsUniqueViolation(wrapped) {
		t.Errorf("IsUniqueViolation(wrapped %v) = false; want true", dupErr)
	}

	_, notNullErr := store.DB().Exec(`INSERT INTO boards (name) VALUES ('x')`)
	if notNullErr == nil {
		t.Fatal("insert without slug succeeded; want a NOT NULL failure")
	}
	for name, err := range map[string]error{
		"nil":         nil,
		"ErrNotFound": db.ErrNotFound,
		"not null":    notNullErr,
	} {
		if db.IsUniqueViolation(err) {
			t.Errorf("IsUniqueViolation(%s: %v) = true; want false", name, err)
		}
	}
}
