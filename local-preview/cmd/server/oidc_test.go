package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGithubActionsToken(t *testing.T) {
	var gotAuth, gotAudience string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAudience = r.URL.Query().Get("audience")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"value": "minted.jwt.token"})
	}))
	defer srv.Close()

	// The runner exposes the request URL with its own query already set; the
	// helper must preserve it while adding audience.
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", srv.URL+"?api-version=2.0")
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "runner-secret")

	tok, err := githubActionsToken(context.Background(), "https://preview.example.com")
	if err != nil {
		t.Fatalf("githubActionsToken: %v", err)
	}
	if tok != "minted.jwt.token" {
		t.Fatalf("token = %q, want minted.jwt.token", tok)
	}
	if gotAuth != "Bearer runner-secret" {
		t.Fatalf("Authorization = %q, want Bearer runner-secret", gotAuth)
	}
	if gotAudience != "https://preview.example.com" {
		t.Fatalf("audience = %q, want the requested audience", gotAudience)
	}
}

func TestGithubActionsTokenNotInActions(t *testing.T) {
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_URL", "")
	t.Setenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "")
	if _, err := githubActionsToken(context.Background(), "aud"); err == nil {
		t.Fatal("expected an error when not running under GitHub Actions")
	}
}

func TestUploadTokenPrecedence(t *testing.T) {
	// A pre-fetched token wins and needs neither --oidc nor the runner.
	t.Setenv("PREVIEW_UPLOAD_TOKEN", "explicit-token")
	tok, err := uploadToken(context.Background(), false, "", "http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	if tok != "explicit-token" {
		t.Fatalf("token = %q, want explicit-token", tok)
	}

	// Without a token and without --oidc, uploads stay unauthenticated.
	t.Setenv("PREVIEW_UPLOAD_TOKEN", "")
	tok, err = uploadToken(context.Background(), false, "", "http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	if tok != "" {
		t.Fatalf("token = %q, want empty (no auth)", tok)
	}
}
