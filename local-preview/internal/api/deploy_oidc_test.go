package api

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/githuboidc"
	"github.com/jmelahman/local-preview/internal/githubsso"
)

// deployBody builds a POST /api/deploys request body for repo and ref.
func deployBody(repo, ref string) []byte {
	return []byte(`{"repo":"` + repo + `","ref":"` + ref + `"}`)
}

// doDeploy posts a deploy request through h with an optional bearer token.
func doDeploy(h http.Handler, token string, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/api/deploys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestDeployOIDCRouteGate covers which routes a GitHub Actions OIDC token may
// authenticate. The token proves only which workflow minted it, so it is an
// identity with no authority until a handler binds it to a repo. Creating a
// deploy and reading one back can do that; the rest must be refused outright
// even though the token verifies, or every repo's CI would be authorized for
// everything.
func TestDeployOIDCRouteGate(t *testing.T) {
	claims := githuboidc.Claims{Repository: "acme/app"}

	cases := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{"create deploy is allowed", "POST", "/api/deploys", http.StatusOK},
		{"reading one deploy is allowed", "GET", "/api/deploys/1", http.StatusOK},
		{"listing deploys is not", "GET", "/api/deploys", http.StatusUnauthorized},
		{"deploy logs are not", "GET", "/api/deploys/1/logs", http.StatusUnauthorized},
		{"deploy stats are not", "GET", "/api/deploys/1/stats", http.StatusUnauthorized},
		{"stopping a deploy is not", "POST", "/api/deploys/1/stop", http.StatusUnauthorized},
		{"deleting a deploy is not", "DELETE", "/api/deploys/1", http.StatusUnauthorized},
		{"registering a repo is not", "POST", "/api/repos", http.StatusUnauthorized},
		{"deleting a repo is not", "DELETE", "/api/repos/demo", http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := ssoDeps(t)
			d.UploadAuth = fakeVerifier{claims: claims}
			h := AuthMiddleware(d, okHandler())

			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Authorization", "Bearer some.jwt.token")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tc.want {
				t.Fatalf("got %d, want %d (%s)", rec.Code, tc.want, rec.Body)
			}
		})
	}
}

// TestDeployOIDCNotConfigured checks that an OIDC token buys nothing on a
// server with no verifier: without --github-oidc-audience there is nothing to
// check a token against, so it is just an unrecognized bearer token.
func TestDeployOIDCNotConfigured(t *testing.T) {
	d := ssoDeps(t) // UploadAuth left nil
	h := AuthMiddleware(d, okHandler())

	rec := doDeploy(h, "some.jwt.token", deployBody("demo", "main"))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401 (%s)", rec.Code, rec.Body)
	}
}

// TestDeployOIDCUnverifiable checks that a token the verifier rejects — wrong
// audience, bad signature, wrong issuer — does not fall through to the
// unauthenticated 401 path by some other route.
func TestDeployOIDCUnverifiable(t *testing.T) {
	d := ssoDeps(t)
	d.UploadAuth = fakeVerifier{err: errors.New("bad signature")}
	h := AuthMiddleware(d, okHandler())

	rec := doDeploy(h, "some.jwt.token", deployBody("demo", "main"))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401 (%s)", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q, want Bearer", got)
	}
}

// TestDeployOIDCRepoBinding is the substance of the gate: a verified token
// deploys only the repo whose registered source is the GitHub repository the
// token was minted for. Without this, any onyx-dot-app workflow could deploy
// any registered repo simply by naming it in the body.
func TestDeployOIDCRepoBinding(t *testing.T) {
	newMux := func(t *testing.T, tokenRepo string) http.Handler {
		t.Helper()
		deps, _ := newTestDeps(t)
		deps.SSO = fakeSSO{identity: githubsso.Identity{Login: "octocat"}}
		deps.DashboardOrigin = "http://localhost:8080"
		deps.UploadAuth = fakeVerifier{claims: githuboidc.Claims{Repository: tokenRepo}}
		// Inserted straight into the store rather than registered through the
		// API: the binding compares against a GitHub source URL, and going
		// through POST /api/repos would try to clone it for real.
		if _, err := deps.Store.CreateRepo("demo", "https://github.com/acme/app", t.TempDir(), db.RepoReady); err != nil {
			t.Fatal(err)
		}
		return AuthMiddleware(deps, NewMux(deps))
	}

	t.Run("token for the same repo is accepted", func(t *testing.T) {
		h := newMux(t, "acme/app")
		rec := doDeploy(h, "some.jwt.token", deployBody("demo", "main"))
		// Past the auth gate. The deploy itself may still fail on the ref,
		// which is not what this test is about — only a 401/403/404 would mean
		// the gate turned it away.
		if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
			t.Fatalf("got %d, want the request past the auth gate (%s)", rec.Code, rec.Body)
		}
	})

	t.Run("token for another repo is refused", func(t *testing.T) {
		h := newMux(t, "evil/other")
		rec := doDeploy(h, "some.jwt.token", deployBody("demo", "main"))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403 (%s)", rec.Code, rec.Body)
		}
	})

	t.Run("reading another repo's deploy is 404, not 403", func(t *testing.T) {
		// 404 on purpose: deploy IDs are sequential, so a 403 here would
		// distinguish "someone else's deploy" from "no such deploy" and make
		// the whole table enumerable by a token from any repo in the org.
		deps, _ := newTestDeps(t)
		deps.SSO = fakeSSO{identity: githubsso.Identity{Login: "octocat"}}
		deps.DashboardOrigin = "http://localhost:8080"
		deps.UploadAuth = fakeVerifier{claims: githuboidc.Claims{Repository: "evil/other"}}
		repo, err := deps.Store.CreateRepo("demo", "https://github.com/acme/app", t.TempDir(), db.RepoReady)
		if err != nil {
			t.Fatal(err)
		}
		dep, err := deps.Store.CreateDeploy(repo.ID, "0123456789abcdef0123456789abcdef01234567", db.DeployMeta{})
		if err != nil {
			t.Fatal(err)
		}
		h := AuthMiddleware(deps, NewMux(deps))

		req := httptest.NewRequest("GET", "/api/deploys/"+strconv.FormatInt(dep.ID, 10), nil)
		req.Header.Set("Authorization", "Bearer some.jwt.token")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404 (%s)", rec.Code, rec.Body)
		}
	})

	t.Run("unregistered repo is 404, not a probe", func(t *testing.T) {
		h := newMux(t, "acme/app")
		rec := doDeploy(h, "some.jwt.token", deployBody("nosuch", "main"))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404 (%s)", rec.Code, rec.Body)
		}
	})
}
