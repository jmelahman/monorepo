package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/githuboidc"
	"github.com/jmelahman/local-preview/internal/githubsso"
)

const (
	// sessionCookieName holds the apex (dashboard) session token. Host-only,
	// HttpOnly — the proxy's preview access uses a different cookie so
	// untrusted previewed code can never present the dashboard credential.
	sessionCookieName = "preview_session"
	// oauthStateCookieName carries the OAuth state (and the post-login return
	// path) across the GitHub redirect, defending the handshake against CSRF.
	oauthStateCookieName = "preview_oauth_state"

	// sessionScopeApex is the sessions.scope value for dashboard sessions.
	sessionScopeApex = "apex"

	// sessionTTL is how long a login lasts before re-authentication.
	sessionTTL = 7 * 24 * time.Hour
	// stateTTL bounds how long an in-flight OAuth handshake may take.
	stateTTL = 10 * time.Minute
	// grantTTL bounds an apex→preview handoff code's lifetime.
	grantTTL = 2 * time.Minute
	// patCacheTTL bounds how long a personal-access-token verification result
	// is reused, to keep bearer-authenticated requests off GitHub's rate limit.
	patCacheTTL = 2 * time.Minute
)

// handleAuthLogin starts the GitHub OAuth web flow: it mints a state, stashes
// it (with the sanitized return path) in a short-lived HttpOnly cookie, and
// redirects to GitHub. SameSite=Lax so the cookie survives the top-level
// navigation back from github.com.
func (d Deps) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if d.SSO == nil {
		httpError(w, http.StatusNotFound, "sso is not configured")
		return
	}
	state, _, err := newOpaqueToken()
	if err != nil {
		internalError(w, "generate state", err)
		return
	}
	returnTo := sanitizeReturnPath(r.URL.Query().Get("return_to"))
	setCookie(w, oauthStateCookieName, state+"|"+returnTo, "", d.CookiesSecure, stateTTL)
	http.Redirect(w, r, d.SSO.AuthCodeURL(state), http.StatusFound)
}

// handleAuthCallback completes the flow: it checks the state, exchanges the
// code for an allowlisted identity, starts a session, and redirects home.
func (d Deps) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	if d.SSO == nil {
		httpError(w, http.StatusNotFound, "sso is not configured")
		return
	}
	c, err := r.Cookie(oauthStateCookieName)
	if err != nil {
		httpError(w, http.StatusBadRequest, "missing oauth state")
		return
	}
	clearCookie(w, oauthStateCookieName, "", d.CookiesSecure)
	wantState, returnTo, _ := strings.Cut(c.Value, "|")
	gotState := r.URL.Query().Get("state")
	if wantState == "" || subtle.ConstantTimeCompare([]byte(wantState), []byte(gotState)) != 1 {
		httpError(w, http.StatusBadRequest, "invalid oauth state")
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		httpError(w, http.StatusBadRequest, "missing authorization code")
		return
	}
	id, err := d.SSO.Exchange(r.Context(), code)
	if errors.Is(err, githubsso.ErrNotAllowed) {
		httpError(w, http.StatusForbidden, "your github account is not authorized for this instance")
		return
	}
	if err != nil {
		internalError(w, "oauth exchange", err)
		return
	}
	if err := d.startSession(w, id); err != nil {
		internalError(w, "create session", err)
		return
	}
	http.Redirect(w, r, sanitizeReturnPath(returnTo), http.StatusFound)
}

// handleAuthLogout deletes the current session and clears its cookie. It is a
// no-op when there is no session, and safe to call whether or not SSO is on.
func (d Deps) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookieName); err == nil {
		_ = d.Store.DeleteSessionByTokenHash(sessionScopeApex, hashToken(c.Value))
	}
	clearCookie(w, sessionCookieName, "", d.CookiesSecure)
	w.WriteHeader(http.StatusNoContent)
}

// handleAuthMe reports the caller's identity. With SSO off it returns
// {"anonymous":true} so the dashboard renders exactly as before; with SSO on
// it returns the signed-in identity or 401.
func (d Deps) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	if d.SSO == nil {
		writeJSON(w, http.StatusOK, map[string]any{"anonymous": true})
		return
	}
	sess, ok := d.apexSession(r)
	if !ok {
		httpError(w, http.StatusUnauthorized, "not signed in")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"login":      sess.GitHubLogin,
		"email":      sess.Email,
		"avatar_url": sess.AvatarURL,
	})
}

// handleAuthPreviewGrant mints a single-use code the proxy redeems to grant
// preview access to an already-signed-in user. It only ever redirects to a
// preview URL (open-redirect guard), and bounces through login when there is
// no apex session yet.
func (d Deps) handleAuthPreviewGrant(w http.ResponseWriter, r *http.Request) {
	if d.SSO == nil {
		httpError(w, http.StatusNotFound, "sso is not configured")
		return
	}
	returnTo := r.URL.Query().Get("return_to")
	if !d.isPreviewURL(returnTo) {
		httpError(w, http.StatusBadRequest, "return_to must be a preview URL")
		return
	}
	sess, ok := d.apexSession(r)
	if !ok {
		self := "/api/auth/preview-grant?return_to=" + url.QueryEscape(returnTo)
		http.Redirect(w, r, "/api/auth/login?return_to="+url.QueryEscape(self), http.StatusFound)
		return
	}
	code, hash, err := newOpaqueToken()
	if err != nil {
		internalError(w, "generate grant", err)
		return
	}
	if err := d.Store.CreatePreviewGrant(hash, sess.ID, grantTTL); err != nil {
		internalError(w, "create preview grant", err)
		return
	}
	sep := "?"
	if strings.Contains(returnTo, "?") {
		sep = "&"
	}
	http.Redirect(w, r, returnTo+sep+"preview_auth="+url.QueryEscape(code), http.StatusFound)
}

// startSession creates an apex session for id and sets its host-only cookie.
func (d Deps) startSession(w http.ResponseWriter, id githubsso.Identity) error {
	raw, hash, err := newOpaqueToken()
	if err != nil {
		return err
	}
	if _, err := d.Store.CreateSession(db.Session{
		TokenHash:    hash,
		Scope:        sessionScopeApex,
		GitHubLogin:  id.Login,
		GitHubUserID: id.UserID,
		Email:        id.Email,
		AvatarURL:    id.AvatarURL,
	}, sessionTTL); err != nil {
		return err
	}
	setCookie(w, sessionCookieName, raw, "", d.CookiesSecure, sessionTTL)
	return nil
}

// apexSession returns the valid apex session behind the request's cookie.
func (d Deps) apexSession(r *http.Request) (db.Session, bool) {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return db.Session{}, false
	}
	sess, err := d.Store.GetSessionByTokenHash(sessionScopeApex, hashToken(c.Value))
	if err != nil {
		return db.Session{}, false
	}
	return sess, true
}

// isPreviewURL reports whether raw is an http(s) URL on the preview domain (or
// one of its subdomains) — the only redirect target preview-grant will honor.
func (d Deps) isPreviewURL(raw string) bool {
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	dom := strings.ToLower(d.Config.Preview.Domain)
	return dom != "" && (host == dom || strings.HasSuffix(host, "."+dom))
}

// AuthMiddleware gates /api/* behind a GitHub identity when SSO is configured;
// it returns next unchanged when SSO is off, so an unconfigured instance keeps
// its historical open API. A request is allowed when it carries either a valid
// personal-access token (Authorization: Bearer — CSRF-immune, so no Origin
// check) or a valid apex session cookie (with an Origin check on state-changing
// methods). Login, health, the HMAC-authenticated webhook, the OIDC-gated
// uploads, and all non-API paths are exempt.
func AuthMiddleware(d Deps, next http.Handler) http.Handler {
	if d.SSO == nil {
		return next
	}
	cache := &patCache{entries: map[string]patEntry{}}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") || isAuthExempt(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if tok, ok := bearerToken(r); ok {
			if cache.allowed(r.Context(), d.SSO, tok) {
				next.ServeHTTP(w, r)
				return
			}
			// Not a personal-access token. It may still be a GitHub Actions
			// OIDC token, which is what CI has instead of a session — but only
			// on a route that can bind it to a repo (see oidcRoute). The claims
			// ride the context so the handler can enforce that binding without
			// verifying a second time.
			if claims, ok := d.verifyOIDC(r, tok); ok {
				next.ServeHTTP(w, r.WithContext(withOIDCClaims(r.Context(), claims)))
				return
			}
			w.Header().Set("WWW-Authenticate", "Bearer")
			httpError(w, http.StatusUnauthorized, "invalid or unauthorized token")
			return
		}
		if _, ok := d.apexSession(r); ok {
			if isStateChanging(r.Method) && !d.originAllowed(r) {
				httpError(w, http.StatusForbidden, "request origin not allowed")
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("WWW-Authenticate", "Bearer")
		httpError(w, http.StatusUnauthorized, "authentication required")
	})
}

// isAuthExempt lists the API paths that never require a session: liveness, the
// auth flow itself, the HMAC-authenticated webhook, and the uploads (which
// self-gate on GitHub Actions OIDC).
func isAuthExempt(p string) bool {
	switch {
	case p == "/api/health":
		return true
	case strings.HasPrefix(p, "/api/auth/"):
		return true
	case p == "/api/webhooks/github":
		return true
	case strings.HasPrefix(p, "/api/repos/") && strings.Contains(p, "/uploads/"):
		return true
	}
	return false
}

// oidcCtxKey types the context key carrying verified OIDC claims.
type oidcCtxKey struct{}

// withOIDCClaims marks a request as OIDC-authenticated, carrying the verified
// claims to the handler that must bind them to a repo.
func withOIDCClaims(ctx context.Context, claims githuboidc.Claims) context.Context {
	return context.WithValue(ctx, oidcCtxKey{}, claims)
}

// oidcClaimsFrom returns the verified claims the middleware attached, if the
// request authenticated with a GitHub Actions OIDC token rather than a session
// or a personal-access token.
func oidcClaimsFrom(ctx context.Context) (githuboidc.Claims, bool) {
	claims, ok := ctx.Value(oidcCtxKey{}).(githuboidc.Claims)
	return claims, ok
}

// verifyOIDC verifies tok as a GitHub Actions OIDC token, but only for requests
// on an OIDC-eligible route. Gating on the route matters: a valid token proves
// only which workflow minted it, so it is an identity with no authority until a
// handler checks it against the repo being acted on. Letting one past on a route
// with nothing to bind it to would authorize every repo's CI for everything.
func (d Deps) verifyOIDC(r *http.Request, tok string) (githuboidc.Claims, bool) {
	if d.UploadAuth == nil || !oidcRoute(r.Method, r.URL.Path) {
		return githuboidc.Claims{}, false
	}
	claims, err := d.UploadAuth.Verify(r.Context(), tok)
	if err != nil {
		return githuboidc.Claims{}, false
	}
	return claims, true
}

// oidcRoute lists the non-exempt routes a GitHub Actions OIDC token may
// authenticate — the two `preview upload … --oidc --deploy` needs: creating a
// deploy, and reading it back until it settles. Both name a repo the handler
// can bind the token to, the first in its body and the second on the row it
// loads. The uploads are OIDC-gated too but never reach here — they are exempt
// above and self-gate instead.
//
// Deliberately not the mutating by-ID routes (stop, delete) or the log and
// stats reads: CI has no reason to touch them, and every route added here is
// one more place the binding has to be gotten right.
func oidcRoute(method, path string) bool {
	switch method {
	case http.MethodPost:
		return path == "/api/deploys"
	case http.MethodGet:
		id, ok := strings.CutPrefix(path, "/api/deploys/")
		// Digits only, so this matches the deploy itself and not the /logs,
		// /stats, or /artifacts paths beneath it.
		return ok && id != "" && strings.TrimLeft(id, "0123456789") == ""
	}
	return false
}

// isStateChanging reports whether a method mutates state and so warrants the
// CSRF Origin check.
func isStateChanging(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	return true
}

// originAllowed reports whether a cookie-authenticated state-changing request
// originates from the dashboard itself. It rejects requests with no Origin/
// Referer and, crucially, requests originating from preview subdomains.
func (d Deps) originAllowed(r *http.Request) bool {
	if o := r.Header.Get("Origin"); o != "" {
		return strings.EqualFold(o, d.DashboardOrigin)
	}
	if ref := r.Header.Get("Referer"); ref != "" {
		if u, err := url.Parse(ref); err == nil && u.Scheme != "" && u.Host != "" {
			return strings.EqualFold(u.Scheme+"://"+u.Host, d.DashboardOrigin)
		}
	}
	return false
}

// patCache memoizes personal-access-token verifications for a short window so
// a stream of bearer-authenticated CLI requests doesn't hit the GitHub API on
// every call. It caches definitive outcomes (allowed / not-allowed) but never
// transient failures, so a GitHub blip doesn't lock a valid token out.
type patCache struct {
	mu      sync.Mutex
	entries map[string]patEntry
}

type patEntry struct {
	allowed bool
	expires time.Time
}

func (c *patCache) allowed(ctx context.Context, sso SSOProvider, token string) bool {
	key := hashToken(token)
	now := time.Now()
	c.mu.Lock()
	if e, ok := c.entries[key]; ok && now.Before(e.expires) {
		c.mu.Unlock()
		return e.allowed
	}
	c.mu.Unlock()

	_, err := sso.VerifyToken(ctx, token)
	switch {
	case err == nil:
		c.store(key, true, now)
		return true
	case errors.Is(err, githubsso.ErrNotAllowed):
		c.store(key, false, now)
		return false
	default:
		// Transient (network, unexpected status): don't cache, don't allow.
		return false
	}
}

func (c *patCache) store(key string, allowed bool, now time.Time) {
	c.mu.Lock()
	c.entries[key] = patEntry{allowed: allowed, expires: now.Add(patCacheTTL)}
	c.mu.Unlock()
}

// newOpaqueToken returns a 256-bit random token and its sha256 hex hash. The
// raw value goes in a cookie or URL; only the hash is ever persisted.
func newOpaqueToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, hashToken(raw), nil
}

// hashToken is the one-way mapping from a raw token to its stored form.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// setCookie writes an HttpOnly, SameSite=Lax cookie. domain "" scopes it to
// the exact host (the apex session and OAuth state are host-only).
func setCookie(w http.ResponseWriter, name, value, domain string, secure bool, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Domain:   domain,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(ttl),
		MaxAge:   int(ttl.Seconds()),
	})
}

// clearCookie expires a cookie previously set with setCookie.
func clearCookie(w http.ResponseWriter, name, domain string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		Domain:   domain,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}

// sanitizeReturnPath keeps only a same-origin absolute path, defusing
// open-redirects: absolute URLs and protocol-relative ("//host") targets
// collapse to "/".
func sanitizeReturnPath(s string) string {
	if s == "" || !strings.HasPrefix(s, "/") || strings.HasPrefix(s, "//") {
		return "/"
	}
	if u, err := url.Parse(s); err != nil || u.Scheme != "" || u.Host != "" {
		return "/"
	}
	return s
}
