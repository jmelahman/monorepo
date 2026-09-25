package db

import (
	"strings"
)

// Value is a life area that goals serve (Health, Connection, ...).
type Value struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
	SortOrder   int    `json:"sort_order"`
	CreatedAt   string `json:"created_at"`
}

// ValuePatch holds optional fields for creating or updating a Value.
type ValuePatch struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Color       *string `json:"color"`
	SortOrder   *int    `json:"sort_order"`
}

const valueCols = `id, name, description, color, sort_order, created_at`

func scanValue(r rowScanner) (Value, error) {
	var v Value
	err := r.Scan(&v.ID, &v.Name, &v.Description, &v.Color, &v.SortOrder, &v.CreatedAt)
	return v, err
}

// ListValues returns all values in display order.
func (s *Store) ListValues() ([]Value, error) {
	return queryAll(s, scanValue, `SELECT `+valueCols+` FROM life_values ORDER BY sort_order, id`)
}

// GetValue returns one value by id.
func (s *Store) GetValue(id int64) (Value, error) {
	v, err := scanValue(s.db.QueryRow(`SELECT `+valueCols+` FROM life_values WHERE id = ?`, id))
	return v, notFound(err)
}

// CreateValue inserts a value; Name is required.
func (s *Store) CreateValue(p ValuePatch) (Value, error) {
	v := Value{}
	applyValuePatch(&v, p)
	if v.Name == "" {
		return Value{}, invalid("name is required")
	}
	return scanValue(s.db.QueryRow(
		`INSERT INTO life_values (name, description, color, sort_order) VALUES (?, ?, ?, ?) RETURNING `+valueCols,
		v.Name, v.Description, v.Color, v.SortOrder))
}

// UpdateValue applies p to the value with id.
func (s *Store) UpdateValue(id int64, p ValuePatch) (Value, error) {
	v, err := s.GetValue(id)
	if err != nil {
		return Value{}, err
	}
	applyValuePatch(&v, p)
	if v.Name == "" {
		return Value{}, invalid("name is required")
	}
	return v, s.PutValue(v)
}

// PutValue writes v verbatim (insert or full update by id). Used for undo.
func (s *Store) PutValue(v Value) error {
	_, err := s.db.Exec(`INSERT INTO life_values (`+valueCols+`) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name = excluded.name, description = excluded.description,
		color = excluded.color, sort_order = excluded.sort_order`,
		v.ID, v.Name, v.Description, v.Color, v.SortOrder, v.CreatedAt)
	return err
}

// DeleteValue removes a value; its goals keep existing without a value.
func (s *Store) DeleteValue(id int64) error {
	return rowsAffected(s.db.Exec(`DELETE FROM life_values WHERE id = ?`, id))
}

func applyValuePatch(v *Value, p ValuePatch) {
	if p.Name != nil {
		v.Name = strings.TrimSpace(*p.Name)
	}
	if p.Description != nil {
		v.Description = strings.TrimSpace(*p.Description)
	}
	if p.Color != nil {
		v.Color = strings.TrimSpace(*p.Color)
	}
	if p.SortOrder != nil {
		v.SortOrder = *p.SortOrder
	}
}

// Goal statuses. "resting" is a guilt-free pause, not a failure.
const (
	GoalActive  = "active"
	GoalResting = "resting"
	GoalDone    = "done"
)

// Goal is a roadmap epic.
type Goal struct {
	ID        int64  `json:"id"`
	ValueID   *int64 `json:"value_id"`
	Title     string `json:"title"`
	Why       string `json:"why"`
	Horizon   string `json:"horizon"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// GoalPatch holds optional fields for creating or updating a Goal. Set
// ClearValue to detach the goal from its value.
type GoalPatch struct {
	ValueID    *int64  `json:"value_id"`
	ClearValue bool    `json:"clear_value"`
	Title      *string `json:"title"`
	Why        *string `json:"why"`
	Horizon    *string `json:"horizon"`
	Status     *string `json:"status"`
}

const goalCols = `id, value_id, title, why, horizon, status, created_at, updated_at`

func scanGoal(r rowScanner) (Goal, error) {
	var g Goal
	err := r.Scan(&g.ID, &g.ValueID, &g.Title, &g.Why, &g.Horizon, &g.Status, &g.CreatedAt, &g.UpdatedAt)
	return g, err
}

// ListGoals returns goals, optionally filtered by status ("" for all).
func (s *Store) ListGoals(status string) ([]Goal, error) {
	if status != "" {
		return queryAll(s, scanGoal, `SELECT `+goalCols+` FROM goals WHERE status = ? ORDER BY id`, status)
	}
	return queryAll(s, scanGoal, `SELECT `+goalCols+` FROM goals ORDER BY id`)
}

// GetGoal returns one goal by id.
func (s *Store) GetGoal(id int64) (Goal, error) {
	g, err := scanGoal(s.db.QueryRow(`SELECT `+goalCols+` FROM goals WHERE id = ?`, id))
	return g, notFound(err)
}

// CreateGoal inserts a goal; Title is required.
func (s *Store) CreateGoal(p GoalPatch) (Goal, error) {
	g := Goal{Status: GoalActive}
	if err := s.applyGoalPatch(&g, p); err != nil {
		return Goal{}, err
	}
	return scanGoal(s.db.QueryRow(
		`INSERT INTO goals (value_id, title, why, horizon, status) VALUES (?, ?, ?, ?, ?) RETURNING `+goalCols,
		g.ValueID, g.Title, g.Why, g.Horizon, g.Status))
}

// UpdateGoal applies p to the goal with id.
func (s *Store) UpdateGoal(id int64, p GoalPatch) (Goal, error) {
	g, err := s.GetGoal(id)
	if err != nil {
		return Goal{}, err
	}
	if err := s.applyGoalPatch(&g, p); err != nil {
		return Goal{}, err
	}
	g.UpdatedAt = nowUTC()
	return g, s.PutGoal(g)
}

// PutGoal writes g verbatim (insert or full update by id). Used for undo.
func (s *Store) PutGoal(g Goal) error {
	_, err := s.db.Exec(`INSERT INTO goals (`+goalCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET value_id = excluded.value_id, title = excluded.title,
		why = excluded.why, horizon = excluded.horizon, status = excluded.status,
		updated_at = excluded.updated_at`,
		g.ID, g.ValueID, g.Title, g.Why, g.Horizon, g.Status, g.CreatedAt, g.UpdatedAt)
	return err
}

// DeleteGoal removes a goal; its steps stay on the board without a goal.
func (s *Store) DeleteGoal(id int64) error {
	return rowsAffected(s.db.Exec(`DELETE FROM goals WHERE id = ?`, id))
}

func (s *Store) applyGoalPatch(g *Goal, p GoalPatch) error {
	if p.ClearValue {
		g.ValueID = nil
	} else if p.ValueID != nil {
		if _, err := s.GetValue(*p.ValueID); err != nil {
			return invalid("value %d does not exist", *p.ValueID)
		}
		g.ValueID = p.ValueID
	}
	if p.Title != nil {
		g.Title = strings.TrimSpace(*p.Title)
	}
	if p.Why != nil {
		g.Why = strings.TrimSpace(*p.Why)
	}
	if p.Horizon != nil {
		g.Horizon = strings.TrimSpace(*p.Horizon)
	}
	if p.Status != nil {
		switch *p.Status {
		case GoalActive, GoalResting, GoalDone:
			g.Status = *p.Status
		default:
			return invalid("status must be active, resting, or done")
		}
	}
	if g.Title == "" {
		return invalid("title is required")
	}
	return nil
}
