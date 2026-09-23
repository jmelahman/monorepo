// Package githuboidc verifies GitHub Actions OIDC tokens so a CI workflow can
// authenticate to the API without a shared secret. It is a thin wrapper over
// go-oidc: the discovery document and signing keys (JWKS) come from GitHub's
// OIDC provider, and every token is checked for a valid RS256 signature, the
// expected issuer and audience, and expiry.
//
// This is workload identity — the caller is a GitHub Actions job, not a human.
// It is deliberately independent of any future interactive SSO login.
package githuboidc

import (
	"context"
	"fmt"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
)

// DefaultIssuer is GitHub's hosted Actions OIDC provider. GitHub Enterprise
// Server uses a different issuer; override it via NewVerifier.
const DefaultIssuer = "https://token.actions.githubusercontent.com"

// Claims are the GitHub Actions OIDC claims the authorizer needs. GitHub sets
// many more (run_id, job_workflow_ref, environment, …); we decode only the
// ones used for authorization and logging.
type Claims struct {
	// Repository is "owner/name" of the repo whose workflow minted the token.
	Repository string `json:"repository"`
	// RepositoryOwner is the "owner" half of Repository.
	RepositoryOwner string `json:"repository_owner"`
	// Ref is the git ref the workflow ran for, e.g. "refs/heads/main".
	Ref string `json:"ref"`
	// SHA is the commit the workflow ran for.
	SHA string `json:"sha"`
	// Workflow is the name of the workflow that ran.
	Workflow string `json:"workflow"`
	// Actor is the user who triggered the run.
	Actor string `json:"actor"`
	// Subject is the token's "sub", e.g. "repo:owner/name:ref:refs/heads/main".
	Subject string `json:"sub"`
}

// Verifier verifies GitHub Actions OIDC tokens for a fixed issuer and
// audience. The zero value is not usable; construct one with NewVerifier.
type Verifier struct {
	issuer   string
	audience string

	mu       sync.Mutex
	verifier *oidc.IDTokenVerifier
}

// NewVerifier returns a Verifier that accepts tokens from issuer whose "aud"
// contains audience. issuer defaults to DefaultIssuer when empty. The
// provider's discovery document and keys are fetched lazily on first Verify,
// so constructing a Verifier never blocks or fails on network — a starting
// server doesn't depend on GitHub being reachable at boot.
func NewVerifier(issuer, audience string) *Verifier {
	if issuer == "" {
		issuer = DefaultIssuer
	}
	return &Verifier{issuer: issuer, audience: audience}
}

// Verify checks rawToken's signature, issuer, audience, and expiry, returning
// its GitHub Actions claims. An invalid, expired, wrong-audience, or
// wrong-issuer token is an error.
func (v *Verifier) Verify(ctx context.Context, rawToken string) (Claims, error) {
	ver, err := v.idTokenVerifier(ctx)
	if err != nil {
		return Claims{}, err
	}
	tok, err := ver.Verify(ctx, rawToken)
	if err != nil {
		return Claims{}, fmt.Errorf("verify token: %w", err)
	}
	var c Claims
	if err := tok.Claims(&c); err != nil {
		return Claims{}, fmt.Errorf("decode token claims: %w", err)
	}
	return c, nil
}

// idTokenVerifier lazily performs (and caches) OIDC discovery. The provider
// keeps a RemoteKeySet that refreshes JWKS on its own, including on an unknown
// key id, so a single discovery is enough for the process lifetime. A failed
// discovery isn't cached — the next Verify retries.
func (v *Verifier) idTokenVerifier(ctx context.Context) (*oidc.IDTokenVerifier, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.verifier != nil {
		return v.verifier, nil
	}
	provider, err := oidc.NewProvider(ctx, v.issuer)
	if err != nil {
		return nil, fmt.Errorf("discover OIDC provider %q: %w", v.issuer, err)
	}
	v.verifier = provider.Verifier(&oidc.Config{ClientID: v.audience})
	return v.verifier, nil
}
