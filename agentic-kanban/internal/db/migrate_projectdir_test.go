package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jmelahman/kanban/internal/db"
)

// TestMigrate_BackfillProjectDir covers the one-time conversion of boards
// whose mount_path was pointed at a subdirectory of their repo_path — the
// closest thing to monorepo support before project_dir existed, and broken
// in three ways (see REGRESSIONS.md).
//
// It fakes an older database by dropping the column migrate() adds, so the
// reopen below takes exactly the upgrade path a real old DB takes.
func TestMigrate_BackfillProjectDir(t *testing.T) {
	ctx := context.Background()

	type board struct {
		name           string
		repo           string
		mount          string
		wantProjectDir string
		wantMount      string
	}
	boards := []board{
		{
			name:           "strict descendant converts",
			repo:           "/repos/monorepo",
			mount:          "/repos/monorepo/services/api",
			wantProjectDir: "services/api",
			wantMount:      "",
		},
		{
			name: "mount equal to repo is left alone",
			// A meaningful configuration today: mount the main checkout
			// rather than the per-ticket worktree. Clearing it would
			// silently flip the board onto branch-isolated worktrees.
			repo:      "/repos/plain",
			mount:     "/repos/plain",
			wantMount: "/repos/plain",
		},
		{
			name:      "ancestor is left alone",
			repo:      "/repos/parent/child",
			mount:     "/repos/parent",
			wantMount: "/repos/parent",
		},
		{
			name:      "unrelated is left alone",
			repo:      "/repos/one",
			mount:     "/elsewhere/two",
			wantMount: "/elsewhere/two",
		},
		{
			name:      "sibling with a shared prefix is left alone",
			repo:      "/repos/app",
			mount:     "/repos/app-staging",
			wantMount: "/repos/app-staging",
		},
		{
			name:      "no repo is left alone",
			mount:     "/mnt/only",
			wantMount: "/mnt/only",
		},
	}

	path := filepath.Join(t.TempDir(), "kanban.db")
	store, err := db.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := store.DB().ExecContext(ctx, `ALTER TABLE boards DROP COLUMN project_dir`); err != nil {
		t.Fatalf("drop project_dir (simulating an older schema): %v", err)
	}
	for i, b := range boards {
		if _, err := store.DB().ExecContext(ctx,
			`INSERT INTO boards (name, slug, repo_path, mount_path, base_branch, created_at, position)
			 VALUES (?, ?, ?, ?, 'main', 0, ?)`,
			b.name, b.name, b.repo, b.mount, i,
		); err != nil {
			t.Fatalf("seed %q: %v", b.name, err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	assert := func(t *testing.T, label string) {
		t.Helper()
		store, err := db.Open(path)
		if err != nil {
			t.Fatalf("%s: open: %v", label, err)
		}
		t.Cleanup(func() { _ = store.Close() })
		got, err := store.ListBoards(ctx)
		if err != nil {
			t.Fatalf("%s: ListBoards: %v", label, err)
		}
		bySlug := map[string]db.Board{}
		for _, b := range got {
			bySlug[b.Slug] = b
		}
		for _, want := range boards {
			b, ok := bySlug[want.name]
			if !ok {
				t.Fatalf("%s: board %q missing after migrate", label, want.name)
			}
			if b.ProjectDir != want.wantProjectDir {
				t.Errorf("%s: %q project_dir = %q; want %q", label, want.name, b.ProjectDir, want.wantProjectDir)
			}
			if b.MountPath != want.wantMount {
				t.Errorf("%s: %q mount_path = %q; want %q", label, want.name, b.MountPath, want.wantMount)
			}
		}
	}

	assert(t, "after migrate")
	// The backfill runs inside the `if !hasColumn` branch, so a second open
	// must not re-fire — otherwise a descendant mount_path someone sets on
	// purpose later would be silently rewritten.
	assert(t, "after reopen")
}

// TestSession_WorkspaceFolderRoundTrip covers the column the four exec sites
// read to find the agent's working directory.
func TestSession_WorkspaceFolderRoundTrip(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	board := &db.Board{Name: "b", Slug: "b", RepoPath: "/r", BaseBranch: "main"}
	if err := store.CreateBoard(ctx, board); err != nil {
		t.Fatalf("CreateBoard: %v", err)
	}
	cols, err := store.ListColumns(ctx, board.ID)
	if err != nil || len(cols) == 0 {
		t.Fatalf("ListColumns: %v (%d cols)", err, len(cols))
	}
	ticket := &db.Ticket{BoardID: board.ID, ColumnID: cols[0].ID, Title: "t", Slug: "t"}
	if err := store.CreateTicket(ctx, ticket); err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}
	sess := &db.Session{TicketID: ticket.ID, Status: db.SessionStatusStopped}
	if err := store.UpsertSession(ctx, sess); err != nil {
		t.Fatalf("UpsertSession: %v", err)
	}

	got, err := store.GetSession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.WorkspaceFolder != "" {
		t.Errorf("fresh WorkspaceFolder = %q; want empty", got.WorkspaceFolder)
	}
	if got.WorkspaceDir() != db.DefaultWorkspaceFolder {
		t.Errorf("WorkspaceDir() = %q; want the %q default", got.WorkspaceDir(), db.DefaultWorkspaceFolder)
	}

	want := "/workspace/services/api"
	if err := store.UpdateSessionLifecycle(ctx, sess.ID, db.SessionStatusIdle, nil, nil, nil, &want); err != nil {
		t.Fatalf("UpdateSessionLifecycle: %v", err)
	}
	if got, err = store.GetSession(ctx, sess.ID); err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.WorkspaceDir() != want {
		t.Errorf("WorkspaceDir() = %q; want %q", got.WorkspaceDir(), want)
	}

	// nil leaves the recorded folder alone: Stop must not erase where the
	// container actually worked.
	if err := store.UpdateSessionLifecycle(ctx, sess.ID, db.SessionStatusStopped, nil, nil, nil, nil); err != nil {
		t.Fatalf("UpdateSessionLifecycle(nil): %v", err)
	}
	if got, err = store.GetSession(ctx, sess.ID); err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.WorkspaceDir() != want {
		t.Errorf("after nil patch WorkspaceDir() = %q; want %q preserved", got.WorkspaceDir(), want)
	}
}
