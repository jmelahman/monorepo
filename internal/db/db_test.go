package db

import (
	"database/sql"
	"errors"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestOpenFile(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
}

// TestOpenMigratesOldSchema opens a database whose backend_artifacts table
// predates the init_done_at column and expects Open's migrations to add it.
func TestOpenMigratesOldSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	old, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := old.Exec(`CREATE TABLE backend_artifacts (
		repo_id INTEGER NOT NULL,
		be_hash TEXT NOT NULL,
		forked_from TEXT NOT NULL DEFAULT '',
		state_dir TEXT NOT NULL,
		run_config TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
		PRIMARY KEY (repo_id, be_hash)
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := old.Exec(
		`INSERT INTO backend_artifacts (repo_id, be_hash, state_dir, run_config) VALUES (1, 'be1', '/state', '{}')`,
	); err != nil {
		t.Fatal(err)
	}
	old.Close()

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a, err := s.GetBackendArtifact(1, "be1")
	if err != nil || a.InitDoneAt != "" {
		t.Fatalf("migrated artifact = %+v, %v", a, err)
	}
	if err := s.MarkBackendInitDone(1, "be1"); err != nil {
		t.Fatal(err)
	}
}

func TestRepoCRUD(t *testing.T) {
	s := newTestStore(t)

	r, err := s.CreateRepo("demo", "/src/demo", "/data/repos/demo.git", RepoReady)
	if err != nil {
		t.Fatal(err)
	}
	if r.ID == 0 || r.Name != "demo" || r.CreatedAt == "" {
		t.Fatalf("unexpected repo: %+v", r)
	}

	if _, err := s.CreateRepo("demo", "/elsewhere", "/x", RepoReady); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate name err = %v, want ErrConflict", err)
	}

	got, err := s.GetRepoByName("demo")
	if err != nil || got.ID != r.ID {
		t.Fatalf("GetRepoByName = %+v, %v", got, err)
	}
	if _, err := s.GetRepoByName("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing repo err = %v, want ErrNotFound", err)
	}

	repos, err := s.ListRepos()
	if err != nil || len(repos) != 1 {
		t.Fatalf("ListRepos = %+v, %v", repos, err)
	}

	if r.Watch || r.WatchBranches != "" {
		t.Fatalf("new repo should not be watched: %+v", r)
	}
	w, err := s.SetRepoWatch(r.ID, true, "main,release/*", false)
	if err != nil || !w.Watch || w.WatchBranches != "main,release/*" {
		t.Fatalf("SetRepoWatch = %+v, %v", w, err)
	}
	if got, _ := s.GetRepoByName("demo"); !got.Watch || got.WatchBranches != "main,release/*" {
		t.Fatalf("watch not persisted: %+v", got)
	}
	if w, err = s.SetRepoWatch(r.ID, false, "", false); err != nil || w.Watch || w.WatchBranches != "" {
		t.Fatalf("unwatch = %+v, %v", w, err)
	}
	if _, err := s.SetRepoWatch(999, true, "", false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SetRepoWatch missing repo err = %v, want ErrNotFound", err)
	}
}

func TestWatchBaseline(t *testing.T) {
	s := newTestStore(t)
	r, err := s.CreateRepo("demo", "/src", "/bare", RepoReady)
	if err != nil {
		t.Fatal(err)
	}

	// Switching watching on arms the baseline; backfill declines it.
	if w, err := s.SetRepoWatch(r.ID, true, "", false); err != nil || w.WatchBaselined {
		t.Fatalf("watch on = %+v, %v; want baseline armed", w, err)
	}
	if err := s.SetWatchBaseline(r.ID, []string{shaA, shaB}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetRepoByName("demo"); !got.WatchBaselined {
		t.Fatalf("baseline capture not persisted: %+v", got)
	}

	// Editing the branch globs of an already-watched repo must not re-arm:
	// tips it hasn't deployed yet would silently become unreachable.
	if w, err := s.SetRepoWatch(r.ID, true, "main", false); err != nil || !w.WatchBaselined {
		t.Fatalf("branch edit re-armed the baseline: %+v, %v", w, err)
	}
	if base, _ := s.WatchBaseline(r.ID); len(base) != 2 {
		t.Fatalf("branch edit dropped the baseline: %v", base)
	}

	// A sha that is no longer a tip has done its job.
	if err := s.PruneWatchBaseline(r.ID, []string{shaA}); err != nil {
		t.Fatal(err)
	}
	if base, _ := s.WatchBaseline(r.ID); len(base) != 1 || !base[shaA] {
		t.Fatalf("prune = %v, want %s alone", base, shaA)
	}

	// Unwatching drops it; watching again with backfill leaves nothing to
	// hold deploys back.
	if _, err := s.SetRepoWatch(r.ID, false, "", false); err != nil {
		t.Fatal(err)
	}
	if base, _ := s.WatchBaseline(r.ID); len(base) != 0 {
		t.Fatalf("unwatch kept a baseline: %v", base)
	}
	if w, err := s.SetRepoWatch(r.ID, true, "", true); err != nil || !w.WatchBaselined {
		t.Fatalf("backfill = %+v, %v; want no baseline armed", w, err)
	}
}

const (
	shaA = "aaaaaaa1111111111111111111111111111111111"
	shaB = "bbbbbbb2222222222222222222222222222222222"
)

func TestDeployLifecycle(t *testing.T) {
	s := newTestStore(t)
	r, err := s.CreateRepo("demo", "/src", "/bare", RepoReady)
	if err != nil {
		t.Fatal(err)
	}

	d, err := s.CreateDeploy(r.ID, shaA, DeployMeta{
		Ref: "main", Branch: "main", AuthorName: "Ada", AuthorEmail: "ada@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != DeployQueued || d.ShortSHA != shaA[:7] || d.Ref != "main" {
		t.Fatalf("unexpected deploy: %+v", d)
	}
	if d.Branch != "main" || d.AuthorName != "Ada" || d.AuthorEmail != "ada@example.com" {
		t.Fatalf("commit metadata not stored: %+v", d)
	}

	if err := s.SetDeployBuilding(d.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.SetDeployHashes(d.ID, "fe1", "be1", "/logs/fe1.log", "/logs/be1.log"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetDeployReady(d.ID); err != nil {
		t.Fatal(err)
	}

	row, err := s.GetDeployByID(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != DeployReady || row.FeHash != "fe1" || row.RepoName != "demo" {
		t.Fatalf("unexpected row: %+v", row)
	}

	if err := s.SetDeployFailed(d.ID, "boom"); err != nil {
		t.Fatal(err)
	}
	if err := s.ResetDeploy(d.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetDeployBySHA(r.ID, shaA)
	if got.Status != DeployQueued || got.Error != "" || got.AttemptCount != 1 {
		t.Fatalf("after reset: %+v", got)
	}

	if err := s.SetDeployReady(9999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update of missing deploy = %v, want ErrNotFound", err)
	}
}

func TestShortSHAGrowsOnCollision(t *testing.T) {
	s := newTestStore(t)
	r, _ := s.CreateRepo("demo", "/src", "/bare", RepoReady)

	// Two shas sharing a 7-char prefix force the second to use 8 chars.
	shaX := "abcdef0" + strings.Repeat("1", 33)
	shaY := "abcdef0" + strings.Repeat("2", 33)
	d1, err := s.CreateDeploy(r.ID, shaX, DeployMeta{})
	if err != nil {
		t.Fatal(err)
	}
	d2, err := s.CreateDeploy(r.ID, shaY, DeployMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if d1.ShortSHA != "abcdef0" {
		t.Fatalf("d1.ShortSHA = %q", d1.ShortSHA)
	}
	if d2.ShortSHA != "abcdef02" {
		t.Fatalf("d2.ShortSHA = %q, want 8-char prefix", d2.ShortSHA)
	}

	// Same sha again is a conflict (redeploy resets the row instead).
	if _, err := s.CreateDeploy(r.ID, shaX, DeployMeta{}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate sha err = %v, want ErrConflict", err)
	}
}

func TestListDeploysFilter(t *testing.T) {
	s := newTestStore(t)
	r1, _ := s.CreateRepo("app", "/src/app", "/bare/app", RepoReady)
	r2, _ := s.CreateRepo("lib", "/src/lib", "/bare/lib", RepoReady)

	shaB := "bbbbbbb2222222222222222222222222222222222"
	shaC := "ccccccc3333333333333333333333333333333333"
	seed := []struct {
		repoID int64
		sha    string
		meta   DeployMeta
	}{
		{r1.ID, shaA, DeployMeta{Branch: "main", AuthorName: "Ada Lovelace", AuthorEmail: "ada@example.com"}},
		{r1.ID, shaB, DeployMeta{Ref: "v1.2.3", Branch: "feature", AuthorName: "Grace Hopper", AuthorEmail: "grace@example.com"}},
		{r2.ID, shaC, DeployMeta{Branch: "main", AuthorName: "Ada Lovelace", AuthorEmail: "ada@example.com"}},
	}
	ids := make([]int64, len(seed))
	for i, sd := range seed {
		d, err := s.CreateDeploy(sd.repoID, sd.sha, sd.meta)
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = d.ID
	}
	if err := s.SetDeployReady(ids[1]); err != nil {
		t.Fatal(err)
	}

	for name, tc := range map[string]struct {
		f    DeployFilter
		want int
	}{
		"all":                      {DeployFilter{}, 3},
		"repo":                     {DeployFilter{Repo: "app"}, 2},
		"branch":                   {DeployFilter{Branch: "main"}, 2},
		"repo+branch":              {DeployFilter{Repo: "app", Branch: "main"}, 1},
		"author name":              {DeployFilter{Author: "grace hopper"}, 1},
		"author email":             {DeployFilter{Author: "ADA@example"}, 2},
		"author substring":         {DeployFilter{Author: "lovelace"}, 2},
		"author wildcards":         {DeployFilter{Author: "%"}, 0},
		"no match":                 {DeployFilter{Branch: "gone"}, 0},
		"status ready":             {DeployFilter{Status: DeployReady}, 1},
		"status queued":            {DeployFilter{Status: DeployQueued}, 2},
		"query sha prefix":         {DeployFilter{Query: "bbbb"}, 1},
		"query sha case":           {DeployFilter{Query: "BBBBBBB2"}, 1},
		"query sha is prefix-only": {DeployFilter{Query: "2222"}, 0},
		"query branch substring":   {DeployFilter{Query: "feat"}, 1},
		"query repo substring":     {DeployFilter{Query: "li"}, 1},
		"query ref":                {DeployFilter{Query: "v1.2"}, 1},
		"query author":             {DeployFilter{Query: "grace"}, 1},
		"query wildcards":          {DeployFilter{Query: "%"}, 0},
		"query+repo":               {DeployFilter{Query: "main", Repo: "app"}, 1},
		"query+status":             {DeployFilter{Query: "grace", Status: DeployReady}, 1},
		"query no match":           {DeployFilter{Query: "zzz"}, 0},
	} {
		got, err := s.ListDeploys(tc.f)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(got) != tc.want {
			t.Errorf("%s: %d deploys, want %d", name, len(got), tc.want)
		}
	}
}

// TestListDeploysCrashedFilter: runtime state lives in the supervisor, not
// in a column, so the caller hands in the crashed process keys and the
// filter matches deploy rows against them — by be_hash for a backend (every
// deploy sharing that artifact is served by the one process) and by the
// fe_hash/be_hash pair for a process-mode frontend.
func TestListDeploysCrashedFilter(t *testing.T) {
	s := newTestStore(t)
	r, _ := s.CreateRepo("app", "/src/app", "/bare/app", RepoReady)
	shaB := "bbbbbbb2222222222222222222222222222222222"
	shaC := "ccccccc3333333333333333333333333333333333"
	hashes := map[string][2]string{
		shaA: {"fe1", "be1"}, // shares be1 with shaB
		shaB: {"fe2", "be1"},
		shaC: {"fe3", "be2"},
	}
	byID := map[string]int64{}
	for _, sha := range []string{shaA, shaB, shaC} {
		d, err := s.CreateDeploy(r.ID, sha, DeployMeta{})
		if err != nil {
			t.Fatal(err)
		}
		h := hashes[sha]
		if err := s.SetDeployHashes(d.ID, h[0], h[1], "", ""); err != nil {
			t.Fatal(err)
		}
		if err := s.SetDeployReady(d.ID); err != nil {
			t.Fatal(err)
		}
		byID[sha] = d.ID
	}

	deadBackend := ProcKey{RepoID: r.ID, BeHash: "be1"}
	deadFrontend := ProcKey{RepoID: r.ID, FeHash: "fe3", BeHash: "be2"}
	for name, tc := range map[string]struct {
		f    DeployFilter
		want []string
	}{
		"backend crash covers both deploys on it": {
			DeployFilter{Status: DeployReady, Crashed: CrashedOnly, CrashedProcs: []ProcKey{deadBackend}},
			[]string{shaB, shaA},
		},
		"ready excludes them": {
			DeployFilter{Status: DeployReady, Crashed: CrashedNone, CrashedProcs: []ProcKey{deadBackend}},
			[]string{shaC},
		},
		"frontend crash is keyed by its backend too": {
			DeployFilter{Status: DeployReady, Crashed: CrashedOnly, CrashedProcs: []ProcKey{deadFrontend}},
			[]string{shaC},
		},
		"a frontend key doesn't match its peer's other deploys": {
			DeployFilter{Status: DeployReady, Crashed: CrashedOnly,
				CrashedProcs: []ProcKey{{RepoID: r.ID, FeHash: "fe1", BeHash: "be1"}}},
			[]string{shaA},
		},
		"another repo's crash matches nothing": {
			DeployFilter{Status: DeployReady, Crashed: CrashedOnly,
				CrashedProcs: []ProcKey{{RepoID: r.ID + 1, BeHash: "be1"}}},
			nil,
		},
		"nothing crashed: only matches nothing": {
			DeployFilter{Status: DeployReady, Crashed: CrashedOnly},
			nil,
		},
		"nothing crashed: none matches everything": {
			DeployFilter{Status: DeployReady, Crashed: CrashedNone},
			[]string{shaC, shaB, shaA},
		},
	} {
		got, err := s.ListDeploys(tc.f)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		shas := make([]string, len(got))
		for i, d := range got {
			shas[i] = d.SHA
		}
		if !slices.Equal(shas, tc.want) {
			t.Errorf("%s: %v, want %v", name, shas, tc.want)
		}
		n, err := s.CountDeploys(tc.f)
		if err != nil {
			t.Fatalf("%s: count: %v", name, err)
		}
		if n != len(tc.want) {
			t.Errorf("%s: count %d, want %d", name, n, len(tc.want))
		}
	}
}

func TestListDeploysLimit(t *testing.T) {
	s := newTestStore(t)
	r, err := s.CreateRepo("app", "/src/app", "/bare/app", RepoReady)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]int64, 3)
	for i := range ids {
		d, err := s.CreateDeploy(r.ID, strings.Repeat(strconv.Itoa(i+1), 40), DeployMeta{})
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = d.ID
	}

	got, err := s.ListDeploys(DeployFilter{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != ids[2] || got[1].ID != ids[1] {
		t.Fatalf("limit 2 = %+v, want the two newest (%d, %d)", got, ids[2], ids[1])
	}
	if got, err = s.ListDeploys(DeployFilter{Limit: 10}); err != nil || len(got) != 3 {
		t.Fatalf("limit beyond rows = %d deploys (%v), want 3", len(got), err)
	}
	if got, err = s.ListDeploys(DeployFilter{}); err != nil || len(got) != 3 {
		t.Fatalf("no limit = %d deploys (%v), want 3", len(got), err)
	}
}

func TestListDeploysOffsetAndCount(t *testing.T) {
	s := newTestStore(t)
	r, err := s.CreateRepo("app", "/src/app", "/bare/app", RepoReady)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]int64, 5)
	for i := range ids {
		d, err := s.CreateDeploy(r.ID, strings.Repeat(strconv.Itoa(i+1), 40), DeployMeta{})
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = d.ID
		// Evict the oldest two so counts can be checked against a filter that
		// doesn't match everything.
		if i < 2 {
			if err := s.SetDeployEvicted(d.ID); err != nil {
				t.Fatal(err)
			}
		}
	}

	// Walking pages of 2 must cover every row exactly once, newest first.
	var seen []int64
	for off := 0; off < 5; off += 2 {
		page, err := s.ListDeploys(DeployFilter{Limit: 2, Offset: off})
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range page {
			seen = append(seen, d.ID)
		}
	}
	want := []int64{ids[4], ids[3], ids[2], ids[1], ids[0]}
	if !slices.Equal(seen, want) {
		t.Errorf("paged ids = %v, want %v", seen, want)
	}

	// An offset with no limit still skips — SQLite won't parse OFFSET without
	// a LIMIT clause in front of it.
	rest, err := s.ListDeploys(DeployFilter{Offset: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(rest) != 2 || rest[0].ID != ids[1] {
		t.Errorf("offset without limit = %d rows, want 2 starting at %d", len(rest), ids[1])
	}

	// Count ignores Limit/Offset but honors the predicates.
	for name, tc := range map[string]struct {
		f    DeployFilter
		want int
	}{
		"all":           {DeployFilter{}, 5},
		"paged":         {DeployFilter{Limit: 2, Offset: 2}, 5},
		"evicted":       {DeployFilter{Status: DeployEvicted}, 2},
		"evicted paged": {DeployFilter{Status: DeployEvicted, Limit: 1}, 2},
		"other repo":    {DeployFilter{Repo: "nope"}, 0},
		"queued":        {DeployFilter{Status: DeployQueued}, 3},
	} {
		got, err := s.CountDeploys(tc.f)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got != tc.want {
			t.Errorf("%s: count = %d, want %d", name, got, tc.want)
		}
	}
}

func TestDeployRowHasFeProcess(t *testing.T) {
	s := newTestStore(t)
	r, err := s.CreateRepo("app", "/src/app", "/bare/app", RepoReady)
	if err != nil {
		t.Fatal(err)
	}
	d, err := s.CreateDeploy(r.ID, shaA, DeployMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetDeployHashes(d.ID, "fe1", "be1", "", ""); err != nil {
		t.Fatal(err)
	}

	row, err := s.GetDeployByID(d.ID)
	if err != nil || row.HasFeProcess {
		t.Fatalf("before frontend_artifacts row: HasFeProcess = %v (%v), want false", row.HasFeProcess, err)
	}
	if err := s.CreateFrontendArtifact(FrontendArtifact{RepoID: r.ID, FeHash: "fe1", RunConfig: "{}"}); err != nil {
		t.Fatal(err)
	}
	if row, err = s.GetDeployByID(d.ID); err != nil || !row.HasFeProcess {
		t.Fatalf("after frontend_artifacts row: HasFeProcess = %v (%v), want true", row.HasFeProcess, err)
	}
	rows, err := s.ListDeploys(DeployFilter{})
	if err != nil || len(rows) != 1 || !rows[0].HasFeProcess {
		t.Fatalf("ListDeploys HasFeProcess = %+v (%v), want true", rows, err)
	}
}

func TestMigrateAddsColumnsToExistingDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	old, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	// A repos table from before the watch columns existed.
	if _, err := old.Exec(`CREATE TABLE repos (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		source TEXT NOT NULL,
		bare_path TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatal(err)
	}
	// A deploys table from before the commit-metadata columns existed.
	if _, err := old.Exec(`CREATE TABLE deploys (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		repo_id INTEGER NOT NULL,
		sha TEXT NOT NULL,
		short_sha TEXT NOT NULL,
		ref TEXT NOT NULL DEFAULT '',
		fe_hash TEXT NOT NULL DEFAULT '',
		be_hash TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'queued',
		error TEXT NOT NULL DEFAULT '',
		attempt_count INTEGER NOT NULL DEFAULT 0,
		fe_build_log_path TEXT NOT NULL DEFAULT '',
		be_build_log_path TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT '',
		UNIQUE (repo_id, sha),
		UNIQUE (repo_id, short_sha)
	)`); err != nil {
		t.Fatal(err)
	}
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := s.CreateRepo("demo", "/src", "/bare", RepoReady)
	if err != nil {
		t.Fatal(err)
	}
	d, err := s.CreateDeploy(r.ID, shaA, DeployMeta{Branch: "main", AuthorName: "Ada"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Branch != "main" || d.AuthorName != "Ada" {
		t.Fatalf("migrated columns not usable: %+v", d)
	}
	if r, err = s.SetRepoWatch(r.ID, true, "main", false); err != nil || !r.Watch {
		t.Fatalf("migrated repos columns not usable: %+v, %v", r, err)
	}
}

func TestDeployArtifactsRoundTrip(t *testing.T) {
	s := newTestStore(t)
	r, err := s.CreateRepo("demo", "/src", "/bare", RepoReady)
	if err != nil {
		t.Fatal(err)
	}
	d, err := s.CreateDeploy(r.ID, shaA, DeployMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if d.Artifacts != nil {
		t.Fatalf("fresh deploy artifacts = %+v, want none", d.Artifacts)
	}

	refs := map[string]ArtifactRef{
		"cli":   {Hash: "abc", LogPath: "/logs/demo/dl/abc.log"},
		"agent": {Hash: "def", LogPath: "/logs/demo/dl/def.log"},
	}
	if err := s.SetDeployArtifacts(d.ID, refs); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetDeployByID(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Artifacts) != 2 || got.Artifacts["cli"] != refs["cli"] || got.Artifacts["agent"] != refs["agent"] {
		t.Fatalf("artifacts = %+v, want %+v", got.Artifacts, refs)
	}

	// Per-artifact build outcomes land on the stored refs one at a time.
	if err := s.SetDeployArtifactStatus(d.ID, "cli", ArtifactFailed, "boom"); err != nil {
		t.Fatal(err)
	}
	got, err = s.GetDeployByID(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ref := got.Artifacts["cli"]; ref.Status != ArtifactFailed || ref.Error != "boom" || ref.Hash != "abc" {
		t.Fatalf("cli ref after status update = %+v", ref)
	}
	if ref := got.Artifacts["agent"]; ref.Status != "" || ref.Error != "" {
		t.Fatalf("agent ref touched by cli update = %+v", ref)
	}
	if err := s.SetDeployArtifactStatus(d.ID, "nope", ArtifactReady, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("status update of unknown artifact = %v, want ErrNotFound", err)
	}

	// An empty map clears the column (a rebuild whose manifest dropped them).
	if err := s.SetDeployArtifacts(d.ID, nil); err != nil {
		t.Fatal(err)
	}
	got, err = s.GetDeployByID(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Artifacts != nil {
		t.Fatalf("cleared artifacts = %+v, want none", got.Artifacts)
	}
}

// TestListUnfinishedDeployIDs: startup resume picks up queued/building rows
// and ready rows whose post-ready artifact builds were interrupted — not
// ready rows whose artifacts finished (or predate per-artifact statuses).
func TestListUnfinishedDeployIDs(t *testing.T) {
	s := newTestStore(t)
	r, err := s.CreateRepo("demo", "/src", "/bare", RepoReady)
	if err != nil {
		t.Fatal(err)
	}
	mk := func(sha string, status string, refs map[string]ArtifactRef) int64 {
		t.Helper()
		d, err := s.CreateDeploy(r.ID, sha, DeployMeta{})
		if err != nil {
			t.Fatal(err)
		}
		if refs != nil {
			if err := s.SetDeployArtifacts(d.ID, refs); err != nil {
				t.Fatal(err)
			}
		}
		switch status {
		case DeployBuilding:
			err = s.SetDeployBuilding(d.ID)
		case DeployReady:
			err = s.SetDeployReady(d.ID)
		case DeployFailed:
			err = s.SetDeployFailed(d.ID, "boom")
		}
		if err != nil {
			t.Fatal(err)
		}
		return d.ID
	}

	queued := mk("a000000000000000000000000000000000000000", DeployQueued, nil)
	building := mk("b000000000000000000000000000000000000000", DeployBuilding, nil)
	interrupted := mk("c000000000000000000000000000000000000000", DeployReady, map[string]ArtifactRef{"cli": {Hash: "a", Status: ArtifactBuilding}})
	mk("d000000000000000000000000000000000000000", DeployReady, map[string]ArtifactRef{"cli": {Hash: "a", Status: ArtifactReady}})
	mk("e000000000000000000000000000000000000000", DeployReady, map[string]ArtifactRef{"cli": {Hash: "a"}})
	mk("f000000000000000000000000000000000000000", DeployReady, nil)
	mk("0a00000000000000000000000000000000000000", DeployFailed, map[string]ArtifactRef{"cli": {Hash: "a", Status: ArtifactBuilding}})

	ids, err := s.ListUnfinishedDeployIDs()
	if err != nil {
		t.Fatal(err)
	}
	want := []int64{queued, building, interrupted}
	if len(ids) != len(want) || ids[0] != want[0] || ids[1] != want[1] || ids[2] != want[2] {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
}

func TestDeploysBySHAPrefix(t *testing.T) {
	s := newTestStore(t)
	r, _ := s.CreateRepo("demo", "/src", "/bare", RepoReady)
	if _, err := s.CreateDeploy(r.ID, shaA, DeployMeta{}); err != nil {
		t.Fatal(err)
	}

	got, err := s.DeploysBySHAPrefix(r.ID, "aaaaaaa")
	if err != nil || len(got) != 1 {
		t.Fatalf("prefix match = %+v, %v", got, err)
	}
	got, err = s.DeploysBySHAPrefix(r.ID, "bbbb999")
	if err != nil || len(got) != 0 {
		t.Fatalf("no-match = %+v, %v", got, err)
	}
	if _, err := s.DeploysBySHAPrefix(r.ID, "AB%'--"); err == nil {
		t.Fatal("non-hex prefix should be rejected")
	}
}

func TestBackendArtifactsAndProcesses(t *testing.T) {
	s := newTestStore(t)
	r, _ := s.CreateRepo("demo", "/src", "/bare", RepoReady)

	a := BackendArtifact{RepoID: r.ID, BeHash: "be1", StateDir: "/state/demo/be1", RunConfig: `{"run":["x"]}`}
	if err := s.CreateBackendArtifact(a); err != nil {
		t.Fatal(err)
	}
	// Idempotent.
	a2 := a
	a2.ForkedFrom = "should-not-replace"
	if err := s.CreateBackendArtifact(a2); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetBackendArtifact(r.ID, "be1")
	if err != nil || got.ForkedFrom != "" || got.RunConfig != a.RunConfig {
		t.Fatalf("GetBackendArtifact = %+v, %v", got, err)
	}
	if _, err := s.GetBackendArtifact(r.ID, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing artifact err = %v", err)
	}

	if got.InitDoneAt != "" {
		t.Fatalf("InitDoneAt before mark = %q, want empty", got.InitDoneAt)
	}
	if err := s.MarkBackendInitDone(r.ID, "be1"); err != nil {
		t.Fatal(err)
	}
	got, err = s.GetBackendArtifact(r.ID, "be1")
	if err != nil || got.InitDoneAt == "" {
		t.Fatalf("after MarkBackendInitDone: %+v, %v", got, err)
	}

	rec := ProcessRecord{RepoID: r.ID, BeHash: "be1", PID: 123, PGID: 123, Port: 40001}
	if err := s.UpsertProcessRecord(rec); err != nil {
		t.Fatal(err)
	}
	rec.Port = 40002
	if err := s.UpsertProcessRecord(rec); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListProcessRecords()
	if err != nil || len(list) != 1 || list[0].Port != 40002 {
		t.Fatalf("ListProcessRecords = %+v, %v", list, err)
	}
	if err := s.DeleteProcessRecord(r.ID, "be1"); err != nil {
		t.Fatal(err)
	}
	list, _ = s.ListProcessRecords()
	if len(list) != 0 {
		t.Fatalf("records after delete = %+v", list)
	}

	if err := s.AddProcessEvent(r.ID, "be1", "start_attempt", "port 40001"); err != nil {
		t.Fatal(err)
	}
}
