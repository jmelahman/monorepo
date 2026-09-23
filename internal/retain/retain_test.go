package retain

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/store"
	"github.com/jmelahman/local-preview/internal/supervise"
)

type fixture struct {
	t     *testing.T
	db    *db.Store
	files *store.Store
	sw    *Sweeper
	repo  db.Repo
	seq   int
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	root := t.TempDir()
	files := store.New(
		filepath.Join(root, "artifacts"),
		filepath.Join(root, "state"),
		filepath.Join(root, "tmp"))
	super := supervise.New(database, files, filepath.Join(root, "logs"))
	repo, err := database.CreateRepo("web", "/src/web", "/data/web.git", db.RepoReady)
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{t: t, db: database, files: files, sw: New(database, super, files), repo: repo}
}

// addDeploy creates a deploy with the given status and backend hash, and
// materializes the hash's artifact dir, state dir, and build log on disk.
func (f *fixture) addDeploy(status, beHash string) db.Deploy {
	f.t.Helper()
	f.seq++
	d, err := f.db.CreateDeploy(f.repo.ID, fmt.Sprintf("%040x", f.seq), db.DeployMeta{Branch: "main"})
	if err != nil {
		f.t.Fatal(err)
	}
	if beHash != "" {
		if err := f.db.SetDeployHashes(d.ID, "", beHash, "", ""); err != nil {
			f.t.Fatal(err)
		}
		f.mkdirWithFile(f.files.BackendDir(f.repo.Name, beHash))
		f.mkdirWithFile(f.files.StateDirPath(f.repo.Name, beHash))
	}
	switch status {
	case db.DeployReady:
		err = f.db.SetDeployReady(d.ID)
	case db.DeployFailed:
		err = f.db.SetDeployFailed(d.ID, "boom")
	case db.DeployBuilding:
		err = f.db.SetDeployBuilding(d.ID)
	case db.DeployQueued:
	}
	if err != nil {
		f.t.Fatal(err)
	}
	return d
}

func (f *fixture) mkdirWithFile(dir string) {
	f.t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data"), []byte("x"), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

// statusByID reads every deploy's current status.
func (f *fixture) statusByID() map[int64]string {
	f.t.Helper()
	rows, err := f.db.ListDeploys(db.DeployFilter{})
	if err != nil {
		f.t.Fatal(err)
	}
	out := map[int64]string{}
	for _, r := range rows {
		out[r.ID] = r.Status
	}
	return out
}

func (f *fixture) setPolicy(p Policy) {
	f.t.Helper()
	if err := SavePolicy(f.db, p); err != nil {
		f.t.Fatal(err)
	}
}

func TestSweepCountLimit(t *testing.T) {
	f := newFixture(t)
	var deploys []db.Deploy
	for i := 0; i < 5; i++ {
		deploys = append(deploys, f.addDeploy(db.DeployReady, fmt.Sprintf("be%d", i)))
	}
	f.setPolicy(Policy{MaxDeploysPerRepo: 2})

	res, err := f.sw.Sweep()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Evicted) != 3 {
		t.Fatalf("evicted %d deploys, want 3: %+v", len(res.Evicted), res.Evicted)
	}
	status := f.statusByID()
	for i, d := range deploys {
		want := db.DeployEvicted
		if i >= 3 { // the two newest survive
			want = db.DeployReady
		}
		if status[d.ID] != want {
			t.Errorf("deploy %d: status %s, want %s", d.ID, status[d.ID], want)
		}
	}
	for i := 0; i < 3; i++ {
		if f.files.HasBackend(f.repo.Name, fmt.Sprintf("be%d", i)) {
			t.Errorf("evicted artifact be%d still on disk", i)
		}
	}
	for i := 3; i < 5; i++ {
		if !f.files.HasBackend(f.repo.Name, fmt.Sprintf("be%d", i)) {
			t.Errorf("surviving artifact be%d was removed", i)
		}
	}

	// A second sweep is a no-op: evicted deploys don't count toward the
	// limit and are never re-evicted.
	res, err = f.sw.Sweep()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Evicted) != 0 {
		t.Fatalf("second sweep evicted %d deploys, want 0", len(res.Evicted))
	}
}

func TestSweepAgeLimit(t *testing.T) {
	f := newFixture(t)
	old := f.addDeploy(db.DeployReady, "be-old")
	failed := f.addDeploy(db.DeployFailed, "be-failed")
	newest := f.addDeploy(db.DeployReady, "be-new")
	f.setPolicy(Policy{MaxAgeDays: 30})
	// Everything was just created; pretend the sweep runs 40 days later.
	f.sw.now = func() time.Time { return time.Now().AddDate(0, 0, 40) }

	res, err := f.sw.Sweep()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Evicted) != 2 {
		t.Fatalf("evicted %d deploys, want 2: %+v", len(res.Evicted), res.Evicted)
	}
	status := f.statusByID()
	if status[newest.ID] != db.DeployReady {
		t.Errorf("newest ready deploy must survive an age sweep, got %s", status[newest.ID])
	}
	if status[old.ID] != db.DeployEvicted || status[failed.ID] != db.DeployEvicted {
		t.Errorf("old deploys not evicted: %v", status)
	}
}

func TestSweepProtections(t *testing.T) {
	f := newFixture(t)
	aliased := f.addDeploy(db.DeployReady, "be-aliased")
	if err := f.db.SetBranchAlias(f.repo.ID, "feature", aliased.ID); err != nil {
		t.Fatal(err)
	}
	failed := f.addDeploy(db.DeployFailed, "be-x")
	queued := f.addDeploy(db.DeployQueued, "")
	building := f.addDeploy(db.DeployBuilding, "be-y")
	ready := f.addDeploy(db.DeployReady, "be-z")
	f.setPolicy(Policy{MaxDeploysPerRepo: 1})

	res, err := f.sw.Sweep()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Evicted) != 1 || res.Evicted[0].ID != failed.ID {
		t.Fatalf("want exactly the failed deploy evicted, got %+v", res.Evicted)
	}
	status := f.statusByID()
	for _, d := range []db.Deploy{aliased, queued, building, ready} {
		if status[d.ID] == db.DeployEvicted {
			t.Errorf("protected deploy %d was evicted", d.ID)
		}
	}
}

func TestSweepKeepsSharedHash(t *testing.T) {
	f := newFixture(t)
	f.addDeploy(db.DeployReady, "be-shared")
	f.addDeploy(db.DeployReady, "be-shared")
	f.setPolicy(Policy{MaxDeploysPerRepo: 1})

	res, err := f.sw.Sweep()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Evicted) != 1 {
		t.Fatalf("evicted %d deploys, want 1", len(res.Evicted))
	}
	if !f.files.HasBackend(f.repo.Name, "be-shared") {
		t.Error("artifact shared with a surviving deploy was removed")
	}
}

func TestSweepDisabledPolicy(t *testing.T) {
	f := newFixture(t)
	f.addDeploy(db.DeployReady, "be0")
	f.addDeploy(db.DeployReady, "be1")

	res, err := f.sw.Sweep()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Evicted) != 0 {
		t.Fatalf("disabled policy evicted %d deploys", len(res.Evicted))
	}
}

func TestPolicyPersistence(t *testing.T) {
	f := newFixture(t)
	p, err := LoadPolicy(f.db)
	if err != nil || p.Enabled() {
		t.Fatalf("unset policy: got %+v, %v", p, err)
	}
	if err := SavePolicy(f.db, Policy{MaxDeploysPerRepo: -1}); err == nil {
		t.Fatal("negative limit saved without error")
	}
	want := Policy{MaxDeploysPerRepo: 10, MaxAgeDays: 30}
	if err := SavePolicy(f.db, want); err != nil {
		t.Fatal(err)
	}
	if p, err = LoadPolicy(f.db); err != nil || p != want {
		t.Fatalf("got %+v, %v; want %+v", p, err, want)
	}
}
