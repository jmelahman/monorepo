package db

import (
	"strings"
)

// Step lanes, in board order. LetGo is the gentle alternative to deleting.
const (
	LaneSomeday = "someday"
	LaneWeek    = "week"
	LaneToday   = "today"
	LaneDone    = "done"
	LaneLetGo   = "let_go"
)

// ValidLane reports whether lane is one of the known lanes.
func ValidLane(lane string) bool {
	switch lane {
	case LaneSomeday, LaneWeek, LaneToday, LaneDone, LaneLetGo:
		return true
	}
	return false
}

// Step is a small, concrete action on the board.
type Step struct {
	ID                int64   `json:"id"`
	GoalID            *int64  `json:"goal_id"`
	Title             string  `json:"title"`
	Notes             string  `json:"notes"`
	EnergyCost        int     `json:"energy_cost"`
	Lane              string  `json:"lane"`
	SortOrder         int     `json:"sort_order"`
	LaneChangedAt     string  `json:"lane_changed_at"`
	CompletedAt       *string `json:"completed_at"`
	PredictedPleasure *int    `json:"predicted_pleasure"`
	Mastery           *int    `json:"mastery"`
	Pleasure          *int    `json:"pleasure"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
}

// StepPatch holds optional fields for creating or updating a Step. Lane and
// ordering changes go through MoveStep so sort orders stay consistent.
type StepPatch struct {
	GoalID            *int64  `json:"goal_id"`
	ClearGoal         bool    `json:"clear_goal"`
	Title             *string `json:"title"`
	Notes             *string `json:"notes"`
	EnergyCost        *int    `json:"energy_cost"`
	PredictedPleasure *int    `json:"predicted_pleasure"`
	Mastery           *int    `json:"mastery"`
	Pleasure          *int    `json:"pleasure"`
}

// StepFilter narrows ListSteps. Zero values mean "no filter".
type StepFilter struct {
	Lanes  []string
	GoalID *int64
	// CompletedSince keeps only steps completed at or after this UTC
	// timestamp (RFC 3339). Implies the done lane.
	CompletedSince string
}

const stepCols = `id, goal_id, title, notes, energy_cost, lane, sort_order, lane_changed_at,
	completed_at, predicted_pleasure, mastery, pleasure, created_at, updated_at`

func scanStep(r rowScanner) (Step, error) {
	var st Step
	err := r.Scan(&st.ID, &st.GoalID, &st.Title, &st.Notes, &st.EnergyCost, &st.Lane, &st.SortOrder,
		&st.LaneChangedAt, &st.CompletedAt, &st.PredictedPleasure, &st.Mastery, &st.Pleasure,
		&st.CreatedAt, &st.UpdatedAt)
	return st, err
}

// ListSteps returns steps matching f, ordered by lane position.
func (s *Store) ListSteps(f StepFilter) ([]Step, error) {
	var where []string
	var args []any
	if len(f.Lanes) > 0 {
		ph := make([]string, len(f.Lanes))
		for i, l := range f.Lanes {
			ph[i] = "?"
			args = append(args, l)
		}
		where = append(where, "lane IN ("+strings.Join(ph, ",")+")")
	}
	if f.GoalID != nil {
		where = append(where, "goal_id = ?")
		args = append(args, *f.GoalID)
	}
	if f.CompletedSince != "" {
		where = append(where, "lane = 'done' AND completed_at >= ?")
		args = append(args, f.CompletedSince)
	}
	q := `SELECT ` + stepCols + ` FROM steps`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY sort_order, id"
	return queryAll(s, scanStep, q, args...)
}

// GetStep returns one step by id.
func (s *Store) GetStep(id int64) (Step, error) {
	st, err := scanStep(s.db.QueryRow(`SELECT `+stepCols+` FROM steps WHERE id = ?`, id))
	return st, notFound(err)
}

// CreateStep inserts a step at the end of lane (default someday).
func (s *Store) CreateStep(lane string, p StepPatch) (Step, error) {
	if lane == "" {
		lane = LaneSomeday
	}
	if !ValidLane(lane) {
		return Step{}, invalid("unknown lane %q", lane)
	}
	st := Step{EnergyCost: 1, Lane: lane}
	if err := s.applyStepPatch(&st, p); err != nil {
		return Step{}, err
	}
	var completedAt *string
	if lane == LaneDone {
		now := nowUTC()
		completedAt = &now
	}
	return scanStep(s.db.QueryRow(`INSERT INTO steps
		(goal_id, title, notes, energy_cost, lane, sort_order, completed_at, predicted_pleasure, mastery, pleasure)
		VALUES (?, ?, ?, ?, ?, (SELECT COALESCE(MAX(sort_order), -1) + 1 FROM steps WHERE lane = ?), ?, ?, ?, ?)
		RETURNING `+stepCols,
		st.GoalID, st.Title, st.Notes, st.EnergyCost, lane, lane, completedAt,
		st.PredictedPleasure, st.Mastery, st.Pleasure))
}

// UpdateStep applies p to the step with id.
func (s *Store) UpdateStep(id int64, p StepPatch) (Step, error) {
	st, err := s.GetStep(id)
	if err != nil {
		return Step{}, err
	}
	if err := s.applyStepPatch(&st, p); err != nil {
		return Step{}, err
	}
	st.UpdatedAt = nowUTC()
	return st, s.PutStep(st)
}

// MoveStep moves a step into lane at position index (nil = end of lane) and
// renumbers the affected lanes. Moving into done stamps completed_at; moving
// out of done clears it.
func (s *Store) MoveStep(id int64, lane string, index *int) (Step, error) {
	if !ValidLane(lane) {
		return Step{}, invalid("unknown lane %q", lane)
	}
	st, err := s.GetStep(id)
	if err != nil {
		return Step{}, err
	}
	err = s.Tx(func(t *Store) error { return t.moveStep(st, lane, index) })
	if err != nil {
		return Step{}, err
	}
	return s.GetStep(id)
}

func (s *Store) moveStep(st Step, lane string, index *int) error {
	id, tx := st.ID, s.db
	rows, err := tx.Query(`SELECT id FROM steps WHERE lane = ? AND id != ? ORDER BY sort_order, id`, lane, id)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var sid int64
		if err := rows.Scan(&sid); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, sid)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	pos := len(ids)
	if index != nil && *index >= 0 && *index < pos {
		pos = *index
	}
	ids = append(ids[:pos], append([]int64{id}, ids[pos:]...)...)
	for i, sid := range ids {
		if _, err := tx.Exec(`UPDATE steps SET sort_order = ? WHERE id = ?`, i, sid); err != nil {
			return err
		}
	}

	now := nowUTC()
	if lane != st.Lane {
		completedAt := st.CompletedAt
		switch {
		case lane == LaneDone:
			completedAt = &now
		case st.Lane == LaneDone:
			completedAt = nil
		}
		if _, err := tx.Exec(`UPDATE steps SET lane = ?, lane_changed_at = ?, completed_at = ?, updated_at = ? WHERE id = ?`,
			lane, now, completedAt, now, id); err != nil {
			return err
		}
	} else if index == nil {
		// Moving a step to the lane it's already in, without reordering, is a
		// fresh commitment (e.g. "keep for today"), so it's no longer carried over.
		if _, err := tx.Exec(`UPDATE steps SET lane_changed_at = ?, updated_at = ? WHERE id = ?`, now, now, id); err != nil {
			return err
		}
	}
	return nil
}

// CompleteStep moves a step to done and records its mastery/pleasure ratings.
func (s *Store) CompleteStep(id int64, mastery, pleasure *int) (Step, error) {
	if _, err := s.UpdateStep(id, StepPatch{Mastery: mastery, Pleasure: pleasure}); err != nil {
		return Step{}, err
	}
	return s.MoveStep(id, LaneDone, nil)
}

// PutStep writes st verbatim (insert or full update by id). Used for undo.
func (s *Store) PutStep(st Step) error {
	_, err := s.db.Exec(`INSERT INTO steps (`+stepCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET goal_id = excluded.goal_id, title = excluded.title,
		notes = excluded.notes, energy_cost = excluded.energy_cost, lane = excluded.lane,
		sort_order = excluded.sort_order, lane_changed_at = excluded.lane_changed_at,
		completed_at = excluded.completed_at, predicted_pleasure = excluded.predicted_pleasure,
		mastery = excluded.mastery, pleasure = excluded.pleasure, updated_at = excluded.updated_at`,
		st.ID, st.GoalID, st.Title, st.Notes, st.EnergyCost, st.Lane, st.SortOrder, st.LaneChangedAt,
		st.CompletedAt, st.PredictedPleasure, st.Mastery, st.Pleasure, st.CreatedAt, st.UpdatedAt)
	return err
}

// DeleteStep permanently removes a step. The UI prefers the let_go lane; this
// exists for cleanup and for undoing an AI-created step.
func (s *Store) DeleteStep(id int64) error {
	return rowsAffected(s.db.Exec(`DELETE FROM steps WHERE id = ?`, id))
}

func (s *Store) applyStepPatch(st *Step, p StepPatch) error {
	if p.ClearGoal {
		st.GoalID = nil
	} else if p.GoalID != nil {
		if _, err := s.GetGoal(*p.GoalID); err != nil {
			return invalid("goal %d does not exist", *p.GoalID)
		}
		st.GoalID = p.GoalID
	}
	if p.Title != nil {
		st.Title = strings.TrimSpace(*p.Title)
	}
	if p.Notes != nil {
		st.Notes = strings.TrimSpace(*p.Notes)
	}
	if p.EnergyCost != nil {
		if *p.EnergyCost < 1 || *p.EnergyCost > 3 {
			return invalid("energy_cost must be 1, 2, or 3")
		}
		st.EnergyCost = *p.EnergyCost
	}
	for _, r := range []struct {
		name string
		src  *int
		dst  **int
	}{
		{"predicted_pleasure", p.PredictedPleasure, &st.PredictedPleasure},
		{"mastery", p.Mastery, &st.Mastery},
		{"pleasure", p.Pleasure, &st.Pleasure},
	} {
		if r.src == nil {
			continue
		}
		if *r.src < 0 || *r.src > 10 {
			return invalid("%s must be between 0 and 10", r.name)
		}
		v := *r.src
		*r.dst = &v
	}
	if st.Title == "" {
		return invalid("title is required")
	}
	return nil
}
