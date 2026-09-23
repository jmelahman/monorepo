package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)

// uploadToken resolves the bearer token `preview upload` presents to the
// server, in precedence order:
//
//  1. $PREVIEW_UPLOAD_TOKEN — a pre-fetched token, used verbatim. This is the
//     escape hatch for non-Actions OIDC (or tests) and works without --oidc.
//  2. --oidc — mint a token from the GitHub Actions runner for the audience
//     (flag, else $PREVIEW_GITHUB_OIDC_AUDIENCE, else the server URL).
//  3. otherwise no token (unauthenticated, the default).
func uploadToken(ctx context.Context, oidc bool, audience, serverURL string) (string, error) {
	if tok := os.Getenv("PREVIEW_UPLOAD_TOKEN"); tok != "" {
		return tok, nil
	}
	if !oidc {
		return "", nil
	}
	if audience == "" {
		audience = os.Getenv("PREVIEW_GITHUB_OIDC_AUDIENCE")
	}
	if audience == "" {
		audience = serverURL
	}
	return githubActionsToken(ctx, audience)
}

// githubActionsToken mints a GitHub Actions OIDC ID token for audience using
// the request URL and token the runner exposes to a job with
// `permissions: id-token: write`. It mirrors actions/core's getIDToken.
func githubActionsToken(ctx context.Context, audience string) (string, error) {
	reqURL := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL")
	reqTok := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
	if reqURL == "" || reqTok == "" {
		return "", fmt.Errorf("no GitHub Actions OIDC token available: " +
			"ACTIONS_ID_TOKEN_REQUEST_URL is unset — add `permissions: id-token: write` to the job")
	}
	u, err := url.Parse(reqURL)
	if err != nil {
		return "", fmt.Errorf("parse ACTIONS_ID_TOKEN_REQUEST_URL: %w", err)
	}
	if audience != "" {
		q := u.Query()
		q.Set("audience", audience)
		u.RawQuery = q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+reqTok)
	req.Header.Set("Accept", "application/json; api-version=2.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request OIDC token: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("request OIDC token: unexpected status %d", resp.StatusCode)
	}
	var out struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("decode OIDC token response: %w", err)
	}
	if out.Value == "" {
		return "", fmt.Errorf("OIDC token response contained no token")
	}
	return out.Value, nil
}
