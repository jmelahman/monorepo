package watch

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmelahman/local-preview/internal/build"
	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/gitrepo"
	"github.com/jmelahman/local-preview/internal/store"
	"github.com/jmelahman/local-preview/internal/supervise"
)

func runTestGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func commitFile(t *testing.T, dir, name, content, msg string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, dir, "add", "-A")
	runTestGit(t, dir, "commit", "-qm", msg)
	return runTestGit(t, dir, "rev-parse", "HEAD")
}

// newWatchFixture builds a source repo with a main branch, registers it
// (watched, with the given branch filter), and returns everything a poll
// needs. Watching is enabled with backfill, so the tips that already exist
// are fair game — tests that care about the baseline turn it off. The
// queue's workers are never started, so requested deploys stay queued —
// these tests assert on rows, not builds.
func newWatchFixture(t *testing.T, branches string) (*Watcher, db.Repo, string) {
	t.Helper()
	return newWatchFixtureOpts(t, branches, true)
}

func newWatchFixtureOpts(t *testing.T, branches string, backfill bool) (*Watcher, db.Repo, string) {
	t.Helper()
	src := t.TempDir()
	runTestGit(t, src, "init", "-q", "-b", "main")
	commitFile(t, src, "a.txt", "one", "initial")

	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	root := t.TempDir()
	files := store.New(
		filepath.Join(root, "artifacts"),
		filepath.Join(root, "state"),
		filepath.Join(root, "tmp"),
	)
	gitMgr := gitrepo.NewManager(filepath.Join(root, "repos"))
	super := supervise.New(database, files, filepath.Join(root, "logs"))
	queue := build.NewQueue(database, gitMgr, files, super, filepath.Join(root, "logs"), nil)

	gr, err := gitMgr.Add(context.Background(), "demo", src, nil)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := database.CreateRepo("demo", src, gr.Path, db.RepoReady)
	if err != nil {
		t.Fatal(err)
	}
	if repo, err = database.SetRepoWatch(repo.ID, true, branches, backfill); err != nil {
		t.Fatal(err)
	}
	return New(database, gitMgr, queue, DefaultInterval), repo, src
}

// deploysByBranch maps branch → sha for every deploy row.
func deploysByBranch(t *testing.T, w *Watcher) map[string]string {
	t.Helper()
	rows, err := w.db.ListDeploys(db.DeployFilter{})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, d := range rows {
		got[d.Branch] = d.SHA
	}
	return got
}

func TestPollDeploysNewBranchTips(t *testing.T) {
	w, _, src := newWatchFixture(t, "")
	ctx := context.Background()

	w.PollAll(ctx)
	first := deploysByBranch(t, w)
	if len(first) != 1 || first["main"] == "" {
		t.Fatalf("after first poll: %v", first)
	}

	// Unchanged tips must not re-deploy.
	w.PollAll(ctx)
	if rows, _ := w.db.ListDeploys(db.DeployFilter{}); len(rows) != 1 {
		t.Fatalf("re-poll created rows: %d", len(rows))
	}

	// A new commit on a new branch deploys once the poller sees it.
	runTestGit(t, src, "checkout", "-qb", "feature")
	sha := commitFile(t, src, "b.txt", "two", "feature work")
	w.PollAll(ctx)
	got := deploysByBranch(t, w)
	if got["feature"] != sha {
		t.Fatalf("feature tip not deployed: %v", got)
	}

	// Advancing an existing branch deploys the new tip alongside the old.
	sha2 := commitFile(t, src, "c.txt", "three", "more feature work")
	w.PollAll(ctx)
	rows, _ := w.db.ListDeploys(db.DeployFilter{})
	if len(rows) != 3 {
		t.Fatalf("deploy count = %d, want 3", len(rows))
	}
	found := false
	for _, d := range rows {
		found = found || d.SHA == sha2
	}
	if !found {
		t.Fatalf("advanced tip %s not deployed: %+v", sha2, rows)
	}
}

func TestPollEvictsDeletedBranchDeploys(t *testing.T) {
	w, repo, src := newWatchFixture(t, "")
	ctx := context.Background()

	// main plus an unmerged feature branch each get a (queued) deploy.
	runTestGit(t, src, "checkout", "-qb", "feature")
	featSHA := commitFile(t, src, "b.txt", "two", "feature work")
	runTestGit(t, src, "checkout", "-q", "main")
	mainSHA := runTestGit(t, src, "rev-parse", "HEAD")
	w.PollAll(ctx)
	if rows, _ := w.db.ListDeploys(db.DeployFilter{}); len(rows) != 2 {
		t.Fatalf("deploy count = %d, want 2", len(rows))
	}

	// Deleting the unmerged branch evicts its deploy on the next poll; the
	// main deploy is untouched because its commit is still a live tip.
	runTestGit(t, src, "branch", "-D", "feature")
	w.PollAll(ctx)

	feat, err := w.db.GetDeployBySHA(repo.ID, featSHA)
	if err != nil {
		t.Fatal(err)
	}
	if feat.Status != db.DeployEvicted {
		t.Fatalf("feature deploy status = %q, want %q", feat.Status, db.DeployEvicted)
	}
	main, err := w.db.GetDeployBySHA(repo.ID, mainSHA)
	if err != nil {
		t.Fatal(err)
	}
	if main.Status == db.DeployEvicted {
		t.Fatal("main deploy was evicted, want it left alone")
	}

	// Re-poll is idempotent: an already-evicted deploy is not re-processed and
	// no live deploy is disturbed.
	w.PollAll(ctx)
	if feat, _ = w.db.GetDeployBySHA(repo.ID, featSHA); feat.Status != db.DeployEvicted {
		t.Fatalf("after re-poll feature status = %q, want %q", feat.Status, db.DeployEvicted)
	}
}

// Quiet polls (no ref movement) must make the same decisions a full pass
// would: a deploy deleted between polls is re-requested from the cached
// tips even though no graph walk runs.
func TestQuietPollRestoresDeletedDeploy(t *testing.T) {
	w, repo, _ := newWatchFixture(t, "")
	ctx := context.Background()

	w.PollAll(ctx)
	rows, err := w.db.ListDeploys(db.DeployFilter{Repo: repo.Name})
	if err != nil || len(rows) != 1 {
		t.Fatalf("after first poll: %d deploys (%v), want 1", len(rows), err)
	}
	mainSHA := rows[0].SHA
	if err := w.db.DeleteDeploy(rows[0].ID); err != nil {
		t.Fatal(err)
	}

	// No commits since the last poll — this poll takes the quiet path.
	w.PollAll(ctx)
	if _, err := w.db.GetDeployBySHA(repo.ID, mainSHA); err != nil {
		t.Fatalf("deleted tip deploy was not restored by a quiet poll: %v", err)
	}
}

// Eviction inputs can change without a ref change only when a deploy row is
// created between polls; a quiet poll must still evict such a deploy when
// its commit is unreachable (present in the mirror, on no branch).
func TestQuietPollEvictsNewUnreachableDeploy(t *testing.T) {
	w, repo, src := newWatchFixture(t, "")
	ctx := context.Background()

	// A branch is deployed, then deleted: its commit stays in the mirror's
	// object store but is unreachable, and its deploy row is evicted.
	runTestGit(t, src, "checkout", "-qb", "feature")
	featSHA := commitFile(t, src, "b.txt", "two", "feature work")
	runTestGit(t, src, "checkout", "-q", "main")
	w.PollAll(ctx)
	runTestGit(t, src, "branch", "-D", "feature")
	w.PollAll(ctx)

	// A fresh deploy row for that dead commit, created with no ref change —
	// as if a user re-deployed the sha of a deleted branch.
	old, err := w.db.GetDeployBySHA(repo.ID, featSHA)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.db.DeleteDeploy(old.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := w.db.CreateDeploy(repo.ID, featSHA, db.DeployMeta{}); err != nil {
		t.Fatal(err)
	}

	w.PollAll(ctx)
	d, err := w.db.GetDeployBySHA(repo.ID, featSHA)
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != db.DeployEvicted {
		t.Fatalf("unreachable deploy status = %q after quiet poll, want %q", d.Status, db.DeployEvicted)
	}
}

func TestPollHonorsBranchFilter(t *testing.T) {
	w, _, src := newWatchFixture(t, "main,release/*")
	runTestGit(t, src, "branch", "release/1.0")
	runTestGit(t, src, "checkout", "-qb", "scratch")
	commitFile(t, src, "s.txt", "x", "scratch work")

	w.PollAll(context.Background())
	rows, _ := w.db.ListDeploys(db.DeployFilter{})
	// main and release/1.0 share a tip, so one deploy covers both; the
	// scratch tip is filtered out.
	if len(rows) != 1 {
		t.Fatalf("deploy count = %d, want 1: %+v", len(rows), rows)
	}
	if got := deploysByBranch(t, w); got["scratch"] != "" {
		t.Fatalf("filtered branch deployed: %v", got)
	}
}

func TestPollBaselinesExistingTips(t *testing.T) {
	w, repo, src := newWatchFixtureOpts(t, "", false)
	ctx := context.Background()

	// Branches that predate watching are recorded, not deployed.
	runTestGit(t, src, "checkout", "-qb", "old-feature")
	commitFile(t, src, "b.txt", "two", "old work")
	runTestGit(t, src, "checkout", "-q", "main")
	w.PollAll(ctx)
	if rows, _ := w.db.ListDeploys(db.DeployFilter{}); len(rows) != 0 {
		t.Fatalf("baseline poll deployed %d row(s): %+v", len(rows), rows)
	}
	base, err := w.db.WatchBaseline(repo.ID)
	if err != nil || len(base) != 2 {
		t.Fatalf("baseline = %v, %v; want both tips", base, err)
	}

	// A tip that moves afterwards is a change, and deploys.
	sha := commitFile(t, src, "c.txt", "three", "new work")
	w.PollAll(ctx)
	got := deploysByBranch(t, w)
	if len(got) != 1 || got["main"] != sha {
		t.Fatalf("post-baseline deploys = %v, want main@%s alone", got, sha)
	}

	// main's old tip has left the baseline with it, so the entries drain as
	// the repo moves on.
	if base, _ = w.db.WatchBaseline(repo.ID); len(base) != 1 {
		t.Fatalf("baseline = %v, want only the untouched branch", base)
	}
}

func TestPollBackfillDeploysExistingTips(t *testing.T) {
	w, repo, _ := newWatchFixtureOpts(t, "", true)
	w.PollAll(context.Background())
	if rows, _ := w.db.ListDeploys(db.DeployFilter{}); len(rows) != 1 {
		t.Fatalf("backfill deployed %d row(s), want the existing tip", len(rows))
	}
	if base, _ := w.db.WatchBaseline(repo.ID); len(base) != 0 {
		t.Fatalf("backfill recorded a baseline: %v", base)
	}
}

func TestPollSkipsUnwatchedRepos(t *testing.T) {
	w, repo, _ := newWatchFixture(t, "")
	if _, err := w.db.SetRepoWatch(repo.ID, false, "", false); err != nil {
		t.Fatal(err)
	}
	w.PollAll(context.Background())
	if rows, _ := w.db.ListDeploys(db.DeployFilter{}); len(rows) != 0 {
		t.Fatalf("unwatched repo deployed: %d rows", len(rows))
	}
}

func TestMatchBranch(t *testing.T) {
	cases := []struct {
		patterns string
		branch   string
		want     bool
	}{
		{"", "anything", true},
		{"main", "main", true},
		{"main", "master", false},
		{"main,develop", "develop", true},
		{"release/*", "release/1.0", true},
		{"release/*", "release/1.0/hotfix", false}, // globs don't cross '/'
		{"release/*", "main", false},
		{" main , develop ", "develop", true},
		// negation: `!` excludes and always wins.
		{"!main", "main", false},
		{"!main", "feature", true},
		{"!release/*", "release/1.0", false},
		{"!release/*", "main", true},
		{"!main,!develop", "feature", true}, // excludes only ⇒ everything else
		{"release/*,!release/experimental", "release/1.0", true},
		{"release/*,!release/experimental", "release/experimental", false}, // exclude beats include
		{"release/*,!release/experimental", "main", false},                 // include present, no match
		// `**` spans separators; a plain `*` does not.
		{"gh-readonly-queue/*", "gh-readonly-queue/main/pr-42-abc", false},      // one `*` can't cross `/`
		{"gh-readonly-queue/**", "gh-readonly-queue/main/pr-42-abc", true},      // base = main
		{"gh-readonly-queue/**", "gh-readonly-queue/release/1.0/pr-7-de", true}, // base with a slash
		{"gh-readonly-queue/**", "gh-readonly-queue", true},                     // ** matches zero segments
		{"gh-readonly-queue/**", "feature/x", false},
		{"!gh-readonly-queue/**", "gh-readonly-queue/main/pr-42-abc", false}, // the merge-queue exclusion
		{"!gh-readonly-queue/**", "feature/x", true},
		{"**/pr-*", "gh-readonly-queue/main/pr-42-abc", true}, // ** as a prefix
		{"**", "anything/at/all", true},
	}
	for _, c := range cases {
		if got := MatchBranch(SplitPatterns(c.patterns), c.branch); got != c.want {
			t.Errorf("MatchBranch(%q, %q) = %v, want %v", c.patterns, c.branch, got, c.want)
		}
	}
}

func TestValidatePatterns(t *testing.T) {
	if got, err := ValidatePatterns(" main , release/* ,"); err != nil || got != "main,release/*" {
		t.Errorf("ValidatePatterns = %q, %v", got, err)
	}
	if got, err := ValidatePatterns(""); err != nil || got != "" {
		t.Errorf("ValidatePatterns(empty) = %q, %v", got, err)
	}
	if _, err := ValidatePatterns("release/["); err == nil {
		t.Error("bad pattern accepted")
	}
	if got, err := ValidatePatterns(" !main , release/* "); err != nil || got != "!main,release/*" {
		t.Errorf("ValidatePatterns(negation) = %q, %v", got, err)
	}
	if _, err := ValidatePatterns("!"); err == nil {
		t.Error("bare ! accepted")
	}
	if _, err := ValidatePatterns("!release/["); err == nil {
		t.Error("bad negated pattern accepted")
	}
	if got, err := ValidatePatterns("!gh-readonly-queue/**"); err != nil || got != "!gh-readonly-queue/**" {
		t.Errorf("ValidatePatterns(doublestar) = %q, %v", got, err)
	}
	if _, err := ValidatePatterns("gh-readonly-queue/**/["); err == nil {
		t.Error("bad segment after ** accepted")
	}
}
