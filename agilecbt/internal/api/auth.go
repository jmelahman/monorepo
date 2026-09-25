package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
)

// sessionCookie holds a token derived from the secret, so rotating
// APP_SECRET logs out every browser.
const sessionCookie = "agilecbt_session"

// sessionToken derives the cookie value from the secret. It is not the secret
// itself, so a leaked cookie doesn't reveal the password.
func sessionToken(secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("agilecbt-session-v1"))
	return hex.EncodeToString(mac.Sum(nil))
}

func equal(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// authenticated reports whether r carries a valid cookie or bearer token.
// Always true when auth is disabled.
func (d Deps) authenticated(r *http.Request) bool {
	if d.Secret == "" {
		return true
	}
	if bearer, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
		return equal(bearer, d.Secret)
	}
	c, err := r.Cookie(sessionCookie)
	return err == nil && equal(c.Value, sessionToken(d.Secret))
}

// requireAuth guards /api/ and /mcp. The SPA's static files, health, and
// login stay open so the browser can render the login screen.
func (d Deps) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		guarded := strings.HasPrefix(p, "/api/") || p == "/mcp" || strings.HasPrefix(p, "/mcp/")
		open := p == "/api/health" || p == "/api/login" || p == "/api/logout"
		if guarded && !open && !d.authenticated(r) {
			httpError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (d Deps) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Secret string `json:"secret"`
	}
	if err := decode(r, &req); err != nil {
		fail(w, r, err)
		return
	}
	if d.Secret == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !equal(req.Secret, d.Secret) {
		// Slow down guessing; this is a single-user app so latency is free.
		time.Sleep(500 * time.Millisecond)
		httpError(w, http.StatusUnauthorized, "wrong secret")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    sessionToken(d.Secret),
		Path:     "/",
		MaxAge:   int((365 * 24 * time.Hour).Seconds()),
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (d Deps) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}
