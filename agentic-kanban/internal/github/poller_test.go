package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"
	"time"

	"github.com/jmelahman/kanban/internal/metrics"
)

func TestParseRemoteURL(t *testing.T) {
	cases := []struct {
		in    string
		owner string
		repo  string
		host  string
		bad   bool
	}{
		{in: "git@github.com:owner/repo.git", owner: "owner", repo: "repo", host: "github.com"},
		{in: "git@github.com:owner/repo", owner: "owner", repo: "repo", host: "github.com"},
		{in: "ssh://git@github.com/owner/repo.git", owner: "owner", repo: "repo", host: "github.com"},
		{in: "https://github.com/owner/repo.git", owner: "owner", repo: "repo", host: "github.com"},
		{in: "https://github.com/owner/repo", owner: "owner", repo: "repo", host: "github.com"},
		{in: "git@ghe.example.com:team/proj.git", owner: "team", repo: "proj", host: "ghe.example.com"},
		{in: "https://ghe.example.com/team/proj", owner: "team", repo: "proj", host: "ghe.example.com"},
		{in: "git@github.com:owner", bad: true},
		{in: "https://github.com/onlyone", bad: true},
		{in: "not a url", bad: true},
	}
	for _, c := range cases {
		owner, repo, host, err := parseRemoteURL(c.in)
		if c.bad {
			if err == nil {
				t.Errorf("parseRemoteURL(%q): want error, got %s/%s/%s", c.in, owner, repo, host)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseRemoteURL(%q): unexpected error: %v", c.in, err)
			continue
		}
		if owner != c.owner || repo != c.repo || host != c.host {
			t.Errorf("parseRemoteURL(%q): got %s/%s@%s, want %s/%s@%s",
				c.in, owner, repo, host, c.owner, c.repo, c.host)
		}
	}
}

func TestApiBaseFor(t *testing.T) {
	t.Setenv("GITHUB_API_URL", "")
	if got := APIBaseFor("github.com"); got != defaultAPIBase {
		t.Errorf("github.com: got %q, want %q", got, defaultAPIBase)
	}
	if got := APIBaseFor(""); got != defaultAPIBase {
		t.Errorf("empty host: got %q, want %q", got, defaultAPIBase)
	}
	if got := APIBaseFor("ghe.example.com"); got != "https://ghe.example.com/api/v3" {
		t.Errorf("GHE host: got %q", got)
	}
	t.Setenv("GITHUB_API_URL", "https://override.example/api/")
	if got := APIBaseFor("github.com"); got != "https://override.example/api" {
		t.Errorf("override: got %q", got)
	}
}

func TestParseNextLink(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "", want: ""},
		{
			in:   `<https://api.github.com/repositories/1/pulls?page=2>; rel="next", <https://api.github.com/repositories/1/pulls?page=5>; rel="last"`,
			want: "https://api.github.com/repositories/1/pulls?page=2",
		},
		{
			in:   `<https://api.github.com/repositories/1/pulls?page=5>; rel="last"`,
			want: "",
		},
	}
	for _, c := range cases {
		if got := ParseNextLink(c.in); got != c.want {
			t.Errorf("ParseNextLink(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestClassify(t *testing.T) {
	merged := "2024-01-01T00:00:00Z"
	cases := []struct {
		name string
		pr   ghPR
		want string
	}{
		{name: "open", pr: ghPR{State: "open"}, want: PRStateOpen},
		{name: "open draft", pr: ghPR{State: "open", Draft: true}, want: PRStateDraft},
		{name: "closed unmerged", pr: ghPR{State: "closed"}, want: PRStateClosed},
		{name: "closed merged", pr: ghPR{State: "closed", MergedAt: &merged}, want: PRStateMerged},
		{name: "unknown", pr: ghPR{State: ""}, want: ""},
	}
	for _, c := range cases {
		if got := classify(c.pr); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestTokenPrefersGHToken(t *testing.T) {
	resetTokenCache()
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "fallback")
	if got := Token(""); got != "fallback" {
		t.Errorf("fallback: got %q", got)
	}
	t.Setenv("GH_TOKEN", "primary")
	if got := Token(""); got != "primary" {
		t.Errorf("primary: got %q", got)
	}
}

func TestTokenFallsBackToGHCLI(t *testing.T) {
	resetTokenCache()
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")

	origLook, origRun := ghTokenLookPath, ghTokenRun
	t.Cleanup(func() {
		ghTokenLookPath, ghTokenRun = origLook, origRun
		resetTokenCache()
	})

	ghTokenLookPath = func(string) (string, error) { return "/usr/bin/gh", nil }
	calls := 0
	var lastArgs []string
	ghTokenRun = func(name string, args ...string) ([]byte, error) {
		calls++
		lastArgs = args
		return []byte("ghcli-token\n"), nil
	}

	if got := Token("github.com"); got != "ghcli-token" {
		t.Errorf("github.com: got %q", got)
	}
	if got := Token("github.com"); got != "ghcli-token" {
		t.Errorf("cached: got %q", got)
	}
	if calls != 1 {
		t.Errorf("expected 1 gh exec, got %d", calls)
	}

	if got := Token("ghe.example.com"); got != "ghcli-token" {
		t.Errorf("ghe: got %q", got)
	}
	if calls != 2 {
		t.Errorf("expected 2 gh execs after ghe lookup, got %d", calls)
	}
	want := []string{"auth", "token", "--hostname", "ghe.example.com"}
	if len(lastArgs) != len(want) {
		t.Fatalf("ghe args: got %v, want %v", lastArgs, want)
	}
	for i := range want {
		if lastArgs[i] != want[i] {
			t.Fatalf("ghe args: got %v, want %v", lastArgs, want)
		}
	}
}

func TestListPRsUsesETagOn304(t *testing.T) {
	const etag = `W/"pr-page-1"`
	var (
		hits        int
		gotIfNone   string
		modifyAfter int // after this many hits, start returning 200 again
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		gotIfNone = r.Header.Get("If-None-Match")
		w.Header().Set("ETag", etag)
		// No Link header → single-page response, so prCache caches one URL.
		if gotIfNone == etag && hits != modifyAfter {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `[{"number":1,"html_url":"https://x/1","title":"t","state":"open","head":{"ref":"feature/x"}}]`)
	}))
	t.Cleanup(srv.Close)

	repoDir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"remote", "add", "origin", "git@github.com:owner/repo.git"},
	} {
		cmd := exec.Command("git", append([]string{"-C", repoDir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	t.Setenv("GITHUB_API_URL", srv.URL)
	t.Setenv("GH_TOKEN", "dummy")

	p := &Poller{http: &http.Client{Timeout: 5 * time.Second, Transport: metrics.WrapGitHubTransport(nil)}}
	ctx := context.Background()

	// First poll populates the cache.
	prs, err := p.listPRs(ctx, repoDir)
	if err != nil {
		t.Fatalf("first listPRs: %v", err)
	}
	if len(prs) != 1 || prs[0].Number != 1 {
		t.Fatalf("first listPRs: got %+v", prs)
	}
	if gotIfNone != "" {
		t.Errorf("first request sent If-None-Match=%q, want empty", gotIfNone)
	}

	// Second poll should send the ETag and reuse the cached body on 304.
	prs, err = p.listPRs(ctx, repoDir)
	if err != nil {
		t.Fatalf("second listPRs: %v", err)
	}
	if len(prs) != 1 || prs[0].Number != 1 {
		t.Fatalf("cached listPRs: got %+v", prs)
	}
	if gotIfNone != etag {
		t.Errorf("second request If-None-Match=%q, want %q", gotIfNone, etag)
	}
	if hits != 2 {
		t.Errorf("hits=%d, want 2 (one fresh + one revalidate)", hits)
	}

	// Force a 200 so the cache refreshes; subsequent revalidate should still 304.
	modifyAfter = 3
	if _, err := p.listPRs(ctx, repoDir); err != nil {
		t.Fatalf("refresh listPRs: %v", err)
	}
	if _, err := p.listPRs(ctx, repoDir); err != nil {
		t.Fatalf("post-refresh listPRs: %v", err)
	}
	if hits != 4 {
		t.Errorf("hits=%d, want 4", hits)
	}
}

func resetTokenCache() {
	ghTokenCache.Range(func(k, _ any) bool {
		ghTokenCache.Delete(k)
		return true
	})
}
