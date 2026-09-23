// Package proxy is the top-level request router. Hosts under the preview
// domain (<label>-<repo>.<domain>) serve deployed previews — static frontend
// files plus /api/* reverse-proxied to the supervised backend. Every other
// host (the apex domain, localhost, raw IPs) gets the dashboard.
//
// Preview hosts must stay a single DNS label: a wildcard matches one label
// only, so this is what lets one *.<domain> record and one wildcard
// certificate cover every repo.
//
// Routing state is cached in memory with a short TTL so the single-connection
// SQLite database isn't queried for every asset request.
package proxy

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jmelahman/local-preview/internal/db"
	"github.com/jmelahman/local-preview/internal/fleet"
	"github.com/jmelahman/local-preview/internal/store"
	"github.com/jmelahman/local-preview/internal/supervise"
)

// coldStartWait is how long a request waits for a backend before returning
// the interim "starting" response. The start itself continues in the
// background either way.
const coldStartWait = 1500 * time.Millisecond

// cacheTTL bounds staleness of the routing cache; interim status pages poll
// on the same order, so transitions surface promptly.
const cacheTTL = 2 * time.Second

// previewSessionTTL is how long a preview-access session lasts before the
// browser must re-run the apex→preview handshake.
const previewSessionTTL = 7 * 24 * time.Hour

// previewGrantCookieName is the preview-scoped session cookie the proxy sets
// after redeeming a grant. It is deliberately distinct from the apex dashboard
// cookie (internal/api's "preview_session"): a preview subdomain runs
// untrusted code and must never hold the dashboard credential.
const previewGrantCookieName = "preview_grant"

// previewAuthParam is the one-time handoff code the apex redirects back with.
const previewAuthParam = "preview_auth"

// onyxReturnCookieName stashes the preview URL a browser was headed for when it
// got bounced to the canonical onyx host to log in. It is domain-scoped (like
// the grant cookie) so it survives the cross-host hop, and consumed once the
// user lands back on the canonical host with a session. Distinct from any onyx
// cookie: it carries no credential, only a return address.
const onyxReturnCookieName = "onyx_return"

// onyxReturnTTL bounds how long a stashed return address lives — long enough to
// complete a Google login, short enough that a stale marker can't yank a user
// browsing the canonical app straight to some old preview.
const onyxReturnTTL = 15 * time.Minute

// widenedOnyxTTL bounds the domain-wide onyx cookie the return-dance mints from
// an existing canonical session (see setWidenedOnyxCookie). It mirrors onyx's
// SESSION_EXPIRE_TIME_SECONDS default (7 days); the JWT's own exp is the real
// authority, so a cookie outliving the token just triggers a fresh login.
const widenedOnyxTTL = 7 * 24 * time.Hour

// defaultOnyxCookieName mirrors onyx's FASTAPI_USERS_AUTH_COOKIE_NAME default
// (its AUTH_COOKIE_NAME env, unset). The proxy only checks this cookie's
// presence — it never validates the JWT (that's each onyx backend's job with
// the shared USER_AUTH_SECRET).
const defaultOnyxCookieName = "fastapiusersauth"

// Backends is the slice of the supervisor the proxy needs. EnsureRunning
// returns the "host:port" the started process serves on — loopback for a
// process on this node (single-node / worker-local), or a worker's address when
// the control node routes to an elastic worker tier. The proxy is transport-
// agnostic: the same interface is satisfied by a local Manager adapter and by a
// remote worker-API client.
type Backends interface {
	EnsureRunning(ctx context.Context, k supervise.Key, repoName string) (addr string, err error)
}

// Router routes by Host header. It wraps the dashboard handler and serves
// previews itself.
type Router struct {
	db        *db.Store
	files     *store.Store
	backends  Backends
	domain    string
	dashboard http.Handler

	// authEnabled gates preview subdomains behind the SSO login; when off the
	// Router behaves exactly as before. authBaseURL is the dashboard origin the
	// grant handshake bounces through; authSecure marks the preview cookie
	// Secure.
	authEnabled bool
	authBaseURL string
	authSecure  bool

	// runLogs, when set, lets the interim "starting" pages stream the
	// process's run log while it boots. See SetRunLogs.
	runLogs RunLogs

	// procStatus, when set, lets the interim "starting" pages narrate the side
	// actually doing the work (a frontend cold start runs inside its backend's
	// init). See SetProcStatus.
	procStatus ProcStatus

	// reserved maps a preview-domain label (the part before ".<domain>") to a
	// fixed upstream "host:port". A matching host is reverse-proxied wholesale
	// to that upstream — behind the same SSO gate as previews, but outside the
	// deploy/build machinery. It exists for always-on companion services (e.g. a
	// canonical app instance) that must live under the preview domain to inherit
	// the wildcard cert and the login gate, yet aren't per-commit previews.
	reserved map[string]string

	// onyxAuthLabel, when set, turns on single-sign-on for onyx previews: a
	// per-commit preview host without an onyx session cookie is bounced to this
	// reserved label (the one canonical onyx host that owns the Google OAuth
	// client + registered redirect URI) to log in, and the minted session — a
	// stateless JWT signed by the shared USER_AUTH_SECRET — rides back on the
	// preview domain so every preview validates it locally. onyxCookieName is
	// the onyx session cookie the proxy watches for (presence only). Empty label
	// leaves previews as they were (browse-as-admin, no onyx login).
	onyxAuthLabel  string
	onyxCookieName string

	mu    sync.Mutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	repoID    int64
	repoName  string
	label     string // sha prefix the host asked for
	deploy    db.Deploy
	err       string // non-empty: resolution failed with this message
	ambiguous []string
	expires   time.Time

	// Routing shape of a ready deploy, resolved once per cache fill.
	feProcess   bool     // frontend is a supervised process, not a static dist
	stripAPI    bool     // remove the /api prefix before proxying
	extraRoutes []string // additional unstripped prefixes routed to the backend
}

// New returns a Router serving previews under domain and everything else
// from dashboard.
func New(database *db.Store, files *store.Store, backends Backends, domain string, dashboard http.Handler) *Router {
	return &Router{
		db:        database,
		files:     files,
		backends:  backends,
		domain:    strings.ToLower(domain),
		dashboard: dashboard,
		cache:     make(map[string]cacheEntry),
	}
}

// SetPreviewAuth turns on SSO gating of preview subdomains. baseURL is the
// dashboard origin (scheme://host) the grant handshake redirects through, and
// secure marks the preview cookie Secure. The zero value (never calling this)
// leaves previews open, so existing callers and tests are unaffected.
func (rt *Router) SetPreviewAuth(enabled bool, baseURL string, secure bool) {
	rt.authEnabled = enabled
	rt.authBaseURL = strings.TrimRight(baseURL, "/")
	rt.authSecure = secure
}

// SetReservedUpstreams registers fixed <label> → "host:port" upstreams served
// under the preview domain. Labels are the single DNS label below the domain
// (e.g. "app" for app.<domain>); each is reverse-proxied wholesale to its
// upstream behind the SSO gate, shadowing any per-commit preview of the same
// label. A nil or empty map (the default) leaves routing unchanged.
func (rt *Router) SetReservedUpstreams(m map[string]string) {
	if len(m) == 0 {
		rt.reserved = nil
		return
	}
	rt.reserved = make(map[string]string, len(m))
	for label, addr := range m {
		rt.reserved[strings.ToLower(label)] = addr
	}
}

// SetOnyxAuth turns on onyx single-sign-on for preview subdomains. label is the
// reserved upstream that runs the canonical onyx (the sole Google-OAuth host);
// it must also be registered via SetReservedUpstreams. cookieName is the onyx
// session cookie to watch for (empty → onyx's "fastapiusersauth" default). An
// empty label (the zero value) leaves previews un-gated by onyx, as before.
func (rt *Router) SetOnyxAuth(label, cookieName string) {
	rt.onyxAuthLabel = strings.ToLower(strings.TrimSpace(label))
	if cookieName == "" {
		cookieName = defaultOnyxCookieName
	}
	rt.onyxCookieName = cookieName
}

// onyxAuthEnabled reports whether onyx SSO gating is configured.
func (rt *Router) onyxAuthEnabled() bool { return rt.onyxAuthLabel != "" }

func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sub, ok := rt.parseHost(r.Host)
	if !ok {
		rt.dashboard.ServeHTTP(w, r)
		return
	}
	// Gate preview traffic before any DB lookup, so an unauthenticated caller
	// can't even probe which previews exist.
	if rt.authEnabled && !rt.previewAuthorized(w, r) {
		return
	}
	// A reserved label is an always-on companion service, not a deploy: route it
	// straight to its upstream (still behind the gate above), skipping resolve.
	if addr, ok := rt.reserved[sub]; ok {
		rt.serveReserved(w, r, sub, addr)
		return
	}
	// onyx SSO: a per-commit preview host (never the canonical label above) that
	// lacks an onyx session gets bounced to the canonical host to log in.
	if rt.onyxAuthEnabled() && rt.needsOnyxLogin(r) {
		rt.redirectToOnyxLogin(w, r)
		return
	}
	entry := rt.resolve(sub)
	switch {
	case len(entry.ambiguous) > 0:
		rt.errorPage(w, r, http.StatusNotFound, "Ambiguous preview address",
			fmt.Sprintf("%q matches several deploys (%s) — use a longer sha prefix.",
				entry.label, strings.Join(entry.ambiguous, ", ")))
	case entry.err != "":
		rt.errorPage(w, r, http.StatusNotFound, "Unknown preview", entry.err)
	default:
		rt.servePreview(w, r, entry)
	}
}

// previewAuthorized authorizes a preview request. It redeems a one-time
// ?preview_auth code into a preview-scoped cookie, accepts an existing valid
// preview cookie, or bounces the browser through the dashboard's grant
// handshake. It returns true only when the request may proceed; otherwise it
// has already written a redirect and the caller must stop.
func (rt *Router) previewAuthorized(w http.ResponseWriter, r *http.Request) bool {
	if code := r.URL.Query().Get(previewAuthParam); code != "" {
		return rt.redeemPreviewGrant(w, r, code)
	}
	if c, err := r.Cookie(previewGrantCookieName); err == nil {
		if _, err := rt.db.GetSessionByTokenHash("preview", hashToken(c.Value)); err == nil {
			return true
		}
	}
	rt.redirectToGrant(w, r)
	return false
}

// redeemPreviewGrant consumes a handoff code and, on success, establishes a
// preview-scoped session (copying the apex session's identity) and sets its
// cookie for the whole preview domain, then redirects to the clean URL. Any
// failure restarts the handshake rather than leaking a reason.
func (rt *Router) redeemPreviewGrant(w http.ResponseWriter, r *http.Request, code string) bool {
	apexID, err := rt.db.RedeemPreviewGrant(hashToken(code))
	if err != nil {
		rt.redirectToGrant(w, r)
		return false
	}
	apex, err := rt.db.GetSessionByID(apexID)
	if err != nil {
		rt.redirectToGrant(w, r)
		return false
	}
	raw, hash, err := newToken()
	if err != nil {
		rt.errorPage(w, r, http.StatusInternalServerError, "Preview auth error",
			"Could not establish a preview session. Try again.")
		return false
	}
	if _, err := rt.db.CreateSession(db.Session{
		TokenHash:    hash,
		Scope:        "preview",
		GitHubLogin:  apex.GitHubLogin,
		GitHubUserID: apex.GitHubUserID,
		Email:        apex.Email,
		AvatarURL:    apex.AvatarURL,
	}, previewSessionTTL); err != nil {
		rt.errorPage(w, r, http.StatusInternalServerError, "Preview auth error",
			"Could not establish a preview session. Try again.")
		return false
	}
	http.SetCookie(w, &http.Cookie{
		Name:     previewGrantCookieName,
		Value:    raw,
		Path:     "/",
		Domain:   "." + rt.domain,
		HttpOnly: true,
		Secure:   rt.authSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(previewSessionTTL),
		MaxAge:   int(previewSessionTTL.Seconds()),
	})
	http.Redirect(w, r, previewReturnURL(r), http.StatusFound)
	return false
}

// redirectToGrant sends the browser to the dashboard's preview-grant endpoint,
// which mints a code (after logging the user in if needed) and redirects back.
func (rt *Router) redirectToGrant(w http.ResponseWriter, r *http.Request) {
	target := rt.authBaseURL + "/api/auth/preview-grant?return_to=" +
		url.QueryEscape(previewReturnURL(r))
	http.Redirect(w, r, target, http.StatusFound)
}

// needsOnyxLogin reports whether a preview request must first go get an onyx
// session on the canonical host. It intercepts the onyx login page outright —
// single-tenant onyx would otherwise render a password form there, and an
// expired session redirects the browser to it — and otherwise gates only
// top-level browser navigations that carry no session cookie. Assets and
// XHR/fetch fall through to the preview's onyx (which 401s on its own), so a
// logged-in SPA keeps working and non-browser callers aren't bounced.
func (rt *Router) needsOnyxLogin(r *http.Request) bool {
	if r.URL.Path == "/auth/login" {
		return true
	}
	if !wantsHTML(r) {
		return false
	}
	_, err := r.Cookie(rt.onyxCookieName)
	return err != nil
}

// redirectToOnyxLogin stashes where the browser was headed (domain-scoped so it
// survives the hop to the canonical host) and 302s to that host's /auth/login,
// which bounces on to Google. The onyx login page itself returns to the preview
// root, not back to /auth/login, so there is no loop.
func (rt *Router) redirectToOnyxLogin(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     onyxReturnCookieName,
		Value:    onyxReturnURL(r),
		Path:     "/",
		Domain:   "." + rt.domain,
		HttpOnly: true,
		Secure:   rt.authSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(onyxReturnTTL),
		MaxAge:   int(onyxReturnTTL.Seconds()),
	})
	target := requestScheme(r) + "://" + rt.onyxAuthLabel + "." + rt.domain + "/auth/login"
	http.Redirect(w, r, target, http.StatusFound)
}

// onyxReturnURL is where login should land the browser afterward: the current
// preview URL, except on the login page itself (return to the preview root, so
// the marker can't point back at /auth/login and loop).
func onyxReturnURL(r *http.Request) string {
	if r.URL.Path == "/auth/login" {
		return requestScheme(r) + "://" + r.Host + "/"
	}
	return previewReturnURL(r)
}

// safeOnyxReturn guards the stashed return address against open redirects: it
// must be an http(s) URL whose host is the preview domain or a label under it.
// The marker is domain-scoped, so a malicious previewed backend could set it —
// this keeps the post-login redirect inside our own hosts.
func (rt *Router) safeOnyxReturn(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	host := u.Hostname()
	return host == rt.domain || strings.HasSuffix(host, "."+rt.domain)
}

// clearOnyxReturn expires the return marker once it has been consumed.
func (rt *Router) clearOnyxReturn(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     onyxReturnCookieName,
		Value:    "",
		Path:     "/",
		Domain:   "." + rt.domain,
		HttpOnly: true,
		Secure:   rt.authSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// previewReturnURL is the absolute URL of the current preview request with the
// one-time code stripped — where the handshake should land the browser.
func previewReturnURL(r *http.Request) string {
	u := *r.URL
	q := u.Query()
	q.Del(previewAuthParam)
	u.RawQuery = q.Encode()
	u.Scheme = requestScheme(r)
	u.Host = r.Host
	return u.String()
}

// requestScheme reports the public scheme, honoring a TLS-terminating proxy's
// X-Forwarded-Proto before the local connection's TLS state.
func requestScheme(r *http.Request) string {
	if p := r.Header.Get("X-Forwarded-Proto"); p != "" {
		return p
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

// newToken returns a 256-bit random token and its sha256 hex hash. Only the
// hash is persisted; the raw value lives in the cookie.
func newToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, hashToken(raw), nil
}

// hashToken maps a raw token to its stored form.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// stripCookie removes a single named cookie from a request's Cookie header,
// leaving the others intact.
func stripCookie(r *http.Request, name string) {
	cookies := r.Cookies()
	r.Header.Del("Cookie")
	for _, c := range cookies {
		if c.Name == name {
			continue
		}
		r.AddCookie(c)
	}
}

// parseHost returns the single label below the preview domain, unsplit —
// telling <label>-<repo> apart needs the repo registry, which lookup has.
// ok=false means "not a preview host" → dashboard.
func (rt *Router) parseHost(hostport string) (sub string, ok bool) {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	rest, found := strings.CutSuffix(host, "."+rt.domain)
	if !found {
		return "", false
	}
	// One label only; a deeper host isn't ours.
	if rest == "" || strings.Contains(rest, ".") {
		return "", false
	}
	return rest, true
}

// resolve maps a preview label to a deploy through the TTL cache.
func (rt *Router) resolve(sub string) cacheEntry {
	key := sub
	rt.mu.Lock()
	if e, ok := rt.cache[key]; ok && time.Now().Before(e.expires) {
		rt.mu.Unlock()
		return e
	}
	rt.mu.Unlock()

	e := rt.lookup(sub)
	e.expires = time.Now().Add(cacheTTL)
	rt.mu.Lock()
	rt.cache[key] = e
	rt.mu.Unlock()
	return e
}

// splitSub resolves <label>-<repo> against the repo registry. Repo names may
// themselves contain hyphens, so the first hyphen isn't necessarily the
// separator: try every split whose left side is a sha prefix and take the
// first one naming a registered repo. guess is the leftmost candidate's repo,
// used only to word the error when nothing matches.
func (rt *Router) splitSub(sub string) (label string, repo db.Repo, guess string, ok bool) {
	for i, c := range sub {
		if c != '-' {
			continue
		}
		left, right := sub[:i], sub[i+1:]
		if left == "" || right == "" || !isHex(left) {
			continue
		}
		if guess == "" {
			guess = right
		}
		r, err := rt.db.GetRepoByName(right)
		if err != nil {
			continue
		}
		return left, r, guess, true
	}
	return "", db.Repo{}, guess, false
}

func (rt *Router) lookup(sub string) cacheEntry {
	label, repo, guess, ok := rt.splitSub(sub)
	if !ok {
		if guess == "" {
			// No hyphen split had a sha prefix on the left, so the host is
			// the wrong shape rather than naming an unknown repo.
			return cacheEntry{err: fmt.Sprintf("%q is not a preview address (expected <sha>-<repo>).", sub)}
		}
		return cacheEntry{err: fmt.Sprintf("No repo named %q is registered.", guess)}
	}
	repoName := repo.Name
	matches, err := rt.db.DeploysBySHAPrefix(repo.ID, label)
	if err != nil || len(matches) == 0 {
		return cacheEntry{err: fmt.Sprintf("No deploy of %s matches %q. Deploy it with: preview deploy %s", repoName, label, label)}
	}
	// Distinct shas → ambiguous. (The same sha can't appear twice per repo.)
	if len(matches) > 1 {
		shorts := make([]string, len(matches))
		for i, m := range matches {
			shorts[i] = m.ShortSHA
		}
		return cacheEntry{repoName: repoName, label: label, ambiguous: shorts}
	}
	e := cacheEntry{repoID: repo.ID, repoName: repoName, label: label, deploy: matches[0]}
	if e.deploy.Status == db.DeployReady {
		if e.deploy.FeHash != "" {
			if _, err := rt.db.GetFrontendArtifact(repo.ID, e.deploy.FeHash); err == nil {
				e.feProcess = true
			}
		}
		if art, err := rt.db.GetBackendArtifact(repo.ID, e.deploy.BeHash); err == nil {
			var cfg struct {
				StripAPIPrefix bool     `json:"strip_api_prefix"`
				ExtraRoutes    []string `json:"extra_routes"`
			}
			if json.Unmarshal([]byte(art.RunConfig), &cfg) == nil {
				e.stripAPI = cfg.StripAPIPrefix
				e.extraRoutes = cfg.ExtraRoutes
			}
		}
	}
	return e
}

func (rt *Router) servePreview(w http.ResponseWriter, r *http.Request, e cacheEntry) {
	d := e.deploy
	repoName := e.repoName
	switch d.Status {
	case db.DeployQueued, db.DeployBuilding:
		rt.interimResponse(w, r, http.StatusServiceUnavailable, rt.buildingInterim(d, repoName))
	case db.DeployFailed:
		rt.errorPage(w, r, http.StatusBadGateway, "Build failed",
			fmt.Sprintf("Deploy %s failed: %s (full logs: preview deploy logs %d)", d.ShortSHA, d.Error, d.ID))
	case db.DeployEvicted:
		rt.errorPage(w, r, http.StatusGone, "Preview cleaned up",
			fmt.Sprintf("Deploy %s was garbage-collected. Redeploy it with: preview deploy %s", d.ShortSHA, d.SHA))
	case db.DeployReady:
		if strings.HasPrefix(r.URL.Path, "/api/") || matchesRoute(e.extraRoutes, r.URL.Path) {
			rt.proxyAPI(w, r, e, repoName)
			return
		}
		if e.feProcess {
			rt.proxyFrontend(w, r, e, repoName)
			return
		}
		rt.serveStatic(w, r, repoName, d.FeHash)
	default:
		rt.errorPage(w, r, http.StatusInternalServerError, "Unknown state", d.Status)
	}
}

// buildingInterim describes a queued/building deploy's current phase, chosen
// fresh on every poll (servePreview re-runs per poll), so the page narrates
// the build as it moves: resolving → frontend build (streaming its log) →
// backend build (its log) → persisting. Builds run on this node even in a
// fleet — the control node builds, workers serve — so the log paths in the
// deploy row are local files. The fe log streams as poll attempt 1 and the be
// log as attempt 2: the attempt bump is what tells the page to clear the pane
// between phases.
func (rt *Router) buildingInterim(d db.Deploy, repoName string) interim {
	it := interim{
		state: "building", title: "Building preview…",
		detail: fmt.Sprintf("Deploy %s of %s is %s.", d.ShortSHA, repoName, d.Status),
	}
	if d.Status == db.DeployQueued || d.FeHash == "" {
		// Queued, or still resolving the manifest and partition hashes — there
		// is no build log to stream yet.
		return it
	}
	switch {
	case !rt.files.HasFrontend(repoName, d.FeHash):
		it.title = "Building preview frontend…"
		it.detail = fmt.Sprintf("Deploy %s of %s is building its frontend.", d.ShortSHA, repoName)
		it.logPath, it.logAttempt = d.FeBuildLogPath, 1
	case d.BeHash != "" && !rt.files.HasBackend(repoName, d.BeHash):
		it.title = "Building preview backend…"
		it.detail = fmt.Sprintf("Deploy %s of %s is building its backend.", d.ShortSHA, repoName)
		it.logPath, it.logAttempt = d.BeBuildLogPath, 2
	default:
		// Both sides published but the deploy hasn't flipped ready: the
		// synchronous durable-tier persist is running (see SetDeployReady's
		// gate) — previously an invisible phase.
		it.detail = fmt.Sprintf("Deploy %s of %s built; persisting artifacts.", d.ShortSHA, repoName)
	}
	return it
}

// proxyAPI routes backend traffic, stripping the /api prefix when the
// manifest asks for it (extra routes are never stripped).
func (rt *Router) proxyAPI(w http.ResponseWriter, r *http.Request, e cacheEntry, repoName string) {
	if e.stripAPI && strings.HasPrefix(r.URL.Path, "/api/") {
		r = r.Clone(r.Context())
		r.URL.Path = strings.TrimPrefix(r.URL.Path, "/api")
	}
	rt.ensureAndProxy(w, r, e, repoName, supervise.BackendKey(e.repoID, e.deploy.BeHash), "backend")
}

// proxyFrontend routes page traffic to a process-mode frontend.
func (rt *Router) proxyFrontend(w http.ResponseWriter, r *http.Request, e cacheEntry, repoName string) {
	rt.ensureAndProxy(w, r, e, repoName,
		supervise.FrontendKey(e.repoID, e.deploy.FeHash, e.deploy.BeHash), "frontend")
}

// ensureAndProxy cold-starts the keyed process if needed (bounded wait; the
// start continues in the background) and reverse-proxies to it.
func (rt *Router) ensureAndProxy(w http.ResponseWriter, r *http.Request, e cacheEntry, repoName string, k supervise.Key, what string) {
	ctx, cancel := context.WithTimeout(r.Context(), coldStartWait)
	defer cancel()
	addr, err := rt.backends.EnsureRunning(ctx, k, repoName)
	if err != nil {
		if ctx.Err() != nil && r.Context().Err() == nil {
			// Still starting — an interim page whose polls stream the run log.
			// A process-mode frontend starts by first ensuring its backend:
			// the whole backend init runs INSIDE the frontend's start attempt
			// (supervise: "start backend for {backend_url}"), so while that
			// backend isn't running yet, IT is the story — narrate and stream
			// the backend, not a frontend that hasn't opened its log. The
			// page flips to the frontend on the poll after the backend is up.
			side, hash, subject := string(k.Side), k.Hash, what
			if k.Side == supervise.SideFrontend && k.Peer != "" && rt.procStatus != nil &&
				rt.procStatus.Status(supervise.BackendKey(k.RepoID, k.Peer)) != supervise.StatusRunning {
				side, hash, subject = string(supervise.SideBackend), k.Peer, "backend"
			}
			rt.interimResponse(w, r, http.StatusServiceUnavailable, interim{
				state: "starting", title: "Starting " + subject + "…",
				detail: "The preview " + subject + " is starting.",
				repo:   repoName, side: side, hash: hash,
			})
			return
		}
		if errors.Is(err, fleet.ErrNoWorker) && r.Context().Err() == nil {
			// The fleet is at zero — and this very request registered the
			// unserved demand that scales it out, so a worker is on its way.
			// Not a failure: an interim page whose 1s polls re-enter this
			// function, each re-registering demand (keeping the scale-out
			// signal alive while the node boots) until a worker joins — at
			// which point the poll lands in the cold-start path above and the
			// same page becomes the "starting" screen, then loads the preview.
			rt.interimResponse(w, r, http.StatusServiceUnavailable, interim{
				state: "waking", title: "Waking the preview fleet…",
				detail: "No preview server is running, so one is being launched for this request — " +
					"usually two to four minutes. The preview loads automatically once it's up.",
				repo: repoName, side: string(k.Side), hash: k.Hash,
			})
			return
		}
		detail := fmt.Sprintf("The preview %s failed to start: %s (run logs: preview deploy logs %d)", what, err, e.deploy.ID)
		if isPoll(r) {
			// The interim page is watching: report the failure (with the final
			// log chunk) in place instead of reloading onto a bare error page.
			rt.pollJSON(w, r, http.StatusBadGateway, interim{
				state: "failed", title: "Preview unavailable", detail: detail,
				repo: repoName, side: string(k.Side), hash: k.Hash,
			})
			return
		}
		rt.errorPage(w, r, http.StatusBadGateway, "Preview unavailable", detail)
		return
	}
	target := &url.URL{Scheme: "http", Host: addr}
	proxy := &httputil.ReverseProxy{
		// SetURL also rewrites the outbound Host to the target: backends see
		// themselves, not the preview subdomain (which would confuse any app
		// that routes on Host — including a nested local-preview). The
		// original host travels in X-Forwarded-Host.
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.SetXForwarded()
			// Never hand the orchestrator's preview-access credential to the
			// previewed (untrusted) app — httputil forwards Cookie verbatim
			// otherwise, letting a malicious backend replay it against the API.
			stripCookie(pr.Out, previewGrantCookieName)
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			rt.errorPage(w, r, http.StatusBadGateway, "Preview error", err.Error())
		},
	}
	proxy.ServeHTTP(w, r)
}

// serveReserved reverse-proxies a request wholesale to a reserved upstream. It
// mirrors ensureAndProxy's transport handling — Host rewritten to the upstream,
// original host in X-Forwarded-Host, and the domain-wide preview-access cookie
// stripped so the companion app never receives the control-plane credential —
// but has no cold-start, deploy state, or /api-prefix logic: every path passes
// through unchanged. The upstream owns its own routing and (for the canonical
// onyx) its own auth.
//
// When this is the onyx SSO host, it also (a) completes the post-login handoff
// — a browser landing here with a fresh onyx session and a stashed return
// address is sent back to the preview it came from — and (b) rewrites the onyx
// session cookie to the preview domain on the way out, so the JWT minted here
// validates on every preview host.
func (rt *Router) serveReserved(w http.ResponseWriter, r *http.Request, sub, addr string) {
	isOnyxAuth := rt.onyxAuthEnabled() && sub == rt.onyxAuthLabel
	if isOnyxAuth {
		if c, err := r.Cookie(rt.onyxCookieName); err == nil {
			if ret, err := r.Cookie(onyxReturnCookieName); err == nil && rt.safeOnyxReturn(ret.Value) {
				// The browser reached the canonical host carrying an onyx
				// session, but that cookie is host-only to this host — the
				// preview it's being sent back to would never receive it, so
				// needsOnyxLogin would bounce it right back here, forever.
				// Widen the session to the preview domain before redirecting,
				// so the return actually sticks. (onyx only sets — and thus
				// rewriteOnyxCookieDomain only widens — the cookie on a fresh
				// login; an already-logged-in user never re-triggers that.)
				rt.setWidenedOnyxCookie(w, c.Value)
				rt.clearOnyxReturn(w)
				http.Redirect(w, r, ret.Value, http.StatusFound)
				return
			}
		}
	}
	target := &url.URL{Scheme: "http", Host: addr}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.SetXForwarded()
			stripCookie(pr.Out, previewGrantCookieName)
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			rt.errorPage(w, r, http.StatusBadGateway, "Service unavailable", err.Error())
		},
	}
	if isOnyxAuth {
		proxy.ModifyResponse = rt.rewriteOnyxCookieDomain
	}
	proxy.ServeHTTP(w, r)
}

// rewriteOnyxCookieDomain widens the onyx session cookie set by the canonical
// host to the whole preview domain, so the browser presents it to every preview
// subdomain. onyx's CookieTransport sets a host-only cookie (no Domain); we add
// ".<domain>". Only the auth cookie is touched — other cookies (CSRF, PKCE)
// belong to the canonical host and pass through unchanged.
func (rt *Router) rewriteOnyxCookieDomain(resp *http.Response) error {
	cookies := resp.Cookies()
	changed := false
	for _, c := range cookies {
		if c.Name == rt.onyxCookieName && c.Domain == "" {
			c.Domain = "." + rt.domain
			changed = true
		}
	}
	if !changed {
		return nil
	}
	resp.Header.Del("Set-Cookie")
	for _, c := range cookies {
		if s := c.String(); s != "" {
			resp.Header.Add("Set-Cookie", s)
		}
	}
	return nil
}

// setWidenedOnyxCookie emits a preview-domain copy of the onyx session cookie
// from an existing canonical session, for the return-dance path where the user
// is already logged in and onyx will not re-set (and rewriteOnyxCookieDomain
// will not widen) the cookie. Request cookies carry no attributes, so we mint
// onyx's own: HttpOnly, Path=/, Lax, Secure per the deployment. The value is the
// stateless JWT — every preview validates it locally with the shared secret.
func (rt *Router) setWidenedOnyxCookie(w http.ResponseWriter, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     rt.onyxCookieName,
		Value:    value,
		Path:     "/",
		Domain:   "." + rt.domain,
		HttpOnly: true,
		Secure:   rt.authSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(widenedOnyxTTL),
		MaxAge:   int(widenedOnyxTTL.Seconds()),
	})
}

// matchesRoute reports whether path falls under any of the prefixes.
func matchesRoute(routes []string, path string) bool {
	for _, route := range routes {
		if path == route || strings.HasPrefix(path, route+"/") {
			return true
		}
	}
	return false
}

// serveStatic serves the frontend artifact with SPA fallback, mirroring the
// embedded-dashboard handler's behavior but rooted at a disk directory.
func (rt *Router) serveStatic(w http.ResponseWriter, r *http.Request, repoName, feHash string) {
	// A static frontend has no supervised process, so nothing on the serving
	// path would otherwise hydrate it — yet with a durable tier its local files
	// are a cache that the sweeper can reclaim while the deploy stays ready.
	// Bring it back here, mirroring supervise.start's hydrate-on-serve: absence
	// of files is a cache miss to refill, not an eviction to 404. (No tier =
	// today's behavior exactly: local disk is authoritative, so skip the dance.)
	if rt.files.ArtifactTier() != nil {
		if !rt.files.HasFrontend(repoName, feHash) {
			ctx, cancel := context.WithTimeout(r.Context(), coldStartWait)
			defer cancel()
			if err := rt.files.Hydrate(ctx, repoName, "fe", feHash); err != nil {
				switch {
				case errors.Is(err, store.ErrNotInTier):
					rt.errorPage(w, r, http.StatusBadGateway, "Preview unavailable",
						"The preview frontend is missing from durable storage. Redeploy to rebuild it.")
				case ctx.Err() != nil && r.Context().Err() == nil:
					// Still landing — tell the client to come back shortly.
					rt.interimResponse(w, r, http.StatusServiceUnavailable, interim{
						state: "hydrating", title: "Loading preview…",
						detail: "The preview frontend is being restored from durable storage.",
					})
				default:
					rt.errorPage(w, r, http.StatusBadGateway, "Preview unavailable",
						"The preview frontend could not be restored: "+err.Error())
				}
				return
			}
		}
		// Mark it hot on every served request so a busy static frontend doesn't
		// sort coldest (its dir mtime is pinned at publish time) and get evicted
		// out from under live traffic.
		rt.files.NoteAccess(repoName, "fe", feHash)
	}

	// Explicit revalidation instead of heuristic freshness: previews are
	// content-addressed, but the URL only carries the commit sha — a
	// --rebuild of the same sha can change file contents under the same
	// URL, so nothing here may be marked immutable (target repos also
	// aren't guaranteed to content-hash their asset names). no-cache keeps
	// every response a cheap 304 via the files' Last-Modified until the
	// artifact really changes.
	w.Header().Set("Cache-Control", "no-cache")
	root := http.Dir(rt.files.FrontendDir(repoName, feHash))
	fileServer := http.FileServer(root)
	p := strings.TrimPrefix(r.URL.Path, "/")
	if p == "" {
		fileServer.ServeHTTP(w, r)
		return
	}
	if f, err := root.Open(p); err != nil {
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		fileServer.ServeHTTP(w, r2)
		return
	} else {
		f.Close()
	}
	fileServer.ServeHTTP(w, r)
}

func isHex(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// wantsHTML sniffs whether the client is a browser navigation (serve an
// HTML status page) or an API/fetch caller (serve JSON).
func wantsHTML(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

// errorPage is a terminal status: no polling, no refresh. A poll that lands
// here comes back without the interim marker, so the interim page reloads
// onto it.
func (rt *Router) errorPage(w http.ResponseWriter, r *http.Request, status int, title, detail string) {
	// Terminal for this request, but not for the address: a redeploy or
	// retry changes the answer, so nothing may cache this page.
	w.Header().Set("Cache-Control", "no-store")
	if !wantsHTML(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprintf(w, `{"error":%q}`+"\n", title+": "+detail)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<!doctype html><html><head><title>%s</title>
<style>body{font-family:system-ui,sans-serif;display:flex;min-height:100vh;align-items:center;justify-content:center;background:#111;color:#eee}
main{max-width:36rem;padding:2rem}h1{font-size:1.25rem}p{color:#aaa;line-height:1.5}</style>
</head><body><main><h1>%s</h1><p>%s</p></main></body></html>
`, html.EscapeString(title), html.EscapeString(title), html.EscapeString(detail))
}
