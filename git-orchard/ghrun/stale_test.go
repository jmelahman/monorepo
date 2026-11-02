package ghrun

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// fakeAPI serves routes, by method and path with query, like "GET /repos/o/r",
// and 404s the rest, returning the requests it got. Everything goes to it for
// the rest of the test.
func fakeAPI(t *testing.T, routes map[string]string) (calls func() []string) {
	t.Helper()
	var (
		mu  sync.Mutex
		log []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := r.Method + " " + r.URL.RequestURI()
		mu.Lock()
		log = append(log, route)
		mu.Unlock()
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("%s: Authorization = %q", route, got)
		}
		body, ok := routes[route]
		if !ok {
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("GITHUB_API_URL", srv.URL)
	t.Setenv("GH_TOKEN", "test-token")
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(log)
	}
}

func TestDependabotStale(t *testing.T) {
	for _, tc := range []struct {
		name   string
		routes map[string]string
		stale  bool
	}{
		{"configured", map[string]string{
			"GET /repos/o/r/contents/.github/dependabot.yml?ref=main": "{}",
			"GET /repos/o/r/automated-security-fixes":                 `{"enabled":false}`,
		}, false},
		{"configured as .yaml", map[string]string{
			"GET /repos/o/r/contents/.github/dependabot.yaml?ref=main": "{}",
			"GET /repos/o/r/automated-security-fixes":                  `{"enabled":false}`,
		}, false},
		{"security updates on", map[string]string{
			"GET /repos/o/r/automated-security-fixes": `{"enabled":true}`,
		}, false},
		{"neither", map[string]string{
			"GET /repos/o/r/automated-security-fixes": `{"enabled":false}`,
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeAPI(t, tc.routes)
			why, err := DependabotStale(context.Background(), "o/r", "main")
			if err != nil {
				t.Fatal(err)
			}
			if got := why != ""; got != tc.stale {
				t.Errorf("stale = %t (%q), want %t", got, why, tc.stale)
			}
		})
	}
}

func TestDependabotStaleCantTell(t *testing.T) {
	// No access to the security fixes setting: a 404, like no such setting.
	fakeAPI(t, nil)
	if why, err := DependabotStale(context.Background(), "o/r", "main"); err == nil {
		t.Errorf("got %q, want an error", why)
	}
}

func TestDeleteRun(t *testing.T) {
	calls := fakeAPI(t, map[string]string{"DELETE /repos/o/r/actions/runs/1": ""})
	if err := DeleteRun(context.Background(), "o/r", 1); err != nil {
		t.Fatal(err)
	}
	// Gone already, as after a retry.
	if err := DeleteRun(context.Background(), "o/r", 2); err != nil {
		t.Errorf("deleting a deleted run: %v", err)
	}
	if got, want := calls(), "DELETE /repos/o/r/actions/runs/1"; len(got) == 0 || got[0] != want {
		t.Errorf("requests %v, want first %q", got, want)
	}
}

const runJSON = `{"id":7,"workflow_id":3,"name":"Tests","head_branch":"v1","head_sha":"abc",
	"status":"completed","conclusion":"failure","html_url":"https://github.test/7","event":"push",
	"display_title":"a title","created_at":"2026-01-02T03:04:05Z"}`

func TestGH(t *testing.T) {
	fakeAPI(t, map[string]string{
		"GET /repos/o/r/actions/runs?event=push&head_sha=abc&per_page=100": `{"workflow_runs":[` + runJSON + `]}`,
	})
	runs, err := GH(context.Background(), "o/r", "abc")
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs = %+v", runs)
	}
	r := runs[0]
	if r.ID != 7 || r.WorkflowID != 3 || r.Workflow != "Tests" || r.HeadBranch != "v1" || r.HeadSHA != "abc" ||
		r.Status != "completed" || r.Conclusion != "failure" || r.URL != "https://github.test/7" ||
		r.Event != "push" || r.Title != "a title" || r.CreatedAt.Year() != 2026 {
		t.Errorf("run = %+v", r)
	}
}

// An unfinished run's conclusion is null.
func TestRunInProgress(t *testing.T) {
	fakeAPI(t, map[string]string{
		"GET /repos/o/r/actions/runs?event=push&head_sha=abc&per_page=100": `{"workflow_runs":[{"id":1,"status":"in_progress","conclusion":null}]}`,
	})
	runs, err := GH(context.Background(), "o/r", "abc")
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].Done() || runs[0].State() != "in_progress" {
		t.Errorf("runs = %+v", runs)
	}
}

func TestListRunsFollowsPagesToLimit(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "" {
			w.Header().Set("Link", `<`+srv.URL+r.URL.Path+`?page=2>; rel="next", <`+srv.URL+`/last>; rel="last"`)
			_, _ = w.Write([]byte(`{"workflow_runs":[{"id":1},{"id":2}]}`))
			return
		}
		w.Header().Set("Link", `<`+srv.URL+r.URL.Path+`?page=3>; rel="next"`)
		_, _ = w.Write([]byte(`{"workflow_runs":[{"id":3},{"id":4}]}`))
	}))
	defer srv.Close()
	t.Setenv("GITHUB_API_URL", srv.URL)
	t.Setenv("GH_TOKEN", "test-token")

	got, err := listRuns(context.Background(), "o/r", 3, url.Values{}, 3)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]int64, 0, len(got))
	for _, r := range got {
		ids = append(ids, r.ID)
	}
	if !slices.Equal(ids, []int64{1, 2, 3}) {
		t.Errorf("ids = %v", ids)
	}
}

func TestLatestSkipsPullRequestsAndFindsCrowdedOutWorkflows(t *testing.T) {
	fakeAPI(t, map[string]string{
		"GET /repos/o/r/actions/workflows?per_page=100": `{"workflows":[
			{"id":1,"name":"Tests","state":"active"},
			{"id":2,"name":"Nightly","state":"active"},
			{"id":3,"name":"Old","state":"disabled_manually"}]}`,
		"GET /repos/o/r/actions/runs?branch=main&per_page=100": `{"workflow_runs":[
			{"id":10,"workflow_id":1,"name":"Tests","event":"pull_request","created_at":"2026-01-03T00:00:00Z"},
			{"id":11,"workflow_id":1,"name":"Tests","event":"push","created_at":"2026-01-02T00:00:00Z"},
			{"id":12,"workflow_id":3,"name":"Old","event":"push","created_at":"2026-01-04T00:00:00Z"}]}`,
		"GET /repos/o/r/actions/workflows/2/runs?branch=main&per_page=100": `{"workflow_runs":[
			{"id":20,"workflow_id":2,"name":"Nightly","event":"schedule","created_at":"2026-01-01T00:00:00Z"}]}`,
	})
	runs, err := Latest(context.Background(), "o/r", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 || runs[0].ID != 20 || runs[1].ID != 11 {
		t.Errorf("runs = %+v", runs)
	}
}

// A dynamic workflow's runs have titles of their own for names, yet Dependabot's
// are found by the workflow's.
func TestLatestNamesRunsAfterWorkflows(t *testing.T) {
	fakeAPI(t, map[string]string{
		"GET /repos/o/r/actions/workflows?per_page=100":        `{"workflows":[{"id":1,"name":"Dependabot Updates","state":"active"}]}`,
		"GET /repos/o/r/actions/runs?branch=main&per_page=100": `{"workflow_runs":[{"id":10,"workflow_id":1,"name":"npm in / - Update #1","event":"dynamic"}]}`,
	})
	runs, err := Latest(context.Background(), "o/r", "main")
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].Workflow != DependabotWorkflow {
		t.Errorf("runs = %+v", runs)
	}
}

func TestLatestWithoutWorkflows(t *testing.T) {
	fakeAPI(t, map[string]string{"GET /repos/o/r/actions/workflows?per_page=100": `{"total_count":0,"workflows":[]}`})
	runs, err := Latest(context.Background(), "o/r", "main")
	if err != nil || len(runs) != 0 {
		t.Errorf("runs = %+v, err = %v", runs, err)
	}
}

func TestDefaultBranch(t *testing.T) {
	fakeAPI(t, map[string]string{"GET /repos/o/r": `{"default_branch":"trunk"}`})
	got, err := DefaultBranch(context.Background(), "o/r")
	if err != nil || got != "trunk" {
		t.Errorf("got %q, %v", got, err)
	}
}

// Only failed jobs' logs are fetched, in the shape Excerpts reads.
func TestFailedLog(t *testing.T) {
	calls := fakeAPI(t, map[string]string{
		"GET /repos/o/r/actions/runs/7/jobs?filter=latest&per_page=100": `{"jobs":[
			{"id":1,"name":"lint","conclusion":"success"},
			{"id":2,"name":"test","conclusion":"failure"}]}`,
		"GET /repos/o/r/actions/jobs/2/logs": "\xef\xbb\xbf2026-01-01T00:00:00.1Z ##[group]Run go test\r\n" +
			"2026-01-01T00:00:00.2Z ##[endgroup]\r\n" +
			"2026-01-01T00:00:00.3Z FAIL\twith a tab\r\n" +
			"2026-01-01T00:00:00.4Z ##[error]exit 1\r\n",
	})
	log, err := FailedLog(context.Background(), "o/r", 7)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(calls(), "GET /repos/o/r/actions/jobs/1/logs") {
		t.Error("fetched the log of a job that passed")
	}
	excerpts := Excerpts(log, 0)
	if len(excerpts) != 1 || excerpts[0].Job != "test" || excerpts[0].Step != "Run go test" {
		t.Fatalf("excerpts = %+v", excerpts)
	}
	if got := strings.Join(excerpts[0].Lines, "|"); got != "FAIL with a tab|##[error]exit 1" {
		t.Errorf("lines = %q", got)
	}
}

func TestServerErrorIsTriedAgain(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			http.Error(w, "bad gateway", http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"default_branch":"main"}`))
	}))
	defer srv.Close()
	t.Setenv("GITHUB_API_URL", srv.URL)
	t.Setenv("GH_TOKEN", "test-token")
	got, err := DefaultBranch(context.Background(), "o/r")
	if err != nil || got != "main" || requests.Load() != 2 {
		t.Errorf("got %q, %v after %d requests", got, err, requests.Load())
	}
}

// The API's refusals say why, and aren't worth repeating.
func TestRefusalKeepsMessageAndIsNotRetried(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
	}))
	defer srv.Close()
	t.Setenv("GITHUB_API_URL", srv.URL)
	t.Setenv("GH_TOKEN", "secret-token")
	_, err := DefaultBranch(context.Background(), "o/r")
	if err == nil || !strings.Contains(err.Error(), "API rate limit exceeded") || !strings.Contains(err.Error(), "403") {
		t.Errorf("err = %v", err)
	}
	if err != nil && strings.Contains(err.Error(), "secret-token") {
		t.Errorf("the error carries the token: %v", err)
	}
	if requests.Load() != 1 {
		t.Errorf("%d requests", requests.Load())
	}
}

func TestTokenPrecedence(t *testing.T) {
	t.Setenv("GH_TOKEN", "first")
	t.Setenv("GITHUB_TOKEN", "second")
	if got := Token(); got != "first" {
		t.Errorf("Token() = %q", got)
	}
	t.Setenv("GH_TOKEN", "")
	if got := Token(); got != "second" {
		t.Errorf("Token() = %q", got)
	}
	if err := CheckGH("ci"); err != nil {
		t.Error(err)
	}
}
