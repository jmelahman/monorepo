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

// CreateReleaseRun adds a queued run.
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

// ListLatestReleaseRuns returns each subtree's latest run, however long ago.
func (s *Store) ListLatestReleaseRuns(ctx context.Context) ([]ReleaseRun, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+releaseRunColumns+` FROM release_runs WHERE id IN (SELECT MAX(id) FROM release_runs GROUP BY repo_path, prefix)`)
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

// StartReleaseRun moves a queued run to running, as of now.
func (s *Store) StartReleaseRun(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE release_runs SET status=?, started_at=unixepoch() WHERE id=? AND status=?`,
		ReleaseRunRunning, id, ReleaseRunQueued)
	return err
}

// FinishReleaseRun moves a queued or running run to status.
func (s *Store) FinishReleaseRun(ctx context.Context, id int64, status, errText string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE release_runs SET status=?, error=?, finished_at=unixepoch() WHERE id=? AND status IN (?, ?)`,
		status, errText, id, ReleaseRunQueued, ReleaseRunRunning)
	return err
}

// InterruptReleaseRuns marks every queued or running run interrupted. Called at
// startup, since nothing can still be running them.
func (s *Store) InterruptReleaseRuns(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE release_runs SET status=?, finished_at=unixepoch() WHERE status IN (?, ?)`,
		ReleaseRunInterrupted, ReleaseRunQueued, ReleaseRunRunning)
	return err
}

// SaveReleaseProject replaces a repo's last computed releases state, which is
// opaque JSON to the store.
func (s *Store) SaveReleaseProject(ctx context.Context, repo string, state []byte) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO release_projects (repo_path, state) VALUES (?, ?)
		 ON CONFLICT (repo_path) DO UPDATE SET state=excluded.state, updated_at=unixepoch()`,
		repo, string(state))
	return err
}

// ListReleaseProjects returns each repo's last saved releases state by path.
func (s *Store) ListReleaseProjects(ctx context.Context) (map[string][]byte, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT repo_path, state FROM release_projects`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]byte{}
	for rows.Next() {
		var repo, state string
		if err := rows.Scan(&repo, &state); err != nil {
			return nil, err
		}
		out[repo] = []byte(state)
	}
	return out, rows.Err()
}

// GetReleaseSplit returns the split saved for a subtree as of the commit that
// last changed it; false if the saved one is for another commit or none.
func (s *Store) GetReleaseSplit(ctx context.Context, repo, prefix, change string) (string, bool, error) {
	var split string
	err := s.db.QueryRowContext(ctx,
		`SELECT split FROM release_splits WHERE repo_path=? AND prefix=? AND change=?`,
		repo, prefix, change).Scan(&split)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return split, err == nil, err
}

// SetReleaseSplit saves a subtree's split, replacing any older one.
func (s *Store) SetReleaseSplit(ctx context.Context, repo, prefix, change, split string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO release_splits (repo_path, prefix, change, split) VALUES (?, ?, ?, ?)
		 ON CONFLICT (repo_path, prefix) DO UPDATE SET change=excluded.change, split=excluded.split`,
		repo, prefix, change, split)
	return err
}
