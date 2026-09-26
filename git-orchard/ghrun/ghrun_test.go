package ghrun

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeClock is a clock that Sleep advances instantly.
type fakeClock struct{ now time.Time }

func (c *fakeClock) opts() WatchOptions {
	return WatchOptions{
		Interval: 5 * time.Second,
		Grace:    60 * time.Second,
		Now:      func() time.Time { return c.now },
		Sleep: func(ctx context.Context, d time.Duration) error {
			c.now = c.now.Add(d)
			return ctx.Err()
		},
	}
}

// script lists polls[i] on the ith poll, and the last one after that.
func script(t *testing.T, polls ...[]Run) Lister {
	i := 0
	return func(_ context.Context, repo, sha string) ([]Run, error) {
		if repo != "o/r" || sha != "abc" {
			t.Fatalf("listed %s at %s", repo, sha)
		}
		runs := polls[min(i, len(polls)-1)]
		i++
		return runs, nil
	}
}

func run(id int64, status, conclusion string) Run {
	return Run{ID: id, Workflow: "Release", HeadBranch: "v1.0.0", HeadSHA: "abc", Status: status, Conclusion: conclusion, URL: "u"}
}

func TestWatchSucceeds(t *testing.T) {
	c := &fakeClock{}
	var reports []string
	runs, err := Watch(context.Background(), script(t,
		nil,
		[]Run{run(1, "queued", "")},
		[]Run{run(1, "in_progress", "")},
		[]Run{run(1, "in_progress", "")},
		[]Run{run(1, "completed", "success")},
	), "o/r", "v1.0.0", "abc", c.opts(), func(r Run) { reports = append(reports, r.State()) })
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Errorf("got %d runs, want 1", len(runs))
	}
	if want := []string{"queued", "in_progress", "success"}; !equal(reports, want) {
		t.Errorf("reported %v, want %v", reports, want)
	}
}

func TestWatchIgnoresOtherPushes(t *testing.T) {
	c := &fakeClock{}
	ci := run(2, "completed", "failure")
	ci.HeadBranch = "master" // the branch push of the same commit
	other := run(3, "completed", "failure")
	other.HeadSHA = "def"
	runs, err := Watch(context.Background(), script(t,
		[]Run{ci, other, run(1, "completed", "success")},
	), "o/r", "v1.0.0", "abc", c.opts(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].ID != 1 {
		t.Errorf("got %+v, want run 1", runs)
	}
}

func TestWatchNoRuns(t *testing.T) {
	c := &fakeClock{}
	runs, err := Watch(context.Background(), script(t, nil), "o/r", "v1.0.0", "abc", c.opts(), nil)
	if err != nil || runs != nil {
		t.Fatalf("got %v, %v; want no runs", runs, err)
	}
	if c.now.Sub(time.Time{}) != 60*time.Second {
		t.Errorf("gave up after %v, want the 60s grace", c.now.Sub(time.Time{}))
	}
}

func TestWatchWaitsForAll(t *testing.T) {
	c := &fakeClock{}
	pypi := run(2, "completed", "failure")
	pypi.Workflow = "PyPI"
	_, err := Watch(context.Background(), script(t,
		[]Run{run(1, "in_progress", ""), pypi},
		[]Run{run(1, "completed", "success"), pypi},
	), "o/r", "v1.0.0", "abc", c.opts(), nil)
	var failed *FailedError
	if !errors.As(err, &failed) || len(failed.Runs) != 1 || failed.Runs[0].ID != 2 {
		t.Fatalf("got %v, want run 2 failed", err)
	}
	if c.now.Sub(time.Time{}) != 5*time.Second {
		t.Errorf("finished after %v, want one poll interval, once run 1 finished", c.now.Sub(time.Time{}))
	}
}

func TestWatchCanceled(t *testing.T) {
	c := &fakeClock{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Watch(ctx, script(t, []Run{run(1, "in_progress", "")}), "o/r", "v1.0.0", "abc", c.opts(), nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}

func TestParseGitHubRepo(t *testing.T) {
	for url, want := range map[string]string{
		"git@github.com:jmelahman/monorepo.git":                  "jmelahman/monorepo",
		"https://github.com/jmelahman/monorepo":                  "jmelahman/monorepo",
		"https://github.com/jmelahman/monorepo.git/":             "jmelahman/monorepo",
		"ssh://git@github.com/jmelahman/jmelahman.github.io.git": "jmelahman/jmelahman.github.io",
		"git@gitlab.com:jmelahman/monorepo.git":                  "",
	} {
		owner, repo, ok := ParseGitHubRepo(url)
		got := ""
		if ok {
			got = owner + "/" + repo
		}
		if got != want {
			t.Errorf("ParseGitHubRepo(%q) = %q, want %q", url, got, want)
		}
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
