package db

import (
	"errors"
	"path/filepath"
	"testing"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func ptr[T any](v T) *T { return &v }

func TestMigrationsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	for range 2 {
		store, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		var v int
		if err := store.db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
			t.Fatal(err)
		}
		if v < 1 {
			t.Fatalf("user_version = %d, want >= 1", v)
		}
		store.Close()
	}
}

func TestMemoryStoresAreIsolated(t *testing.T) {
	a, b := openTest(t), openTest(t)
	if _, err := a.CreateNote("only in a"); err != nil {
		t.Fatal(err)
	}
	notes, err := b.ListNotes()
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 0 {
		t.Fatalf("store b sees store a's rows: %+v", notes)
	}
}

func TestValueGoalStep(t *testing.T) {
	s := openTest(t)
	v, err := s.CreateValue(ValuePatch{Name: ptr("Health")})
	if err != nil {
		t.Fatal(err)
	}
	g, err := s.CreateGoal(GoalPatch{ValueID: &v.ID, Title: ptr("Move my body")})
	if err != nil {
		t.Fatal(err)
	}
	if g.Status != GoalActive || *g.ValueID != v.ID {
		t.Fatalf("unexpected goal: %+v", g)
	}
	if _, err := s.CreateGoal(GoalPatch{Title: ptr("x"), ValueID: ptr(int64(999))}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for unknown value, got %v", err)
	}
	st, err := s.CreateStep("", StepPatch{GoalID: &g.ID, Title: ptr("10-min walk"), EnergyCost: ptr(2)})
	if err != nil {
		t.Fatal(err)
	}
	if st.Lane != LaneSomeday || st.EnergyCost != 2 {
		t.Fatalf("unexpected step: %+v", st)
	}
	if _, err := s.CreateStep(LaneToday, StepPatch{Title: ptr("x"), EnergyCost: ptr(4)}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for energy 4, got %v", err)
	}

	// Deleting the goal keeps the step, detached.
	if err := s.DeleteGoal(g.ID); err != nil {
		t.Fatal(err)
	}
	st, err = s.GetStep(st.ID)
	if err != nil {
		t.Fatal(err)
	}
	if st.GoalID != nil {
		t.Fatalf("goal_id = %v, want nil after goal delete", *st.GoalID)
	}
}

func TestMoveStepOrdering(t *testing.T) {
	s := openTest(t)
	var ids []int64
	for _, title := range []string{"a", "b", "c"} {
		st, err := s.CreateStep(LaneToday, StepPatch{Title: ptr(title)})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, st.ID)
	}
	// Move c to the top.
	if _, err := s.MoveStep(ids[2], LaneToday, ptr(0)); err != nil {
		t.Fatal(err)
	}
	got := titles(t, s, LaneToday)
	if want := "c,a,b"; got != want {
		t.Fatalf("order = %s, want %s", got, want)
	}

	// Complete a: lands in done with completed_at and ratings.
	done, err := s.CompleteStep(ids[0], ptr(6), ptr(4))
	if err != nil {
		t.Fatal(err)
	}
	if done.Lane != LaneDone || done.CompletedAt == nil || *done.Mastery != 6 || *done.Pleasure != 4 {
		t.Fatalf("unexpected completed step: %+v", done)
	}
	if got := titles(t, s, LaneToday); got != "c,b" {
		t.Fatalf("today after complete = %s", got)
	}

	// Moving back out of done clears completed_at.
	back, err := s.MoveStep(ids[0], LaneWeek, nil)
	if err != nil {
		t.Fatal(err)
	}
	if back.CompletedAt != nil {
		t.Fatalf("completed_at should clear, got %v", *back.CompletedAt)
	}
	if _, err := s.MoveStep(ids[0], "bogus", nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid, got %v", err)
	}
}

func titles(t *testing.T, s *Store, lane string) string {
	t.Helper()
	steps, err := s.ListSteps(StepFilter{Lanes: []string{lane}})
	if err != nil {
		t.Fatal(err)
	}
	out := ""
	for i, st := range steps {
		if i > 0 {
			out += ","
		}
		out += st.Title
	}
	return out
}

func TestCheckinsAndMessages(t *testing.T) {
	s := openTest(t)
	c, err := s.CreateCheckin(CheckinPatch{Date: ptr("2026-09-25"), Mood: ptr(4), Energy: ptr(3)})
	if err != nil {
		t.Fatal(err)
	}
	if c.Kind != KindMorning || *c.Mood != 4 || c.Anxiety != nil {
		t.Fatalf("unexpected checkin: %+v", c)
	}
	if _, err := s.CreateCheckin(CheckinPatch{Date: ptr("2026-09-25"), Mood: ptr(11)}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for mood 11, got %v", err)
	}
	latest, err := s.LatestCheckin("2026-09-25", KindMorning, "")
	if err != nil || latest.ID != c.ID {
		t.Fatalf("LatestCheckin = %+v, %v", latest, err)
	}
	if _, err := s.LatestCheckin("2026-09-25", KindEvening, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	for _, role := range []string{"user", "assistant"} {
		if _, err := s.AppendMessage(c.ID, role, role+" text", ""); err != nil {
			t.Fatal(err)
		}
	}
	msgs, err := s.ListMessages(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 || msgs[0].Seq != 1 || msgs[1].Seq != 2 {
		t.Fatalf("unexpected messages: %+v", msgs)
	}
}

func TestThoughtRecordRoundTrip(t *testing.T) {
	s := openTest(t)
	tr, err := s.CreateThoughtRecord(ThoughtRecordPatch{
		Situation:        ptr("Friend didn't text back"),
		Emotions:         &[]Emotion{{Name: "anxious", Intensity: 70}, {Name: " "}},
		AutomaticThought: ptr("They're mad at me"),
		Distortions:      &[]string{"mind_reading"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tr.Emotions) != 1 || tr.Distortions[0] != "mind_reading" || len(tr.ReratedEmotions) != 0 {
		t.Fatalf("unexpected record: %+v", tr)
	}
	if _, err := s.CreateThoughtRecord(ThoughtRecordPatch{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid for empty record, got %v", err)
	}
}

func TestWeekAndRetro(t *testing.T) {
	s := openTest(t)
	w, err := s.EnsureWeek("2026-09-21")
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.EnsureWeek("2026-09-21")
	if err != nil || again.ID != w.ID {
		t.Fatalf("EnsureWeek not idempotent: %+v %v", again, err)
	}
	if _, err := s.GetRetro(w.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected no retro yet, got %v", err)
	}
	r, err := s.UpsertRetro(w.ID, RetroPatch{WentWell: ptr("walked 3x")})
	if err != nil {
		t.Fatal(err)
	}
	r, err = s.UpsertRetro(w.ID, RetroPatch{TryNext: ptr("walk after lunch")})
	if err != nil {
		t.Fatal(err)
	}
	if r.WentWell != "walked 3x" || r.TryNext != "walk after lunch" {
		t.Fatalf("unexpected retro: %+v", r)
	}
}

func TestPutRestoresDeletedRow(t *testing.T) {
	s := openTest(t)
	n, err := s.CreateNote("prefers mornings")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteNote(n.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.PutNote(n); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetNote(n.ID)
	if err != nil || got != n {
		t.Fatalf("restored note = %+v, %v; want %+v", got, err, n)
	}
}

func TestMoveToSameLaneRecommits(t *testing.T) {
	s := openTest(t)
	st, err := s.CreateStep(LaneToday, StepPatch{Title: ptr("stretch")})
	if err != nil {
		t.Fatal(err)
	}
	old := "2020-01-01T00:00:00Z"
	if _, err := s.db.Exec(`UPDATE steps SET lane_changed_at = ? WHERE id = ?`, old, st.ID); err != nil {
		t.Fatal(err)
	}
	// Reordering within a lane keeps the original commitment time...
	got, err := s.MoveStep(st.ID, LaneToday, ptr(0))
	if err != nil {
		t.Fatal(err)
	}
	if got.LaneChangedAt != old {
		t.Fatalf("reorder changed lane_changed_at to %s", got.LaneChangedAt)
	}
	// ...but "keep for today" (same lane, no index) refreshes it.
	if got, err = s.MoveStep(st.ID, LaneToday, nil); err != nil {
		t.Fatal(err)
	}
	if got.LaneChangedAt == old {
		t.Fatal("same-lane move without index should refresh lane_changed_at")
	}
}
