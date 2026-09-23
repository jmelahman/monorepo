package db

import (
	"math"
	"testing"
	"time"
)

// addEventAt inserts a process_events row with an explicit occurred_at so the
// test can construct measurable start→healthy deltas (AddProcessEvent stamps
// 'now', which would collapse every duration to ~0).
func addEventAt(t *testing.T, s *Store, repoID int64, beHash, event, at string) {
	t.Helper()
	if _, err := s.db.Exec(
		`INSERT INTO process_events (repo_id, be_hash, event, occurred_at) VALUES (?, ?, ?, ?)`,
		repoID, beHash, event, at); err != nil {
		t.Fatal(err)
	}
}

func TestStartupDurations(t *testing.T) {
	s := newTestStore(t)
	r, err := s.CreateRepo("demo", "/src/demo", "/data/repos/demo.git", RepoReady)
	if err != nil {
		t.Fatal(err)
	}

	// A clean 2s start.
	addEventAt(t, s, r.ID, "be1", "start_attempt", "2026-08-19T10:00:00Z")
	addEventAt(t, s, r.ID, "be1", "healthy", "2026-08-19T10:00:02Z")

	// A retry: two start_attempts, then healthy 5s after the *second*. The
	// nearest-prior pairing must measure 5s, not 8s from the first attempt.
	addEventAt(t, s, r.ID, "be2", "start_attempt", "2026-08-19T11:00:00Z")
	addEventAt(t, s, r.ID, "be2", "start_attempt", "2026-08-19T11:00:03Z")
	addEventAt(t, s, r.ID, "be2", "healthy", "2026-08-19T11:00:08Z")

	// A start_attempt that never went healthy contributes no duration.
	addEventAt(t, s, r.ID, "be3", "start_attempt", "2026-08-19T12:00:00Z")

	durs, err := s.StartupDurations(3650) // wide window so the fixed dates qualify
	if err != nil {
		t.Fatal(err)
	}
	if len(durs) != 2 {
		t.Fatalf("got %d durations, want 2: %v", len(durs), durs)
	}
	// Order isn't guaranteed; check the multiset.
	want := map[float64]bool{2: false, 5: false}
	for _, d := range durs {
		matched := false
		for w := range want {
			if math.Abs(d-w) < 0.01 {
				want[w] = true
				matched = true
			}
		}
		if !matched {
			t.Fatalf("unexpected duration %v (want ~2 and ~5)", d)
		}
	}
	for w, seen := range want {
		if !seen {
			t.Fatalf("missing expected duration ~%v in %v", w, durs)
		}
	}
}

func TestStartupDurationsWindow(t *testing.T) {
	s := newTestStore(t)
	r, err := s.CreateRepo("demo", "/src/demo", "/data/repos/demo.git", RepoReady)
	if err != nil {
		t.Fatal(err)
	}
	// Healthy landed well outside a 30-day window (fixed historical date).
	addEventAt(t, s, r.ID, "be1", "start_attempt", "2020-01-01T00:00:00Z")
	addEventAt(t, s, r.ID, "be1", "healthy", "2020-01-01T00:00:03Z")

	durs, err := s.StartupDurations(30)
	if err != nil {
		t.Fatal(err)
	}
	if len(durs) != 0 {
		t.Fatalf("stale start should be excluded by the window, got %v", durs)
	}
}

// AddProcessEventAt is the worker-replay path: events shipped over the
// heartbeat must keep their original occurred_at, because durations are
// timestamp deltas between paired rows — stamping them at replay time would
// read every fleet cold start as ~0s.
func TestAddProcessEventAtFeedsStartupDurations(t *testing.T) {
	s := newTestStore(t)
	r, err := s.CreateRepo("fleet", "/src/fleet", "/data/repos/fleet.git", RepoReady)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	if err := s.AddProcessEventAt(r.ID, "beX", "start_attempt", "", start); err != nil {
		t.Fatal(err)
	}
	if err := s.AddProcessEventAt(r.ID, "beX", "healthy", "port 1", start.Add(42*time.Second)); err != nil {
		t.Fatal(err)
	}
	durs, err := s.StartupDurations(3650)
	if err != nil {
		t.Fatal(err)
	}
	if len(durs) != 1 || math.Abs(durs[0]-42) > 0.5 {
		t.Fatalf("durations = %v, want [42]", durs)
	}
}
