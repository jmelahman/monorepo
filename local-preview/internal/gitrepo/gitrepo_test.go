package gitrepo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newSourceRepo creates a working git repo with one commit containing
// web/index.html and main.go.
func newSourceRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runTestGit(t, dir, "init", "-q", "-b", "main")
	writeFile(t, dir, "web/index.html", "<html>v1</html>")
	writeFile(t, dir, "main.go", "package main")
	runTestGit(t, dir, "add", "-A")
	runTestGit(t, dir, "commit", "-q", "-m", "initial")
	return dir
}

func TestValidateName(t *testing.T) {
	for _, ok := range []string{"demo", "my-app", "a", "a1", "x0-9y"} {
		if err := ValidateName(ok); err != nil {
			t.Errorf("ValidateName(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", "My-App", "has.dot", "-lead", "trail-", "under_score", "spaced name"} {
		if err := ValidateName(bad); err == nil {
			t.Errorf("ValidateName(%q) = nil, want error", bad)
		}
	}
}

func TestAddResolveFetch(t *testing.T) {
	ctx := context.Background()
	src := newSourceRepo(t)
	mgr := NewManager(filepath.Join(t.TempDir(), "repos"))

	repo, err := mgr.Add(ctx, "demo", src, nil)
	if err != nil {
		t.Fatal(err)
	}

	head := runTestGit(t, src, "rev-parse", "HEAD")
	sha, err := repo.ResolveRef(ctx, "main")
	if err != nil {
		t.Fatal(err)
	}
	if sha != head {
		t.Fatalf("ResolveRef(main) = %s, want %s", sha, head)
	}

	// A commit made after the clone resolves via the fetch-and-retry path.
	writeFile(t, src, "web/index.html", "<html>v2</html>")
	runTestGit(t, src, "commit", "-qam", "v2")
	newHead := runTestGit(t, src, "rev-parse", "HEAD")
	sha, err = repo.ResolveRef(ctx, newHead)
	if err != nil {
		t.Fatal(err)
	}
	if sha != newHead {
		t.Fatalf("ResolveRef(new sha) = %s, want %s", sha, newHead)
	}

	if _, err := repo.ResolveRef(ctx, "no-such-ref"); err == nil {
		t.Fatal("ResolveRef of unknown ref should fail")
	}
}

// Fetch must report whether any ref moved: quiet polls skip graph work on
// its word, so a false negative would strand new commits and a false
// positive would defeat the optimization.
func TestFetchReportsRefChanges(t *testing.T) {
	ctx := context.Background()
	src := newSourceRepo(t)
	mgr := NewManager(filepath.Join(t.TempDir(), "repos"))
	repo, err := mgr.Add(ctx, "demo", src, nil)
	if err != nil {
		t.Fatal(err)
	}

	if changed, err := repo.Fetch(ctx); err != nil || changed {
		t.Fatalf("fetch right after clone: changed=%v err=%v, want false", changed, err)
	}

	writeFile(t, src, "web/index.html", "<html>v2</html>")
	runTestGit(t, src, "commit", "-qam", "v2")
	if changed, err := repo.Fetch(ctx); err != nil || !changed {
		t.Fatalf("fetch after new commit: changed=%v err=%v, want true", changed, err)
	}
	if changed, err := repo.Fetch(ctx); err != nil || changed {
		t.Fatalf("fetch after syncing: changed=%v err=%v, want false", changed, err)
	}

	runTestGit(t, src, "branch", "scrap")
	if changed, err := repo.Fetch(ctx); err != nil || !changed {
		t.Fatalf("fetch after new branch: changed=%v err=%v, want true", changed, err)
	}
	runTestGit(t, src, "branch", "-D", "scrap")
	if changed, err := repo.Fetch(ctx); err != nil || !changed {
		t.Fatalf("fetch after branch deletion: changed=%v err=%v, want true", changed, err)
	}
	// The prune must actually have landed for the mirror to read as synced.
	if changed, err := repo.Fetch(ctx); err != nil || changed {
		t.Fatalf("fetch after prune: changed=%v err=%v, want false", changed, err)
	}
}

// Add must replace a mirror left on disk by an earlier registration — a
// deletion whose cleanup failed, or a delete that raced an in-flight fetch —
// rather than refuse the name forever.
func TestAddReplacesLeftoverMirror(t *testing.T) {
	ctx := context.Background()
	mgr := NewManager(filepath.Join(t.TempDir(), "repos"))

	if _, err := mgr.Add(ctx, "demo", newSourceRepo(t), nil); err != nil {
		t.Fatal(err)
	}
	src2 := newSourceRepo(t)
	repo, err := mgr.Add(ctx, "demo", src2, nil)
	if err != nil {
		t.Fatalf("Add over leftover mirror: %v", err)
	}
	sha, err := repo.ResolveRef(ctx, "main")
	if err != nil {
		t.Fatal(err)
	}
	if want := runTestGit(t, src2, "rev-parse", "HEAD"); sha != want {
		t.Fatalf("mirror still tracks old source: main = %s, want %s", sha, want)
	}

	// A partial leftover (a racing fetch re-created only a subdirectory) is
	// not even a repository; it too must be replaced.
	writeFile(t, filepath.Dir(repo.Path), "partial.git/refs/heads/main", "junk")
	repo, err = mgr.Add(ctx, "partial", src2, nil)
	if err != nil {
		t.Fatalf("Add over partial leftover: %v", err)
	}
	if _, err := repo.ResolveRef(ctx, "main"); err != nil {
		t.Fatal(err)
	}

	// A failed clone must not destroy the healthy mirror already in place.
	if _, err := mgr.Add(ctx, "demo", filepath.Join(t.TempDir(), "nope"), nil); err == nil {
		t.Fatal("Add from missing source should fail")
	}
	if _, err := mgr.Open("demo").ResolveRef(ctx, "main"); err != nil {
		t.Fatalf("failed Add clobbered existing mirror: %v", err)
	}
}

func TestCommitMetaAndBranches(t *testing.T) {
	ctx := context.Background()
	src := newSourceRepo(t)
	first := runTestGit(t, src, "rev-parse", "HEAD")
	writeFile(t, src, "a.txt", "a")
	runTestGit(t, src, "add", "-A")
	runTestGit(t, src, "commit", "-qm", "second")
	head := runTestGit(t, src, "rev-parse", "HEAD")

	mgr := NewManager(filepath.Join(t.TempDir(), "repos"))
	repo, err := mgr.Add(ctx, "demo", src, nil)
	if err != nil {
		t.Fatal(err)
	}

	meta, err := repo.CommitMeta(ctx, head)
	if err != nil {
		t.Fatal(err)
	}
	if meta.AuthorName != "test" || meta.AuthorEmail != "test@example.com" {
		t.Fatalf("CommitMeta = %+v", meta)
	}
	if _, err := repo.CommitMeta(ctx, strings.Repeat("0", 40)); err == nil {
		t.Fatal("CommitMeta of unknown sha should fail")
	}

	if !repo.IsBranch(ctx, "main") {
		t.Fatal("IsBranch(main) = false")
	}
	if repo.IsBranch(ctx, "no-such-branch") {
		t.Fatal("IsBranch(no-such-branch) = true")
	}

	branches, err := repo.BranchesPointingAt(ctx, head)
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 1 || branches[0] != "main" {
		t.Fatalf("BranchesPointingAt(head) = %v, want [main]", branches)
	}
	// The first commit is no branch's tip anymore.
	branches, err = repo.BranchesPointingAt(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 0 {
		t.Fatalf("BranchesPointingAt(first) = %v, want none", branches)
	}
}

func TestUnreachableSHAs(t *testing.T) {
	ctx := context.Background()
	src := newSourceRepo(t)
	base := runTestGit(t, src, "rev-parse", "HEAD")

	// A second commit advances main; base is now an ancestor of the tip.
	writeFile(t, src, "a.txt", "a")
	runTestGit(t, src, "add", "-A")
	runTestGit(t, src, "commit", "-qm", "second")
	mainTip := runTestGit(t, src, "rev-parse", "HEAD")

	// An unmerged feature branch off base.
	runTestGit(t, src, "checkout", "-qb", "feature", base)
	writeFile(t, src, "f.txt", "f")
	runTestGit(t, src, "add", "-A")
	runTestGit(t, src, "commit", "-qm", "feature work")
	featTip := runTestGit(t, src, "rev-parse", "HEAD")
	runTestGit(t, src, "checkout", "-q", "main")

	mgr := NewManager(filepath.Join(t.TempDir(), "repos"))
	repo, err := mgr.Add(ctx, "demo", src, nil)
	if err != nil {
		t.Fatal(err)
	}

	candidates := []string{base, mainTip, featTip}

	// While feature survives, nothing is unreachable: base and featTip are
	// both reachable from a tip.
	got, err := repo.UnreachableSHAs(ctx, candidates, []string{mainTip, featTip})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("with all tips: unreachable = %v, want none", got)
	}

	// Drop feature's tip. featTip falls out of history; base stays (still an
	// ancestor of main), mainTip stays (it is the tip).
	got, err = repo.UnreachableSHAs(ctx, candidates, []string{mainTip})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != featTip {
		t.Fatalf("after feature deleted: unreachable = %v, want [%s]", got, featTip)
	}

	// No surviving tips at all → every candidate is unreachable.
	got, err = repo.UnreachableSHAs(ctx, candidates, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(candidates) {
		t.Fatalf("with no tips: unreachable = %v, want all %v", got, candidates)
	}

	// A candidate the mirror doesn't have is skipped, not an error.
	got, err = repo.UnreachableSHAs(ctx, []string{strings.Repeat("0", 40)}, []string{mainTip})
	if err != nil {
		t.Fatalf("missing candidate errored: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("missing candidate: unreachable = %v, want none", got)
	}
}

func TestLsTreeReadFileArchive(t *testing.T) {
	ctx := context.Background()
	src := newSourceRepo(t)
	mgr := NewManager(filepath.Join(t.TempDir(), "repos"))
	repo, err := mgr.Add(ctx, "demo", src, nil)
	if err != nil {
		t.Fatal(err)
	}
	sha, err := repo.ResolveRef(ctx, "main")
	if err != nil {
		t.Fatal(err)
	}

	entries, err := repo.LsTree(ctx, sha, "")
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, e := range entries {
		if e.Type != "blob" || e.OID == "" || e.Mode == "" {
			t.Fatalf("bad entry: %+v", e)
		}
		paths = append(paths, e.Path)
	}
	if strings.Join(paths, ",") != "main.go,web/index.html" {
		t.Fatalf("paths = %v", paths)
	}

	sub, err := repo.LsTree(ctx, sha, "web")
	if err != nil {
		t.Fatal(err)
	}
	if len(sub) != 1 || sub[0].Path != "web/index.html" {
		t.Fatalf("subtree entries = %+v", sub)
	}

	content, err := repo.ReadFile(ctx, sha, "web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "<html>v1</html>" {
		t.Fatalf("ReadFile = %q", content)
	}
	if _, err := repo.ReadFile(ctx, sha, "missing.txt"); err == nil {
		t.Fatal("ReadFile of missing path should fail")
	}

	dest := filepath.Join(t.TempDir(), "extract")
	if err := repo.Archive(ctx, sha, dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "package main" {
		t.Fatalf("extracted main.go = %q", got)
	}
}

// TestArchiveRejectsEscapingSymlinks pins the C1 hardening: a committed symlink
// whose target is an absolute host path or a ../ escape must not be recreated
// pointing outside the extraction root, or an unauthenticated preview visitor
// could read host files through the frontend file server / artifact download.
// A safe in-tree symlink must still extract normally.
func TestArchiveRejectsEscapingSymlinks(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name       string
		linkTarget string
		reject     bool
	}{
		{"absolute", "/etc/passwd", true},
		{"dotdot-escape", "../../../../etc/passwd", true},
		{"in-tree", "real.txt", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := t.TempDir()
			runTestGit(t, src, "init", "-q", "-b", "main")
			writeFile(t, src, "real.txt", "safe content")
			if err := os.Symlink(tc.linkTarget, filepath.Join(src, "leak")); err != nil {
				t.Fatal(err)
			}
			runTestGit(t, src, "add", "-A")
			runTestGit(t, src, "commit", "-q", "-m", "symlink")

			mgr := NewManager(filepath.Join(t.TempDir(), "repos"))
			repo, err := mgr.Add(ctx, "demo", src, nil)
			if err != nil {
				t.Fatal(err)
			}
			sha, err := repo.ResolveRef(ctx, "main")
			if err != nil {
				t.Fatal(err)
			}

			dest := filepath.Join(t.TempDir(), "extract")
			err = repo.Archive(ctx, sha, dest)
			if tc.reject {
				if err == nil {
					t.Fatal("Archive should reject an escaping symlink")
				}
				return
			}
			if err != nil {
				t.Fatalf("Archive of a safe symlink failed: %v", err)
			}
			got, err := os.ReadFile(filepath.Join(dest, "leak"))
			if err != nil {
				t.Fatalf("read in-tree symlink: %v", err)
			}
			if string(got) != "safe content" {
				t.Fatalf("in-tree symlink resolved to %q", got)
			}
		})
	}
}

// TestLsTreeMatchesGit pins LsTree to real `git ls-tree -r` output — order,
// modes, and OIDs feed content-address hashes (see hashkey), so any drift
// silently invalidates every cached build. The fixture exercises git's tree
// sort quirk (directories order as "name/": a-b < a.txt < a/) plus
// executable and symlink modes.
func TestLsTreeMatchesGit(t *testing.T) {
	ctx := context.Background()
	src := t.TempDir()
	runTestGit(t, src, "init", "-q", "-b", "main")
	writeFile(t, src, "a.txt", "file")
	writeFile(t, src, "a-b", "dash sorts before dot")
	writeFile(t, src, "a/nested.txt", "dir sorts after both")
	writeFile(t, src, "tool.sh", "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(src, "tool.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("a.txt", filepath.Join(src, "link")); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, src, "add", "-A")
	runTestGit(t, src, "commit", "-q", "-m", "tree shapes")

	mgr := NewManager(filepath.Join(t.TempDir(), "repos"))
	repo, err := mgr.Add(ctx, "demo", src, nil)
	if err != nil {
		t.Fatal(err)
	}
	sha, err := repo.ResolveRef(ctx, "main")
	if err != nil {
		t.Fatal(err)
	}

	entries, err := repo.LsTree(ctx, sha, "")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, fmt.Sprintf("%s %s %s\t%s", e.Mode, e.Type, e.OID, e.Path))
	}
	want := strings.Split(runTestGit(t, src, "ls-tree", "-r", "--full-tree", sha), "\n")
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("LsTree diverges from git ls-tree -r:\ngot:\n%s\nwant:\n%s",
			strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestAddFromLinkedWorktree registers a linked worktree path (.git is a
// pointer file, refs live in the main clone) as a repo source.
func TestAddFromLinkedWorktree(t *testing.T) {
	ctx := context.Background()
	src := newSourceRepo(t)
	wt := filepath.Join(t.TempDir(), "wt")
	runTestGit(t, src, "worktree", "add", "-q", "-b", "feature", wt)
	writeFile(t, wt, "feature.txt", "from the worktree")
	runTestGit(t, wt, "add", "-A")
	runTestGit(t, wt, "commit", "-q", "-m", "feature commit")
	featureHead := runTestGit(t, wt, "rev-parse", "HEAD")

	mgr := NewManager(filepath.Join(t.TempDir(), "repos"))
	repo, err := mgr.Add(ctx, "demo", wt, nil)
	if err != nil {
		t.Fatal(err)
	}
	sha, err := repo.ResolveRef(ctx, "feature")
	if err != nil {
		t.Fatal(err)
	}
	if sha != featureHead {
		t.Fatalf("ResolveRef(feature) = %s, want %s", sha, featureHead)
	}
}

func TestFirstParentAncestry(t *testing.T) {
	ctx := context.Background()
	src := newSourceRepo(t)
	c1 := runTestGit(t, src, "rev-parse", "HEAD")

	writeFile(t, src, "a.txt", "a")
	runTestGit(t, src, "add", "-A")
	runTestGit(t, src, "commit", "-qm", "c2")
	c2 := runTestGit(t, src, "rev-parse", "HEAD")

	// Merge a side branch so first-parent order is observable.
	runTestGit(t, src, "checkout", "-qb", "side", c1)
	writeFile(t, src, "b.txt", "b")
	runTestGit(t, src, "add", "-A")
	runTestGit(t, src, "commit", "-qm", "side")
	side := runTestGit(t, src, "rev-parse", "HEAD")
	runTestGit(t, src, "checkout", "-q", "main")
	runTestGit(t, src, "merge", "-q", "--no-ff", "-m", "merge side", "side")
	merge := runTestGit(t, src, "rev-parse", "HEAD")

	mgr := NewManager(filepath.Join(t.TempDir(), "repos"))
	repo, err := mgr.Add(ctx, "demo", src, nil)
	if err != nil {
		t.Fatal(err)
	}

	anc, err := repo.FirstParentAncestry(ctx, merge, 10)
	if err != nil {
		t.Fatal(err)
	}
	// First-parent walk from the merge: c2 then c1, never the side branch.
	if len(anc) != 2 || anc[0] != c2 || anc[1] != c1 {
		t.Fatalf("ancestry = %v, want [%s %s]", anc, c2, c1)
	}
	for _, sha := range anc {
		if sha == side {
			t.Fatal("first-parent ancestry must not include the side branch")
		}
	}

	limited, err := repo.FirstParentAncestry(ctx, merge, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(limited) != 1 || limited[0] != c2 {
		t.Fatalf("limited ancestry = %v", limited)
	}
}
