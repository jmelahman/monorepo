package api

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/jmelahman/kanban/internal/config"
)

func TestGuardKeyWorktreesLock(t *testing.T) {
	dir := t.TempDir()
	locked, err := config.Load(dir, filepath.Join(dir, "wt"), 13000, 13099)
	if err != nil {
		t.Fatal(err)
	}
	h := &handlers{config: locked}
	if status, err := h.guardKey("worktrees.root", "x"); err == nil || status != http.StatusConflict {
		t.Fatalf("locked worktrees: status=%d err=%v, want 409", status, err)
	}

	unlocked, err := config.Load(dir, "", 13000, 13099)
	if err != nil {
		t.Fatal(err)
	}
	hu := &handlers{config: unlocked}
	if status, err := hu.guardKey("worktrees.root", "x"); err != nil || status != 0 {
		t.Fatalf("unlocked worktrees: status=%d err=%v, want ok", status, err)
	}
}

func TestGuardKeyHarness(t *testing.T) {
	h := &handlers{}
	if status, err := h.guardKey("harness.id", "definitely-not-a-harness"); err == nil || status != 400 {
		t.Fatalf("unknown harness: status=%d err=%v, want 400", status, err)
	}
	// Empty id (clearing the key) is always allowed.
	if status, err := h.guardKey("harness.id", ""); err != nil || status != 0 {
		t.Fatalf("empty harness: status=%d err=%v, want ok", status, err)
	}
}
