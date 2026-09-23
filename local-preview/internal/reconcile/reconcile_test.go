package reconcile

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/jmelahman/local-preview/internal/s3store"
	"github.com/jmelahman/local-preview/internal/store"
)

// fakeTier is an in-memory reconcile.Tier. Objects carry the integrity metadata
// the pass checks; Save/Delete mutate the set and record what they were asked.
type fakeTier struct {
	mu      sync.Mutex
	objs    map[string]s3store.ObjectInfo
	saved   []string
	deleted []string
}

func fk(repo, side, hash string) string { return repo + "/" + side + "/" + hash }

func (f *fakeTier) put(repo, side, hash string, size, count int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.objs == nil {
		f.objs = map[string]s3store.ObjectInfo{}
	}
	f.objs[fk(repo, side, hash)] = s3store.ObjectInfo{UncompressedSize: size, FileCount: count, CompressedSize: size}
}

func (f *fakeTier) Stat(_ context.Context, repo, side, hash string) (s3store.ObjectInfo, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.objs[fk(repo, side, hash)]
	return o, ok, nil
}

func (f *fakeTier) Save(_ context.Context, repo, side, hash, srcDir string) error {
	// Count regular files so a re-Save lands consistent metadata.
	var count int64
	filepath.WalkDir(srcDir, func(_ string, d os.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			count++
		}
		return nil
	})
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.objs == nil {
		f.objs = map[string]s3store.ObjectInfo{}
	}
	f.saved = append(f.saved, fk(repo, side, hash))
	f.objs[fk(repo, side, hash)] = s3store.ObjectInfo{UncompressedSize: 1, FileCount: count, CompressedSize: 1}
	return nil
}

func (f *fakeTier) Delete(_ context.Context, repo, side, hash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, fk(repo, side, hash))
	delete(f.objs, fk(repo, side, hash))
	return nil
}

func newStore(t *testing.T) *store.Store {
	root := t.TempDir()
	return store.New(filepath.Join(root, "artifacts"), filepath.Join(root, "state"), filepath.Join(root, "tmp"))
}

// publishFE publishes a resident frontend artifact with nFiles regular files.
func publishFE(t *testing.T, s *store.Store, repo, hash string, nFiles int) {
	t.Helper()
	src := t.TempDir()
	for i := 0; i < nFiles; i++ {
		if err := os.WriteFile(filepath.Join(src, "f"+string(rune('a'+i))), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.PublishFrontend(repo, hash, src, false); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileClassifies(t *testing.T) {
	s := newStore(t)
	tier := &fakeTier{}
	r := New(nil, s, tier)

	// A: resident + present + consistent → AlreadyOK.
	publishFE(t, s, "demo", "aaa", 2)
	tier.put("demo", "fe", "aaa", 100, 2)

	// B: resident + absent from tier → Persisted.
	publishFE(t, s, "demo", "bbb", 1)

	// C: absent from tier + not resident → Gap.
	// (nothing published, nothing in tier)

	// D: resident + present but file-count mismatch → Repaired.
	publishFE(t, s, "demo", "ddd", 3)
	tier.put("demo", "fe", "ddd", 100, 99) // wrong count

	refs := []sideRef{
		{"demo", "fe", "aaa"},
		{"demo", "fe", "bbb"},
		{"demo", "fe", "ccc"},
		{"demo", "fe", "ddd"},
	}
	rep := r.reconcile(context.Background(), refs)

	if rep.Checked != 4 {
		t.Fatalf("checked = %d, want 4", rep.Checked)
	}
	if rep.AlreadyOK != 1 {
		t.Fatalf("already-ok = %d, want 1", rep.AlreadyOK)
	}
	if rep.Persisted != 1 {
		t.Fatalf("persisted = %d, want 1", rep.Persisted)
	}
	if rep.Repaired != 1 {
		t.Fatalf("repaired = %d, want 1", rep.Repaired)
	}
	if rep.Gaps != 1 || len(rep.GapKeys) != 1 || rep.GapKeys[0] != "demo/fe/ccc" {
		t.Fatalf("gaps = %d keys=%v, want 1 [demo/fe/ccc]", rep.Gaps, rep.GapKeys)
	}
	// The repaired object was deleted before re-Save, and both B and D uploaded.
	if len(tier.deleted) != 1 || tier.deleted[0] != "demo/fe/ddd" {
		t.Fatalf("deleted = %v, want [demo/fe/ddd]", tier.deleted)
	}
	if len(tier.saved) != 2 {
		t.Fatalf("saved = %v, want 2 (bbb, ddd)", tier.saved)
	}
	// After repair the object is present and consistent: a second pass is clean.
	rep2 := r.reconcile(context.Background(), refs)
	if rep2.AlreadyOK != 3 || rep2.Persisted != 0 || rep2.Repaired != 0 || rep2.Gaps != 1 {
		t.Fatalf("second pass = %+v, want 3 ok / 0 persisted / 0 repaired / 1 gap", rep2)
	}
}
