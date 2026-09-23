package githubsso

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// githubStub is a minimal stand-in for the GitHub REST API: enough of /user,
// /user/emails, and the org/team membership endpoints to drive identity
// resolution without a network or a real token.
type githubStub struct {
	login      string
	id         int64
	emails     []map[string]any
	orgMember  bool
	teamMember bool
}

func newStub(t *testing.T, s githubStub) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/user", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"login": s.login, "id": s.id, "avatar_url": "https://example.com/a.png",
		})
	})
	mux.HandleFunc("/user/emails", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(s.emails)
	})
	// /user/memberships/orgs/{org}
	mux.HandleFunc("/user/memberships/orgs/", func(w http.ResponseWriter, r *http.Request) {
		if !s.orgMember {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"state": "active"})
	})
	// /orgs/{org}/teams/{team}/memberships/{login}
	mux.HandleFunc("/orgs/", func(w http.ResponseWriter, r *http.Request) {
		if !s.teamMember {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"state": "active"})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

func verifiedPrimary(email string) []map[string]any {
	return []map[string]any{{"email": email, "primary": true, "verified": true}}
}

func TestVerifyTokenLoginAllowlist(t *testing.T) {
	base := newStub(t, githubStub{login: "Octocat", id: 1, emails: verifiedPrimary("o@x.com")})
	// Allowlist uses a different case than the account to prove matching is
	// case-insensitive.
	p := New(Config{Allowlist: Allowlist{Logins: map[string]bool{"OCTOCAT": true}}, APIBaseURL: base})
	id, err := p.VerifyToken(context.Background(), "tok")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if id.Login != "Octocat" || id.UserID != 1 {
		t.Fatalf("identity: %+v", id)
	}
}

func TestVerifyTokenNotAllowed(t *testing.T) {
	base := newStub(t, githubStub{login: "eve", id: 2})
	p := New(Config{Allowlist: Allowlist{Logins: map[string]bool{"octocat": true}}, APIBaseURL: base})
	if _, err := p.VerifyToken(context.Background(), "tok"); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("want ErrNotAllowed, got %v", err)
	}
}

func TestVerifyTokenEmailAllowlistUsesVerifiedAddress(t *testing.T) {
	base := newStub(t, githubStub{login: "eve", id: 2, emails: []map[string]any{
		{"email": "unverified@x.com", "primary": false, "verified": false},
		{"email": "Eve@X.com", "primary": true, "verified": true},
	}})
	p := New(Config{Allowlist: Allowlist{Emails: map[string]bool{"eve@x.com": true}}, APIBaseURL: base})
	id, err := p.VerifyToken(context.Background(), "tok")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if id.Email != "Eve@X.com" {
		t.Fatalf("email %q", id.Email)
	}
}

func TestVerifyTokenOrgMembership(t *testing.T) {
	member := newStub(t, githubStub{login: "eve", id: 2, orgMember: true})
	if _, err := New(Config{Allowlist: Allowlist{Org: "acme"}, APIBaseURL: member}).
		VerifyToken(context.Background(), "tok"); err != nil {
		t.Fatalf("member: %v", err)
	}
	nonmember := newStub(t, githubStub{login: "eve", id: 2, orgMember: false})
	if _, err := New(Config{Allowlist: Allowlist{Org: "acme"}, APIBaseURL: nonmember}).
		VerifyToken(context.Background(), "tok"); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("non-member: want ErrNotAllowed, got %v", err)
	}
}

func TestVerifyTokenTeamMembership(t *testing.T) {
	// Org member but not on the team → rejected when a team is required.
	orgOnly := newStub(t, githubStub{login: "eve", id: 2, orgMember: true, teamMember: false})
	if _, err := New(Config{Allowlist: Allowlist{Org: "acme", Team: "eng"}, APIBaseURL: orgOnly}).
		VerifyToken(context.Background(), "tok"); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("org-only: want ErrNotAllowed, got %v", err)
	}
	onTeam := newStub(t, githubStub{login: "eve", id: 2, orgMember: true, teamMember: true})
	if _, err := New(Config{Allowlist: Allowlist{Org: "acme", Team: "eng"}, APIBaseURL: onTeam}).
		VerifyToken(context.Background(), "tok"); err != nil {
		t.Fatalf("on-team: %v", err)
	}
}

func TestAllowlistHelpers(t *testing.T) {
	if !(Allowlist{}).Empty() {
		t.Fatal("zero allowlist should be empty")
	}
	if (Allowlist{Org: "a"}).Empty() {
		t.Fatal("org allowlist is not empty")
	}
	if !(Allowlist{Org: "a"}).WantsOrg() {
		t.Fatal("org allowlist wants org")
	}
	if (Allowlist{Logins: map[string]bool{"x": true}}).WantsOrg() {
		t.Fatal("login-only allowlist should not want org")
	}
}
