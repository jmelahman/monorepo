package app

import (
	"fmt"
	"time"

	"github.com/jmelahman/agilecbt/internal/db"
)

// Setting keys and their defaults.
const (
	SettingCrisisResources = db.SettingCrisisResources
	// SettingCheckinTimes is "morning", "evening", or "both".
	SettingCheckinTimes = "checkin_times"
)

// DefaultCrisisResources is shown until the user edits it in Settings.
const DefaultCrisisResources = `If you might act on thoughts of harming yourself, please reach out now:

- US: call or text 988 (Suicide & Crisis Lifeline), or text HOME to 741741.
- UK & Ireland: call Samaritans on 116 123.
- Elsewhere: find a local line at https://findahelpline.com.
- If you are in immediate danger, call your local emergency number.

You deserve support from a real person right now.`

var settingDefaults = map[string]string{
	SettingCrisisResources: DefaultCrisisResources,
	SettingCheckinTimes:    "both",
}

// Settings returns every known setting, with defaults filled in.
func (a *App) Settings() (map[string]string, error) {
	stored, err := a.Store.GetSettings()
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for k, def := range settingDefaults {
		out[k] = def
		if v, ok := stored[k]; ok && v != "" {
			out[k] = v
		}
	}
	return out, nil
}

// Setting returns one setting (or its default).
func (a *App) Setting(key string) string {
	s, err := a.Settings()
	if err != nil {
		return settingDefaults[key]
	}
	return s[key]
}

// SetSetting validates and stores one setting. An empty value restores the
// default.
func (a *App) SetSetting(key, value string) error {
	if _, ok := settingDefaults[key]; !ok {
		return fmt.Errorf("%w: unknown setting %q", db.ErrInvalid, key)
	}
	if key == SettingCheckinTimes && value != "" {
		switch value {
		case "morning", "evening", "both":
		default:
			return fmt.Errorf("%w: checkin_times must be morning, evening, or both", db.ErrInvalid)
		}
	}
	return a.Store.SetSetting(key, value)
}

// DumpMessage is a chat message including its raw content blocks.
type DumpMessage struct {
	db.Message
	ContentJSON string `json:"content_json"`
}

// Dump is a full export of the user's data.
type Dump struct {
	Format         string             `json:"format"`
	ExportedAt     string             `json:"exported_at"`
	Values         []db.Value         `json:"values"`
	Goals          []db.Goal          `json:"goals"`
	Steps          []db.Step          `json:"steps"`
	Weeks          []db.Week          `json:"weeks"`
	Retros         []db.Retro         `json:"retros"`
	Checkins       []db.Checkin       `json:"checkins"`
	Messages       []DumpMessage      `json:"messages"`
	ThoughtRecords []db.ThoughtRecord `json:"thought_records"`
	Notes          []db.Note          `json:"notes"`
	Settings       map[string]string  `json:"settings"`
}

// DumpFormat identifies the export schema version.
const DumpFormat = "agilecbt-export-v1"

// Export snapshots all user data. The AI action log is left out: it only
// exists to support undo.
func (a *App) Export() (Dump, error) {
	var d Dump
	err := a.Store.Tx(func(s *db.Store) error {
		var err error
		if d.Values, err = s.ListValues(); err != nil {
			return err
		}
		if d.Goals, err = s.ListGoals(""); err != nil {
			return err
		}
		if d.Steps, err = s.ListSteps(db.StepFilter{}); err != nil {
			return err
		}
		if d.Weeks, err = s.ListWeeks(); err != nil {
			return err
		}
		d.Retros = []db.Retro{}
		for _, w := range d.Weeks {
			if r, err := s.GetRetro(w.ID); err == nil {
				d.Retros = append(d.Retros, r)
			} else if err != db.ErrNotFound {
				return err
			}
		}
		if d.Checkins, err = s.ListCheckins("", ""); err != nil {
			return err
		}
		msgs, err := s.ListAllMessages()
		if err != nil {
			return err
		}
		d.Messages = make([]DumpMessage, len(msgs))
		for i, m := range msgs {
			d.Messages[i] = DumpMessage{Message: m, ContentJSON: m.ContentJSON}
		}
		if d.ThoughtRecords, err = s.ListThoughtRecords(0); err != nil {
			return err
		}
		if d.Notes, err = s.ListNotes(); err != nil {
			return err
		}
		d.Settings, err = s.GetSettings()
		return err
	})
	d.Format = DumpFormat
	d.ExportedAt = a.Now().UTC().Format(time.RFC3339)
	return d, err
}

// Import loads a Dump into an empty database in one transaction, keeping ids
// so links between records survive.
func (a *App) Import(d Dump) error {
	if d.Format != DumpFormat {
		return fmt.Errorf("%w: unsupported export format %q", db.ErrInvalid, d.Format)
	}
	return a.Store.Tx(func(s *db.Store) error {
		empty, err := s.IsEmpty()
		if err != nil {
			return err
		}
		if !empty {
			return fmt.Errorf("%w: import requires an empty database (start with a fresh --data-dir)", db.ErrInvalid)
		}
		if err := putAll(d.Values, s.PutValue); err != nil {
			return err
		}
		if err := putAll(d.Goals, s.PutGoal); err != nil {
			return err
		}
		if err := putAll(d.Steps, s.PutStep); err != nil {
			return err
		}
		if err := putAll(d.Weeks, s.PutWeek); err != nil {
			return err
		}
		if err := putAll(d.Retros, s.PutRetro); err != nil {
			return err
		}
		if err := putAll(d.Checkins, s.PutCheckin); err != nil {
			return err
		}
		for _, m := range d.Messages {
			m.Message.ContentJSON = m.ContentJSON
			if err := s.PutMessage(m.Message); err != nil {
				return err
			}
		}
		if err := putAll(d.ThoughtRecords, s.PutThoughtRecord); err != nil {
			return err
		}
		if err := putAll(d.Notes, s.PutNote); err != nil {
			return err
		}
		for k, v := range d.Settings {
			if err := s.SetSetting(k, v); err != nil {
				return err
			}
		}
		return nil
	})
}

func putAll[T any](items []T, put func(T) error) error {
	for _, it := range items {
		if err := put(it); err != nil {
			return err
		}
	}
	return nil
}
