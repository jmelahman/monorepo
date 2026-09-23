package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testWebhookSecret = "hook-secret"

// deliver posts a signed webhook the way GitHub does: JSON body, event
// header, sha256 HMAC signature.
func deliver(t *testing.T, mux *http.ServeMux, event, body string, sign bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/webhooks/github", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", event)
	if sign {
		mac := hmac.New(sha256.New, []byte(testWebhookSecret))
		mac.Write([]byte(body))
		req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// pushPayload builds a minimal GitHub push event whose clone_url is the
// registered source (a local path in tests — the matcher only needs
// equality after normalization).
func pushPayload(source, ref, after string, deleted bool) string {
	return fmt.Sprintf(
		`{"ref":%q,"after":%q,"deleted":%v,"repository":{"full_name":"acme/demo","clone_url":%q}}`,
		ref, after, deleted, source)
}

func TestWebhookPushDeploys(t *testing.T) {
	mux, _ := newTestMux(t)
	src := newSourceRepo(t)
	sha := runTestGit(t, src, "rev-parse", "HEAD")
	registerRepo(t, mux, "demo", src)

	rec := deliver(t, mux, "push", pushPayload(src, "refs/heads/main", sha, false), true)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("push: %d %s", rec.Code, rec.Body)
	}
	var d struct {
		SHA    string `json:"sha"`
		Branch string `json:"branch"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if d.SHA != sha || d.Branch != "main" {
		t.Fatalf("deploy = %+v, want sha %s on main", d, sha)
	}

	// Redelivery is idempotent: same sha, no second deploy row.
	if rec := deliver(t, mux, "push", pushPayload(src, "refs/heads/main", sha, false), true); rec.Code != http.StatusAccepted {
		t.Fatalf("redelivery: %d %s", rec.Code, rec.Body)
	}
	rec = doJSON(t, mux, "GET", "/api/deploys", "")
	var list []json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 1 {
		t.Fatalf("deploys after redelivery = %d (%v)", len(list), err)
	}

	// Drain the triggered build so teardown doesn't race the worker.
	deadline := time.Now().Add(30 * time.Second)
	for {
		var d struct {
			Status string `json:"status"`
		}
		rec := doJSON(t, mux, "GET", "/api/deploys/1", "")
		if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
			t.Fatal(err)
		}
		if d.Status != "queued" && d.Status != "building" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("deploy stuck in %s", d.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestWebhookRejectsBadSignatures(t *testing.T) {
	mux, _ := newTestMux(t)
	body := pushPayload("/nowhere", "refs/heads/main", strings.Repeat("a", 40), false)

	if rec := deliver(t, mux, "push", body, false); rec.Code != http.StatusForbidden {
		t.Fatalf("unsigned: %d, want 403", rec.Code)
	}

	req := httptest.NewRequest("POST", "/api/webhooks/github", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", "sha256="+strings.Repeat("00", 32))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("wrong signature: %d, want 403", rec.Code)
	}
}

func TestWebhookDisabledWithoutSecret(t *testing.T) {
	mux := NewMux(Deps{})
	rec := deliver(t, mux, "push", "{}", true)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("no secret: %d, want 503", rec.Code)
	}
}

func TestWebhookIgnoresNonDeployEvents(t *testing.T) {
	mux, _ := newTestMux(t)

	if rec := deliver(t, mux, "ping", `{"zen":"keep it simple"}`, true); rec.Code != http.StatusOK {
		t.Fatalf("ping: %d %s", rec.Code, rec.Body)
	}
	for _, c := range []struct {
		name, event, payload string
	}{
		{"tag push", "push", pushPayload("/src", "refs/tags/v1", strings.Repeat("a", 40), false)},
		{"branch delete", "push", pushPayload("/src", "refs/heads/gone", strings.Repeat("0", 40), true)},
		{"issues event", "issues", `{"action":"opened"}`},
	} {
		rec := deliver(t, mux, c.event, c.payload, true)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ignored") {
			t.Fatalf("%s: %d %s, want ignored", c.name, rec.Code, rec.Body)
		}
	}

	rec := deliver(t, mux, "push",
		pushPayload("/not/registered", "refs/heads/main", strings.Repeat("a", 40), false), true)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unmatched repo: %d, want 404", rec.Code)
	}
}

func TestWebhookBranchFilter(t *testing.T) {
	mux, _ := newTestMux(t)
	src := newSourceRepo(t)
	sha := runTestGit(t, src, "rev-parse", "HEAD")

	// The filter governs webhooks even with watch off: exclude main, allow
	// everything else.
	rec := doJSON(t, mux, "POST", "/api/repos",
		`{"name":"demo","source":`+jsonQuote(src)+`,"watch_branches":"!main"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create repo: %d %s", rec.Code, rec.Body)
	}
	waitRepoStatus(t, mux, "demo", "ready")

	// An excluded branch is acknowledged but not deployed.
	rec = deliver(t, mux, "push", pushPayload(src, "refs/heads/main", sha, false), true)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "excluded by filter") {
		t.Fatalf("excluded push: %d %s, want 200 ignored", rec.Code, rec.Body)
	}
	if rec := doJSON(t, mux, "GET", "/api/deploys", ""); !strings.Contains(rec.Body.String(), "[]") {
		t.Fatalf("excluded branch created a deploy: %s", rec.Body)
	}

	// A non-excluded branch still deploys.
	rec = deliver(t, mux, "push", pushPayload(src, "refs/heads/feature", sha, false), true)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("allowed push: %d %s", rec.Code, rec.Body)
	}

	// Drain the triggered build so teardown doesn't race the worker.
	deadline := time.Now().Add(30 * time.Second)
	for {
		var d struct {
			Status string `json:"status"`
		}
		rec := doJSON(t, mux, "GET", "/api/deploys/1", "")
		if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
			t.Fatal(err)
		}
		if d.Status != "queued" && d.Status != "building" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("deploy stuck in %s", d.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestNormalizeGitURL(t *testing.T) {
	want := "github.com/acme/app"
	for _, u := range []string{
		"https://github.com/acme/app.git",
		"https://github.com/acme/app",
		"http://github.com/Acme/App.git",
		"git://github.com/acme/app.git",
		"git@github.com:acme/app.git",
		"ssh://git@github.com/acme/app",
		"https://github.com/acme/app/",
	} {
		if got := normalizeGitURL(u); got != want {
			t.Errorf("normalizeGitURL(%q) = %q, want %q", u, got, want)
		}
	}
	if got := normalizeGitURL("/home/dev/app"); got != "/home/dev/app" {
		t.Errorf("local path mangled: %q", got)
	}
	if normalizeGitURL("https://github.com/acme/app") == normalizeGitURL("https://github.com/acme/other") {
		t.Error("distinct repos compare equal")
	}
}
