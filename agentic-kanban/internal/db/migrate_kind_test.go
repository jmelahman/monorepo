package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jmelahman/kanban/internal/db"
)

// TestMigrate_AddsBoardKind opens a database that predates boards.kind (faked
// by dropping the column, as TestMigrate_BackfillProjectDir does) and checks
// existing boards come back as ordinary, and that a stamped kind survives a
// user edit and a reopen.
func TestMigrate_AddsBoardKind(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "kanban.db")
	store, err := db.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := store.DB().ExecContext(ctx, `ALTER TABLE boards DROP COLUMN kind`); err != nil {
		t.Fatalf("drop kind (simulating an older schema): %v", err)
	}
	// A database from before migrations were numbered reports version 0,
	// which is what sends Open down the legacy path that re-adds the column.
	if _, err := store.DB().ExecContext(ctx, `PRAGMA user_version = 0`); err != nil {
		t.Fatalf("reset user_version: %v", err)
	}
	if _, err := store.DB().ExecContext(ctx,
		`INSERT INTO boards (name, slug, base_branch, created_at, position) VALUES ('Errors', 'errors', 'main', 0, 0)`,
	); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	store, err = db.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	board, err := store.GetBoardBySlug(ctx, "errors")
	if err != nil {
		t.Fatalf("GetBoardBySlug: %v", err)
	}
	if board.Kind != "" {
		t.Errorf("kind after migrate = %q; want ordinary", board.Kind)
	}

	if err := store.SetBoardKind(ctx, board.ID, db.BoardKindErrors); err != nil {
		t.Fatalf("SetBoardKind: %v", err)
	}
	// UpdateBoard writes every user-editable field; kind is not one of them.
	board.Name = "Renamed"
	if err := store.UpdateBoard(ctx, board); err != nil {
		t.Fatalf("UpdateBoard: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	store, err = db.Open(path)
	if err != nil {
		t.Fatalf("second reopen: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	boards, err := store.ListBoards(ctx)
	if err != nil {
		t.Fatalf("ListBoards: %v", err)
	}
	if len(boards) != 1 || boards[0].Kind != db.BoardKindErrors || boards[0].Name != "Renamed" {
		t.Errorf("boards = %+v; want one renamed board of kind errors", boards)
	}
}
