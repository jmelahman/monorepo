package db

// BackendArtifact records a provisioned backend: its state dir and the run
// contract (backend section of preview.toml as JSON). Row existence means
// the state dir is fully provisioned.
type BackendArtifact struct {
	RepoID     int64
	BeHash     string
	ForkedFrom string
	StateDir   string
	RunConfig  string
	// InitDoneAt is empty until the manifest's init steps have succeeded
	// against this artifact's state dir (or forever, if none are declared).
	// It can go back to empty: a start that skipped init and then failed
	// revokes it (ClearBackendInitDone), so init re-runs and repairs effects
	// that vanished from an external service.
	InitDoneAt string
	CreatedAt  string
}

// CreateBackendArtifact records a provisioned backend artifact. Idempotent:
// an existing row is left untouched.
func (s *Store) CreateBackendArtifact(a BackendArtifact) error {
	_, err := s.db.Exec(
		`INSERT INTO backend_artifacts (repo_id, be_hash, forked_from, state_dir, run_config)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (repo_id, be_hash) DO NOTHING`,
		a.RepoID, a.BeHash, a.ForkedFrom, a.StateDir, a.RunConfig)
	return err
}

// UpdateBackendArtifactRunConfig refreshes the stored run contract for an
// already-provisioned artifact (run-time-only fields like `networks` don't
// feed the hash, so they land via update rather than a new row).
func (s *Store) UpdateBackendArtifactRunConfig(repoID int64, beHash, runConfig string) error {
	_, err := s.db.Exec(
		`UPDATE backend_artifacts SET run_config = ? WHERE repo_id = ? AND be_hash = ?`,
		runConfig, repoID, beHash)
	return err
}

// GetBackendArtifact returns the artifact row, or ErrNotFound.
func (s *Store) GetBackendArtifact(repoID int64, beHash string) (BackendArtifact, error) {
	var a BackendArtifact
	err := s.db.QueryRow(
		`SELECT repo_id, be_hash, forked_from, state_dir, run_config, init_done_at, created_at
		 FROM backend_artifacts WHERE repo_id = ? AND be_hash = ?`, repoID, beHash,
	).Scan(&a.RepoID, &a.BeHash, &a.ForkedFrom, &a.StateDir, &a.RunConfig, &a.InitDoneAt, &a.CreatedAt)
	if err != nil {
		return BackendArtifact{}, mapNoRows(err)
	}
	return a, nil
}

// MarkBackendInitDone records that the artifact's init steps completed
// successfully; the supervisor skips init on every later start.
func (s *Store) MarkBackendInitDone(repoID int64, beHash string) error {
	_, err := s.db.Exec(
		`UPDATE backend_artifacts
		 SET init_done_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
		 WHERE repo_id = ? AND be_hash = ?`, repoID, beHash)
	return err
}

// ClearBackendInitDone withdraws a recorded init: the next start runs the
// manifest's init steps again. Unset is the empty string, exactly as a
// never-inited row reads. The supervisor calls this when a start that
// skipped init failed — init's effects can live in a service outside this
// system (a per-preview database on shared Postgres), where they can be
// deleted without the flag ever hearing about it.
func (s *Store) ClearBackendInitDone(repoID int64, beHash string) error {
	_, err := s.db.Exec(
		`UPDATE backend_artifacts SET init_done_at = '' WHERE repo_id = ? AND be_hash = ?`,
		repoID, beHash)
	return err
}

// DeleteBackendArtifact removes a backend artifact's provisioning row and its
// process bookkeeping (records + observability events) for one be_hash. Call
// it when the last deploy referencing the hash is removed — never while
// another deploy still shares it, since the artifact and its state dir are
// content-addressed and shared.
func (s *Store) DeleteBackendArtifact(repoID int64, beHash string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{
		`DELETE FROM process_events WHERE repo_id = ? AND be_hash = ?`,
		`DELETE FROM process_records WHERE repo_id = ? AND be_hash = ?`,
		`DELETE FROM backend_artifacts WHERE repo_id = ? AND be_hash = ?`,
	} {
		if _, err := tx.Exec(q, repoID, beHash); err != nil {
			return err
		}
	}
	return tx.Commit()
}
