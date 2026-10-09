package history_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jmelahman/git-orchard/git"
	"github.com/jmelahman/git-orchard/history"
	"github.com/jmelahman/git-orchard/internal/gittest"
)

// newRepo makes a repository with one ordinary commit and chdirs into it,
// since the reader runs git in the working directory.
func newRepo(t *testing.T) git.Repo {
	t.Helper()
	gittest.Isolate(t)
	repo := git.Repo{Dir: t.TempDir()}
	run(t, repo, "init", "--quiet", "--initial-branch=master")
	run(t, repo, "commit", "--quiet", "--allow-empty", "-m", "Start")
	t.Chdir(repo.Dir)
	return repo
}

func run(t *testing.T, repo git.Repo, args ...string) {
	t.Helper()
	if _, err := repo.Output(args...); err != nil {
		t.Fatal(err)
	}
}

func TestGetSubtreesFromHistory(t *testing.T) {
	repo := newRepo(t)
	// What git subtree add writes.
	message := filepath.Join(t.TempDir(), "message")
	if err := os.WriteFile(message, []byte(`Add 'vendor/example/' from commit 'def456'

git-subtree-dir: vendor/example
git-subtree-mainline: abc123
git-subtree-split: def456
`), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, repo, "commit", "--quiet", "--allow-empty", "-F", message)

	subtrees, err := history.NewGitHistoryReader().GetSubtreesFromHistory()
	if err != nil {
		t.Fatal(err)
	}
	want := history.SubtreeHistoryInfo{
		Prefix:      "vendor/example",
		LastCommit:  "abc123",
		LastMessage: "Add 'vendor/example/' from commit 'def456'",
	}
	if got, ok := subtrees["vendor/example"]; !ok || got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	// The reader also files the subject line, read before any trailer has
	// named a prefix, under the empty prefix; its callers have to skip that.
	// Nothing else should be there.
	for prefix := range subtrees {
		if prefix != "vendor/example" && prefix != "" {
			t.Errorf("unexpected subtree %q", prefix)
		}
	}
}

func TestGetSubtreesFromHistory_NoSubtrees(t *testing.T) {
	newRepo(t)
	subtrees, err := history.NewGitHistoryReader().GetSubtreesFromHistory()
	if err != nil {
		t.Fatal(err)
	}
	if len(subtrees) != 0 {
		t.Errorf("got %+v, want none", subtrees)
	}
}
