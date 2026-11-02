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
	"github.com/jmelahman/kanban/internal/releases/kevtest"
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
	w.commitWith(name, "Change "+name)
}

func (w *world) commitWith(name, subject string) {
	w.t.Helper()
	path := filepath.Join(w.env.RepoPath, prefix, name)
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		w.t.Fatal(err)
	}
	kanbantest.MustGit(w.t, w.env.RepoPath, "add", "--", path)
	kanbantest.MustGit(w.t, w.env.RepoPath, "commit", "-q", "-m", subject)
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
		if run.Status == db.ReleaseRunRunning {
			t.Errorf("left running: %+v", run)
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

// tagRelease marks the subtree as released at HEAD, so what follows is unreleased.
func (w *world) tagRelease() {
	w.t.Helper()
	kanbantest.MustGit(w.t, w.env.RepoPath, "tag", prefix+"/v0.1.0")
	kanbantest.MustGit(w.t, w.env.RepoPath, "push", "-q", "origin", prefix+"/v0.1.0")
}

// waitSummary reads the summary until done accepts it.
func (w *world) waitSummary(what string, done func(releases.Summary) bool) releases.Summary {
	w.t.Helper()
	deadline := time.Now().Add(time.Minute)
	for {
		code, data := w.do("GET", "/api/releases", nil)
		if code != 200 {
			w.t.Fatalf("summary: %d %s", code, data)
		}
		sum := decode[releases.Summary](w.t, data)
		if done(sum) {
			return sum
		}
		if time.Now().After(deadline) {
			w.t.Fatalf("%s: %s", what, data)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// assessedAs reports whether the only subtree is assessed as visible, with no
// pass under way.
func assessedAs(visible float64) func(releases.Summary) bool {
	return func(sum releases.Summary) bool {
		if len(sum.Projects) != 1 || sum.Assessment == nil || sum.Assessment.Running {
			return false
		}
		a := sum.Projects[0].Subtrees[0].Assessment
		return a != nil && a.Visible == visible
	}
}

// settled reports whether every refresh and pass has finished.
func settled(sum releases.Summary) bool {
	if len(sum.Projects) != 1 || sum.Projects[0].Refreshing || sum.Projects[0].RefreshedAt == 0 {
		return false
	}
	return sum.Assessment == nil || !sum.Assessment.Running
}

// Off is the default: no column, and nothing to load.
func TestAssessmentIsOffByDefault(t *testing.T) {
	w := newWorld(t)
	w.tagRelease()
	w.commit("a.go")
	sum := w.waitSummary("no refresh", settled)
	if sum.Assessment != nil || sum.Projects[0].Subtrees[0].Assessment != nil {
		t.Errorf("assessed with no model: %+v", sum)
	}
}

// The model is loaded only for commits it hasn't read, and let go of after.
func TestAssessmentLoadsOnlyForNewCommits(t *testing.T) {
	w := newWorld(t)
	w.tagRelease()
	w.commitWith("a.go", "Add the frobnicate flag")
	loader := &kevtest.Loader{Visible: 0.75}
	w.env.Releases.EnableAssessment("kev", loader.Open)

	sum := w.waitSummary("no assessment", assessedAs(0.75))
	if st := sum.Projects[0].Subtrees[0]; st.Unreleased != 1 {
		t.Errorf("subtree %+v", st)
	}
	if !loader.Read("- Add the frobnicate flag") {
		t.Error("the model didn't read the commit")
	}
	if opens, closes := loader.Counts(); opens != 1 || closes != 1 {
		t.Errorf("opened %d, closed %d", opens, closes)
	}

	// A refresh that finds the same commits has nothing to ask.
	if code, data := w.do("POST", "/api/releases/refresh", nil); code != 204 {
		t.Fatalf("refresh: %d %s", code, data)
	}
	w.waitSummary("no refresh", settled)
	if opens, _ := loader.Counts(); opens != 1 {
		t.Errorf("reloaded for the same commits: %d opens", opens)
	}

	// New commits are read again, and the old answer isn't shown for them.
	w.commitWith("b.go", "Fix the crash on start")
	loader.Set(0.25)
	if code, data := w.do("POST", "/api/releases/refresh", nil); code != 204 {
		t.Fatalf("refresh: %d %s", code, data)
	}
	w.waitSummary("no new assessment", assessedAs(0.25))
	if opens, closes := loader.Counts(); opens != 2 || closes != 2 {
		t.Errorf("opened %d, closed %d", opens, closes)
	}
}

// A release leaves nothing to assess, so the old answer goes and nothing loads.
func TestAssessmentSkipsWhatHasNoCommits(t *testing.T) {
	w := newWorld(t)
	w.tagRelease()
	loader := &kevtest.Loader{Visible: 0.75}
	w.env.Releases.EnableAssessment("kev", loader.Open)
	w.waitSummary("no refresh", settled)
	// A refresh having finished, a pass may be yet to start: read once more.
	sum := w.waitSummary("no refresh", settled)
	if st := sum.Projects[0].Subtrees[0]; st.Unreleased != 0 || st.Assessment != nil {
		t.Errorf("subtree %+v", st)
	}
	if opens, _ := loader.Counts(); opens != 0 {
		t.Errorf("loaded a model for nothing: %d opens", opens)
	}
}

// A restart shows the last answer and doesn't load the model to repeat it.
func TestAssessmentSurvivesRestart(t *testing.T) {
	w := newWorld(t)
	w.tagRelease()
	w.commitWith("a.go", "Add the frobnicate flag")
	first := &kevtest.Loader{Visible: 0.75}
	w.env.Releases.EnableAssessment("kev", first.Open)
	w.waitSummary("no assessment", assessedAs(0.75))
	deadline := time.Now().Add(time.Minute)
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
		time.Sleep(20 * time.Millisecond)
	}

	restarted := releases.New(w.env.Store, t.TempDir())
	t.Cleanup(restarted.Close)
	second := &kevtest.Loader{Visible: 0.1}
	restarted.EnableAssessment("kev", second.Open)
	summary := func() releases.Summary {
		t.Helper()
		sum, err := restarted.Summary(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		return sum
	}
	sum := summary()
	if a := sum.Projects[0].Subtrees[0].Assessment; a == nil || a.Visible != 0.75 {
		t.Fatalf("restored %+v", sum.Projects[0].Subtrees[0])
	}
	for !settled(sum) {
		if time.Now().After(deadline) {
			t.Fatal("never settled")
		}
		time.Sleep(20 * time.Millisecond)
		sum = summary()
	}
	if opens, _ := second.Counts(); opens != 0 {
		t.Errorf("loaded a model after a restart: %d opens", opens)
	}

	// A different checkpoint hasn't given this answer, so it has to.
	swapped := releases.New(w.env.Store, t.TempDir())
	t.Cleanup(swapped.Close)
	third := &kevtest.Loader{Visible: 0.5}
	swapped.EnableAssessment("smaller-kev", third.Open)
	for {
		sum, err := swapped.Summary(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if a := sum.Projects[0].Subtrees[0].Assessment; a != nil && a.Visible == 0.5 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("not assessed again: %+v", sum.Projects[0].Subtrees[0])
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// A checkpoint that won't load is reported, and not retried on every read.
func TestAssessmentReportsFailure(t *testing.T) {
	w := newWorld(t)
	w.tagRelease()
	w.commit("a.go")
	loader := &kevtest.Loader{OpenErr: errors.New("no such checkpoint")}
	w.env.Releases.EnableAssessment("kev", loader.Open)
	sum := w.waitSummary("no failure", func(sum releases.Summary) bool {
		return sum.Assessment != nil && sum.Assessment.Error != ""
	})
	if !strings.Contains(sum.Assessment.Error, "no such checkpoint") {
		t.Errorf("error = %q", sum.Assessment.Error)
	}
	w.waitSummary("no refresh", settled)
	if opens, _ := loader.Counts(); opens != 1 {
		t.Errorf("retried on a read: %d opens", opens)
	}
	if st := w.waitSummary("no refresh", settled).Projects[0].Subtrees[0]; st.Assessment != nil {
		t.Errorf("subtree %+v", st)
	}
}

// The real model, if there's one to hand: internal-only commits should read as
// less visible than a user-facing fix and feature.
func TestAssessmentWithKev(t *testing.T) {
	checkpoint := os.Getenv("KEV_CHECKPOINT")
	if checkpoint == "" {
		t.Skip("KEV_CHECKPOINT isn't set")
	}
	visible := func(subjects ...string) float64 {
		w := newWorld(t)
		w.tagRelease()
		for i, subject := range subjects {
			w.commitWith(fmt.Sprintf("f%d.go", i), subject)
		}
		w.env.Releases.EnableAssessment(checkpoint, releases.OpenKev(checkpoint, os.Getenv("KEV_BASE")))
		sum := w.waitSummary("no assessment", func(sum releases.Summary) bool {
			if a := sum.Assessment; a != nil && a.Error != "" {
				t.Fatal(a.Error)
			}
			return len(sum.Projects) == 1 && sum.Projects[0].Subtrees[0].Assessment != nil
		})
		return sum.Projects[0].Subtrees[0].Assessment.Visible
	}
	internal := visible("ci: bump actions/checkout", "docs: fix typo in README", "chore: run formatter")
	user := visible("fix: crash when the config file is empty", "feat: add a --json flag to the list command")
	t.Logf("internal %.3f, user-facing %.3f", internal, user)
	if internal >= user {
		t.Errorf("internal %.3f isn't below user-facing %.3f", internal, user)
	}
}
