package releases_test

import (
	"bytes"
	"context"
	"encoding/json"
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
	path := filepath.Join(w.env.RepoPath, prefix, name)
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		w.t.Fatal(err)
	}
	kanbantest.MustGit(w.t, w.env.RepoPath, "add", "--", path)
	kanbantest.MustGit(w.t, w.env.RepoPath, "commit", "-q", "-m", "Change "+name)
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
