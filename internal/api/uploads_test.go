package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/githuboidc"
)

func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func doUpload(t *testing.T, mux *http.ServeMux, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/octet-stream")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

type uploadResp struct {
	SHA       string `json:"sha"`
	Side      string `json:"side"`
	Name      string `json:"name"`
	Hash      string `json:"hash"`
	Published bool   `json:"published"`
	Files     []struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	} `json:"files"`
}

func TestUploadEndpoints(t *testing.T) {
	mux, _ := newTestMux(t)
	src := newSourceRepo(t)
	manifest := fixtureManifest + `
[artifacts.cli]
path  = "srv"
build = [["sh", "-c", "echo built > mycli"]]
files = ["mycli"]
`
	if err := os.WriteFile(filepath.Join(src, "preview.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, src, "commit", "-qam", "declare artifact")
	registerRepo(t, mux, "demo", src)

	// Frontend upload primes the content store.
	fe := tarGz(t, map[string]string{"index.html": "UPLOADED"})
	rec := doUpload(t, mux, "/api/repos/demo/uploads/frontend?ref=main", fe)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload frontend: %d %s", rec.Code, rec.Body)
	}
	var res uploadResp
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if !res.Published || res.Side != "frontend" || res.Hash == "" {
		t.Fatalf("frontend upload response = %+v", res)
	}
	feHash := res.Hash

	// A second identical upload is a no-op.
	rec = doUpload(t, mux, "/api/repos/demo/uploads/frontend?ref=main", fe)
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Published {
		t.Fatalf("second upload should not have republished: %+v", res)
	}

	// Artifact upload.
	rec = doUpload(t, mux, "/api/repos/demo/uploads/artifacts/cli?ref=main", tarGz(t, map[string]string{"mycli": "UPLOADED-CLI"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("upload artifact: %d %s", rec.Code, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if !res.Published || len(res.Files) != 1 || res.Files[0].Name != "mycli" {
		t.Fatalf("artifact upload response = %+v", res)
	}
	artHash := res.Hash

	// Deploying the commit reuses both uploads: fe_hash matches, and the
	// artifact is ready at the uploaded hash without an artifact build phase.
	rec = doJSON(t, mux, "POST", "/api/deploys", `{"repo":"demo","ref":"main"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create deploy: %d %s", rec.Code, rec.Body)
	}
	var d struct {
		ID        int64  `json:"id"`
		Status    string `json:"status"`
		FeHash    string `json:"fe_hash"`
		Artifacts []struct {
			Name   string `json:"name"`
			Hash   string `json:"hash"`
			Status string `json:"status"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for d.Status != "ready" {
		if d.Status == "failed" || time.Now().After(deadline) {
			logs := doJSON(t, mux, "GET", "/api/deploys/1/logs", "")
			t.Fatalf("deploy status = %s; logs:\n%s", d.Status, logs.Body)
		}
		time.Sleep(50 * time.Millisecond)
		rec = doJSON(t, mux, "GET", "/api/deploys/1", "")
		if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
			t.Fatal(err)
		}
	}
	if d.FeHash != feHash {
		t.Fatalf("deploy fe_hash = %s, want the uploaded %s", d.FeHash, feHash)
	}
	if len(d.Artifacts) != 1 || d.Artifacts[0].Hash != artHash || d.Artifacts[0].Status != "ready" {
		t.Fatalf("deploy artifacts = %+v, want cli ready at %s", d.Artifacts, artHash)
	}

	// Error mapping.
	for _, tc := range []struct {
		name string
		path string
		code int
	}{
		{"unknown repo", "/api/repos/nope/uploads/frontend?ref=main", http.StatusNotFound},
		{"unknown artifact", "/api/repos/demo/uploads/artifacts/ghost?ref=main", http.StatusNotFound},
		{"missing ref", "/api/repos/demo/uploads/frontend", http.StatusBadRequest},
		{"bad ref", "/api/repos/demo/uploads/frontend?ref=no-such-branch", http.StatusBadRequest},
		{"bad overwrite", "/api/repos/demo/uploads/frontend?ref=main&overwrite=maybe", http.StatusBadRequest},
	} {
		if rec := doUpload(t, mux, tc.path, fe); rec.Code != tc.code {
			t.Fatalf("%s: got %d, want %d (%s)", tc.name, rec.Code, tc.code, rec.Body)
		}
	}
}

// TestUploadRejectsOversized covers both size caps on the upload path: the wire
// cap (compressed request body → 413 via MaxBytesReader) and the decompression
// cap (a small gzip expanding past the store's limit → 413 via
// store.ErrArchiveTooLarge). Neither may stream to disk.
func TestUploadRejectsOversized(t *testing.T) {
	t.Run("oversized request body is 413", func(t *testing.T) {
		deps, _ := newTestDeps(t)
		deps.MaxUploadBytes = 10 // any real tar.gz exceeds this on the wire
		mux := NewMux(deps)
		registerRepo(t, mux, "demo", newSourceRepo(t))
		fe := tarGz(t, map[string]string{"index.html": "hello world"})
		rec := doUpload(t, mux, "/api/repos/demo/uploads/frontend?ref=main", fe)
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("got %d, want 413 (%s)", rec.Code, rec.Body)
		}
	})

	t.Run("gzip bomb over the decompression cap is 413", func(t *testing.T) {
		deps, _ := newTestDeps(t)
		deps.MaxUploadBytes = 10 << 20      // wire cap far above the compressed body
		deps.Files.SetMaxExtractBytes(1024) // but the decompressed tree may not exceed 1 KiB
		mux := NewMux(deps)
		registerRepo(t, mux, "demo", newSourceRepo(t))
		// ~64 KiB of highly-compressible content: tiny on the wire, well past the cap decompressed.
		fe := tarGz(t, map[string]string{"index.html": strings.Repeat("A", 64<<10)})
		rec := doUpload(t, mux, "/api/repos/demo/uploads/frontend?ref=main", fe)
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("got %d, want 413 (%s)", rec.Code, rec.Body)
		}
	})
}

// fakeVerifier is a stand-in UploadVerifier so the auth tests need no network
// and no real GitHub token — it returns canned claims (or an error).
type fakeVerifier struct {
	claims githuboidc.Claims
	err    error
}

func (f fakeVerifier) Verify(context.Context, string) (githuboidc.Claims, error) {
	return f.claims, f.err
}

// doUploadTok is doUpload with an optional bearer token.
func doUploadTok(t *testing.T, mux *http.ServeMux, path, token string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/octet-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func newAuthMux(t *testing.T, v UploadVerifier) (*http.ServeMux, Deps) {
	t.Helper()
	deps, _ := newTestDeps(t)
	deps.UploadAuth = v
	return NewMux(deps), deps
}

// TestUploadOIDCAuth covers the auth gate on the upload endpoints when an
// OIDC verifier is configured: the token is verified before the repo is even
// looked up, and a verified token authorizes only the repo whose registered
// source is the same GitHub repository the token names.
func TestUploadOIDCAuth(t *testing.T) {
	fe := tarGz(t, map[string]string{"index.html": "x"})

	t.Run("missing token is 401", func(t *testing.T) {
		mux, _ := newAuthMux(t, fakeVerifier{claims: githuboidc.Claims{Repository: "acme/app"}})
		registerRepo(t, mux, "demo", newSourceRepo(t))
		rec := doUpload(t, mux, "/api/repos/demo/uploads/frontend?ref=main", fe)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401 (%s)", rec.Code, rec.Body)
		}
		if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
			t.Fatalf("WWW-Authenticate = %q, want Bearer", got)
		}
	})

	t.Run("invalid token is 403", func(t *testing.T) {
		mux, _ := newAuthMux(t, fakeVerifier{err: errors.New("bad signature")})
		registerRepo(t, mux, "demo", newSourceRepo(t))
		rec := doUploadTok(t, mux, "/api/repos/demo/uploads/frontend?ref=main", "some.jwt.token", fe)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403 (%s)", rec.Code, rec.Body)
		}
	})

	t.Run("unknown repo is 404 for an authenticated caller", func(t *testing.T) {
		mux, _ := newAuthMux(t, fakeVerifier{claims: githuboidc.Claims{Repository: "acme/app"}})
		rec := doUploadTok(t, mux, "/api/repos/nope/uploads/frontend?ref=main", "some.jwt.token", fe)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404 (%s)", rec.Code, rec.Body)
		}
	})

	t.Run("token for a different repo is 403", func(t *testing.T) {
		mux, deps := newAuthMux(t, fakeVerifier{claims: githuboidc.Claims{Repository: "attacker/app"}})
		mustCreateGitHubRepo(t, deps, "demo", "https://github.com/acme/app")
		rec := doUploadTok(t, mux, "/api/repos/demo/uploads/frontend?ref=main", "some.jwt.token", fe)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403 (%s)", rec.Code, rec.Body)
		}
	})

	t.Run("matching token passes the gate", func(t *testing.T) {
		mux, deps := newAuthMux(t, fakeVerifier{claims: githuboidc.Claims{Repository: "acme/app"}})
		// A github source with no on-disk mirror: the gate must pass (not
		// 401/403); the request then fails downstream in Queue.Upload. That is
		// enough to prove the token authorized the request — the happy-path
		// publish is covered by TestUploadEndpoints.
		mustCreateGitHubRepo(t, deps, "demo", "https://github.com/acme/app")
		rec := doUploadTok(t, mux, "/api/repos/demo/uploads/frontend?ref=main", "some.jwt.token", fe)
		if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
			t.Fatalf("gate blocked a matching token: %d (%s)", rec.Code, rec.Body)
		}
	})
}

// mustCreateGitHubRepo inserts a ready repo row with a GitHub source directly,
// bypassing the clone — the auth tests only need the source for the binding
// check, not a real mirror.
func mustCreateGitHubRepo(t *testing.T, deps Deps, name, source string) {
	t.Helper()
	if _, err := deps.Store.CreateRepo(name, source, deps.Git.Open(name).Path, db.RepoReady); err != nil {
		t.Fatal(err)
	}
}

func TestSourceMatchesGitHubRepo(t *testing.T) {
	const repo = "acme/app"
	for _, src := range []string{
		"https://github.com/acme/app",
		"https://github.com/acme/app.git",
		"git@github.com:acme/app.git",
		"ssh://git@github.com/acme/app.git",
		"https://github.com/Acme/App", // case-insensitive host/path
	} {
		if !sourceMatchesGitHubRepo(src, repo) {
			t.Errorf("source %q should match repository %q", src, repo)
		}
	}
	for _, tc := range []struct{ src, repo string }{
		{"https://github.com/acme/other", "acme/app"},
		{"https://gitlab.com/acme/app", "acme/app"},
		{"/local/path/app", "acme/app"},
		{"https://github.com/acme/app", ""},
	} {
		if sourceMatchesGitHubRepo(tc.src, tc.repo) {
			t.Errorf("source %q should NOT match repository %q", tc.src, tc.repo)
		}
	}
}
