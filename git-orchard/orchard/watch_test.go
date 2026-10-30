package orchard

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jmelahman/git-orchard/ghrun"
)

// watchFixture releases v1.0.0 of a subtree whose upstream, as far as
// WatchRelease can tell, is on GitHub.
func watchFixture(t *testing.T) (*fixture, string) {
	t.Helper()
	f := newFixture(t)
	const github = "https://github.com/owner/foo.git"
	f.git(f.mono, "config", "url."+f.bare+".insteadOf", github)
	f.commit(f.mono, "tools/foo/main.go", "package main\n", "Add main")
	tag, err := f.orchard.Release(f.subtree(), "v1.0.0", ReleaseOptions{Rev: "HEAD", Upstream: true})
	if err != nil {
		t.Fatal(err)
	}
	return f, tag
}

func noSleep(context.Context, time.Duration) error { return nil }

func TestWatchRelease(t *testing.T) {
	f, tag := watchFixture(t)
	s := f.subtree()
	s.Remote = "https://github.com/owner/foo.git"
	commit, err := f.orchard.UpstreamTag(s, "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	calls := 0
	err = f.orchard.WatchRelease(context.Background(), s, "v1.0.0", tag, WatchOptions{
		Upstream: true,
		Output:   &out,
		Watch:    ghrun.WatchOptions{Sleep: noSleep},
		List: func(_ context.Context, repo, sha string) ([]ghrun.Run, error) {
			if repo != "owner/foo" || sha != commit {
				t.Errorf("listed %s at %s", repo, sha)
			}
			calls++
			r := ghrun.Run{ID: 1, Workflow: "Release", HeadBranch: "v1.0.0", HeadSHA: commit, Status: "in_progress", URL: "u"}
			if calls > 1 {
				r.Status, r.Conclusion = "completed", "success"
			}
			return []ghrun.Run{r}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Watching owner/foo's runs for v1.0.0...\n",
		"  Release: in_progress (u)\n",
		"  Release: success\n",
		"Published " + tag + ".\n",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
}

func TestWatchReleaseFailedRun(t *testing.T) {
	f, tag := watchFixture(t)
	s := f.subtree()
	s.Remote = "https://github.com/owner/foo.git"
	commit, err := f.orchard.UpstreamTag(s, "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	err = f.orchard.WatchRelease(context.Background(), s, "v1.0.0", tag, WatchOptions{
		Upstream: true,
		Output:   &bytes.Buffer{},
		Watch:    ghrun.WatchOptions{Sleep: noSleep},
		List: func(context.Context, string, string) ([]ghrun.Run, error) {
			return []ghrun.Run{{ID: 1, HeadBranch: "v1.0.0", HeadSHA: commit, Status: "completed", Conclusion: "failure"}}, nil
		},
	})
	if _, ok := errors.AsType[*ghrun.FailedError](err); !ok {
		t.Fatalf("got %v, want a FailedError", err)
	}
}

func TestWatchReleaseNotOnGitHub(t *testing.T) {
	f, tag := watchFixture(t)

	var out bytes.Buffer
	err := f.orchard.WatchRelease(context.Background(), f.subtree(), "v1.0.0", tag, WatchOptions{Upstream: true, Output: &out})
	if err != nil {
		t.Fatal(err)
	}
	if want := "tools/foo's upstream isn't on GitHub; not watching it.\n"; out.String() != want {
		t.Errorf("got %q, want %q", out.String(), want)
	}
}

func TestWatchReleaseNoUpstreamTag(t *testing.T) {
	f := newFixture(t)
	now := time.Now()
	err := f.orchard.WatchRelease(context.Background(), f.subtree(), "v9.9.9", "tools/foo/v9.9.9", WatchOptions{
		Upstream: true,
		Output:   &bytes.Buffer{},
		Watch: ghrun.WatchOptions{
			Grace: time.Minute,
			Sleep: noSleep,
			Now: func() time.Time {
				now = now.Add(time.Minute)
				return now
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "has no tag v9.9.9") {
		t.Fatalf("got %v", err)
	}
}
