package orchard

import (
	"context"
	"testing"

	"github.com/jmelahman/git-orchard/ghrun"
)

func TestCITargets(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.git(f.mono, "remote", "add", "origin", "https://github.com/owner/mono.git")
	f.git(f.mono, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/trunk")

	targets, err := f.orchard.CITargets(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 {
		t.Fatalf("targets = %+v", targets)
	}
	if m := targets[0]; m.Prefix != MonorepoPrefix || m.Repo != "owner/mono" || m.Branch != "trunk" {
		t.Errorf("monorepo = %+v", m)
	}
	if s := targets[1]; s.Prefix != "tools/foo" || s.Repo != "" || s.Branch != "master" {
		t.Errorf("subtree = %+v", s)
	}

	// Naming prefixes leaves the monorepo out, unless it's named too.
	if targets, err = f.orchard.CITargets(ctx, []string{"tools/foo"}); err != nil || len(targets) != 1 {
		t.Errorf("tools/foo: %+v, %v", targets, err)
	}
	if targets, err = f.orchard.CITargets(ctx, []string{MonorepoPrefix}); err != nil || len(targets) != 1 || targets[0].Prefix != MonorepoPrefix {
		t.Errorf(".: %+v, %v", targets, err)
	}
	if _, err = f.orchard.CITargets(ctx, []string{"nope"}); err == nil {
		t.Error("an unknown prefix was accepted")
	}
}

// With nothing on GitHub, or nothing that can be checked, gh isn't needed.
func TestCheckCIWithoutGH(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	var started, done []string
	results := CheckCI(context.Background(), []CITarget{
		{Prefix: "local", Branch: "master"},
		{Prefix: "broken", Repo: "o/broken", Error: "can't tell"},
	}, CICheckOptions{
		Jobs:    1,
		OnStart: func(p string) { started = append(started, p) },
		OnDone:  func(p string) { done = append(done, p) },
	})
	if len(results) != 2 || results[0].Error != "" || len(results[0].Runs) != 0 || results[1].Error != "can't tell" {
		t.Errorf("results = %+v", results)
	}
	if len(started) != 2 || len(done) != 2 {
		t.Errorf("started %v, done %v", started, done)
	}
}

func TestCIRunFailing(t *testing.T) {
	failed := ghrun.Run{Status: "completed", Conclusion: "failure"}
	if !(CIRun{Run: failed}).Failing() {
		t.Error("a failed run isn't failing")
	}
	if (CIRun{Run: failed, Stale: "gone"}).Failing() {
		t.Error("a stale failure is failing")
	}
}
