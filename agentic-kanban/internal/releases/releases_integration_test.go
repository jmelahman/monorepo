package releases_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmelahman/git-orchard/config"
	"github.com/jmelahman/git-orchard/ghrun"
	"github.com/jmelahman/git-orchard/orchard"

	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/kanbantest"
	"github.com/jmelahman/kanban/internal/releases"
)

const prefix = "tools/foo"

// world is a server with one board on a monorepo that has an origin and a
// subtree whose upstream is in sync.
type world struct {
	t        *testing.T
	env      *kanbantest.Env
	origin   string
	upstream string
}

func newWorld(t *testing.T) *world {
	t.Helper()
	env := kanbantest.New(t)
	dir := t.TempDir()
	w := &world{t: t, env: env, origin: filepath.Join(dir, "origin.git"), upstream: filepath.Join(dir, "foo.git")}

	kanbantest.MustGit(t, "", "init", "-q", "--bare", "-b", "main", w.origin)
	kanbantest.MustGit(t, "", "init", "-q", "--bare", "-b", "master", w.upstream)
	work := filepath.Join(dir, "foo")
	kanbantest.MustGit(t, "", "init", "-q", "-b", "master", work)
	if err := os.WriteFile(filepath.Join(work, "README"), []byte("foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	kanbantest.MustGit(t, work, "add", "README")
	kanbantest.MustGit(t, work, "-c", "user.name=T", "-c", "user.email=t@example.com", "commit", "-q", "-m", "Start foo")
	kanbantest.MustGit(t, work, "push", "-q", w.upstream, "master")

	o, err := orchard.Open(env.RepoPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := o.Add(config.Subtree{Prefix: prefix, Remote: w.upstream, Branch: "master"}, ""); err != nil {
		t.Fatal(err)
	}
	kanbantest.MustGit(t, env.RepoPath, "add", o.Config.Manifest)
	kanbantest.MustGit(t, env.RepoPath, "commit", "-q", "-m", "List tools/foo")
	kanbantest.MustGit(t, env.RepoPath, "remote", "add", "origin", w.origin)
	w.change("main.go")

	board := &db.Board{Name: "Mono", Slug: "mono", RepoPath: env.RepoPath, BaseBranch: "main", GitAuthorName: "Release Bot", GitAuthorEmail: "bot@example.com"}
	if err := env.Store.CreateBoard(context.Background(), board); err != nil {
		t.Fatal(err)
	}
	return w
}

// commit changes the subtree and pushes the change to origin only, so the
// mirror has yet to publish it.
func (w *world) commit(name string) {
	w.t.Helper()
	w.commitTitled(name, "Change "+name)
}

func (w *world) commitTitled(name, title string) {
	w.t.Helper()
	path := filepath.Join(w.env.RepoPath, prefix, name)
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		w.t.Fatal(err)
	}
	kanbantest.MustGit(w.t, w.env.RepoPath, "add", "--", path)
	kanbantest.MustGit(w.t, w.env.RepoPath, "commit", "-q", "-m", title)
	kanbantest.MustGit(w.t, w.env.RepoPath, "push", "-q", "origin", "main")
}

// change is commit followed by the mirror publishing it.
func (w *world) change(name string) {
	w.t.Helper()
	w.commit(name)
	o, err := orchard.Open(w.env.RepoPath)
	if err != nil {
		w.t.Fatal(err)
	}
	o.NoVerify = true
	sub, _ := o.Config.Lookup(prefix)
	if err := o.Push(sub, "HEAD", orchard.PushOptions{Output: io.Discard}); err != nil {
		w.t.Fatal(err)
	}
}

func (w *world) do(method, path string, body any, headers ...string) (int, []byte) {
	w.t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			w.t.Fatal(err)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, w.env.Server.URL+path, reader)
	if err != nil {
		w.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		w.t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		w.t.Fatal(err)
	}
	return resp.StatusCode, data
}

func decode[T any](t *testing.T, data []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("decode %s: %v", data, err)
	}
	return v
}

func (w *world) preview() releases.Preview {
	w.t.Helper()
	q := url.Values{"repo": {w.env.RepoPath}, "prefix": {prefix}, "bump": {"minor"}}
	code, data := w.do("GET", "/api/releases/preview?"+q.Encode(), nil)
	if code != 200 {
		w.t.Fatalf("preview: %d %s", code, data)
	}
	return decode[releases.Preview](w.t, data)
}

func (w *world) runBody(p releases.Preview) map[string]any {
	return map[string]any{"repo": w.env.RepoPath, "prefix": prefix, "bump": "minor", "version": p.Version, "rev": p.Rev}
}

// output reads a run's stream to its end, failing if it doesn't end.
func (w *world) output(id int64) string {
	w.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/release-runs/%d/output", w.env.Server.URL, id), nil)
	if err != nil {
		w.t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		w.t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		w.t.Fatalf("stream didn't end: %v", err)
	}
	if !strings.Contains(string(data), "event: end") {
		w.t.Fatalf("stream lacks its end:\n%s", data)
	}
	return string(data)
}

func (w *world) run(id int64) *db.ReleaseRun {
	w.t.Helper()
	run, err := w.env.Store.GetReleaseRun(context.Background(), id)
	if err != nil {
		w.t.Fatal(err)
	}
	return run
}

func TestReminders(t *testing.T) {
	w := newWorld(t)
	repo := w.env.RepoPath

	code, data := w.do("POST", "/api/release-reminders", map[string]any{"repo_path": repo, "prefix": prefix, "bump": "minor", "note": "has the new X"})
	if code != 201 {
		t.Fatalf("create: %d %s", code, data)
	}
	r := decode[db.ReleaseReminder](t, data)
	if r.Bump != "minor" || r.Note != "has the new X" || r.Status != db.ReleaseReminderOpen {
		t.Errorf("created %+v", r)
	}

	if code, data := w.do("POST", "/api/release-reminders", map[string]any{"repo_path": repo, "prefix": prefix}); code != 409 {
		t.Errorf("second open reminder: %d %s", code, data)
	}
	if code, data := w.do("POST", "/api/release-reminders", map[string]any{"repo_path": repo, "prefix": "tools/nope"}); code != 404 {
		t.Errorf("unknown prefix: %d %s", code, data)
	}
	if code, data := w.do("POST", "/api/release-reminders", map[string]any{"repo_path": "/nope", "prefix": prefix}); code != 404 {
		t.Errorf("unknown repo: %d %s", code, data)
	}

	path := fmt.Sprintf("/api/release-reminders/%d", r.ID)
	if code, data := w.do("PATCH", path, map[string]any{"bump": "huge"}); code != 400 {
		t.Errorf("bad bump: %d %s", code, data)
	}
	code, data = w.do("PATCH", path, map[string]any{"note": "and Y"})
	if code != 200 {
		t.Fatalf("patch: %d %s", code, data)
	}
	if got := decode[db.ReleaseReminder](t, data); got.Note != "and Y" || got.Bump != "minor" {
		t.Errorf("patched %+v", got)
	}
	code, data = w.do("PATCH", path, map[string]any{"bump": ""})
	if code != 200 {
		t.Fatalf("patch empty bump: %d %s", code, data)
	}
	if got := decode[db.ReleaseReminder](t, data); got.Bump != "patch" {
		t.Errorf("empty bump became %q", got.Bump)
	}

	if code, data := w.do("DELETE", path, nil); code != 204 {
		t.Fatalf("dismiss: %d %s", code, data)
	}
	if code, data := w.do("DELETE", path, nil); code != 404 {
		t.Errorf("dismiss twice: %d %s", code, data)
	}
	if code, data := w.do("POST", "/api/release-reminders", map[string]any{"repo_path": repo, "prefix": prefix}); code != 201 {
		t.Errorf("reminder after dismissing: %d %s", code, data)
	}
}

func TestSummary(t *testing.T) {
	w := newWorld(t)
	var sum releases.Summary
	deadline := time.Now().Add(time.Minute)
	for {
		code, data := w.do("GET", "/api/releases", nil)
		if code != 200 {
			t.Fatalf("summary: %d %s", code, data)
		}
		sum = decode[releases.Summary](t, data)
		if len(sum.Projects) == 1 && sum.Projects[0].RefreshedAt != 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no refresh: %s", data)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if sum.Reminders == nil || sum.Runs == nil {
		t.Errorf("null lists: %+v", sum)
	}
	p := sum.Projects[0]
	if p.Error != "" || len(p.Subtrees) != 1 {
		t.Fatalf("project %+v", p)
	}
	st := p.Subtrees[0]
	if st.Prefix != prefix || st.Error != "" || st.Ahead != 0 || !st.Counted || st.Unreleased == 0 {
		t.Errorf("subtree %+v", st)
	}
}

// settled returns the summary once no refresh is running.
func (w *world) settled() releases.Summary {
	w.t.Helper()
	deadline := time.Now().Add(time.Minute)
	for {
		code, data := w.do("GET", "/api/releases", nil)
		if code != 200 {
			w.t.Fatalf("summary: %d %s", code, data)
		}
		sum := decode[releases.Summary](w.t, data)
		if len(sum.Projects) == 1 && sum.Projects[0].RefreshedAt != 0 && !sum.Projects[0].Refreshing {
			return sum
		}
		if time.Now().After(deadline) {
			w.t.Fatalf("no refresh: %s", data)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// refreshed returns the summary after a refresh that began now.
func (w *world) refreshed() releases.Summary {
	w.t.Helper()
	w.settled()
	if code, data := w.do("POST", "/api/releases/refresh", nil); code != 204 {
		w.t.Fatalf("refresh: %d %s", code, data)
	}
	return w.settled()
}

func TestSuggestedBump(t *testing.T) {
	w := newWorld(t)
	subtree := func() releases.SubtreeState {
		t.Helper()
		return w.refreshed().Projects[0].Subtrees[0]
	}
	if st := subtree(); st.Suggested != "" || st.LatestAt != 0 {
		t.Errorf("never released: %+v", st)
	}

	kanbantest.MustGit(t, w.env.RepoPath, "tag", prefix+"/v1.0.0")
	if st := subtree(); st.Suggested != "" || st.LatestAt == 0 {
		t.Errorf("just released: %+v", st)
	}

	w.commitTitled("a.go", "chore(foo): tidy")
	if st := subtree(); st.Suggested != releases.BumpNone {
		t.Errorf("chore: %+v", st)
	}
	w.commitTitled("b.go", "fix(foo): stop it")
	if st := subtree(); st.Suggested != "patch" {
		t.Errorf("fix: %+v", st)
	}
	w.commitTitled("c.go", "feat(foo): add it")
	w.commitTitled("d.go", "wip")
	if st := subtree(); st.Suggested != "minor" {
		t.Errorf("feat: %+v", st)
	}
	w.commitTitled("e.go", "fix(foo)!: change it")
	if st := subtree(); st.Suggested != "major" {
		t.Errorf("breaking: %+v", st)
	}

	// A commit that doesn't touch the subtree says nothing about it.
	kanbantest.MustGit(t, w.env.RepoPath, "tag", prefix+"/v2.0.0")
	kanbantest.MustGit(t, w.env.RepoPath, "commit", "-q", "--allow-empty", "-m", "feat: elsewhere")
	kanbantest.MustGit(t, w.env.RepoPath, "push", "-q", "origin", "main")
	if st := subtree(); st.Suggested != "" || st.Unreleased != 0 {
		t.Errorf("elsewhere: %+v", st)
	}
}

// A release made outside the tab can't close a reminder, so the reminder says
// there's been one, until it's edited.
func TestReminderFlagsReleaseSince(t *testing.T) {
	w := newWorld(t)
	code, data := w.do("POST", "/api/release-reminders", map[string]any{"repo_path": w.env.RepoPath, "prefix": prefix, "note": "ship it"})
	if code != 201 {
		t.Fatalf("remind: %d %s", code, data)
	}
	if r := w.refreshed().Reminders[0]; r.ReleasedSince != "" {
		t.Errorf("nothing released: %+v", r)
	}

	kanbantest.MustGit(t, w.env.RepoPath, "tag", prefix+"/v1.0.0")
	setEdited := func(at time.Time) {
		t.Helper()
		if _, err := w.env.Store.DB().Exec(`UPDATE release_reminders SET updated_at=?`, at.Unix()); err != nil {
			t.Fatal(err)
		}
	}
	setEdited(time.Now().Add(-time.Hour))
	if r := w.refreshed().Reminders[0]; r.ReleasedSince != "v1.0.0" {
		t.Errorf("released since: %+v", r)
	}
	setEdited(time.Now().Add(time.Hour))
	if r := w.refreshed().Reminders[0]; r.ReleasedSince != "" {
		t.Errorf("edited since: %+v", r)
	}
}

// A restart shows the last state straight away instead of an empty table
// while the first refresh runs.
func TestSummarySurvivesRestart(t *testing.T) {
	w := newWorld(t)
	deadline := time.Now().Add(time.Minute)
	for {
		_, data := w.do("GET", "/api/releases", nil)
		if sum := decode[releases.Summary](t, data); len(sum.Projects) == 1 && sum.Projects[0].RefreshedAt != 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no refresh")
		}
		time.Sleep(50 * time.Millisecond)
	}

	// The state is saved just after the refresh shows.
	for {
		saved, err := w.env.Store.ListReleaseProjects(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := saved[w.env.RepoPath]; ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("nothing saved")
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Only the first Summary shows what was saved: it starts a refresh of a
	// repo it doesn't know, which would fill in the state by itself.
	restarted := releases.New(w.env.Store, t.TempDir())
	t.Cleanup(restarted.Close)
	sum, err := restarted.Summary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.Projects) != 1 {
		t.Fatalf("restored %+v", sum.Projects)
	}
	if p := sum.Projects[0]; p.RefreshedAt == 0 || p.Refreshing || len(p.Subtrees) != 1 || p.Subtrees[0].Prefix != prefix || !p.Subtrees[0].Counted {
		t.Fatalf("restored %+v", p)
	}
}

func TestMirrorLagBlocks(t *testing.T) {
	w := newWorld(t)
	w.commit("lag.go")
	p := w.preview()
	if p.Ahead != 1 || p.Blocked == "" {
		t.Fatalf("preview %+v", p)
	}
	if code, data := w.do("POST", "/api/releases/run", w.runBody(p)); code != 409 {
		t.Errorf("run: %d %s", code, data)
	}
}

// Asked to, a release pushes what the mirror hasn't. The checkout may be on any
// branch, and that branch mustn't reach origin.
func TestRunUpstream(t *testing.T) {
	w := newWorld(t)
	w.commit("lag.go")
	kanbantest.MustGit(t, w.env.RepoPath, "checkout", "-q", "-b", "elsewhere")
	p := w.preview()
	if p.Ahead != 1 || p.Blocked == "" {
		t.Fatalf("preview %+v", p)
	}
	body := w.runBody(p)
	body["upstream"] = true
	code, data := w.do("POST", "/api/releases/run", body)
	if code != 202 {
		t.Fatalf("run: %d %s", code, data)
	}
	run := decode[db.ReleaseRun](t, data)
	output := w.output(run.ID)
	if got := w.run(run.ID); got.Status != db.ReleaseRunReleased {
		t.Fatalf("run %+v:\n%s", got, output)
	}

	refsOf := func(dir string) string {
		t.Helper()
		out, err := exec.Command("git", "-C", dir, "for-each-ref", "--format=%(refname)").CombinedOutput()
		if err != nil {
			t.Fatalf("for-each-ref: %v\n%s", err, out)
		}
		return string(out)
	}
	if refs := refsOf(w.upstream); !strings.Contains(refs, "refs/tags/"+p.Version) {
		t.Errorf("upstream lacks the tag:\n%s", refs)
	}
	refs := refsOf(w.origin)
	if !strings.Contains(refs, "refs/tags/"+prefix+"/"+p.Version) {
		t.Errorf("origin lacks the tag:\n%s", refs)
	}
	if strings.Contains(refs, "elsewhere") {
		t.Errorf("origin got the checkout's branch:\n%s", refs)
	}
	if got := w.preview(); got.Ahead != 0 {
		t.Errorf("after the release: %+v", got)
	}
}

// A subtree's split is reused until the subtree changes, so the counts must
// follow the subtree's commits and not be frozen by the cache.
func TestPreviewReusesSplit(t *testing.T) {
	w := newWorld(t)
	if p := w.preview(); p.Ahead != 0 {
		t.Fatalf("preview %+v", p)
	}
	// A commit elsewhere in the repo leaves the subtree as it was.
	other := filepath.Join(w.env.RepoPath, "elsewhere.txt")
	if err := os.WriteFile(other, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	kanbantest.MustGit(t, w.env.RepoPath, "add", "--", other)
	kanbantest.MustGit(t, w.env.RepoPath, "commit", "-q", "-m", "Change elsewhere")
	kanbantest.MustGit(t, w.env.RepoPath, "push", "-q", "origin", "main")
	if p := w.preview(); p.Ahead != 0 {
		t.Fatalf("after an unrelated commit: %+v", p)
	}
	w.commit("lag.go")
	if p := w.preview(); p.Ahead != 1 || p.Blocked == "" {
		t.Fatalf("after a subtree commit: %+v", p)
	}
}

// No ref holds a split, so git gc prunes it while its row stays saved.
func TestPreviewSplitsAgainAfterPrune(t *testing.T) {
	w := newWorld(t)
	w.commit("lag.go")
	p := w.preview()
	if p.Ahead != 1 {
		t.Fatalf("preview %+v", p)
	}

	o, err := orchard.Open(w.env.RepoPath)
	if err != nil {
		t.Fatal(err)
	}
	sub, _ := o.Config.Lookup(prefix)
	change, err := o.LastChange(sub, p.Rev)
	if err != nil {
		t.Fatal(err)
	}
	split, ok, err := w.env.Store.GetReleaseSplit(context.Background(), w.env.RepoPath, prefix, change)
	if err != nil || !ok {
		t.Fatalf("saved split: %v %v", ok, err)
	}
	kanbantest.MustGit(t, w.env.RepoPath, "reflog", "expire", "--expire=now", "--all")
	kanbantest.MustGit(t, w.env.RepoPath, "gc", "-q", "--prune=now")
	if _, err := o.Repo.Output("cat-file", "-e", split+"^{commit}"); err == nil {
		t.Fatalf("split %s wasn't pruned", split)
	}

	if p := w.preview(); p.Ahead != 1 {
		t.Fatalf("after the prune: %+v", p)
	}
}

func TestStaleConfirmation(t *testing.T) {
	w := newWorld(t)
	p := w.preview()

	stale := w.runBody(p)
	stale["version"] = "v9.9.9"
	if code, data := w.do("POST", "/api/releases/run", stale); code != 409 {
		t.Errorf("stale version: %d %s", code, data)
	}

	w.change("newer.go")
	if code, data := w.do("POST", "/api/releases/run", w.runBody(p)); code != 409 {
		t.Errorf("stale rev: %d %s", code, data)
	}
}

func TestRequestGuards(t *testing.T) {
	w := newWorld(t)
	p := w.preview()
	if code, data := w.do("POST", "/api/releases/run", w.runBody(p), "Sec-Fetch-Site", "cross-site"); code != 403 {
		t.Errorf("cross-site: %d %s", code, data)
	}
	if code, data := w.do("POST", "/api/releases/run", w.runBody(p), "Content-Type", "text/plain"); code != 403 {
		t.Errorf("text/plain: %d %s", code, data)
	}
	if code, data := w.do("POST", "/api/release-reminders", map[string]any{"repo_path": w.env.RepoPath, "prefix": prefix}, "Content-Type", "text/plain"); code != 403 {
		t.Errorf("reminder as text/plain: %d %s", code, data)
	}
	for _, route := range [][2]string{
		{"POST", "/api/releases/refresh"},
		{"POST", "/api/release-reminders"},
		{"PATCH", "/api/release-reminders/1"},
		{"DELETE", "/api/release-reminders/1"},
	} {
		if code, data := w.do(route[0], route[1], nil, "Sec-Fetch-Site", "cross-site"); code != 403 {
			t.Errorf("cross-site %s %s: %d %s", route[0], route[1], code, data)
		}
	}
	body := w.runBody(p)
	body["prefix"] = "tools/nope"
	if code, data := w.do("POST", "/api/releases/run", body); code != 404 {
		t.Errorf("unknown prefix: %d %s", code, data)
	}
	if out, err := exec.Command("git", "-C", w.origin, "tag", "--list").Output(); err != nil || len(out) != 0 {
		t.Errorf("tags on origin: %q, %v", out, err)
	}
}

func TestRun(t *testing.T) {
	w := newWorld(t)
	code, data := w.do("POST", "/api/release-reminders", map[string]any{"repo_path": w.env.RepoPath, "prefix": prefix, "bump": "minor", "note": "n"})
	if code != 201 {
		t.Fatalf("remind: %d %s", code, data)
	}
	reminder := decode[db.ReleaseReminder](t, data)

	p := w.preview()
	if p.Blocked != "" || p.Version != "v0.1.0" {
		t.Fatalf("preview %+v", p)
	}
	body := w.runBody(p)
	body["reminder_id"] = reminder.ID
	code, data = w.do("POST", "/api/releases/run", body)
	if code != 202 {
		t.Fatalf("run: %d %s", code, data)
	}
	run := decode[db.ReleaseRun](t, data)

	// The stream follows the run to its end, then a late reader sees it all.
	first := w.output(run.ID)
	tag := prefix + "/v0.1.0"
	if want := "Tagged " + tag; !strings.Contains(first, want) {
		t.Errorf("output lacks %q:\n%s", want, first)
	}
	if late := w.output(run.ID); late != first {
		t.Errorf("late reader saw:\n%s\nnot:\n%s", late, first)
	}

	if got := w.run(run.ID); got.Status != db.ReleaseRunReleased || got.FinishedAt == nil {
		t.Errorf("run %+v", got)
	}
	got, err := w.env.Store.GetReleaseReminder(context.Background(), reminder.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != db.ReleaseReminderReleased {
		t.Errorf("reminder %+v", got)
	}
	out, err := exec.Command("git", "-C", w.origin, "for-each-ref", "--format=%(objectname:short=40) %(*objectname) %(taggername)", "refs/tags/"+tag).Output()
	if err != nil {
		t.Fatal(err)
	}
	if fields := strings.SplitN(strings.TrimSpace(string(out)), " ", 3); len(fields) != 3 || fields[1] != p.Rev || fields[2] != "Release Bot" {
		t.Errorf("tag on origin: %q, want it at %s by Release Bot", out, p.Rev)
	}
}

// A run cut off by a restart is interrupted, and running it again finishes it:
// the tag it left behind is published again.
func TestInterruptedRunRetries(t *testing.T) {
	w := newWorld(t)
	p := w.preview()
	code, data := w.do("POST", "/api/releases/run", w.runBody(p))
	if code != 202 {
		t.Fatalf("run: %d %s", code, data)
	}
	first := decode[db.ReleaseRun](t, data)
	w.output(first.ID)

	stuck := &db.ReleaseRun{RepoPath: w.env.RepoPath, Prefix: prefix, Rev: p.Rev, Version: p.Version, Bump: "minor"}
	if err := w.env.Store.CreateReleaseRun(context.Background(), stuck); err != nil {
		t.Fatal(err)
	}
	restarted := releases.New(w.env.Store, t.TempDir())
	defer restarted.Close()
	if got := w.run(stuck.ID); got.Status != db.ReleaseRunInterrupted || got.FinishedAt == nil {
		t.Fatalf("after restart: %+v", got)
	}

	again := w.preview()
	if !again.Released || again.Version != p.Version || again.Rev != p.Rev {
		t.Fatalf("preview of a tagged rev: %+v", again)
	}
	code, data = w.do("POST", "/api/releases/run", w.runBody(again))
	if code != 202 {
		t.Fatalf("retry: %d %s", code, data)
	}
	retry := decode[db.ReleaseRun](t, data)
	w.output(retry.ID)
	if got := w.run(retry.ID); got.Status != db.ReleaseRunReleased {
		t.Errorf("retry %+v", got)
	}
}

func TestOutputOfUnknownRun(t *testing.T) {
	w := newWorld(t)
	if code, data := w.do("GET", "/api/release-runs/999/output", nil); code != 404 {
		t.Errorf("%d %s", code, data)
	}
}

func TestSummaryCI(t *testing.T) {
	w := newWorld(t)
	// These remotes aren't on GitHub, so there's nothing to check and no token
	// is needed.
	deadline := time.Now().Add(30 * time.Second)
	for {
		code, data := w.do("GET", "/api/releases", nil)
		if code != 200 {
			t.Fatalf("summary: %d %s", code, data)
		}
		sum := decode[releases.Summary](t, data)
		if ci := sum.Projects[0].Subtrees[0].CI; ci != nil {
			if ci.Status != "none" || ci.Error != "" {
				t.Errorf("ci = %+v", ci)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("CI was never checked")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestHealth(t *testing.T) {
	passed := ghrun.Run{Workflow: "test", Status: "completed", Conclusion: "success"}
	failed := ghrun.Run{Workflow: "lint", Status: "completed", Conclusion: "failure", URL: "https://example.com/1"}
	running := ghrun.Run{Workflow: "e2e", Status: "in_progress"}
	cancelled := ghrun.Run{Workflow: "old", Status: "completed", Conclusion: "cancelled"}
	for _, tc := range []struct {
		name string
		res  orchard.CIResult
		want string
	}{
		{"nothing", orchard.CIResult{}, "none"},
		{"passing", orchard.CIResult{Runs: []orchard.CIRun{{Run: passed}}}, "passing"},
		{"running beats passing", orchard.CIResult{Runs: []orchard.CIRun{{Run: passed}, {Run: running}}}, "running"},
		{"failing beats running", orchard.CIResult{Runs: []orchard.CIRun{{Run: running}, {Run: failed}}}, "failing"},
		{"stale failures are history", orchard.CIResult{Runs: []orchard.CIRun{{Run: failed, Stale: "gone"}, {Run: passed}}}, "passing"},
		{"cancelled isn't passing", orchard.CIResult{Runs: []orchard.CIRun{{Run: cancelled}}}, "none"},
		{"a failure shows despite an error", orchard.CIResult{Error: "EOF", Runs: []orchard.CIRun{{Run: failed}}}, "failing"},
	} {
		if got := releases.Health(tc.res); got.Status != tc.want {
			t.Errorf("%s: %+v, want %s", tc.name, got, tc.want)
		}
	}
	got := releases.Health(orchard.CIResult{Error: "EOF", Runs: []orchard.CIRun{{Run: failed}}})
	if len(got.Failed) != 1 || got.Failed[0] != "lint" || got.URL != failed.URL || got.Error != "EOF" {
		t.Errorf("health = %+v", got)
	}
}

// A failing remote is git's fault, not a server bug worth reporting.
func TestPreviewGitFailure(t *testing.T) {
	w := newWorld(t)
	if err := os.RemoveAll(w.origin); err != nil {
		t.Fatal(err)
	}
	q := url.Values{"repo": {w.env.RepoPath}, "prefix": {prefix}}
	if code, data := w.do("GET", "/api/releases/preview?"+q.Encode(), nil); code != 502 {
		t.Errorf("preview without origin: %d %s", code, data)
	}
}

func TestRunAfterClose(t *testing.T) {
	w := newWorld(t)
	p := w.preview()
	svc := releases.New(w.env.Store, t.TempDir())
	svc.Close()
	_, err := svc.Run(context.Background(), releases.RunRequest{RepoPath: w.env.RepoPath, Prefix: prefix, Bump: "minor", Version: p.Version, Rev: p.Rev})
	if !errors.Is(err, releases.ErrConflict) {
		t.Errorf("got %v, want ErrConflict", err)
	}
	runs, err := w.env.Store.ListReleaseRuns(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, run := range runs {
		if run.Status == db.ReleaseRunQueued || run.Status == db.ReleaseRunRunning {
			t.Errorf("left %s: %+v", run.Status, run)
		}
	}
}

// As with git orchard release, the default bump continues a pre-release.
func TestPreReleaseContinues(t *testing.T) {
	w := newWorld(t)
	preview := func() releases.Preview {
		t.Helper()
		q := url.Values{"repo": {w.env.RepoPath}, "prefix": {prefix}, "bump": {"patch"}, "suffix": {"rc"}}
		code, data := w.do("GET", "/api/releases/preview?"+q.Encode(), nil)
		if code != 200 {
			t.Fatalf("preview: %d %s", code, data)
		}
		return decode[releases.Preview](t, data)
	}
	p := preview()
	if p.Version != "v0.0.1-rc" {
		t.Fatalf("first pre-release: %+v", p)
	}
	code, data := w.do("POST", "/api/releases/run", map[string]any{"repo": w.env.RepoPath, "prefix": prefix, "bump": "patch", "suffix": "rc", "version": p.Version, "rev": p.Rev})
	if code != 202 {
		t.Fatalf("run: %d %s", code, data)
	}
	w.output(decode[db.ReleaseRun](t, data).ID)

	w.commit("next.go")
	if got := preview(); got.Version != "v0.0.1-rc.1" {
		t.Errorf("second pre-release: %+v", got)
	}
}

// addSubtree adds another subtree like the world's first, whose upstream is in
// sync, with the given extra manifest lines.
func (w *world) addSubtree(sub, manifest string) {
	w.t.Helper()
	dir := w.t.TempDir()
	upstream := filepath.Join(dir, "up.git")
	work := filepath.Join(dir, "work")
	kanbantest.MustGit(w.t, "", "init", "-q", "--bare", "-b", "master", upstream)
	kanbantest.MustGit(w.t, "", "init", "-q", "-b", "master", work)
	if err := os.WriteFile(filepath.Join(work, "README"), []byte(sub+"\n"), 0o644); err != nil {
		w.t.Fatal(err)
	}
	kanbantest.MustGit(w.t, work, "add", "README")
	kanbantest.MustGit(w.t, work, "-c", "user.name=T", "-c", "user.email=t@example.com", "commit", "-q", "-m", "Start")
	kanbantest.MustGit(w.t, work, "push", "-q", upstream, "master")

	o, err := orchard.Open(w.env.RepoPath)
	if err != nil {
		w.t.Fatal(err)
	}
	if err := o.Add(config.Subtree{Prefix: sub, Remote: upstream, Branch: "master"}, ""); err != nil {
		w.t.Fatal(err)
	}
	if manifest != "" {
		file, err := os.OpenFile(filepath.Join(w.env.RepoPath, o.Config.Manifest), os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			w.t.Fatal(err)
		}
		_, err = fmt.Fprintf(file, "\t%s\n", manifest)
		if cerr := file.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			w.t.Fatal(err)
		}
	}
	kanbantest.MustGit(w.t, w.env.RepoPath, "add", o.Config.Manifest)
	kanbantest.MustGit(w.t, w.env.RepoPath, "commit", "-q", "-m", "List "+sub)
	kanbantest.MustGit(w.t, w.env.RepoPath, "push", "-q", "origin", "main")
}

// gate returns a script that waits until the returned func is called, which
// happens when the test ends at the latest.
func gate(t *testing.T) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "open")
	script := filepath.Join(dir, "wait.sh")
	body := fmt.Sprintf("#!/bin/sh\ni=0; while [ ! -e %q ] && [ $i -lt 1200 ]; do sleep 0.05; i=$((i+1)); done\n", path)
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	open := func() {
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(open)
	return script, open
}

// holdTagPushes makes origin wait on a gate before accepting any push, which
// is the last thing a release does in the repo.
func (w *world) holdTagPushes() func() {
	w.t.Helper()
	wait, open := gate(w.t)
	hook := filepath.Join(w.origin, "hooks", "pre-receive")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nexec "+wait+"\n"), 0o755); err != nil {
		w.t.Fatal(err)
	}
	return open
}

func (w *world) startRun(sub string) *db.ReleaseRun {
	w.t.Helper()
	q := url.Values{"repo": {w.env.RepoPath}, "prefix": {sub}, "bump": {"patch"}}
	code, data := w.do("GET", "/api/releases/preview?"+q.Encode(), nil)
	if code != 200 {
		w.t.Fatalf("preview %s: %d %s", sub, code, data)
	}
	p := decode[releases.Preview](w.t, data)
	code, data = w.do("POST", "/api/releases/run", map[string]any{"repo": w.env.RepoPath, "prefix": sub, "bump": "patch", "version": p.Version, "rev": p.Rev})
	if code != 202 {
		w.t.Fatalf("run %s: %d %s", sub, code, data)
	}
	return new(decode[db.ReleaseRun](w.t, data))
}

// waitForStatuses waits until the runs have these counts of each status.
func (w *world) waitForStatuses(runs []*db.ReleaseRun, want map[string]int) {
	w.t.Helper()
	deadline := time.Now().Add(time.Minute)
	for {
		got := map[string]int{}
		for _, run := range runs {
			got[w.run(run.ID).Status]++
		}
		if fmt.Sprint(got) == fmt.Sprint(want) {
			return
		}
		if time.Now().After(deadline) {
			w.t.Fatalf("statuses %v, want %v", got, want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// A repo's subtrees release at once up to a limit, and the rest wait in turn,
// but a subtree can't have two: the second's version would be out of date.
func TestRunsQueueAndRunInParallel(t *testing.T) {
	w := newWorld(t)
	subs := []string{prefix, "tools/a", "tools/b", "tools/c", "tools/d"}
	for _, sub := range subs[1:] {
		w.addSubtree(sub, "")
	}
	open := w.holdTagPushes()

	runs := make([]*db.ReleaseRun, len(subs))
	for i, sub := range subs {
		runs[i] = w.startRun(sub)
	}
	w.waitForStatuses(runs, map[string]int{db.ReleaseRunRunning: 4, db.ReleaseRunQueued: 1})

	if sum := w.settled(); sum.Projects[0].Subtrees[0].Run == nil || sum.Projects[0].Subtrees[0].Run.Status == db.ReleaseRunReleased {
		t.Errorf("subtree's run in the summary: %+v", sum.Projects[0].Subtrees[0])
	}
	q := url.Values{"repo": {w.env.RepoPath}, "prefix": {prefix}, "bump": {"patch"}}
	_, data := w.do("GET", "/api/releases/preview?"+q.Encode(), nil)
	p := decode[releases.Preview](t, data)
	if code, data := w.do("POST", "/api/releases/run", map[string]any{"repo": w.env.RepoPath, "prefix": prefix + "/", "bump": "patch", "version": p.Version, "rev": p.Rev}); code != 409 {
		t.Errorf("a second release of a queued subtree: %d %s", code, data)
	}

	open()
	queued := 0
	for _, run := range runs {
		output := w.output(run.ID)
		if got := w.run(run.ID); got.Status != db.ReleaseRunReleased {
			t.Errorf("run %+v:\n%s", got, output)
		}
		if strings.Contains(output, "Queued behind") {
			queued++
		}
	}
	if queued != 1 {
		t.Errorf("%d logs say they queued, want 1", queued)
	}
	out, err := exec.Command("git", "-C", w.origin, "tag", "--list").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, sub := range subs {
		if !strings.Contains(string(out), sub+"/v0.0.1") {
			t.Errorf("origin lacks %s's tag:\n%s", sub, out)
		}
	}

	// Having finished, the subtree can be released again.
	w.commit("next.go")
	w.change("next2.go")
	w.output(w.startRun(prefix).ID)
}

// Pre-release hooks run in the one checkout, so they take turns, without
// holding up subtrees that have none.
func TestPreReleaseHooksTakeTurns(t *testing.T) {
	w := newWorld(t)
	wait, open := gate(t)
	w.addSubtree("tools/a", "pre-release = "+wait)
	w.addSubtree("tools/b", "pre-release = "+wait)

	hooked := []*db.ReleaseRun{w.startRun("tools/a"), w.startRun("tools/b")}
	w.waitForStatuses(hooked, map[string]int{db.ReleaseRunRunning: 1, db.ReleaseRunQueued: 1})

	plain := w.startRun(prefix)
	if output := w.output(plain.ID); w.run(plain.ID).Status != db.ReleaseRunReleased {
		t.Errorf("run without a hook %+v:\n%s", w.run(plain.ID), output)
	}
	w.waitForStatuses(hooked, map[string]int{db.ReleaseRunRunning: 1, db.ReleaseRunQueued: 1})

	open()
	for _, run := range hooked {
		if output := w.output(run.ID); w.run(run.ID).Status != db.ReleaseRunReleased {
			t.Errorf("hooked run %+v:\n%s", w.run(run.ID), output)
		}
	}
}

// Stopping the server interrupts what's waiting along with what's running.
func TestCloseInterruptsQueuedRuns(t *testing.T) {
	w := newWorld(t)
	subs := []string{prefix, "tools/a", "tools/b", "tools/c", "tools/d"}
	for _, sub := range subs[1:] {
		w.addSubtree(sub, "")
	}
	open := w.holdTagPushes()

	svc := releases.New(w.env.Store, t.TempDir())
	runs := make([]*db.ReleaseRun, len(subs))
	for i, sub := range subs {
		p, err := svc.Preview(context.Background(), w.env.RepoPath, sub, "patch", "")
		if err != nil {
			t.Fatal(err)
		}
		runs[i], err = svc.Run(context.Background(), releases.RunRequest{RepoPath: w.env.RepoPath, Prefix: sub, Bump: "patch", Version: p.Version, Rev: p.Rev})
		if err != nil {
			t.Fatal(err)
		}
	}
	w.waitForStatuses(runs, map[string]int{db.ReleaseRunRunning: 4, db.ReleaseRunQueued: 1})

	closed := make(chan struct{})
	go func() {
		svc.Close()
		close(closed)
	}()
	// Origin's hook outlives the push that started it, and holds its output open.
	for _, run := range runs {
		for w.run(run.ID).Status == db.ReleaseRunQueued {
			time.Sleep(20 * time.Millisecond)
		}
	}
	open()
	select {
	case <-closed:
	case <-time.After(time.Minute):
		t.Fatal("Close didn't return")
	}
	w.waitForStatuses(runs, map[string]int{db.ReleaseRunInterrupted: 5})
}
