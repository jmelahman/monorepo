package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

// ReleaseSubtree is one subtree's state on the Releases tab.
type ReleaseSubtree struct {
	Prefix     string `json:"prefix"`
	Remote     string `json:"remote"`
	Ahead      int    `json:"ahead"`
	Behind     int    `json:"behind"`
	Latest     string `json:"latest"`
	Unreleased int    `json:"unreleased"`
	Counted    bool   `json:"counted"`
	Error      string `json:"error,omitempty"`
	// CI is nil until the first check finishes.
	CI *ReleaseCI `json:"ci,omitempty"`
}

// ReleaseCI is the state of a subtree's latest GitHub Actions runs.
type ReleaseCI struct {
	Status string   `json:"status"`
	Failed []string `json:"failed,omitempty"`
	URL    string   `json:"url,omitempty"`
	Error  string   `json:"error,omitempty"`
}

// ReleaseProject is a repo with a git-orchard manifest.
type ReleaseProject struct {
	RepoPath   string           `json:"repo_path"`
	Base       string           `json:"base"`
	Rev        string           `json:"rev"`
	Subtrees   []ReleaseSubtree `json:"subtrees"`
	Refreshing bool             `json:"refreshing"`
	Error      string           `json:"error,omitempty"`
}

// ReleaseReminder is a note that a subtree is meant to be released.
type ReleaseReminder struct {
	ID       int64  `json:"id"`
	RepoPath string `json:"repo_path"`
	Prefix   string `json:"prefix"`
	Bump     string `json:"bump"`
	Suffix   string `json:"suffix"`
	Note     string `json:"note"`
	Status   string `json:"status"`
}

// ReleaseRun is one attempt at a release.
type ReleaseRun struct {
	ID         int64  `json:"id"`
	ReminderID *int64 `json:"reminder_id,omitempty"`
	RepoPath   string `json:"repo_path"`
	Prefix     string `json:"prefix"`
	Rev        string `json:"rev"`
	Version    string `json:"version"`
	Bump       string `json:"bump"`
	Suffix     string `json:"suffix"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
}

// ReleaseSummary mirrors GET /api/releases.
type ReleaseSummary struct {
	Unavailable string            `json:"unavailable,omitempty"`
	Projects    []ReleaseProject  `json:"projects"`
	Reminders   []ReleaseReminder `json:"reminders"`
	Runs        []ReleaseRun      `json:"runs"`
}

// ReleasePreview is what a release would do right now.
type ReleasePreview struct {
	RepoPath string `json:"repo_path"`
	Prefix   string `json:"prefix"`
	Version  string `json:"version"`
	Rev      string `json:"rev"`
	Released bool   `json:"released"`
	Ahead    int    `json:"ahead"`
	Behind   int    `json:"behind"`
	Blocked  string `json:"blocked,omitempty"`
}

// CreateReleaseReminderArgs is the body of POST /api/release-reminders.
type CreateReleaseReminderArgs struct {
	RepoPath string `json:"repo_path"`
	Prefix   string `json:"prefix"`
	Bump     string `json:"bump,omitempty"`
	Suffix   string `json:"suffix,omitempty"`
	Note     string `json:"note"`
}

// RunReleaseArgs is the body of POST /api/releases/run. Version and Rev pin
// what the caller previewed.
type RunReleaseArgs struct {
	Repo       string `json:"repo"`
	Prefix     string `json:"prefix"`
	Bump       string `json:"bump"`
	Suffix     string `json:"suffix"`
	Version    string `json:"version"`
	Rev        string `json:"rev"`
	ReminderID *int64 `json:"reminder_id,omitempty"`
}

// Releases calls GET /api/releases and returns the raw response.
func (c *Client) Releases(ctx context.Context) (json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, "/api/releases", nil, http.StatusOK)
}

// ReleaseSummary calls GET /api/releases.
func (c *Client) ReleaseSummary(ctx context.Context) (ReleaseSummary, error) {
	var s ReleaseSummary
	raw, err := c.Releases(ctx)
	if err != nil {
		return s, err
	}
	err = json.Unmarshal(raw, &s)
	return s, err
}

// CreateReleaseReminder calls POST /api/release-reminders.
func (c *Client) CreateReleaseReminder(ctx context.Context, a CreateReleaseReminderArgs) (ReleaseReminder, error) {
	var r ReleaseReminder
	raw, err := c.do(ctx, http.MethodPost, "/api/release-reminders", a, http.StatusCreated)
	if err != nil {
		return r, err
	}
	err = json.Unmarshal(raw, &r)
	return r, err
}

// DismissReleaseReminder calls DELETE /api/release-reminders/{id}.
func (c *Client) DismissReleaseReminder(ctx context.Context, id int64) error {
	_, err := c.do(ctx, http.MethodDelete, "/api/release-reminders/"+strconv.FormatInt(id, 10), nil, http.StatusNoContent)
	return err
}

// PreviewRelease calls GET /api/releases/preview, which fetches first.
func (c *Client) PreviewRelease(ctx context.Context, repo, prefix, bump, suffix string) (ReleasePreview, error) {
	var p ReleasePreview
	q := url.Values{"repo": {repo}, "prefix": {prefix}, "bump": {bump}, "suffix": {suffix}}
	raw, err := c.do(ctx, http.MethodGet, "/api/releases/preview?"+q.Encode(), nil, http.StatusOK)
	if err != nil {
		return p, err
	}
	err = json.Unmarshal(raw, &p)
	return p, err
}

// RunRelease calls POST /api/releases/run. The release keeps running on the
// server whatever happens to this client.
func (c *Client) RunRelease(ctx context.Context, a RunReleaseArgs) (ReleaseRun, error) {
	var r ReleaseRun
	raw, err := c.do(ctx, http.MethodPost, "/api/releases/run", a, http.StatusAccepted)
	if err != nil {
		return r, err
	}
	err = json.Unmarshal(raw, &r)
	return r, err
}

// StreamReleaseRunOutput follows GET /api/release-runs/{id}/output from the
// start of the log. It returns nil once the run ends.
func (c *Client) StreamReleaseRunOutput(ctx context.Context, id int64, onLine func(string)) error {
	return c.streamOutput(ctx, "/api/release-runs/"+strconv.FormatInt(id, 10)+"/output", onLine)
}
