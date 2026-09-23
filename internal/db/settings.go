package db

// GetSetting returns the value stored under key, or ErrNotFound.
func (s *Store) GetSetting(key string) (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if err != nil {
		return "", mapNoRows(err)
	}
	return v, nil
}

// SetSetting stores value under key, replacing any previous value.
func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT (key) DO UPDATE SET value = excluded.value,
		     updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')`,
		key, value)
	return err
}

// SetBranchAlias points a repo's branch at a deploy, replacing any previous
// target.
func (s *Store) SetBranchAlias(repoID int64, branch string, deployID int64) error {
	_, err := s.db.Exec(
		`INSERT INTO branch_aliases (repo_id, branch, deploy_id) VALUES (?, ?, ?)
		 ON CONFLICT (repo_id, branch) DO UPDATE SET deploy_id = excluded.deploy_id,
		     updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')`,
		repoID, branch, deployID)
	return err
}

// BranchAliasDeployIDs returns the set of deploy IDs some branch alias
// routes to — deploys a retention sweep must never evict, since evicting
// one would break its branch URL.
func (s *Store) BranchAliasDeployIDs() (map[int64]bool, error) {
	rows, err := s.db.Query(`SELECT DISTINCT deploy_id FROM branch_aliases`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}
