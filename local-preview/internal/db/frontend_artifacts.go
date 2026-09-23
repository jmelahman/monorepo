package db

// FrontendArtifact records a process-mode frontend: the run contract
// (frontend section of preview.toml as JSON). Row existence means the
// frontend artifact is a server process rather than a static bundle.
type FrontendArtifact struct {
	RepoID    int64
	FeHash    string
	RunConfig string
	CreatedAt string
}

// CreateFrontendArtifact records a process-mode frontend artifact.
// Idempotent, but the run contract is refreshed: run-time-only fields like
// `networks` don't feed the hash, so their changes land via upsert.
func (s *Store) CreateFrontendArtifact(a FrontendArtifact) error {
	_, err := s.db.Exec(
		`INSERT INTO frontend_artifacts (repo_id, fe_hash, run_config)
		 VALUES (?, ?, ?)
		 ON CONFLICT (repo_id, fe_hash) DO UPDATE SET run_config = excluded.run_config`,
		a.RepoID, a.FeHash, a.RunConfig)
	return err
}

// GetFrontendArtifact returns the artifact row, or ErrNotFound.
func (s *Store) GetFrontendArtifact(repoID int64, feHash string) (FrontendArtifact, error) {
	var a FrontendArtifact
	err := s.db.QueryRow(
		`SELECT repo_id, fe_hash, run_config, created_at
		 FROM frontend_artifacts WHERE repo_id = ? AND fe_hash = ?`, repoID, feHash,
	).Scan(&a.RepoID, &a.FeHash, &a.RunConfig, &a.CreatedAt)
	if err != nil {
		return FrontendArtifact{}, mapNoRows(err)
	}
	return a, nil
}

// DeleteFrontendArtifact removes a process-mode frontend's provisioning row
// for one fe_hash. Call it when the last deploy referencing the hash is
// removed; the artifact is shared, so never while another deploy uses it.
func (s *Store) DeleteFrontendArtifact(repoID int64, feHash string) error {
	_, err := s.db.Exec(
		`DELETE FROM frontend_artifacts WHERE repo_id = ? AND fe_hash = ?`, repoID, feHash)
	return err
}
