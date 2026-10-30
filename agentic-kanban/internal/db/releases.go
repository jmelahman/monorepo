package db

import (
	"context"
	"database/sql"
	"errors"
)

const releaseReminderColumns = `id, repo_path, prefix, bump, suffix, note, status, created_at, updated_at`

func scanReleaseReminder(sc scanner) (*ReleaseReminder, error) {
	var r ReleaseReminder
	err := sc.Scan(&r.ID, &r.RepoPath, &r.Prefix, &r.Bump, &r.Suffix, &r.Note, &r.Status, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &r, err
}

// CreateReleaseReminder adds an open reminder. A subtree that already has one
// fails with a unique violation; see IsUniqueViolation.
func (s *Store) CreateReleaseReminder(ctx context.Context, r *ReleaseReminder) error {
	row := s.db.QueryRowContext(ctx,
		`INSERT INTO release_reminders (repo_path, prefix, bump, suffix, note) VALUES (?, ?, ?, ?, ?) RETURNING `+releaseReminderColumns,
		r.RepoPath, r.Prefix, r.Bump, r.Suffix, r.Note)
	created, err := scanReleaseReminder(row)
	if err != nil {
		return err
	}
	*r = *created
	return nil
}

func (s *Store) GetReleaseReminder(ctx context.Context, id int64) (*ReleaseReminder, error) {
	return scanReleaseReminder(s.db.QueryRowContext(ctx,
		`SELECT `+releaseReminderColumns+` FROM release_reminders WHERE id=?`, id))
}

// ListOpenReleaseReminders returns the open reminders, oldest first.
func (s *Store) ListOpenReleaseReminders(ctx context.Context) ([]ReleaseReminder, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+releaseReminderColumns+` FROM release_reminders WHERE status=? ORDER BY created_at, id`, ReleaseReminderOpen)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReleaseReminder{}
	for rows.Next() {
		r, err := scanReleaseReminder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// UpdateReleaseReminder changes an open reminder's bump, suffix and note.
func (s *Store) UpdateReleaseReminder(ctx context.Context, r *ReleaseReminder) error {
	row := s.db.QueryRowContext(ctx,
		`UPDATE release_reminders SET bump=?, suffix=?, note=?, updated_at=unixepoch() WHERE id=? AND status=? RETURNING `+releaseReminderColumns,
		r.Bump, r.Suffix, r.Note, r.ID, ReleaseReminderOpen)
	updated, err := scanReleaseReminder(row)
	if err != nil {
		return err
	}
	*r = *updated
	return nil
}

// CloseReleaseReminder moves an open reminder to status, reporting whether it
// was still open.
func (s *Store) CloseReleaseReminder(ctx context.Context, id int64, status string) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE release_reminders SET status=?, updated_at=unixepoch() WHERE id=? AND status=?`,
		status, id, ReleaseReminderOpen)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

const releaseRunColumns = `id, reminder_id, repo_path, prefix, rev, version, bump, suffix, status, error, started_at, finished_at`

func scanReleaseRun(sc scanner) (*ReleaseRun, error) {
	var r ReleaseRun
	var reminder, finished sql.NullInt64
	err := sc.Scan(&r.ID, &reminder, &r.RepoPath, &r.Prefix, &r.Rev, &r.Version, &r.Bump, &r.Suffix, &r.Status, &r.Error, &r.StartedAt, &finished)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if reminder.Valid {
		r.ReminderID = &reminder.Int64
	}
	if finished.Valid {
		r.FinishedAt = &finished.Int64
	}
	return &r, nil
}

// CreateReleaseRun adds a running run.
func (s *Store) CreateReleaseRun(ctx context.Context, r *ReleaseRun) error {
	row := s.db.QueryRowContext(ctx,
		`INSERT INTO release_runs (reminder_id, repo_path, prefix, rev, version, bump, suffix) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING `+releaseRunColumns,
		r.ReminderID, r.RepoPath, r.Prefix, r.Rev, r.Version, r.Bump, r.Suffix)
	created, err := scanReleaseRun(row)
	if err != nil {
		return err
	}
	*r = *created
	return nil
}

func (s *Store) GetReleaseRun(ctx context.Context, id int64) (*ReleaseRun, error) {
	return scanReleaseRun(s.db.QueryRowContext(ctx,
		`SELECT `+releaseRunColumns+` FROM release_runs WHERE id=?`, id))
}

// ListReleaseRuns returns the latest runs, newest first.
func (s *Store) ListReleaseRuns(ctx context.Context, limit int) ([]ReleaseRun, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+releaseRunColumns+` FROM release_runs ORDER BY started_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReleaseRun{}
	for rows.Next() {
		r, err := scanReleaseRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// FinishReleaseRun moves a running run to status.
func (s *Store) FinishReleaseRun(ctx context.Context, id int64, status, errText string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE release_runs SET status=?, error=?, finished_at=unixepoch() WHERE id=? AND status=?`,
		status, errText, id, ReleaseRunRunning)
	return err
}

// InterruptReleaseRuns marks every running run interrupted. Called at startup,
// since nothing can still be running them.
func (s *Store) InterruptReleaseRuns(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE release_runs SET status=?, finished_at=unixepoch() WHERE status=?`,
		ReleaseRunInterrupted, ReleaseRunRunning)
	return err
}
