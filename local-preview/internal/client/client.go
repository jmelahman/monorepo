// Package client is a small HTTP client for the app's REST API, used by the
// CLI subcommands in cmd/server.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Repo mirrors the API's repo shape.
type Repo struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Source        string `json:"source"`
	Watch         bool   `json:"watch"`
	WatchBranches string `json:"watch_branches"`
	// Status is the mirror clone's state (cloning/ready/failed); Error is
	// set when it failed, Progress while it's still cloning.
	Status    string `json:"status"`
	Error     string `json:"error"`
	Progress  string `json:"progress"`
	CreatedAt string `json:"created_at"`
}

// Deploy mirrors the API's deploy shape.
type Deploy struct {
	ID           int64            `json:"id"`
	Repo         string           `json:"repo"`
	SHA          string           `json:"sha"`
	ShortSHA     string           `json:"short_sha"`
	Ref          string           `json:"ref"`
	Branch       string           `json:"branch"`
	AuthorName   string           `json:"author_name"`
	AuthorEmail  string           `json:"author_email"`
	CreatedBy    string           `json:"created_by"`
	FeHash       string           `json:"fe_hash"`
	BeHash       string           `json:"be_hash"`
	Status       string           `json:"status"`
	Error        string           `json:"error"`
	AttemptCount int64            `json:"attempt_count"`
	PreviewURL   string           `json:"preview_url"`
	Process      string           `json:"process"`
	FeProcess    string           `json:"fe_process"`
	Artifacts    []DeployArtifact `json:"artifacts,omitempty"`
	CreatedAt    string           `json:"created_at"`
	UpdatedAt    string           `json:"updated_at"`
}

// DeployArtifact mirrors the API's downloadable-artifact shape on ready
// deploys. Artifacts build after the deploy turns ready, so Status can lag
// the deploy's: building, ready, or failed (Error holds the summary).
type DeployArtifact struct {
	Name   string         `json:"name"`
	Hash   string         `json:"hash"`
	Status string         `json:"status"`
	Error  string         `json:"error,omitempty"`
	Files  []ArtifactFile `json:"files"`
}

// ArtifactFile is one downloadable file within an artifact. URL is a path
// on the server (join it with the server's base URL).
type ArtifactFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	URL  string `json:"url"`
}

// Client talks to a running `preview serve` over HTTP.
type Client struct {
	base  string
	http  *http.Client
	token string
}

// New returns a client for the server at base. Pass nil to use
// http.DefaultClient.
func New(base string, hc *http.Client) *Client {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Client{base: strings.TrimRight(base, "/"), http: hc}
}

// SetToken sets a bearer token sent as "Authorization: Bearer <token>" on
// every request. Used by `preview upload` to present a GitHub Actions OIDC
// token; returns the client for chaining.
func (c *Client) SetToken(token string) *Client {
	c.token = token
	return c
}

// Health mirrors the API's health shape.
type Health struct {
	Status        string `json:"status"`
	Version       string `json:"version"`
	PreviewDomain string `json:"preview_domain"`
}

// GetHealth pings the server and returns what it reports about itself.
func (c *Client) GetHealth(ctx context.Context) (Health, error) {
	raw, err := c.do(ctx, http.MethodGet, "/api/health", nil)
	if err != nil {
		return Health{}, err
	}
	var h Health
	if err := json.Unmarshal(raw, &h); err != nil {
		return Health{}, fmt.Errorf("decode health: %w", err)
	}
	return h, nil
}

// CreateRepo registers a repo, optionally watched from the start. backfill
// deploys the branch tips it already has, rather than only what moves after
// registration.
func (c *Client) CreateRepo(ctx context.Context, name, source string, watch bool, watchBranches string, backfill bool) (Repo, error) {
	body, err := json.Marshal(map[string]any{
		"name": name, "source": source,
		"watch": watch, "watch_branches": watchBranches,
		"backfill": backfill,
	})
	if err != nil {
		return Repo{}, err
	}
	raw, err := c.do(ctx, http.MethodPost, "/api/repos", body)
	if err != nil {
		return Repo{}, err
	}
	var r Repo
	if err := json.Unmarshal(raw, &r); err != nil {
		return Repo{}, fmt.Errorf("decode repo: %w", err)
	}
	return r, nil
}

// GetRepo fetches one repo by name.
func (c *Client) GetRepo(ctx context.Context, name string) (Repo, error) {
	raw, err := c.do(ctx, http.MethodGet, "/api/repos/"+url.PathEscape(name), nil)
	if err != nil {
		return Repo{}, err
	}
	var r Repo
	if err := json.Unmarshal(raw, &r); err != nil {
		return Repo{}, fmt.Errorf("decode repo: %w", err)
	}
	return r, nil
}

// SetRepoWatch enables or disables watching for a repo. branches == nil
// leaves the stored branch filter unchanged. backfill also deploys the
// branch tips that already exist, rather than only what moves from here.
func (c *Client) SetRepoWatch(ctx context.Context, name string, watch bool, branches *string, backfill bool) (Repo, error) {
	payload := map[string]any{"watch": watch}
	if branches != nil {
		payload["watch_branches"] = *branches
	}
	if backfill {
		payload["backfill"] = true
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Repo{}, err
	}
	raw, err := c.do(ctx, http.MethodPatch, "/api/repos/"+url.PathEscape(name), body)
	if err != nil {
		return Repo{}, err
	}
	var r Repo
	if err := json.Unmarshal(raw, &r); err != nil {
		return Repo{}, fmt.Errorf("decode repo: %w", err)
	}
	return r, nil
}

// ListRepos fetches all registered repos.
func (c *Client) ListRepos(ctx context.Context) ([]Repo, error) {
	raw, err := c.do(ctx, http.MethodGet, "/api/repos", nil)
	if err != nil {
		return nil, err
	}
	var repos []Repo
	if err := json.Unmarshal(raw, &repos); err != nil {
		return nil, fmt.Errorf("decode repos: %w", err)
	}
	return repos, nil
}

// DeleteRepo unregisters a repo and deletes its deploys, artifacts, state,
// and mirror clone.
func (c *Client) DeleteRepo(ctx context.Context, name string) error {
	_, err := c.do(ctx, http.MethodDelete, "/api/repos/"+url.PathEscape(name), nil)
	return err
}

// CreateDeploy requests a deploy of ref in repo.
func (c *Client) CreateDeploy(ctx context.Context, repo, ref string, rebuild bool) (Deploy, error) {
	body, err := json.Marshal(map[string]any{"repo": repo, "ref": ref, "rebuild": rebuild})
	if err != nil {
		return Deploy{}, err
	}
	raw, err := c.do(ctx, http.MethodPost, "/api/deploys", body)
	if err != nil {
		return Deploy{}, err
	}
	var d Deploy
	if err := json.Unmarshal(raw, &d); err != nil {
		return Deploy{}, fmt.Errorf("decode deploy: %w", err)
	}
	return d, nil
}

// DeployFilter narrows ListDeploys; zero-value fields don't filter. It
// mirrors the API's query params: repo, branch, and status match exactly
// (status also takes "crashed", the ready deploys whose process died, which
// "ready" excludes), author is a case-insensitive substring of the author
// name or email, and query is a free-text search (sha prefix, or a
// substring of the repo, branch, ref, or author).
type DeployFilter struct {
	Repo   string
	Branch string
	Author string
	Status string
	Query  string
}

// ListDeploys fetches deploys narrowed by the filter.
func (c *Client) ListDeploys(ctx context.Context, f DeployFilter) ([]Deploy, error) {
	q := url.Values{}
	for key, val := range map[string]string{
		"repo": f.Repo, "branch": f.Branch, "author": f.Author,
		"status": f.Status, "q": f.Query,
	} {
		if val != "" {
			q.Set(key, val)
		}
	}
	path := "/api/deploys"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	raw, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var deploys []Deploy
	if err := json.Unmarshal(raw, &deploys); err != nil {
		return nil, fmt.Errorf("decode deploys: %w", err)
	}
	return deploys, nil
}

// StopDeploy stops the deploy's supervised processes without removing it;
// they cold-start again on the next request.
func (c *Client) StopDeploy(ctx context.Context, id int64) (Deploy, error) {
	raw, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/api/deploys/%d/stop", id), nil)
	if err != nil {
		return Deploy{}, err
	}
	var d Deploy
	if err := json.Unmarshal(raw, &d); err != nil {
		return Deploy{}, fmt.Errorf("decode deploy: %w", err)
	}
	return d, nil
}

// DeleteDeploy removes a deploy and garbage-collects any artifacts and state
// no surviving deploy still references.
func (c *Client) DeleteDeploy(ctx context.Context, id int64) error {
	_, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/api/deploys/%d", id), nil)
	return err
}

// GetDeploy fetches one deploy by id.
func (c *Client) GetDeploy(ctx context.Context, id int64) (Deploy, error) {
	raw, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/deploys/%d", id), nil)
	if err != nil {
		return Deploy{}, err
	}
	var d Deploy
	if err := json.Unmarshal(raw, &d); err != nil {
		return Deploy{}, fmt.Errorf("decode deploy: %w", err)
	}
	return d, nil
}

// GetDeployLogs fetches the plain-text build log snapshot.
func (c *Client) GetDeployLogs(ctx context.Context, id int64) (string, error) {
	raw, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/deploys/%d/logs", id), nil)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// RunLogChunk mirrors the API's incremental run-log slice.
type RunLogChunk struct {
	Side      string `json:"side"`
	Attempt   int    `json:"attempt"`
	Offset    int64  `json:"offset"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
	Process   string `json:"process"`
}

// GetDeployRunLog fetches a slice of the deploy's process run log. side is
// "be" or "fe". Zero attempt/offset fetch a fresh tail; passing the prior
// chunk's values back yields only bytes appended since.
func (c *Client) GetDeployRunLog(ctx context.Context, id int64, side string, attempt int, offset int64) (RunLogChunk, error) {
	q := url.Values{"side": {side}}
	if attempt > 0 {
		q.Set("attempt", strconv.Itoa(attempt))
		q.Set("offset", strconv.FormatInt(offset, 10))
	}
	raw, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/deploys/%d/logs/run?%s", id, q.Encode()), nil)
	if err != nil {
		return RunLogChunk{}, err
	}
	var chunk RunLogChunk
	if err := json.Unmarshal(raw, &chunk); err != nil {
		return RunLogChunk{}, fmt.Errorf("decode run log: %w", err)
	}
	return chunk, nil
}

// SideStats mirrors one side of the API's deploy stats response. Sampled
// fields are nil/zero while the process isn't running; CPUPercent needs two
// samples, so it is nil on the first call after a start.
type SideStats struct {
	State            string   `json:"state"`
	Runtime          string   `json:"runtime"`
	CPUPercent       *float64 `json:"cpu_percent"`
	MemoryBytes      *uint64  `json:"memory_bytes"`
	MemoryLimitBytes uint64   `json:"memory_limit_bytes"`
	StartedAt        string   `json:"started_at"`
}

// DeployStats is live resource usage of a deploy's processes; a side the
// deploy doesn't have is nil.
type DeployStats struct {
	Backend  *SideStats `json:"backend"`
	Frontend *SideStats `json:"frontend"`
}

// GetDeployStats fetches live resource usage of the deploy's processes.
func (c *Client) GetDeployStats(ctx context.Context, id int64) (DeployStats, error) {
	raw, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/deploys/%d/stats", id), nil)
	if err != nil {
		return DeployStats{}, err
	}
	var s DeployStats
	if err := json.Unmarshal(raw, &s); err != nil {
		return DeployStats{}, fmt.Errorf("decode stats: %w", err)
	}
	return s, nil
}

// UploadResult mirrors the API's upload response: the resolved commit, the
// content-address the upload targeted, and whether bytes actually landed
// (false when the artifact was already present and overwrite wasn't set).
type UploadResult struct {
	SHA       string       `json:"sha"`
	ShortSHA  string       `json:"short_sha"`
	Side      string       `json:"side"`
	Name      string       `json:"name,omitempty"`
	Hash      string       `json:"hash"`
	Published bool         `json:"published"`
	Files     []UploadFile `json:"files,omitempty"`
}

// UploadFile is one published file of an uploaded downloadable artifact.
type UploadFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// Upload streams a CI-built side (a tar or tar.gz of the frontend bundle,
// backend tree, or a named artifact's files) into the content-addressed store
// for repo at ref. side is "frontend", "backend", or "artifact" (with name);
// overwrite replaces an already-present artifact. The server resolves ref and
// computes the hash — the same one a build would target.
func (c *Client) Upload(ctx context.Context, repo, side, name, ref string, overwrite bool, body io.Reader) (UploadResult, error) {
	var path string
	switch side {
	case "artifact":
		path = fmt.Sprintf("/api/repos/%s/uploads/artifacts/%s", url.PathEscape(repo), url.PathEscape(name))
	default:
		path = fmt.Sprintf("/api/repos/%s/uploads/%s", url.PathEscape(repo), side)
	}
	q := url.Values{"ref": {ref}}
	if overwrite {
		q.Set("overwrite", "true")
	}
	raw, err := c.doStream(ctx, http.MethodPost, path+"?"+q.Encode(), "application/octet-stream", body)
	if err != nil {
		return UploadResult{}, err
	}
	var res UploadResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return UploadResult{}, fmt.Errorf("decode upload result: %w", err)
	}
	return res, nil
}

// do performs a request and returns the response body, converting non-2xx
// responses into errors using the API's {"error": "..."} body when present.
func (c *Client) do(ctx context.Context, method, path string, body []byte) (json.RawMessage, error) {
	if body == nil {
		return c.doStream(ctx, method, path, "", nil)
	}
	return c.doStream(ctx, method, path, "application/json", bytes.NewReader(body))
}

// doStream is do with a streaming body and explicit content type — used for
// tar uploads, which must not be buffered whole into memory.
func (c *Client) doStream(ctx context.Context, method, path, contentType string, body io.Reader) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(raw, &e) == nil && e.Error != "" {
			if resp.StatusCode == http.StatusUnauthorized {
				// The server wants a credential this client didn't (or
				// couldn't) present — say how to fix it instead of leaving a
				// bare "authentication required".
				hint := "sign in to the GitHub CLI (`gh auth login`) so its token is used automatically, or set one explicitly: `preview configure --token <github-pat>` / $PREVIEW_TOKEN"
				if c.token != "" {
					hint = "the presented GitHub token was rejected — it may be expired, revoked, or its account not on the server's allowlist"
				}
				return nil, fmt.Errorf("%s %s: %s (%s)", method, path, e.Error, hint)
			}
			return nil, fmt.Errorf("%s %s: %s", method, path, e.Error)
		}
		return nil, fmt.Errorf("%s %s: unexpected status %d", method, path, resp.StatusCode)
	}
	return raw, nil
}
