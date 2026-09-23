package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Deploy build-outcome statuses. Process runtime status is a separate,
// supervisor-owned concept merged into API views at read time.
const (
	DeployQueued   = "queued"
	DeployBuilding = "building"
	DeployReady    = "ready"
	DeployFailed   = "failed"
	DeployEvicted  = "evicted"
)

// Artifact build statuses. Downloadable artifacts build after the deploy
// itself turns ready — they never gate the preview — so a ready deploy may
// still carry building artifacts. An empty status on a stored ref means
// built (rows written before per-artifact statuses existed).
const (
	ArtifactBuilding = "building"
	ArtifactReady    = "ready"
	ArtifactFailed   = "failed"
)

// ArtifactRef locates one named downloadable artifact of a deploy: its
// content hash, build log path, and build outcome. Stored as JSON in the
// deploys.artifacts column, keyed by artifact name.
type ArtifactRef struct {
	Hash    string `json:"hash"`
	LogPath string `json:"log_path"`
	Status  string `json:"status,omitempty"`
	// Error is the build failure summary while Status is ArtifactFailed.
	Error string `json:"error,omitempty"`
}

// Deploy is a row in the deploys table.
type Deploy struct {
	ID             int64                  `json:"id"`
	RepoID         int64                  `json:"-"`
	SHA            string                 `json:"sha"`
	ShortSHA       string                 `json:"short_sha"`
	Ref            string                 `json:"ref,omitempty"`
	Branch         string                 `json:"branch,omitempty"`
	AuthorName     string                 `json:"author_name,omitempty"`
	AuthorEmail    string                 `json:"author_email,omitempty"`
	FeHash         string                 `json:"fe_hash,omitempty"`
	BeHash         string                 `json:"be_hash,omitempty"`
	Artifacts      map[string]ArtifactRef `json:"-"`
	Status         string                 `json:"status"`
	Error          string                 `json:"error,omitempty"`
	AttemptCount   int64                  `json:"attempt_count"`
	FeBuildLogPath string                 `json:"-"`
	BeBuildLogPath string                 `json:"-"`
	CreatedAt      string                 `json:"created_at"`
	UpdatedAt      string                 `json:"updated_at"`
	// CreatedBy is the identity that triggered the deploy: a GitHub login for
	// an interactive/SSO caller, the GitHub Actions actor for a CI/OIDC caller,
	// or empty for the automatic poller (which has no identity). Distinct from
	// AuthorName/AuthorEmail, which are the commit's git author metadata.
	CreatedBy string `json:"created_by,omitempty"`
}

// DeployRow is a deploy joined with its repo name.
type DeployRow struct {
	Deploy
	RepoName string `json:"repo"`
	// HasFeProcess reports whether a frontend_artifacts row exists for the
	// deploy's fe_hash — a process-mode frontend rather than a static
	// bundle. Resolved by the same query that loads the row so list views
	// don't pay a per-row lookup.
	HasFeProcess bool `json:"-"`
}

// deployCols is the scan-ordered column list; deployColsD is the same list
// qualified for joins (repos shares column names like created_at).
const deployCols = `id, repo_id, sha, short_sha, ref, branch, author_name, ` +
	`author_email, fe_hash, be_hash, artifacts, status, error, attempt_count, ` +
	`fe_build_log_path, be_build_log_path, created_at, updated_at, created_by`

var deployColsD = "d." + strings.ReplaceAll(deployCols, ", ", ", d.")

// hasFeProcessExpr computes DeployRow.HasFeProcess inline (index-backed via
// frontend_artifacts' primary key), so row loads need no follow-up query.
const hasFeProcessExpr = `EXISTS(SELECT 1 FROM frontend_artifacts fa
	WHERE fa.repo_id = d.repo_id AND fa.fe_hash = d.fe_hash)`

func scanDeploy(row interface{ Scan(...any) error }, extra ...any) (Deploy, error) {
	var d Deploy
	var artifacts string
	dest := []any{&d.ID, &d.RepoID, &d.SHA, &d.ShortSHA, &d.Ref, &d.Branch,
		&d.AuthorName, &d.AuthorEmail, &d.FeHash, &d.BeHash, &artifacts,
		&d.Status, &d.Error, &d.AttemptCount, &d.FeBuildLogPath, &d.BeBuildLogPath,
		&d.CreatedAt, &d.UpdatedAt, &d.CreatedBy}
	dest = append(dest, extra...)
	if err := row.Scan(dest...); err != nil {
		return Deploy{}, err
	}
	if artifacts != "" {
		if err := json.Unmarshal([]byte(artifacts), &d.Artifacts); err != nil {
			return Deploy{}, fmt.Errorf("deploy %d: parse artifacts: %w", d.ID, err)
		}
	}
	return d, nil
}

// DeployMeta is the commit metadata captured when a deploy is created. All
// fields are optional; Branch and the author fields are best-effort.
type DeployMeta struct {
	Ref         string
	Branch      string
	AuthorName  string
	AuthorEmail string
	// CreatedBy is the identity that triggered the deploy (SSO login, OIDC
	// actor, or empty for the poller). See Deploy.CreatedBy.
	CreatedBy string
}

// CreateDeploy inserts a queued deploy, computing the shortest sha prefix
// (>=7 chars) unique among the repo's deploys. The UNIQUE(repo_id,
// short_sha) constraint backstops races: on collision the prefix grows.
func (s *Store) CreateDeploy(repoID int64, sha string, meta DeployMeta) (Deploy, error) {
	for n := 7; n <= len(sha); n++ {
		d, err := s.insertDeploy(repoID, sha, sha[:n], meta)
		if err == nil {
			return d, nil
		}
		if !isUniqueErr(err) {
			return Deploy{}, err
		}
		// Same sha already deployed → caller should have checked; surface
		// as conflict. Short-sha collision with a different sha → grow.
		if _, gerr := s.GetDeployBySHA(repoID, sha); gerr == nil {
			return Deploy{}, ErrConflict
		}
	}
	return Deploy{}, fmt.Errorf("could not find a unique short sha for %s", sha)
}

func (s *Store) insertDeploy(repoID int64, sha, shortSHA string, meta DeployMeta) (Deploy, error) {
	return scanDeploy(s.db.QueryRow(
		`INSERT INTO deploys (repo_id, sha, short_sha, ref, branch, author_name, author_email, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 RETURNING `+deployCols,
		repoID, sha, shortSHA, meta.Ref, meta.Branch, meta.AuthorName, meta.AuthorEmail, meta.CreatedBy))
}

// GetDeployBySHA returns the deploy for (repo, sha), or ErrNotFound.
func (s *Store) GetDeployBySHA(repoID int64, sha string) (Deploy, error) {
	d, err := scanDeploy(s.db.QueryRow(
		`SELECT `+deployColsD+` FROM deploys d WHERE d.repo_id = ? AND d.sha = ?`, repoID, sha))
	if err != nil {
		return Deploy{}, mapNoRows(err)
	}
	return d, nil
}

// GetDeployByID returns a deploy joined with its repo name, or ErrNotFound.
func (s *Store) GetDeployByID(id int64) (DeployRow, error) {
	var name string
	var feProc bool
	d, err := scanDeploy(s.db.QueryRow(
		`SELECT `+deployColsD+`, r.name, `+hasFeProcessExpr+` FROM deploys d
		 JOIN repos r ON r.id = d.repo_id WHERE d.id = ?`, id), &name, &feProc)
	if err != nil {
		return DeployRow{}, mapNoRows(err)
	}
	return DeployRow{Deploy: d, RepoName: name, HasFeProcess: feProc}, nil
}

// DeployFilter narrows ListDeploys; zero-value fields don't filter.
type DeployFilter struct {
	// Repo and Branch match exactly.
	Repo   string
	Branch string
	// Author is a case-insensitive substring of the author name or email.
	Author string
	// Status matches the build status exactly (queued/building/ready/...).
	Status string
	// Crashed narrows on runtime state, which no column holds: the caller
	// passes the supervisor's currently-crashed process keys in
	// CrashedProcs, and Crashed says whether to keep only the deploys they
	// name (CrashedOnly) or drop them (CrashedNone). A crashed deploy is a
	// ready one whose process died, so it would otherwise answer to
	// status=ready.
	Crashed      CrashedMode
	CrashedProcs []ProcKey
	// Query is a free-text search: a case-insensitive prefix of the commit
	// sha, or a case-insensitive substring of the repo name, branch, ref,
	// author name, or author email.
	Query string
	// Limit caps how many rows are returned (newest first); 0 means all.
	Limit int
	// Offset skips that many matching rows before returning any. Paging is
	// by descending id, so a deploy created between two page fetches shifts
	// the window rather than corrupting it.
	Offset int
}

// CrashedMode selects how a filter treats deploys with a crashed side.
type CrashedMode int

// Crashed filter modes.
const (
	CrashedAny  CrashedMode = iota // runtime state is ignored
	CrashedOnly                    // only deploys with a crashed side
	CrashedNone                    // only deploys with no crashed side
)

// ProcKey names one supervised process by the deploy columns that identify
// it: a backend by its be_hash, a process-mode frontend by its fe_hash
// paired with the backend it was started against.
type ProcKey struct {
	RepoID int64
	FeHash string // empty for a backend process
	BeHash string
}

// IsDeployStatus reports whether s is one of the deploy build statuses.
func IsDeployStatus(s string) bool {
	switch s {
	case DeployQueued, DeployBuilding, DeployReady, DeployFailed, DeployEvicted:
		return true
	}
	return false
}

// deployWhere renders the filter's predicates. ListDeploys and CountDeploys
// share it so a paged listing and its total can never disagree about what
// the filter means.
func deployWhere(f DeployFilter) (string, []any) {
	where := []string{}
	args := []any{}
	if f.Repo != "" {
		where = append(where, `r.name = ?`)
		args = append(args, f.Repo)
	}
	if f.Branch != "" {
		where = append(where, `d.branch = ?`)
		args = append(args, f.Branch)
	}
	if f.Author != "" {
		// instr instead of LIKE so % and _ in the query aren't wildcards.
		where = append(where,
			`(instr(lower(d.author_name), lower(?)) > 0 OR instr(lower(d.author_email), lower(?)) > 0)`)
		args = append(args, f.Author, f.Author)
	}
	if f.Status != "" {
		where = append(where, `d.status = ?`)
		args = append(args, f.Status)
	}
	if f.Crashed != CrashedAny {
		procs := []string{}
		for _, k := range f.CrashedProcs {
			if k.FeHash == "" {
				procs = append(procs, `(d.repo_id = ? AND d.be_hash = ?)`)
				args = append(args, k.RepoID, k.BeHash)
				continue
			}
			procs = append(procs, `(d.repo_id = ? AND d.fe_hash = ? AND d.be_hash = ?)`)
			args = append(args, k.RepoID, k.FeHash, k.BeHash)
		}
		switch {
		case len(procs) == 0:
			// Nothing is crashed: "only" matches no row, "none" every row.
			if f.Crashed == CrashedOnly {
				where = append(where, `0`)
			}
		case f.Crashed == CrashedOnly:
			where = append(where, `(`+strings.Join(procs, ` OR `)+`)`)
		default:
			where = append(where, `NOT (`+strings.Join(procs, ` OR `)+`)`)
		}
	}
	if f.Query != "" {
		// instr = 1 is a wildcard-safe prefix match (shas are stored
		// lowercase); short_sha needs no clause of its own, being a prefix of
		// sha itself.
		where = append(where, `(instr(d.sha, lower(?)) = 1
			OR instr(lower(d.branch), lower(?)) > 0
			OR instr(lower(d.ref), lower(?)) > 0
			OR instr(lower(r.name), lower(?)) > 0
			OR instr(lower(d.author_name), lower(?)) > 0
			OR instr(lower(d.author_email), lower(?)) > 0)`)
		args = append(args, f.Query, f.Query, f.Query, f.Query, f.Query, f.Query)
	}
	if len(where) == 0 {
		return "", args
	}
	return ` WHERE ` + strings.Join(where, ` AND `), args
}

// CountDeploys returns how many deploys match the filter, ignoring Limit and
// Offset — the total a paged listing pages through.
func (s *Store) CountDeploys(f DeployFilter) (int, error) {
	clause, args := deployWhere(f)
	q := `SELECT COUNT(*) FROM deploys d JOIN repos r ON r.id = d.repo_id` + clause
	var n int
	if err := s.db.QueryRow(q, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// ListDeploys returns deploys newest first, narrowed by the filter.
func (s *Store) ListDeploys(f DeployFilter) ([]DeployRow, error) {
	clause, args := deployWhere(f)
	q := `SELECT ` + deployColsD + `, r.name, ` + hasFeProcessExpr +
		` FROM deploys d JOIN repos r ON r.id = d.repo_id` + clause +
		` ORDER BY d.id DESC`
	if f.Limit > 0 {
		q += ` LIMIT ?`
		args = append(args, f.Limit)
	}
	if f.Offset > 0 {
		// SQLite only accepts OFFSET after a LIMIT; -1 is its idiom for
		// "no limit", so an offset without one still skips correctly.
		if f.Limit <= 0 {
			q += ` LIMIT -1`
		}
		q += ` OFFSET ?`
		args = append(args, f.Offset)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DeployRow{}
	for rows.Next() {
		var name string
		var feProc bool
		d, err := scanDeploy(rows, &name, &feProc)
		if err != nil {
			return nil, err
		}
		out = append(out, DeployRow{Deploy: d, RepoName: name, HasFeProcess: feProc})
	}
	return out, rows.Err()
}

var hexRE = regexp.MustCompile(`^[0-9a-f]+$`)

// DeploysBySHAPrefix returns the repo's deploys whose sha starts with
// prefix. The prefix must be lowercase hex (routing labels are validated
// before they reach the database).
func (s *Store) DeploysBySHAPrefix(repoID int64, prefix string) ([]Deploy, error) {
	if !hexRE.MatchString(prefix) {
		return nil, fmt.Errorf("invalid sha prefix %q", prefix)
	}
	rows, err := s.db.Query(
		`SELECT `+deployColsD+` FROM deploys d
		 WHERE d.repo_id = ? AND d.sha LIKE ? || '%' ORDER BY d.id DESC`, repoID, prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Deploy{}
	for rows.Next() {
		d, err := scanDeploy(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// LatestDeployID returns the repo's newest deploy row id, 0 when it has
// none. The watcher uses it to detect deploy rows created between polls.
func (s *Store) LatestDeployID(repoID int64) (int64, error) {
	var id int64
	err := s.db.QueryRow(
		`SELECT COALESCE(MAX(id), 0) FROM deploys WHERE repo_id = ?`, repoID).Scan(&id)
	return id, err
}

// ListUnfinishedDeployIDs returns deploys with interrupted work to resume at
// startup: rows stuck in queued/building, plus ready rows whose artifact
// builds (which run after readiness) a shutdown cut short.
func (s *Store) ListUnfinishedDeployIDs() ([]int64, error) {
	rows, err := s.db.Query(
		`SELECT id, status, artifacts FROM deploys
		 WHERE status IN (?, ?) OR (status = ? AND artifacts != '') ORDER BY id`,
		DeployQueued, DeployBuilding, DeployReady)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		var status, artifacts string
		if err := rows.Scan(&id, &status, &artifacts); err != nil {
			return nil, err
		}
		if status == DeployReady && !anyArtifactBuilding(artifacts) {
			continue
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// anyArtifactBuilding reports whether the encoded artifact refs hold at
// least one artifact still marked building.
func anyArtifactBuilding(encoded string) bool {
	refs := map[string]ArtifactRef{}
	if json.Unmarshal([]byte(encoded), &refs) != nil {
		return false
	}
	for _, ref := range refs {
		if ref.Status == ArtifactBuilding {
			return true
		}
	}
	return false
}

// ResetDeploy re-queues a failed or evicted deploy for another attempt.
func (s *Store) ResetDeploy(id int64) error {
	return s.updateDeploy(id,
		`status = ?, error = '', attempt_count = attempt_count + 1`, DeployQueued)
}

// SetDeployBuilding marks the deploy as building.
func (s *Store) SetDeployBuilding(id int64) error {
	return s.updateDeploy(id, `status = ?`, DeployBuilding)
}

// SetDeployHashes records the computed partition hashes and log paths as
// soon as they're known — visible even if the build later fails.
func (s *Store) SetDeployHashes(id int64, feHash, beHash, feLog, beLog string) error {
	return s.updateDeploy(id,
		`fe_hash = ?, be_hash = ?, fe_build_log_path = ?, be_build_log_path = ?`,
		feHash, beHash, feLog, beLog)
}

// SetDeployArtifacts records the deploy's named downloadable artifacts —
// like the partition hashes, written as soon as they're known so they're
// visible even if the build later fails. An empty map clears the column.
func (s *Store) SetDeployArtifacts(id int64, refs map[string]ArtifactRef) error {
	encoded := ""
	if len(refs) > 0 {
		b, err := json.Marshal(refs)
		if err != nil {
			return err
		}
		encoded = string(b)
	}
	return s.updateDeploy(id, `artifacts = ?`, encoded)
}

// SetDeployArtifactStatus records one named artifact's build outcome on the
// deploy's stored refs. Only the build worker holding the deploy writes the
// artifacts column, so read-modify-write is race-free.
func (s *Store) SetDeployArtifactStatus(id int64, name, status, errMsg string) error {
	var encoded string
	if err := s.db.QueryRow(
		`SELECT artifacts FROM deploys WHERE id = ?`, id).Scan(&encoded); err != nil {
		return mapNoRows(err)
	}
	refs := map[string]ArtifactRef{}
	if encoded != "" {
		if err := json.Unmarshal([]byte(encoded), &refs); err != nil {
			return fmt.Errorf("deploy %d: parse artifacts: %w", id, err)
		}
	}
	ref, ok := refs[name]
	if !ok {
		return fmt.Errorf("deploy %d has no artifact %q: %w", id, name, ErrNotFound)
	}
	ref.Status = status
	ref.Error = errMsg
	refs[name] = ref
	return s.SetDeployArtifacts(id, refs)
}

// SetDeployReady marks the deploy ready to serve.
func (s *Store) SetDeployReady(id int64) error {
	return s.updateDeploy(id, `status = ?, error = ''`, DeployReady)
}

// SetDeployFailed marks the deploy failed with a short error summary (full
// detail lives in the build logs).
func (s *Store) SetDeployFailed(id int64, msg string) error {
	return s.updateDeploy(id, `status = ?, error = ?`, DeployFailed, msg)
}

// SetDeployEvicted marks a deploy whose artifacts were garbage-collected.
func (s *Store) SetDeployEvicted(id int64) error {
	return s.updateDeploy(id, `status = ?`, DeployEvicted)
}

// DeleteDeploy removes a single deploy row and any branch alias pointing at
// it. Artifact and process bookkeeping keyed by hash is shared across
// deploys, so it is reclaimed separately by the caller once a hash is fully
// orphaned (see supervise.Manager.GCDeploy).
func (s *Store) DeleteDeploy(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{
		`DELETE FROM branch_aliases WHERE deploy_id = ?`,
		`DELETE FROM deploys WHERE id = ?`,
	} {
		if _, err := tx.Exec(q, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) updateDeploy(id int64, set string, args ...any) error {
	args = append(args, id)
	res, err := s.db.Exec(
		`UPDATE deploys SET `+set+`, updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE id = ?`,
		args...)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// mapNoRows converts sql.ErrNoRows into the package's ErrNotFound.
func mapNoRows(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
