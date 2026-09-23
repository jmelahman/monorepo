package db

import (
	"errors"
	"fmt"
	"strings"
)

// ErrConflict is returned when an insert violates a uniqueness constraint.
var ErrConflict = errors.New("already exists")

// Repo mirror-clone statuses. Registration returns while the clone runs in
// the background; a repo is deployable only once RepoReady.
const (
	RepoCloning = "cloning"
	RepoReady   = "ready"
	RepoFailed  = "failed"
)

// Repo is a row in the repos table.
type Repo struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Source   string `json:"source"`
	BarePath string `json:"-"`
	// Watch marks the repo for polling: new branch tips deploy
	// automatically. WatchBranches narrows which branches (comma-separated
	// globs, empty = all).
	Watch         bool   `json:"watch"`
	WatchBranches string `json:"watch_branches"`
	// WatchBaselined reports whether the tips that predate watching have
	// been recorded; until they are, the watcher deploys nothing.
	WatchBaselined bool `json:"-"`
	// Status is the mirror clone's outcome; Error carries the failure
	// message while Status is RepoFailed.
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	CreatedAt string `json:"created_at"`
}

const repoCols = `id, name, source, bare_path, watch, watch_branches, watch_baselined, status, error, created_at`

func (r *Repo) scanFields() []any {
	return []any{&r.ID, &r.Name, &r.Source, &r.BarePath, &r.Watch, &r.WatchBranches, &r.WatchBaselined, &r.Status, &r.Error, &r.CreatedAt}
}

// CreateRepo inserts a repo with the given clone status and returns it.
// Returns ErrConflict if the name is taken.
func (s *Store) CreateRepo(name, source, barePath, status string) (Repo, error) {
	var r Repo
	err := s.db.QueryRow(
		`INSERT INTO repos (name, source, bare_path, status) VALUES (?, ?, ?, ?)
		 RETURNING `+repoCols,
		name, source, barePath, status,
	).Scan(r.scanFields()...)
	if isUniqueErr(err) {
		return Repo{}, ErrConflict
	}
	if err != nil {
		return Repo{}, err
	}
	return r, nil
}

// SetRepoStatus records the mirror clone's outcome (errMsg only meaningful
// for RepoFailed), or ErrNotFound if the repo was deleted meanwhile.
func (s *Store) SetRepoStatus(id int64, status, errMsg string) error {
	res, err := s.db.Exec(`UPDATE repos SET status = ?, error = ? WHERE id = ?`, status, errMsg, id)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetRepoByName returns the repo named name, or ErrNotFound.
func (s *Store) GetRepoByName(name string) (Repo, error) {
	var r Repo
	err := s.db.QueryRow(
		`SELECT `+repoCols+` FROM repos WHERE name = ?`, name,
	).Scan(r.scanFields()...)
	if err != nil {
		return Repo{}, mapNoRows(err)
	}
	return r, nil
}

// ListRepos returns all repos, oldest first.
func (s *Store) ListRepos() ([]Repo, error) {
	rows, err := s.db.Query(`SELECT ` + repoCols + ` FROM repos ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	repos := []Repo{}
	for rows.Next() {
		var r Repo
		if err := rows.Scan(r.scanFields()...); err != nil {
			return nil, err
		}
		repos = append(repos, r)
	}
	return repos, rows.Err()
}

// SetRepoWatch updates a repo's watch settings and returns the updated row,
// or ErrNotFound.
//
// Switching watching on arms the baseline: the next poll records the tips
// that already exist and deploys none of them, so watching picks up changes
// from here rather than the whole branch list. backfill skips that, which is
// what "deploy everything currently out there" asks for.
func (s *Store) SetRepoWatch(id int64, watch bool, branches string, backfill bool) (Repo, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Repo{}, err
	}
	defer tx.Rollback()

	var was bool
	if err := tx.QueryRow(`SELECT watch FROM repos WHERE id = ?`, id).Scan(&was); err != nil {
		return Repo{}, mapNoRows(err)
	}
	// Only the off→on edge re-arms: editing the branch globs of an already
	// watched repo must not re-hide tips it hasn't got to yet.
	arm := watch && !was
	if arm || !watch {
		if _, err := tx.Exec(`DELETE FROM watch_baselines WHERE repo_id = ?`, id); err != nil {
			return Repo{}, err
		}
	}
	baselined := true
	if arm && !backfill {
		baselined = false
	}
	var r Repo
	err = tx.QueryRow(
		`UPDATE repos
		    SET watch = ?, watch_branches = ?,
		        watch_baselined = CASE WHEN ? THEN ? ELSE watch_baselined END
		  WHERE id = ?
		 RETURNING `+repoCols,
		watch, branches, arm || !watch, baselined, id,
	).Scan(r.scanFields()...)
	if err != nil {
		return Repo{}, mapNoRows(err)
	}
	return r, tx.Commit()
}

// SetWatchBaseline records the tips a repo had before watching started and
// marks it baselined. Replaces any existing baseline.
func (s *Store) SetWatchBaseline(repoID int64, shas []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM watch_baselines WHERE repo_id = ?`, repoID); err != nil {
		return err
	}
	for _, sha := range shas {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO watch_baselines (repo_id, sha) VALUES (?, ?)`,
			repoID, sha,
		); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`UPDATE repos SET watch_baselined = 1 WHERE id = ?`, repoID); err != nil {
		return err
	}
	return tx.Commit()
}

// PruneWatchBaseline drops baseline entries that are no longer a branch tip.
// A sha only earns its place by having been a tip when watching started, so
// once the branch moves off it the entry has done its job — and dropping it
// means a later push back to that commit deploys like any other change. The
// baseline empties itself as the repo moves on.
func (s *Store) PruneWatchBaseline(repoID int64, tips []string) error {
	if len(tips) == 0 {
		_, err := s.db.Exec(`DELETE FROM watch_baselines WHERE repo_id = ?`, repoID)
		return err
	}
	args := make([]any, 0, len(tips)+1)
	args = append(args, repoID)
	for _, sha := range tips {
		args = append(args, sha)
	}
	q := fmt.Sprintf(
		`DELETE FROM watch_baselines WHERE repo_id = ? AND sha NOT IN (?%s)`,
		strings.Repeat(", ?", len(tips)-1))
	_, err := s.db.Exec(q, args...)
	return err
}

// WatchBaseline returns the recorded pre-watch tips, as a set.
func (s *Store) WatchBaseline(repoID int64) (map[string]bool, error) {
	rows, err := s.db.Query(`SELECT sha FROM watch_baselines WHERE repo_id = ?`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var sha string
		if err := rows.Scan(&sha); err != nil {
			return nil, err
		}
		out[sha] = true
	}
	return out, rows.Err()
}

// DeleteRepo removes a repo and every row that references it (deploys,
// branch aliases, artifacts, process bookkeeping) in one transaction.
func (s *Store) DeleteRepo(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{
		`DELETE FROM process_events WHERE repo_id = ?`,
		`DELETE FROM process_records WHERE repo_id = ?`,
		`DELETE FROM backend_artifacts WHERE repo_id = ?`,
		`DELETE FROM frontend_artifacts WHERE repo_id = ?`,
		`DELETE FROM watch_baselines WHERE repo_id = ?`,
		`DELETE FROM branch_aliases WHERE repo_id = ?`,
		`DELETE FROM deploys WHERE repo_id = ?`,
		`DELETE FROM repos WHERE id = ?`,
	} {
		if _, err := tx.Exec(q, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// isUniqueErr reports whether err is a SQLite uniqueness violation.
func isUniqueErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
