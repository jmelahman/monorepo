package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/jmelahman/agilecbt/internal/db"
)

// SettingCheckinTimes is "morning", "evening", or "both".
const SettingCheckinTimes = "checkin_times"

// DefaultCrisisResources is what the coach shares unless crisis_resources
// is set in config.toml.
const DefaultCrisisResources = `If you might act on thoughts of harming yourself, please reach out now:

- US: call or text 988 (Suicide & Crisis Lifeline), or text HOME to 741741.
- UK & Ireland: call Samaritans on 116 123.
- Elsewhere: find a local line at https://findahelpline.com.
- If you are in immediate danger, call your local emergency number.

You deserve support from a real person right now.`

// settingDefaults lists the settings the app (and PATCH /api/settings)
// accepts, with their defaults.
var settingDefaults = map[string]string{
	SettingCheckinTimes: "both",
}

// CrisisResources returns the crisis lines the coach shares: crisis_resources
// from the config if set, else a list saved by an older version's Settings
// page (so an edited list isn't silently lost), else the default. It's
// configuration rather than a setting so it can't be changed by accident.
func (a *App) CrisisResources() string {
	if a.ConfigCrisisResources != "" {
		return a.ConfigCrisisResources
	}
	if v := a.LegacyCrisisResources(); v != "" {
		return v
	}
	return DefaultCrisisResources
}

// LegacyCrisisResources returns the crisis list older versions stored in the
// database from their Settings page, or "".
func (a *App) LegacyCrisisResources() string {
	stored, err := a.Store.GetSettings()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(stored[db.SettingCrisisResources])
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
		if d.Settings, err = s.GetSettings(); err != nil {
			return err
		}
		for k := range d.Settings {
			if db.IsSecretSetting(k) {
				delete(d.Settings, k)
			}
		}
		return nil
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
			if db.IsSecretSetting(k) {
				continue
			}
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
