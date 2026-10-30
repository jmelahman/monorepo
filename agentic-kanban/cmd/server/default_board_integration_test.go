package server

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jmelahman/kanban/internal/db"
)

func writeBoardDefault(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[board]\ndefault = "+strconv.Quote(value)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestResolveBoardIdent_DefaultBoard covers board.default end to end: two boards
// tie on one repo, which inference refuses to guess at, and the config settles it.
func TestResolveBoardIdent_DefaultBoard(t *testing.T) {
	userFile := filepath.Join(t.TempDir(), "config.toml")
	t.Setenv("KANBAN_CONFIG", userFile)
	srv, store, board := newKanbanCLITestServer(t)

	other := &db.Board{Name: "Other", Slug: "other", RepoPath: board.RepoPath, BaseBranch: "main"}
	if err := store.CreateBoard(context.Background(), other); err != nil {
		t.Fatal(err)
	}
	otherID := strconv.FormatInt(other.ID, 10)
	boardID := strconv.FormatInt(board.ID, 10)
	nested := filepath.Join(board.RepoPath, "services", "api")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	rootFile := filepath.Join(board.RepoPath, ".config", "kanban", "config.toml")
	nestedFile := filepath.Join(nested, ".kanban.toml")

	t.Chdir(board.RepoPath)
	if _, err := resolveBoardIdent(t.Context(), srv.URL, nil); err == nil {
		t.Fatal("expected a tie without board.default")
	}

	// The project file settles the tie, by slug or by name.
	writeBoardDefault(t, rootFile, "other")
	assertIdent(t, srv.URL, nil, otherID)
	writeBoardDefault(t, rootFile, "Other")
	assertIdent(t, srv.URL, nil, otherID)

	// An explicit board still wins.
	assertIdent(t, srv.URL, []string{"cli-board"}, "cli-board")

	// From a subdirectory: the repo root's default applies, until a nearer
	// project file overrides it.
	t.Chdir(nested)
	assertIdent(t, srv.URL, nil, otherID)
	writeBoardDefault(t, nestedFile, "cli-board")
	assertIdent(t, srv.URL, nil, boardID)

	// The nearest file can also opt out, leaving the tie for inference.
	writeBoardDefault(t, nestedFile, "")
	if _, err := resolveBoardIdent(t.Context(), srv.URL, nil); err == nil {
		t.Fatal("expected a tie once the subproject clears board.default")
	}

	// The user file beats every project file, even outside a repo.
	writeBoardDefault(t, userFile, "cli-board")
	assertIdent(t, srv.URL, nil, boardID)
	t.Chdir(t.TempDir())
	assertIdent(t, srv.URL, nil, boardID)

	// A stale value is an error naming the key, not a fall through to inference.
	writeBoardDefault(t, userFile, "gone")
	t.Chdir(board.RepoPath)
	_, err := resolveBoardIdent(t.Context(), srv.URL, nil)
	if err == nil || !strings.Contains(err.Error(), "board.default") {
		t.Fatalf("err = %v, want one naming board.default", err)
	}

	// A stale id gets the same error rather than passing through unchecked.
	writeBoardDefault(t, userFile, "999999")
	_, err = resolveBoardIdent(t.Context(), srv.URL, nil)
	if err == nil || !strings.Contains(err.Error(), "board.default") {
		t.Fatalf("err = %v, want one naming board.default", err)
	}
	// Ids still resolve, to a board that exists.
	writeBoardDefault(t, userFile, otherID)
	assertIdent(t, srv.URL, nil, otherID)

	// A name two boards share is an error listing both, while the slug still
	// picks one.
	dup := &db.Board{Name: "Other", Slug: "other-2", RepoPath: board.RepoPath, BaseBranch: "main"}
	if err := store.CreateBoard(context.Background(), dup); err != nil {
		t.Fatal(err)
	}
	writeBoardDefault(t, userFile, "Other")
	_, err = resolveBoardIdent(t.Context(), srv.URL, nil)
	if err == nil || !strings.Contains(err.Error(), "other, other-2") {
		t.Fatalf("err = %v, want one listing both boards", err)
	}
	writeBoardDefault(t, userFile, "other-2")
	assertIdent(t, srv.URL, nil, strconv.FormatInt(dup.ID, 10))

	// An empty user value opts out of project defaults, leaving the tie for
	// inference, even though the repo root's file still sets one.
	writeBoardDefault(t, userFile, "")
	if _, err := resolveBoardIdent(t.Context(), srv.URL, nil); err == nil {
		t.Fatal("expected a tie once the user file clears board.default")
	}
}

func assertIdent(t *testing.T, url string, args []string, want string) {
	t.Helper()
	got, err := resolveBoardIdent(t.Context(), url, args)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("ident = %q, want %q", got, want)
	}
}
