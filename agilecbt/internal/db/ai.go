package db

import (
	"strings"
)

// Note is a fact the AI curator remembers about the user.
type Note struct {
	ID        int64  `json:"id"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
}

func scanNote(r rowScanner) (Note, error) {
	var n Note
	err := r.Scan(&n.ID, &n.Text, &n.CreatedAt)
	return n, err
}

// ListNotes returns all curator notes, oldest first.
func (s *Store) ListNotes() ([]Note, error) {
	return queryAll(s, scanNote, `SELECT id, text, created_at FROM curator_notes ORDER BY id`)
}

// GetNote returns one note by id.
func (s *Store) GetNote(id int64) (Note, error) {
	n, err := scanNote(s.db.QueryRow(`SELECT id, text, created_at FROM curator_notes WHERE id = ?`, id))
	return n, notFound(err)
}

// CreateNote inserts a note.
func (s *Store) CreateNote(text string) (Note, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Note{}, invalid("text is required")
	}
	return scanNote(s.db.QueryRow(`INSERT INTO curator_notes (text) VALUES (?) RETURNING id, text, created_at`, text))
}

// UpdateNote replaces a note's text.
func (s *Store) UpdateNote(id int64, text string) (Note, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Note{}, invalid("text is required")
	}
	if err := rowsAffected(s.db.Exec(`UPDATE curator_notes SET text = ? WHERE id = ?`, text, id)); err != nil {
		return Note{}, err
	}
	return s.GetNote(id)
}

// PutNote writes n verbatim. Used for undo.
func (s *Store) PutNote(n Note) error {
	_, err := s.db.Exec(`INSERT INTO curator_notes (id, text, created_at) VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET text = excluded.text`, n.ID, n.Text, n.CreatedAt)
	return err
}

// DeleteNote removes a note.
func (s *Store) DeleteNote(id int64) error {
	return rowsAffected(s.db.Exec(`DELETE FROM curator_notes WHERE id = ?`, id))
}

// Action is one logged AI write. BeforeJSON is nil when the action created
// the entity; AfterJSON is nil when it deleted it.
type Action struct {
	ID         int64   `json:"id"`
	CheckinID  *int64  `json:"checkin_id"`
	Source     string  `json:"source"`
	Tool       string  `json:"tool"`
	Summary    string  `json:"summary"`
	Entity     string  `json:"entity"`
	EntityID   int64   `json:"entity_id"`
	BeforeJSON *string `json:"-"`
	AfterJSON  *string `json:"-"`
	CreatedAt  string  `json:"created_at"`
	UndoneAt   *string `json:"undone_at"`
}

const actionCols = `id, checkin_id, source, tool, summary, entity, entity_id, before_json, after_json, created_at, undone_at`

func scanAction(r rowScanner) (Action, error) {
	var a Action
	err := r.Scan(&a.ID, &a.CheckinID, &a.Source, &a.Tool, &a.Summary, &a.Entity, &a.EntityID,
		&a.BeforeJSON, &a.AfterJSON, &a.CreatedAt, &a.UndoneAt)
	return a, err
}

// CreateAction logs an AI write.
func (s *Store) CreateAction(a Action) (Action, error) {
	return scanAction(s.db.QueryRow(`INSERT INTO ai_actions
		(checkin_id, source, tool, summary, entity, entity_id, before_json, after_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING `+actionCols,
		a.CheckinID, a.Source, a.Tool, a.Summary, a.Entity, a.EntityID, a.BeforeJSON, a.AfterJSON))
}

// GetAction returns one action by id.
func (s *Store) GetAction(id int64) (Action, error) {
	a, err := scanAction(s.db.QueryRow(`SELECT `+actionCols+` FROM ai_actions WHERE id = ?`, id))
	return a, notFound(err)
}

// ListActions returns actions for a check-in (nil = the most recent limit
// actions across all sources), newest first.
func (s *Store) ListActions(checkinID *int64, limit int) ([]Action, error) {
	if limit <= 0 {
		limit = 50
	}
	if checkinID != nil {
		return queryAll(s, scanAction, `SELECT `+actionCols+` FROM ai_actions WHERE checkin_id = ? ORDER BY id DESC LIMIT ?`, *checkinID, limit)
	}
	return queryAll(s, scanAction, `SELECT `+actionCols+` FROM ai_actions ORDER BY id DESC LIMIT ?`, limit)
}

// MarkActionUndone stamps undone_at.
func (s *Store) MarkActionUndone(id int64) error {
	return rowsAffected(s.db.Exec(`UPDATE ai_actions SET undone_at = ? WHERE id = ? AND undone_at IS NULL`, nowUTC(), id))
}

// SettingCrisisResources is where older versions kept the crisis lines
// edited in Settings. They're config now; see app.CrisisResources.
const SettingCrisisResources = "crisis_resources"

// Settings for the coach's model, set in Settings → Coach. They override
// config.toml and the environment; a missing row means "use the config".
const (
	SettingLLM                = "llm"
	SettingLLMBaseURL         = "llm_base_url"
	SettingLLMModel           = "llm_model"
	SettingLLMReasoningEffort = "llm_reasoning_effort"
	// SettingLLMAPIKey is only sent to SettingLLMAPIKeyURL, the base URL it
	// was saved for. Neither leaves the server: they're kept out of the API
	// and exports.
	SettingLLMAPIKey    = "llm_api_key"
	SettingLLMAPIKeyURL = "llm_api_key_url"
)

// IsSecretSetting reports whether a setting must never leave the server.
func IsSecretSetting(key string) bool {
	return key == SettingLLMAPIKey || key == SettingLLMAPIKeyURL
}

// GetSettings returns all settings as a map.
func (s *Store) GetSettings() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// SetSetting upserts one setting.
func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// DeleteSetting removes one setting, restoring its default.
func (s *Store) DeleteSetting(key string) error {
	_, err := s.db.Exec(`DELETE FROM settings WHERE key = ?`, key)
	return err
}
