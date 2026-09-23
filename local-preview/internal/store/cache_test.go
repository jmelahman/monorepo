package store

import (
	"strings"
	"testing"
	"time"
)

// publishSideBytes publishes a frontend artifact whose single file is n bytes,
// returning nothing — the test reads sizes back via ResidentArtifactBytes.
func publishSideBytes(t *testing.T, s *Store, repo, hash string, n int) {
	t.Helper()
	src := t.TempDir()
	writeTree(t, src, map[string]string{"blob": strings.Repeat("x", n)})
	if err := s.PublishFrontend(repo, hash, src, false); err != nil {
		t.Fatal(err)
	}
}

func TestEvictNoopWithoutTier(t *testing.T) {
	s := newStore(t)
	publishSideBytes(t, s, "demo", "a", 1000)
	// No tier: local disk is the only copy, so eviction must never touch it,
	// even far over budget.
	freed, err := s.EvictCacheToWatermark(0, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if freed != 0 || !s.HasFrontend("demo", "a") {
		t.Fatalf("evicted without a tier: freed=%d present=%v", freed, s.HasFrontend("demo", "a"))
	}
}

func TestEvictColdestFirst(t *testing.T) {
	s := newStore(t)
	s.SetArtifactTier(&fakeTier{})
	publishSideBytes(t, s, "demo", "cold", 1000)
	publishSideBytes(t, s, "demo", "hot", 10)
	s.NoteAccess("demo", "fe", "hot") // make hot the most-recently-used

	if got := s.ResidentArtifactBytes(); got != 1010 {
		t.Fatalf("resident bytes = %d, want 1010", got)
	}

	// Budget fits only the small hot artifact; the cold one must be reclaimed.
	freed, err := s.EvictCacheToWatermark(100, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if freed != 1000 {
		t.Fatalf("freed = %d, want 1000", freed)
	}
	if s.HasFrontend("demo", "cold") {
		t.Fatal("cold artifact should have been evicted")
	}
	if !s.HasFrontend("demo", "hot") {
		t.Fatal("hot artifact should have survived")
	}
}

func TestEvictRespectsMinAge(t *testing.T) {
	s := newStore(t)
	s.SetArtifactTier(&fakeTier{})
	publishSideBytes(t, s, "demo", "fresh", 1000)
	// Everything is freshly published; a large minAge protects it all.
	freed, err := s.EvictCacheToWatermark(0, time.Hour, nil)
	if err != nil {
		t.Fatal(err)
	}
	if freed != 0 || !s.HasFrontend("demo", "fresh") {
		t.Fatalf("min-age guard failed: freed=%d present=%v", freed, s.HasFrontend("demo", "fresh"))
	}
}

func TestEvictRespectsProtect(t *testing.T) {
	s := newStore(t)
	s.SetArtifactTier(&fakeTier{})
	publishSideBytes(t, s, "demo", "pinned", 1000)
	protect := func(repo, side, hash string) bool { return hash == "pinned" }
	freed, err := s.EvictCacheToWatermark(0, 0, protect)
	if err != nil {
		t.Fatal(err)
	}
	if freed != 0 || !s.HasFrontend("demo", "pinned") {
		t.Fatalf("protect predicate ignored: freed=%d present=%v", freed, s.HasFrontend("demo", "pinned"))
	}
}
