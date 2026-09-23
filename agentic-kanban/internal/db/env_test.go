package db_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/secrets"
)

func newEnvStore(t *testing.T) (*db.Store, *db.Board) {
	t.Helper()
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open(:memory:): %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	key, err := secrets.NewRandomKey()
	if err != nil {
		t.Fatalf("NewRandomKey: %v", err)
	}
	box, err := secrets.NewBox(key)
	if err != nil {
		t.Fatalf("NewBox: %v", err)
	}
	store.SetEnvCipher(box)

	b := &db.Board{Name: "Env", Slug: "env", BaseBranch: "main", RepoPath: "/tmp/x"}
	if err := store.CreateBoard(t.Context(), b); err != nil {
		t.Fatalf("CreateBoard: %v", err)
	}
	return store, b
}

func TestBoardEnvVars_Lifecycle(t *testing.T) {
	store, b := newEnvStore(t)
	ctx := t.Context()

	keys, err := store.ListBoardEnvVarKeys(ctx, b.ID)
	if err != nil {
		t.Fatalf("ListBoardEnvVarKeys (empty): %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("keys = %v; want empty", keys)
	}

	if err := store.SetBoardEnvVar(ctx, b.ID, "MY_API_KEY", "s3cret"); err != nil {
		t.Fatalf("SetBoardEnvVar: %v", err)
	}
	if err := store.SetBoardEnvVar(ctx, b.ID, "ANOTHER", "value2"); err != nil {
		t.Fatalf("SetBoardEnvVar: %v", err)
	}

	keys, err = store.ListBoardEnvVarKeys(ctx, b.ID)
	if err != nil {
		t.Fatalf("ListBoardEnvVarKeys: %v", err)
	}
	if want := []string{"ANOTHER", "MY_API_KEY"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %v; want %v", keys, want)
	}

	vars, err := store.GetBoardEnvVars(ctx, b.ID)
	if err != nil {
		t.Fatalf("GetBoardEnvVars: %v", err)
	}
	if want := map[string]string{"MY_API_KEY": "s3cret", "ANOTHER": "value2"}; !reflect.DeepEqual(vars, want) {
		t.Fatalf("vars = %v; want %v", vars, want)
	}

	// Upsert overwrites in place.
	if err := store.SetBoardEnvVar(ctx, b.ID, "MY_API_KEY", "rotated"); err != nil {
		t.Fatalf("SetBoardEnvVar (upsert): %v", err)
	}
	vars, err = store.GetBoardEnvVars(ctx, b.ID)
	if err != nil {
		t.Fatalf("GetBoardEnvVars: %v", err)
	}
	if vars["MY_API_KEY"] != "rotated" {
		t.Fatalf("MY_API_KEY = %q; want %q", vars["MY_API_KEY"], "rotated")
	}

	// Delete is idempotent.
	for range 2 {
		if err := store.DeleteBoardEnvVar(ctx, b.ID, "ANOTHER"); err != nil {
			t.Fatalf("DeleteBoardEnvVar: %v", err)
		}
	}
	keys, err = store.ListBoardEnvVarKeys(ctx, b.ID)
	if err != nil {
		t.Fatalf("ListBoardEnvVarKeys: %v", err)
	}
	if want := []string{"MY_API_KEY"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %v; want %v", keys, want)
	}
}

// TestBoardEnvVars_EncryptedAtRest reads the raw column and asserts the
// plaintext never touches disk.
func TestBoardEnvVars_EncryptedAtRest(t *testing.T) {
	store, b := newEnvStore(t)
	ctx := t.Context()

	if err := store.SetBoardEnvVar(ctx, b.ID, "MY_API_KEY", "super-plaintext-secret"); err != nil {
		t.Fatalf("SetBoardEnvVar: %v", err)
	}

	var raw string
	if err := store.DB().QueryRowContext(ctx,
		`SELECT value FROM board_env_vars WHERE board_id = ? AND key = ?`,
		b.ID, "MY_API_KEY").Scan(&raw); err != nil {
		t.Fatalf("raw select: %v", err)
	}
	if !strings.HasPrefix(raw, "v1:") {
		t.Fatalf("stored value missing ciphertext prefix: %q", raw)
	}
	if strings.Contains(raw, "super-plaintext-secret") {
		t.Fatalf("stored value contains plaintext: %q", raw)
	}
}

func TestBoardEnvVars_CascadeOnBoardDelete(t *testing.T) {
	store, b := newEnvStore(t)
	ctx := t.Context()

	if err := store.SetBoardEnvVar(ctx, b.ID, "MY_API_KEY", "s3cret"); err != nil {
		t.Fatalf("SetBoardEnvVar: %v", err)
	}
	if err := store.DeleteBoard(ctx, b.ID); err != nil {
		t.Fatalf("DeleteBoard: %v", err)
	}

	var n int
	if err := store.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM board_env_vars WHERE board_id = ?`, b.ID).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("orphaned board_env_vars rows after board delete: %d", n)
	}
}

// TestBoardEnvVars_NoCipher ensures the Store refuses to read or write values
// without a configured cipher — there is no silent-plaintext fallback.
func TestBoardEnvVars_NoCipher(t *testing.T) {
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open(:memory:): %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	ctx := t.Context()

	b := &db.Board{Name: "Env", Slug: "env", BaseBranch: "main", RepoPath: "/tmp/x"}
	if err := store.CreateBoard(ctx, b); err != nil {
		t.Fatalf("CreateBoard: %v", err)
	}

	if err := store.SetBoardEnvVar(ctx, b.ID, "K", "v"); !errors.Is(err, secrets.ErrNoCipher) {
		t.Fatalf("SetBoardEnvVar without cipher: err = %v; want ErrNoCipher", err)
	}
	if _, err := store.GetBoardEnvVars(ctx, b.ID); !errors.Is(err, secrets.ErrNoCipher) {
		t.Fatalf("GetBoardEnvVars without cipher: err = %v; want ErrNoCipher", err)
	}
	// Listing key names needs no cipher.
	if _, err := store.ListBoardEnvVarKeys(ctx, b.ID); err != nil {
		t.Fatalf("ListBoardEnvVarKeys without cipher: %v", err)
	}
}
