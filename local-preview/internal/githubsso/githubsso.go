// Package githubsso implements interactive "Sign in with GitHub" login for
// human dashboard users via the GitHub OAuth web-application flow, and the
// same identity resolution for a GitHub personal-access token a CLI presents
// as a bearer. Both paths reduce a GitHub token to a verified Identity and
// then apply one Allowlist, so the browser and the CLI authorize identically.
//
// It is deliberately independent of internal/githuboidc, which verifies GitHub
// Actions *workload* identity tokens for CI uploads, not human logins.
package githubsso

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

// defaultAPIBase is GitHub's REST API root. Tests override it via Config.
const defaultAPIBase = "https://api.github.com"

// maxBody bounds a GitHub API response read.
const maxBody = 1 << 20

// ErrNotAllowed means authentication succeeded but the account is not on the
// allowlist. Callers map it to 403, distinct from an authentication failure.
var ErrNotAllowed = errors.New("github identity is not on the allowlist")

// Allowlist decides which authenticated GitHub accounts may use the instance.
// An account is permitted if it matches any configured rule: an explicit login
// or verified email, or membership of Org (further narrowed to Team when set).
// Logins and Emails keys are compared case-insensitively.
type Allowlist struct {
	Logins map[string]bool
	Emails map[string]bool
	Org    string
	Team   string
}

// WantsOrg reports whether the allowlist consults org/team membership and so
// needs the read:org scope. Exported so the server can log/validate config.
func (a Allowlist) WantsOrg() bool { return a.Org != "" }

// Empty reports whether no rule is configured — a fail-closed guard for the
// caller, since an empty allowlist would otherwise permit no one.
func (a Allowlist) Empty() bool {
	return len(a.Logins) == 0 && len(a.Emails) == 0 && a.Org == ""
}

// Identity is the resolved GitHub account behind a session or bearer token.
type Identity struct {
	Login     string
	Email     string
	AvatarURL string
	UserID    int64
}

// Config constructs a Provider.
type Config struct {
	ClientID     string
	ClientSecret string
	CallbackURL  string
	Allowlist    Allowlist
	// APIBaseURL overrides https://api.github.com; tests point it at an
	// httptest.Server so identity resolution needs no network.
	APIBaseURL string
}

// Provider runs the GitHub OAuth web flow and resolves tokens to allowlisted
// identities.
type Provider struct {
	oauth   *oauth2.Config
	allow   Allowlist
	apiBase string
	client  *http.Client
}

// New builds a Provider. The requested scopes are read:user and user:email,
// plus read:org only when the allowlist gates by org/team.
func New(cfg Config) *Provider {
	scopes := []string{"read:user", "user:email"}
	if cfg.Allowlist.WantsOrg() {
		scopes = append(scopes, "read:org")
	}
	apiBase := cfg.APIBaseURL
	if apiBase == "" {
		apiBase = defaultAPIBase
	}
	return &Provider{
		oauth: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.CallbackURL,
			Endpoint:     github.Endpoint,
			Scopes:       scopes,
		},
		allow:   normalizeAllowlist(cfg.Allowlist),
		apiBase: strings.TrimRight(apiBase, "/"),
		client:  http.DefaultClient,
	}
}

// normalizeAllowlist lowercases login/email keys so lookups are
// case-insensitive regardless of how the operator wrote them.
func normalizeAllowlist(a Allowlist) Allowlist {
	out := Allowlist{Org: a.Org, Team: a.Team}
	if len(a.Logins) > 0 {
		out.Logins = make(map[string]bool, len(a.Logins))
		for k := range a.Logins {
			out.Logins[strings.ToLower(k)] = true
		}
	}
	if len(a.Emails) > 0 {
		out.Emails = make(map[string]bool, len(a.Emails))
		for k := range a.Emails {
			out.Emails[strings.ToLower(k)] = true
		}
	}
	return out
}

// AuthCodeURL is the GitHub authorize URL to redirect the browser to, carrying
// an opaque state the callback verifies.
func (p *Provider) AuthCodeURL(state string) string {
	return p.oauth.AuthCodeURL(state)
}

// Exchange trades an authorization code for the caller's identity, applying
// the allowlist. ErrNotAllowed means the login succeeded but the account is
// not permitted.
func (p *Provider) Exchange(ctx context.Context, code string) (Identity, error) {
	tok, err := p.oauth.Exchange(ctx, code)
	if err != nil {
		return Identity{}, fmt.Errorf("exchange code: %w", err)
	}
	return p.resolveIdentity(ctx, tok.AccessToken)
}

// VerifyToken resolves a raw GitHub token (a personal-access token a CLI
// presents) to an allowlisted identity, or ErrNotAllowed.
func (p *Provider) VerifyToken(ctx context.Context, token string) (Identity, error) {
	return p.resolveIdentity(ctx, token)
}

// resolveIdentity fetches the token owner's profile and verified email, then
// checks the allowlist.
func (p *Provider) resolveIdentity(ctx context.Context, token string) (Identity, error) {
	var user struct {
		Login     string `json:"login"`
		ID        int64  `json:"id"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := p.getJSON(ctx, token, "/user", &user); err != nil {
		return Identity{}, err
	}
	id := Identity{Login: user.Login, Email: user.Email, AvatarURL: user.AvatarURL, UserID: user.ID}

	// /user.email is null unless the profile makes it public, and an
	// email-based allowlist must match a *verified* address — resolve it
	// explicitly from /user/emails.
	if email, err := p.primaryVerifiedEmail(ctx, token); err != nil {
		return Identity{}, err
	} else if email != "" {
		id.Email = email
	}

	if p.identityAllowed(ctx, token, id) {
		return id, nil
	}
	return Identity{}, ErrNotAllowed
}

// identityAllowed applies the allowlist rules to a resolved identity.
func (p *Provider) identityAllowed(ctx context.Context, token string, id Identity) bool {
	if id.Login != "" && p.allow.Logins[strings.ToLower(id.Login)] {
		return true
	}
	if id.Email != "" && p.allow.Emails[strings.ToLower(id.Email)] {
		return true
	}
	if p.allow.Org != "" {
		if p.allow.Team != "" {
			return p.teamMemberOK(ctx, token, id.Login)
		}
		return p.orgMemberOK(ctx, token)
	}
	return false
}

// primaryVerifiedEmail returns the token owner's primary verified email, or
// the first verified one, or "" when none is available (or the token lacks the
// user:email scope — not fatal unless the allowlist is email-based).
func (p *Provider) primaryVerifiedEmail(ctx context.Context, token string) (string, error) {
	status, body, err := p.get(ctx, token, "/user/emails")
	if err != nil {
		return "", err
	}
	if status == http.StatusForbidden || status == http.StatusNotFound {
		return "", nil
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("github GET /user/emails: unexpected status %d", status)
	}
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.Unmarshal(body, &emails); err != nil {
		return "", fmt.Errorf("decode github /user/emails: %w", err)
	}
	var fallback string
	for _, e := range emails {
		if !e.Verified {
			continue
		}
		if e.Primary {
			return e.Email, nil
		}
		if fallback == "" {
			fallback = e.Email
		}
	}
	return fallback, nil
}

// orgMemberOK reports whether the token owner is an active member of the
// configured org, via their own membership record.
func (p *Provider) orgMemberOK(ctx context.Context, token string) bool {
	status, body, err := p.get(ctx, token, "/user/memberships/orgs/"+url.PathEscape(p.allow.Org))
	if err != nil || status != http.StatusOK {
		return false
	}
	return membershipActive(body)
}

// teamMemberOK reports whether login is an active member of the configured
// team within the org.
func (p *Provider) teamMemberOK(ctx context.Context, token, login string) bool {
	if login == "" {
		return false
	}
	path := fmt.Sprintf("/orgs/%s/teams/%s/memberships/%s",
		url.PathEscape(p.allow.Org), url.PathEscape(p.allow.Team), url.PathEscape(login))
	status, body, err := p.get(ctx, token, path)
	if err != nil || status != http.StatusOK {
		return false
	}
	return membershipActive(body)
}

// membershipActive reports whether a membership response is in the "active"
// state (as opposed to "pending" for an unaccepted invitation).
func membershipActive(body []byte) bool {
	var m struct {
		State string `json:"state"`
	}
	if json.Unmarshal(body, &m) != nil {
		return false
	}
	return m.State == "active"
}

// getJSON performs a GET expecting 200 and decodes the body into out.
func (p *Provider) getJSON(ctx context.Context, token, path string, out any) error {
	status, body, err := p.get(ctx, token, path)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("github GET %s: unexpected status %d", path, status)
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("decode github %s: %w", path, err)
		}
	}
	return nil
}

// get issues an authenticated GET to the GitHub REST API and returns the
// status and (bounded) body.
func (p *Provider) get(ctx context.Context, token, path string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.apiBase+path, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := p.client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("github GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return 0, nil, fmt.Errorf("read github %s: %w", path, err)
	}
	return resp.StatusCode, body, nil
}
