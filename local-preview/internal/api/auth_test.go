package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jmelahman/local-preview/internal/config"
	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/githubsso"
)

// fakeSSO stands in for *githubsso.Provider so middleware/handler tests need
// no network or GitHub app. VerifyToken accepts exactly "goodtoken"; Exchange
// returns identity unless exchangeErr is set.
type fakeSSO struct {
	identity    githubsso.Identity
	exchangeErr error
}

func (f fakeSSO) AuthCodeURL(state string) string {
	return "https://github.com/login/oauth/authorize?state=" + state
}

func (f fakeSSO) Exchange(_ context.Context, _ string) (githubsso.Identity, error) {
	if f.exchangeErr != nil {
		return githubsso.Identity{}, f.exchangeErr
	}
	return f.identity, nil
}

func (f fakeSSO) VerifyToken(_ context.Context, token string) (githubsso.Identity, error) {
	if token == "goodtoken" {
		return f.identity, nil
	}
	return githubsso.Identity{}, githubsso.ErrNotAllowed
}

func ssoDeps(t *testing.T) Deps {
	t.Helper()
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return Deps{
		Store:           store,
		SSO:             fakeSSO{identity: githubsso.Identity{Login: "octocat", UserID: 1, Email: "o@x.com"}},
		DashboardOrigin: "http://localhost:8080",
		Config:          config.Config{Preview: config.PreviewBase{Domain: "preview.localhost"}},
	}
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestMeAnonymousWhenSSOOff(t *testing.T) {
	rec := httptest.NewRecorder()
	Deps{}.handleAuthMe(rec, httptest.NewRequest("GET", "/api/auth/me", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"anonymous":true`) {
		t.Fatalf("want 200 anonymous, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestMiddlewareNoopWhenSSOOff(t *testing.T) {
	h := AuthMiddleware(Deps{}, okHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/api/deploys", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want passthrough 200, got %d", rec.Code)
	}
}

func TestMiddlewareRequiresAuth(t *testing.T) {
	h := AuthMiddleware(ssoDeps(t), okHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/deploys", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Fatal("want WWW-Authenticate header")
	}
}

func TestMiddlewareBearerSkipsOriginCheck(t *testing.T) {
	h := AuthMiddleware(ssoDeps(t), okHandler())
	// A hostile Origin on a state-changing request must NOT matter for a
	// bearer token: PATs aren't attached by browsers, so they're CSRF-immune.
	req := httptest.NewRequest("POST", "/api/deploys", nil)
	req.Header.Set("Authorization", "Bearer goodtoken")
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 for valid bearer, got %d", rec.Code)
	}
}

func TestMiddlewareRejectsBadBearer(t *testing.T) {
	h := AuthMiddleware(ssoDeps(t), okHandler())
	req := httptest.NewRequest("GET", "/api/deploys", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestMiddlewareSessionOriginCheck(t *testing.T) {
	d := ssoDeps(t)
	setup := httptest.NewRecorder()
	if err := d.startSession(setup, githubsso.Identity{Login: "octocat", UserID: 1}); err != nil {
		t.Fatal(err)
	}
	cookie := findCookie(setup.Result().Cookies(), sessionCookieName)
	if cookie == nil {
		t.Fatal("no session cookie")
	}
	h := AuthMiddleware(d, okHandler())

	// GET needs no Origin.
	get := httptest.NewRequest("GET", "/api/deploys", nil)
	get.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, get)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET want 200, got %d", rec.Code)
	}

	// State-changing request with a foreign Origin is rejected.
	bad := httptest.NewRequest("POST", "/api/deploys", nil)
	bad.AddCookie(cookie)
	bad.Header.Set("Origin", "https://evil.example.com")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, bad)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("POST foreign origin want 403, got %d", rec.Code)
	}

	// Same request from the dashboard origin is allowed.
	good := httptest.NewRequest("POST", "/api/deploys", nil)
	good.AddCookie(cookie)
	good.Header.Set("Origin", d.DashboardOrigin)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, good)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST dashboard origin want 200, got %d", rec.Code)
	}
}

func TestMiddlewareExemptPaths(t *testing.T) {
	h := AuthMiddleware(ssoDeps(t), okHandler())
	cases := []struct {
		method, path string
	}{
		{"GET", "/api/health"},
		{"GET", "/api/auth/login"},
		{"POST", "/api/webhooks/github"},
		{"POST", "/api/repos/demo/uploads/frontend"},
		{"GET", "/"},            // SPA
		{"GET", "/assets/x.js"}, // static asset
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(c.method, c.path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s: want passthrough 200, got %d", c.method, c.path, rec.Code)
		}
	}
}

func TestCallbackHappyPath(t *testing.T) {
	d := ssoDeps(t)
	state, cookie := loginState(t, d)
	req := httptest.NewRequest("GET", "/api/auth/callback?state="+url.QueryEscape(state)+"&code=abc", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	d.handleAuthCallback(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("want 302, got %d (%s)", rec.Code, rec.Body.String())
	}
	if findCookie(rec.Result().Cookies(), sessionCookieName) == nil {
		t.Fatal("callback set no session cookie")
	}
}

func TestCallbackRejectsBadState(t *testing.T) {
	d := ssoDeps(t)
	_, cookie := loginState(t, d)
	req := httptest.NewRequest("GET", "/api/auth/callback?state=WRONG&code=abc", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	d.handleAuthCallback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for bad state, got %d", rec.Code)
	}
}

func TestCallbackNotAllowed(t *testing.T) {
	d := ssoDeps(t)
	d.SSO = fakeSSO{exchangeErr: githubsso.ErrNotAllowed}
	state, cookie := loginState(t, d)
	req := httptest.NewRequest("GET", "/api/auth/callback?state="+url.QueryEscape(state)+"&code=abc", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	d.handleAuthCallback(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403 for disallowed account, got %d", rec.Code)
	}
}

// loginState runs handleAuthLogin and returns the OAuth state and its cookie.
func loginState(t *testing.T, d Deps) (string, *http.Cookie) {
	t.Helper()
	rec := httptest.NewRecorder()
	d.handleAuthLogin(rec, httptest.NewRequest("GET", "/api/auth/login", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("login want 302, got %d", rec.Code)
	}
	cookie := findCookie(rec.Result().Cookies(), oauthStateCookieName)
	if cookie == nil {
		t.Fatal("login set no state cookie")
	}
	state, _, _ := strings.Cut(cookie.Value, "|")
	return state, cookie
}
