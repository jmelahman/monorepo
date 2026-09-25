package db

import (
	"strings"
)

// Check-in kinds.
const (
	KindMorning = "morning"
	KindEvening = "evening"
	KindAdhoc   = "adhoc"
)

// Checkin is one daily standup: how you're arriving, plus the curator chat.
type Checkin struct {
	ID         int64  `json:"id"`
	Date       string `json:"date"`
	Kind       string `json:"kind"`
	Mood       *int   `json:"mood"`
	Energy     *int   `json:"energy"`
	Anxiety    *int   `json:"anxiety"`
	Note       string `json:"note"`
	Summary    string `json:"summary"`
	LLMSession string `json:"-"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// CheckinPatch holds optional fields for creating or updating a Checkin.
type CheckinPatch struct {
	Date    *string `json:"date"`
	Kind    *string `json:"kind"`
	Mood    *int    `json:"mood"`
	Energy  *int    `json:"energy"`
	Anxiety *int    `json:"anxiety"`
	Note    *string `json:"note"`
	Summary *string `json:"summary"`
}

const checkinCols = `id, date, kind, mood, energy, anxiety, note, summary, llm_session, created_at, updated_at`

func scanCheckin(r rowScanner) (Checkin, error) {
	var c Checkin
	err := r.Scan(&c.ID, &c.Date, &c.Kind, &c.Mood, &c.Energy, &c.Anxiety, &c.Note, &c.Summary,
		&c.LLMSession, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

// ListCheckins returns check-ins with from <= date <= to (inclusive, either
// may be empty), oldest first.
func (s *Store) ListCheckins(from, to string) ([]Checkin, error) {
	if from == "" {
		from = "0000-00-00"
	}
	if to == "" {
		to = "9999-99-99"
	}
	return queryAll(s, scanCheckin,
		`SELECT `+checkinCols+` FROM checkins WHERE date >= ? AND date <= ? ORDER BY date, id`, from, to)
}

// GetCheckin returns one check-in by id.
func (s *Store) GetCheckin(id int64) (Checkin, error) {
	c, err := scanCheckin(s.db.QueryRow(`SELECT `+checkinCols+` FROM checkins WHERE id = ?`, id))
	return c, notFound(err)
}

// LatestCheckin returns the most recent check-in of kind on date ("" kind =
// any kind), or ErrNotFound.
func (s *Store) LatestCheckin(date, kind string) (Checkin, error) {
	q := `SELECT ` + checkinCols + ` FROM checkins WHERE date = ?`
	args := []any{date}
	if kind != "" {
		q += ` AND kind = ?`
		args = append(args, kind)
	}
	c, err := scanCheckin(s.db.QueryRow(q+` ORDER BY id DESC LIMIT 1`, args...))
	return c, notFound(err)
}

// CreateCheckin inserts a check-in. Date is required.
func (s *Store) CreateCheckin(p CheckinPatch) (Checkin, error) {
	c := Checkin{Kind: KindMorning}
	if err := applyCheckinPatch(&c, p); err != nil {
		return Checkin{}, err
	}
	return scanCheckin(s.db.QueryRow(`INSERT INTO checkins (date, kind, mood, energy, anxiety, note, summary)
		VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING `+checkinCols,
		c.Date, c.Kind, c.Mood, c.Energy, c.Anxiety, c.Note, c.Summary))
}

// UpdateCheckin applies p to the check-in with id.
func (s *Store) UpdateCheckin(id int64, p CheckinPatch) (Checkin, error) {
	c, err := s.GetCheckin(id)
	if err != nil {
		return Checkin{}, err
	}
	if err := applyCheckinPatch(&c, p); err != nil {
		return Checkin{}, err
	}
	c.UpdatedAt = nowUTC()
	return c, s.PutCheckin(c)
}

// SetCheckinSession records the LLM backend's session handle for resuming.
func (s *Store) SetCheckinSession(id int64, session string) error {
	return rowsAffected(s.db.Exec(`UPDATE checkins SET llm_session = ? WHERE id = ?`, session, id))
}

// PutCheckin writes c verbatim (insert or full update by id). Used for undo.
func (s *Store) PutCheckin(c Checkin) error {
	_, err := s.db.Exec(`INSERT INTO checkins (`+checkinCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET date = excluded.date, kind = excluded.kind, mood = excluded.mood,
		energy = excluded.energy, anxiety = excluded.anxiety, note = excluded.note,
		summary = excluded.summary, updated_at = excluded.updated_at`,
		c.ID, c.Date, c.Kind, c.Mood, c.Energy, c.Anxiety, c.Note, c.Summary, c.LLMSession, c.CreatedAt, c.UpdatedAt)
	return err
}

// DeleteCheckin removes a check-in and its conversation.
func (s *Store) DeleteCheckin(id int64) error {
	return rowsAffected(s.db.Exec(`DELETE FROM checkins WHERE id = ?`, id))
}

func applyCheckinPatch(c *Checkin, p CheckinPatch) error {
	if p.Date != nil {
		c.Date = strings.TrimSpace(*p.Date)
	}
	if p.Kind != nil {
		switch *p.Kind {
		case KindMorning, KindEvening, KindAdhoc:
			c.Kind = *p.Kind
		default:
			return invalid("kind must be morning, evening, or adhoc")
		}
	}
	for _, r := range []struct {
		name string
		src  *int
		dst  **int
	}{
		{"mood", p.Mood, &c.Mood},
		{"energy", p.Energy, &c.Energy},
		{"anxiety", p.Anxiety, &c.Anxiety},
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
	if p.Note != nil {
		c.Note = strings.TrimSpace(*p.Note)
	}
	if p.Summary != nil {
		c.Summary = strings.TrimSpace(*p.Summary)
	}
	if len(c.Date) != len("2006-01-02") {
		return invalid("date must be YYYY-MM-DD")
	}
	return nil
}

// Message is one turn of the curator conversation.
type Message struct {
	ID          int64  `json:"id"`
	CheckinID   int64  `json:"checkin_id"`
	Seq         int    `json:"seq"`
	Role        string `json:"role"`
	Text        string `json:"text"`
	ContentJSON string `json:"-"`
	CreatedAt   string `json:"created_at"`
}

const messageCols = `id, checkin_id, seq, role, text, content_json, created_at`

func scanMessage(r rowScanner) (Message, error) {
	var m Message
	err := r.Scan(&m.ID, &m.CheckinID, &m.Seq, &m.Role, &m.Text, &m.ContentJSON, &m.CreatedAt)
	return m, err
}

// AppendMessage adds a turn to a check-in's conversation.
func (s *Store) AppendMessage(checkinID int64, role, text, contentJSON string) (Message, error) {
	return scanMessage(s.db.QueryRow(`INSERT INTO checkin_messages (checkin_id, seq, role, text, content_json)
		VALUES (?, (SELECT COALESCE(MAX(seq), 0) + 1 FROM checkin_messages WHERE checkin_id = ?), ?, ?, ?)
		RETURNING `+messageCols, checkinID, checkinID, role, text, contentJSON))
}

// ListMessages returns a check-in's conversation in order.
func (s *Store) ListMessages(checkinID int64) ([]Message, error) {
	return queryAll(s, scanMessage,
		`SELECT `+messageCols+` FROM checkin_messages WHERE checkin_id = ? ORDER BY seq`, checkinID)
}

// ListAllMessages returns every chat message, for export.
func (s *Store) ListAllMessages() ([]Message, error) {
	return queryAll(s, scanMessage, `SELECT `+messageCols+` FROM checkin_messages ORDER BY checkin_id, seq`)
}

// PutMessage writes m verbatim, for import.
func (s *Store) PutMessage(m Message) error {
	_, err := s.db.Exec(`INSERT INTO checkin_messages (`+messageCols+`) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.CheckinID, m.Seq, m.Role, m.Text, m.ContentJSON, m.CreatedAt)
	return err
}
