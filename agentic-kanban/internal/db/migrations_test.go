package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jmelahman/kanban/internal/metrics"
)

// setMigrations swaps the package-level migration list for one test. Tests
// using it must not run in parallel, since the list is shared.
func setMigrations(t *testing.T, ms []migration) {
	t.Helper()
	prev := migrations
	migrations = ms
	t.Cleanup(func() { migrations = prev })
}

// openRaw opens the database file without going through Open, so a test can
// inspect or doctor it without triggering the migration runner.
func openRaw(t *testing.T, path string) *sql.DB {
	t.Helper()
	raw, err := sql.Open(metrics.InstrumentedDriverName, "file:"+path)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	return raw
}

func userVersion(t *testing.T, conn *sql.DB) int {
	t.Helper()
	var v int
	if err := conn.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	return v
}

func tableExists(t *testing.T, conn *sql.DB, name string) bool {
	t.Helper()
	var n int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&n); err != nil {
		t.Fatalf("look up table %q: %v", name, err)
	}
	return n > 0
}

// createBaselineDB leaves a closed database at baselineVersion on disk, as a
// binary with no numbered migrations would.
func createBaselineDB(t *testing.T) string {
	t.Helper()
	setMigrations(t, nil)
	path := filepath.Join(t.TempDir(), "kanban.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return path
}

// baselineSchemaSHA256 pins schema.sql. The file runs only on a database at
// user_version 0, so an edit to it reaches fresh databases (every test) and
// no existing one.
const baselineSchemaSHA256 = "02659bf9e3779103562f3e2fe496dfdff6a5e3c81dfb586c0f5d5fece508fcc1"

// TestMigrate_BaselineSchemaIsFrozen fails when schema.sql is edited, which
// would otherwise pass every test and silently skip upgraded installs.
func TestMigrate_BaselineSchemaIsFrozen(t *testing.T) {
	sum := sha256.Sum256([]byte(schemaSQL))
	if got := hex.EncodeToString(sum[:]); got != baselineSchemaSHA256 {
		t.Errorf("schema.sql changed (sha256 %s): it is the frozen version-%d baseline and "+
			"no longer runs on existing databases. Revert it and append an entry to migrations "+
			"in migrations.go instead. See REGRESSIONS.md: %q.",
			got, baselineVersion, "Schema changes go in the numbered migration list")
	}
}

func TestMigrate_FreshDatabaseIsAtCurrentVersion(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "kanban.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if got := userVersion(t, store.DB()); got != schemaVersion() {
		t.Errorf("user_version = %d, want %d", got, schemaVersion())
	}
}

func TestMigrate_AppliesPendingOnce(t *testing.T) {
	path := createBaselineDB(t)
	setMigrations(t, []migration{{
		name: "create probe",
		apply: func(tx *sql.Tx) error {
			_, err := tx.Exec(`CREATE TABLE probe (id INTEGER PRIMARY KEY)`)
			return err
		},
	}})

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open (migrate): %v", err)
	}
	if !tableExists(t, store.DB(), "probe") {
		t.Errorf("probe table missing after migrate")
	}
	if got := userVersion(t, store.DB()); got != baselineVersion+1 {
		t.Errorf("user_version = %d, want %d", got, baselineVersion+1)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// A second run of the entry would fail on CREATE TABLE probe, so a clean
	// reopen shows it was skipped.
	store, err = Open(path)
	if err != nil {
		t.Fatalf("Open (already migrated): %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if got := userVersion(t, store.DB()); got != baselineVersion+1 {
		t.Errorf("user_version after reopen = %d, want %d", got, baselineVersion+1)
	}
}

func TestMigrate_FailureRollsBack(t *testing.T) {
	path := createBaselineDB(t)
	setMigrations(t, []migration{{
		name: "half-done probe",
		apply: func(tx *sql.Tx) error {
			if _, err := tx.Exec(`CREATE TABLE probe (id INTEGER PRIMARY KEY)`); err != nil {
				return err
			}
			return errors.New("boom")
		},
	}})

	store, err := Open(path)
	if err == nil {
		_ = store.Close()
		t.Fatalf("Open succeeded, want the migration's error")
	}
	if !strings.Contains(err.Error(), "half-done probe") {
		t.Errorf("error %q does not name the migration", err)
	}

	raw := openRaw(t, path)
	if got := userVersion(t, raw); got != baselineVersion {
		t.Errorf("user_version = %d after failed migration, want %d", got, baselineVersion)
	}
	if tableExists(t, raw, "probe") {
		t.Errorf("probe table survived a failed migration")
	}
}

func TestMigrate_RefusesNewerDatabase(t *testing.T) {
	path := createBaselineDB(t)
	raw := openRaw(t, path)
	if _, err := raw.Exec(`PRAGMA user_version = 9999`); err != nil {
		t.Fatalf("set user_version: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	store, err := Open(path)
	if err == nil {
		_ = store.Close()
		t.Fatalf("Open succeeded on a database from a newer binary")
	}
	if !strings.Contains(err.Error(), "newer than this binary") {
		t.Errorf("error %q does not explain the version mismatch", err)
	}
	// The refusal must come before any write, so the newer version is intact.
	if got := userVersion(t, openRaw(t, path)); got != 9999 {
		t.Errorf("user_version = %d after refusal, want 9999", got)
	}
}

func TestMigrate_LegacyDatabaseStillUpgrades(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kanban.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := store.DB().Exec(`ALTER TABLE sessions DROP COLUMN workspace_folder`); err != nil {
		t.Fatalf("drop workspace_folder: %v", err)
	}
	// A database from before migrations were numbered reports version 0,
	// which is what sends Open down the legacy path that re-adds the column.
	if _, err := store.DB().Exec(`PRAGMA user_version = 0`); err != nil {
		t.Fatalf("reset user_version: %v", err)
	}
	// It also still has the columns the numbered migrations drop; without
	// them the reopen fails in "drop sessions path overrides".
	if _, err := store.DB().Exec(`ALTER TABLE sessions ADD COLUMN mount_path TEXT; ALTER TABLE sessions ADD COLUMN repo_path TEXT`); err != nil {
		t.Fatalf("restore dropped session columns: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	store, err = Open(path)
	if err != nil {
		t.Fatalf("Open (migrate): %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if _, err := store.DB().Exec(`SELECT workspace_folder FROM sessions`); err != nil {
		t.Errorf("sessions.workspace_folder missing after migrate: %v", err)
	}
	if got := userVersion(t, store.DB()); got != schemaVersion() {
		t.Errorf("user_version = %d, want %d", got, schemaVersion())
	}
}

// TestMigrate_ConcurrentOpensApplyOnce starts several Opens against the same
// file at once, as two kanban processes launched together would, and checks
// the pending migration runs in exactly one of them while the rest succeed.
// The unversioned case matters separately: there each opener also stamps the
// baseline, which must not wind the version back over a migration another
// opener has already applied.
func TestMigrate_ConcurrentOpensApplyOnce(t *testing.T) {
	for _, tc := range []struct {
		name string
		path func(t *testing.T) string
	}{
		{"from baseline", createBaselineDB},
		{"from unversioned", func(t *testing.T) string {
			path := createBaselineDB(t)
			raw := openRaw(t, path)
			if _, err := raw.Exec(`PRAGMA user_version = 0`); err != nil {
				t.Fatalf("reset user_version: %v", err)
			}
			if err := raw.Close(); err != nil {
				t.Fatalf("close: %v", err)
			}
			return path
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := tc.path(t)
			var applied atomic.Int32
			setMigrations(t, []migration{{
				name: "create probe",
				apply: func(tx *sql.Tx) error {
					applied.Add(1)
					_, err := tx.Exec(`CREATE TABLE probe (id INTEGER PRIMARY KEY)`)
					return err
				},
			}})

			const openers = 8
			errs := make([]error, openers)
			start := make(chan struct{})
			var wg sync.WaitGroup
			for i := range openers {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					store, err := Open(path)
					if err != nil {
						errs[i] = err
						return
					}
					errs[i] = store.Close()
				}()
			}
			close(start)
			wg.Wait()

			for i, err := range errs {
				if err != nil {
					t.Errorf("opener %d: %v", i, err)
				}
			}
			if got := applied.Load(); got != 1 {
				t.Errorf("migration applied %d times, want 1", got)
			}
			if got := userVersion(t, openRaw(t, path)); got != baselineVersion+1 {
				t.Errorf("user_version = %d, want %d", got, baselineVersion+1)
			}
		})
	}
}

// TestMigrate_DropsSessionPathOverrides checks the columns are gone from a
// fresh database and that every session query still agrees with the table.
func TestMigrate_DropsSessionPathOverrides(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "kanban.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	ctx := t.Context()

	rows, err := store.DB().Query(`SELECT name FROM pragma_table_info('sessions')`)
	if err != nil {
		t.Fatalf("read sessions columns: %v", err)
	}
	cols := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan column name: %v", err)
		}
		cols[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate sessions columns: %v", err)
	}
	_ = rows.Close()
	for _, gone := range []string{"mount_path", "repo_path"} {
		if cols[gone] {
			t.Errorf("sessions.%s still present after migrate", gone)
		}
	}
	for _, kept := range []string{"worktree_path", "workspace_folder"} {
		if !cols[kept] {
			t.Errorf("sessions.%s missing after migrate", kept)
		}
	}

	b := &Board{Name: "Drop", Slug: "drop", BaseBranch: "main", RepoPath: "/tmp/x"}
	if err := store.CreateBoard(ctx, b); err != nil {
		t.Fatalf("CreateBoard: %v", err)
	}
	columns, err := store.ListColumns(ctx, b.ID)
	if err != nil {
		t.Fatalf("ListColumns: %v", err)
	}
	tk := &Ticket{BoardID: b.ID, ColumnID: columns[0].ID, Title: "t", Slug: "t"}
	if err := store.CreateTicket(ctx, tk); err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}
	sess := &Session{
		TicketID:     tk.ID,
		WorktreePath: "/tmp/wt",
		BranchName:   "kanban/old",
		Status:       SessionStatusStopped,
	}
	if err := store.UpsertSession(ctx, sess); err != nil {
		t.Fatalf("UpsertSession (insert): %v", err)
	}
	sess.BranchName = "kanban/new"
	if err := store.UpsertSession(ctx, sess); err != nil {
		t.Fatalf("UpsertSession (update): %v", err)
	}

	got, err := store.GetSession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.BranchName != "kanban/new" {
		t.Errorf("GetSession branch_name = %q; want %q", got.BranchName, "kanban/new")
	}
	got, err = store.GetSessionByTicket(ctx, tk.ID)
	if err != nil {
		t.Fatalf("GetSessionByTicket: %v", err)
	}
	if got.BranchName != "kanban/new" {
		t.Errorf("GetSessionByTicket branch_name = %q; want %q", got.BranchName, "kanban/new")
	}
}
