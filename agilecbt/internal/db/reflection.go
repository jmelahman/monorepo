package db

import (
	"encoding/json"
	"strings"
)

// Emotion is a named feeling with an intensity from 0 to 100.
type Emotion struct {
	Name      string `json:"name"`
	Intensity int    `json:"intensity"`
}

// ThoughtRecord is a CBT thought record.
type ThoughtRecord struct {
	ID               int64     `json:"id"`
	Situation        string    `json:"situation"`
	Emotions         []Emotion `json:"emotions"`
	AutomaticThought string    `json:"automatic_thought"`
	Distortions      []string  `json:"distortions"`
	EvidenceFor      string    `json:"evidence_for"`
	EvidenceAgainst  string    `json:"evidence_against"`
	BalancedThought  string    `json:"balanced_thought"`
	ReratedEmotions  []Emotion `json:"rerated_emotions"`
	CreatedAt        string    `json:"created_at"`
	UpdatedAt        string    `json:"updated_at"`
}

// ThoughtRecordPatch holds optional fields for creating or updating a record.
type ThoughtRecordPatch struct {
	Situation        *string    `json:"situation"`
	Emotions         *[]Emotion `json:"emotions"`
	AutomaticThought *string    `json:"automatic_thought"`
	Distortions      *[]string  `json:"distortions"`
	EvidenceFor      *string    `json:"evidence_for"`
	EvidenceAgainst  *string    `json:"evidence_against"`
	BalancedThought  *string    `json:"balanced_thought"`
	ReratedEmotions  *[]Emotion `json:"rerated_emotions"`
}

const thoughtCols = `id, situation, emotions_json, automatic_thought, distortions_json, evidence_for,
	evidence_against, balanced_thought, rerated_emotions_json, created_at, updated_at`

func scanThought(r rowScanner) (ThoughtRecord, error) {
	var t ThoughtRecord
	var emotions, distortions, rerated string
	if err := r.Scan(&t.ID, &t.Situation, &emotions, &t.AutomaticThought, &distortions, &t.EvidenceFor,
		&t.EvidenceAgainst, &t.BalancedThought, &rerated, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return t, err
	}
	t.Emotions, t.Distortions, t.ReratedEmotions = []Emotion{}, []string{}, []Emotion{}
	for _, f := range []struct {
		raw string
		dst any
	}{{emotions, &t.Emotions}, {distortions, &t.Distortions}, {rerated, &t.ReratedEmotions}} {
		if err := json.Unmarshal([]byte(f.raw), f.dst); err != nil {
			return t, err
		}
	}
	return t, nil
}

// ListThoughtRecords returns records newest first, at most limit (0 = all).
func (s *Store) ListThoughtRecords(limit int) ([]ThoughtRecord, error) {
	if limit <= 0 {
		limit = -1
	}
	return queryAll(s, scanThought, `SELECT `+thoughtCols+` FROM thought_records ORDER BY id DESC LIMIT ?`, limit)
}

// ListThoughtRecordsSince returns records created at or after since (UTC).
func (s *Store) ListThoughtRecordsSince(since string) ([]ThoughtRecord, error) {
	return queryAll(s, scanThought, `SELECT `+thoughtCols+` FROM thought_records WHERE created_at >= ? ORDER BY id`, since)
}

// GetThoughtRecord returns one record by id.
func (s *Store) GetThoughtRecord(id int64) (ThoughtRecord, error) {
	t, err := scanThought(s.db.QueryRow(`SELECT `+thoughtCols+` FROM thought_records WHERE id = ?`, id))
	return t, notFound(err)
}

// CreateThoughtRecord inserts a record. At least a situation or an automatic
// thought is required.
func (s *Store) CreateThoughtRecord(p ThoughtRecordPatch) (ThoughtRecord, error) {
	now := nowUTC()
	t := ThoughtRecord{Emotions: []Emotion{}, Distortions: []string{}, ReratedEmotions: []Emotion{}, CreatedAt: now, UpdatedAt: now}
	if err := applyThoughtPatch(&t, p); err != nil {
		return ThoughtRecord{}, err
	}
	res, err := s.db.Exec(`INSERT INTO thought_records (situation, emotions_json, automatic_thought,
		distortions_json, evidence_for, evidence_against, balanced_thought, rerated_emotions_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		t.Situation, mustJSON(t.Emotions), t.AutomaticThought, mustJSON(t.Distortions),
		t.EvidenceFor, t.EvidenceAgainst, t.BalancedThought, mustJSON(t.ReratedEmotions))
	if err != nil {
		return ThoughtRecord{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return ThoughtRecord{}, err
	}
	return s.GetThoughtRecord(id)
}

// UpdateThoughtRecord applies p to the record with id.
func (s *Store) UpdateThoughtRecord(id int64, p ThoughtRecordPatch) (ThoughtRecord, error) {
	t, err := s.GetThoughtRecord(id)
	if err != nil {
		return ThoughtRecord{}, err
	}
	if err := applyThoughtPatch(&t, p); err != nil {
		return ThoughtRecord{}, err
	}
	t.UpdatedAt = nowUTC()
	return t, s.PutThoughtRecord(t)
}

// PutThoughtRecord writes t verbatim (insert or full update by id).
func (s *Store) PutThoughtRecord(t ThoughtRecord) error {
	_, err := s.db.Exec(`INSERT INTO thought_records (`+thoughtCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET situation = excluded.situation, emotions_json = excluded.emotions_json,
		automatic_thought = excluded.automatic_thought, distortions_json = excluded.distortions_json,
		evidence_for = excluded.evidence_for, evidence_against = excluded.evidence_against,
		balanced_thought = excluded.balanced_thought, rerated_emotions_json = excluded.rerated_emotions_json,
		updated_at = excluded.updated_at`,
		t.ID, t.Situation, mustJSON(t.Emotions), t.AutomaticThought, mustJSON(t.Distortions), t.EvidenceFor,
		t.EvidenceAgainst, t.BalancedThought, mustJSON(t.ReratedEmotions), t.CreatedAt, t.UpdatedAt)
	return err
}

// DeleteThoughtRecord removes a record.
func (s *Store) DeleteThoughtRecord(id int64) error {
	return rowsAffected(s.db.Exec(`DELETE FROM thought_records WHERE id = ?`, id))
}

func applyThoughtPatch(t *ThoughtRecord, p ThoughtRecordPatch) error {
	for _, f := range []struct {
		src *string
		dst *string
	}{
		{p.Situation, &t.Situation}, {p.AutomaticThought, &t.AutomaticThought},
		{p.EvidenceFor, &t.EvidenceFor}, {p.EvidenceAgainst, &t.EvidenceAgainst},
		{p.BalancedThought, &t.BalancedThought},
	} {
		if f.src != nil {
			*f.dst = strings.TrimSpace(*f.src)
		}
	}
	for _, f := range []struct {
		src *[]Emotion
		dst *[]Emotion
	}{{p.Emotions, &t.Emotions}, {p.ReratedEmotions, &t.ReratedEmotions}} {
		if f.src == nil {
			continue
		}
		out := []Emotion{}
		for _, e := range *f.src {
			e.Name = strings.TrimSpace(e.Name)
			if e.Name == "" {
				continue
			}
			if e.Intensity < 0 || e.Intensity > 100 {
				return invalid("emotion intensity must be between 0 and 100")
			}
			out = append(out, e)
		}
		*f.dst = out
	}
	if p.Distortions != nil {
		out := []string{}
		for _, d := range *p.Distortions {
			if d = strings.TrimSpace(d); d != "" {
				out = append(out, d)
			}
		}
		t.Distortions = out
	}
	if t.Situation == "" && t.AutomaticThought == "" {
		return invalid("a situation or automatic thought is required")
	}
	return nil
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// Week is a sprint, keyed by the date of its Monday.
type Week struct {
	ID        int64  `json:"id"`
	StartDate string `json:"start_date"`
	Intention string `json:"intention"`
}

const weekCols = `id, start_date, intention`

func scanWeek(r rowScanner) (Week, error) {
	var w Week
	err := r.Scan(&w.ID, &w.StartDate, &w.Intention)
	return w, err
}

// EnsureWeek returns the week starting on start (a Monday), creating it if
// needed.
func (s *Store) EnsureWeek(start string) (Week, error) {
	if _, err := s.db.Exec(`INSERT INTO weeks (start_date) VALUES (?) ON CONFLICT(start_date) DO NOTHING`, start); err != nil {
		return Week{}, err
	}
	return scanWeek(s.db.QueryRow(`SELECT `+weekCols+` FROM weeks WHERE start_date = ?`, start))
}

// GetWeek returns one week by id.
func (s *Store) GetWeek(id int64) (Week, error) {
	w, err := scanWeek(s.db.QueryRow(`SELECT `+weekCols+` FROM weeks WHERE id = ?`, id))
	return w, notFound(err)
}

// ListWeeks returns all weeks, newest first.
func (s *Store) ListWeeks() ([]Week, error) {
	return queryAll(s, scanWeek, `SELECT `+weekCols+` FROM weeks ORDER BY start_date DESC`)
}

// SetWeekIntention sets the sprint goal for a week.
func (s *Store) SetWeekIntention(id int64, intention string) (Week, error) {
	if err := rowsAffected(s.db.Exec(`UPDATE weeks SET intention = ? WHERE id = ?`, strings.TrimSpace(intention), id)); err != nil {
		return Week{}, err
	}
	return s.GetWeek(id)
}

// PutWeek writes w verbatim. Used for undo.
func (s *Store) PutWeek(w Week) error {
	_, err := s.db.Exec(`INSERT INTO weeks (`+weekCols+`) VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET intention = excluded.intention`, w.ID, w.StartDate, w.Intention)
	return err
}

// Retro is a weekly retrospective.
type Retro struct {
	ID        int64  `json:"id"`
	WeekID    int64  `json:"week_id"`
	WentWell  string `json:"went_well"`
	WasHard   string `json:"was_hard"`
	TryNext   string `json:"try_next"`
	AIDraft   string `json:"ai_draft"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// RetroPatch holds optional fields for updating a Retro.
type RetroPatch struct {
	WentWell *string `json:"went_well"`
	WasHard  *string `json:"was_hard"`
	TryNext  *string `json:"try_next"`
	AIDraft  *string `json:"ai_draft"`
}

const retroCols = `id, week_id, went_well, was_hard, try_next, ai_draft, created_at, updated_at`

func scanRetro(r rowScanner) (Retro, error) {
	var x Retro
	err := r.Scan(&x.ID, &x.WeekID, &x.WentWell, &x.WasHard, &x.TryNext, &x.AIDraft, &x.CreatedAt, &x.UpdatedAt)
	return x, err
}

// GetRetro returns the retro for a week, or ErrNotFound.
func (s *Store) GetRetro(weekID int64) (Retro, error) {
	x, err := scanRetro(s.db.QueryRow(`SELECT `+retroCols+` FROM retros WHERE week_id = ?`, weekID))
	return x, notFound(err)
}

// GetRetroByID returns a retro by its own id.
func (s *Store) GetRetroByID(id int64) (Retro, error) {
	x, err := scanRetro(s.db.QueryRow(`SELECT `+retroCols+` FROM retros WHERE id = ?`, id))
	return x, notFound(err)
}

// UpsertRetro creates or updates the retro for a week.
func (s *Store) UpsertRetro(weekID int64, p RetroPatch) (Retro, error) {
	if _, err := s.GetWeek(weekID); err != nil {
		return Retro{}, err
	}
	if _, err := s.db.Exec(`INSERT INTO retros (week_id) VALUES (?) ON CONFLICT(week_id) DO NOTHING`, weekID); err != nil {
		return Retro{}, err
	}
	x, err := s.GetRetro(weekID)
	if err != nil {
		return Retro{}, err
	}
	for _, f := range []struct {
		src *string
		dst *string
	}{{p.WentWell, &x.WentWell}, {p.WasHard, &x.WasHard}, {p.TryNext, &x.TryNext}, {p.AIDraft, &x.AIDraft}} {
		if f.src != nil {
			*f.dst = strings.TrimSpace(*f.src)
		}
	}
	x.UpdatedAt = nowUTC()
	return x, s.PutRetro(x)
}

// PutRetro writes x verbatim. Used for undo.
func (s *Store) PutRetro(x Retro) error {
	_, err := s.db.Exec(`INSERT INTO retros (`+retroCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET went_well = excluded.went_well, was_hard = excluded.was_hard,
		try_next = excluded.try_next, ai_draft = excluded.ai_draft, updated_at = excluded.updated_at`,
		x.ID, x.WeekID, x.WentWell, x.WasHard, x.TryNext, x.AIDraft, x.CreatedAt, x.UpdatedAt)
	return err
}

// DeleteRetro removes a retro by id.
func (s *Store) DeleteRetro(id int64) error {
	return rowsAffected(s.db.Exec(`DELETE FROM retros WHERE id = ?`, id))
}
