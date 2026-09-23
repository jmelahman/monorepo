package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jmelahman/local-preview/internal/client"
)

// fakeDeployServer serves /api/deploys with the same filter semantics as the
// real API, over a fixed newest-first list.
func fakeDeployServer(t *testing.T, deploys []client.Deploy) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/deploys" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		out := []client.Deploy{}
		for _, d := range deploys {
			if repo := q.Get("repo"); repo != "" && d.Repo != repo {
				continue
			}
			if branch := q.Get("branch"); branch != "" && d.Branch != branch {
				continue
			}
			if text := strings.ToLower(q.Get("q")); text != "" &&
				!strings.HasPrefix(d.SHA, text) &&
				!strings.Contains(strings.ToLower(d.Branch), text) {
				continue
			}
			out = append(out, d)
		}
		json.NewEncoder(w).Encode(out)
	}))
	t.Cleanup(ts.Close)
	return ts
}

// runOpenCmd executes `preview open` against the fake server from a
// directory that is not a git repo, so ref resolution happens server-side.
func runOpenCmd(t *testing.T, serverURL string, args ...string) (string, error) {
	t.Helper()
	t.Chdir(t.TempDir())
	cmd := openCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs(append(args, "--repo", "demo", "--print", "--server", serverURL))
	err := cmd.Execute()
	return out.String(), err
}

func TestOpenCommand(t *testing.T) {
	deploys := []client.Deploy{
		{ID: 3, Repo: "demo", SHA: "ddddddd4444444444444444444444444444444444",
			ShortSHA: "ddddddd", Branch: "feature/abc", Status: "building"},
		{ID: 2, Repo: "demo", SHA: "abc12345555555555555555555555555555555555",
			ShortSHA: "abc1234", Branch: "main", Status: "ready",
			PreviewURL: "http://abc1234-demo.preview.localhost:8080/"},
		{ID: 1, Repo: "demo", SHA: "eeeeeee6666666666666666666666666666666666",
			ShortSHA: "eeeeeee", Branch: "main", Status: "evicted"},
	}
	ts := fakeDeployServer(t, deploys)

	t.Run("branch picks newest deploy", func(t *testing.T) {
		out, err := runOpenCmd(t, ts.URL, "main")
		if err != nil || !strings.Contains(out, "http://abc1234-demo.preview.localhost:8080/") {
			t.Fatalf("out = %q, err = %v", out, err)
		}
	})

	t.Run("sha prefix beats substring match", func(t *testing.T) {
		// "abc" is a substring of newer branch feature/abc, but a sha prefix
		// of deploy 2 — the sha match wins.
		out, err := runOpenCmd(t, ts.URL, "abc")
		if err != nil || !strings.Contains(out, "http://abc1234-demo.preview.localhost:8080/") {
			t.Fatalf("out = %q, err = %v", out, err)
		}
	})

	t.Run("substring match falls back to newest", func(t *testing.T) {
		_, err := runOpenCmd(t, ts.URL, "feature")
		if err == nil || !strings.Contains(err.Error(), "still building") {
			t.Fatalf("err = %v, want still-building error", err)
		}
	})

	t.Run("evicted deploy suggests rebuild", func(t *testing.T) {
		_, err := runOpenCmd(t, ts.URL, "eeeeeee")
		if err == nil || !strings.Contains(err.Error(), "preview deploy eeeeeee6666666666666666666666666666666666") {
			t.Fatalf("err = %v, want rebuild hint", err)
		}
	})

	t.Run("no match errors with hint", func(t *testing.T) {
		_, err := runOpenCmd(t, ts.URL, "nope")
		if err == nil || !strings.Contains(err.Error(), "no deploy") {
			t.Fatalf("err = %v, want no-deploy error", err)
		}
	})

	t.Run("no ref outside a git repo", func(t *testing.T) {
		_, err := runOpenCmd(t, ts.URL)
		if err == nil || !strings.Contains(err.Error(), "no ref given") {
			t.Fatalf("err = %v, want no-ref error", err)
		}
	})
}
