package githuboidc

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// testIssuer is a minimal OIDC provider: a discovery document plus a JWKS
// built from an in-memory RSA key, enough for go-oidc to verify tokens the
// test mints. Using a configurable issuer is exactly how a GitHub Enterprise
// Server deployment would point the verifier at its own provider.
type testIssuer struct {
	srv *httptest.Server
	key *rsa.PrivateKey
	kid string
}

func newTestIssuer(t *testing.T) *testIssuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ti := &testIssuer{key: key, kid: "test-key-1"}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"issuer":                                ti.srv.URL,
			"jwks_uri":                              ti.srv.URL + "/.well-known/jwks",
			"response_types_supported":              []string{"id_token"},
			"subject_types_supported":               []string{"public"},
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/.well-known/jwks", func(w http.ResponseWriter, r *http.Request) {
		pub := ti.key.PublicKey
		writeJSON(w, map[string]any{"keys": []map[string]any{{
			"kty": "RSA",
			"alg": "RS256",
			"use": "sig",
			"kid": ti.kid,
			"n":   b64(pub.N.Bytes()),
			"e":   b64(big.NewInt(int64(pub.E)).Bytes()),
		}}})
	})
	ti.srv = httptest.NewServer(mux)
	t.Cleanup(ti.srv.Close)
	return ti
}

// mint signs an RS256 JWT with the issuer's key (or, when key is non-nil, a
// foreign one to force a signature mismatch).
func (ti *testIssuer) mint(t *testing.T, claims map[string]any, key *rsa.PrivateKey) string {
	t.Helper()
	if key == nil {
		key = ti.key
	}
	header := b64(mustJSON(t, map[string]any{"alg": "RS256", "typ": "JWT", "kid": ti.kid}))
	payload := b64(mustJSON(t, claims))
	signing := header + "." + payload
	sum := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return signing + "." + b64(sig)
}

// goodClaims is a representative GitHub Actions token payload.
func (ti *testIssuer) goodClaims(aud string) map[string]any {
	now := time.Now()
	return map[string]any{
		"iss":              ti.srv.URL,
		"aud":              aud,
		"sub":              "repo:acme/app:ref:refs/heads/main",
		"exp":              now.Add(10 * time.Minute).Unix(),
		"iat":              now.Unix(),
		"nbf":              now.Add(-1 * time.Minute).Unix(),
		"repository":       "acme/app",
		"repository_owner": "acme",
		"ref":              "refs/heads/main",
		"sha":              "a1b2c3d4",
		"workflow":         "deploy",
		"actor":            "octocat",
	}
}

func TestVerifyGoodToken(t *testing.T) {
	ti := newTestIssuer(t)
	v := NewVerifier(ti.srv.URL, "https://preview.example.com")
	tok := ti.mint(t, ti.goodClaims("https://preview.example.com"), nil)

	claims, err := v.Verify(context.Background(), tok)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.Repository != "acme/app" || claims.RepositoryOwner != "acme" {
		t.Fatalf("claims = %+v, want repository acme/app", claims)
	}
	if claims.SHA != "a1b2c3d4" || claims.Actor != "octocat" || claims.Workflow != "deploy" {
		t.Fatalf("claims = %+v, missing expected fields", claims)
	}
}

func TestVerifyRejects(t *testing.T) {
	ti := newTestIssuer(t)
	const aud = "https://preview.example.com"
	v := NewVerifier(ti.srv.URL, aud)

	foreign, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	expired := ti.goodClaims(aud)
	expired["exp"] = time.Now().Add(-1 * time.Minute).Unix()

	for _, tc := range []struct {
		name  string
		token string
	}{
		{"wrong audience", ti.mint(t, ti.goodClaims("https://someone-else"), nil)},
		{"expired", ti.mint(t, expired, nil)},
		{"foreign signing key", ti.mint(t, ti.goodClaims(aud), foreign)},
		{"malformed", "not-a-jwt"},
	} {
		if _, err := v.Verify(context.Background(), tc.token); err == nil {
			t.Errorf("%s: Verify accepted a token it should have rejected", tc.name)
		}
	}
}

// helpers

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
