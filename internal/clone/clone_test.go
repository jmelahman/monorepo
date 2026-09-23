package clone

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/gitrepo"
)

func newSourceRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"add", "-A"},
		{"commit", "-qm", "initial"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func newHarness(t *testing.T) (*db.Store, *gitrepo.Manager) {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return database, gitrepo.NewManager(filepath.Join(t.TempDir(), "repos"))
}

func waitStatus(t *testing.T, database *db.Store, name, want string) db.Repo {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		repo, err := database.GetRepoByName(name)
		if err != nil {
			t.Fatalf("get repo %s: %v", name, err)
		}
		if repo.Status == want {
			return repo
		}
		if repo.Status != db.RepoCloning {
			t.Fatalf("repo %s reached %q (error %q), want %q", name, repo.Status, repo.Error, want)
		}
		if time.Now().After(deadline) {
			t.Fatalf("repo %s stuck in %q, want %q", name, repo.Status, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestBeginClonesAndReports(t *testing.T) {
	database, gitMgr := newHarness(t)
	src := newSourceRepo(t)
	var kicked atomic.Bool
	c := New(database, gitMgr, func() { kicked.Store(true) })
	c.Start(context.Background())

	repo, err := database.CreateRepo("demo", src, gitMgr.Open("demo").Path, db.RepoCloning)
	if err != nil {
		t.Fatal(err)
	}
	c.Begin(repo)

	waitStatus(t, database, "demo", db.RepoReady)
	if !kicked.Load() {
		t.Error("onReady was not called")
	}
	if _, err := os.Stat(repo.BarePath); err != nil {
		t.Errorf("mirror missing after clone: %v", err)
	}
	if got := c.Progress(repo.ID); got != "" {
		t.Errorf("finished clone still reports progress %q", got)
	}
}

func TestBeginRecordsFailure(t *testing.T) {
	database, gitMgr := newHarness(t)
	c := New(database, gitMgr, nil)
	c.Start(context.Background())

	src := filepath.Join(t.TempDir(), "nope")
	repo, err := database.CreateRepo("demo", src, gitMgr.Open("demo").Path, db.RepoCloning)
	if err != nil {
		t.Fatal(err)
	}
	c.Begin(repo)

	repo = waitStatus(t, database, "demo", db.RepoFailed)
	if repo.Error == "" {
		t.Error("failed clone left no error message")
	}
}

func TestStartResumesInterruptedClones(t *testing.T) {
	database, gitMgr := newHarness(t)
	src := newSourceRepo(t)
	// A row a previous run left mid-clone: status cloning, no goroutine.
	if _, err := database.CreateRepo("demo", src, gitMgr.Open("demo").Path, db.RepoCloning); err != nil {
		t.Fatal(err)
	}
	New(database, gitMgr, nil).Start(context.Background())
	waitStatus(t, database, "demo", db.RepoReady)
}

func TestProgressTailKeepsCurrentLine(t *testing.T) {
	p := &progressTail{}
	p.Write([]byte("Counting objects: 10\rCounting objects: 20\r"))
	p.Write([]byte("Compressing objects: 5%"))
	if got, want := p.Last(), "Compressing objects: 5%"; got != want {
		t.Errorf("Last() = %q, want %q", got, want)
	}
	// A trailing newline means the last full line is still current.
	p.Write([]byte("\n"))
	if got, want := p.Last(), "Compressing objects: 5%"; got != want {
		t.Errorf("Last() after newline = %q, want %q", got, want)
	}
}
