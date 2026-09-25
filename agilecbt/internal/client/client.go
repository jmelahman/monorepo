// Package client is a small HTTP client for the AgileCBT REST API, used by
// the CLI subcommands in cmd/server.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/jmelahman/agilecbt/internal/app"
	"github.com/jmelahman/agilecbt/internal/db"
)

// Client talks to a running `agilecbt serve` over HTTP.
type Client struct {
	base   string
	secret string
	http   *http.Client
}

// New returns a client for the server at base. secret is sent as a bearer
// token when non-empty. Pass nil to use http.DefaultClient.
func New(base, secret string, hc *http.Client) *Client {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Client{base: strings.TrimRight(base, "/"), secret: secret, http: hc}
}

// Today fetches the Today snapshot.
func (c *Client) Today(ctx context.Context) (app.Snapshot, error) {
	var s app.Snapshot
	return s, c.do(ctx, http.MethodGet, "/api/today", nil, &s)
}

// ListGoals lists goals, optionally filtered by status.
func (c *Client) ListGoals(ctx context.Context, status string) ([]db.Goal, error) {
	var gs []db.Goal
	path := "/api/goals"
	if status != "" {
		path += "?status=" + url.QueryEscape(status)
	}
	return gs, c.do(ctx, http.MethodGet, path, nil, &gs)
}

// CreateGoal creates a goal.
func (c *Client) CreateGoal(ctx context.Context, p db.GoalPatch) (db.Goal, error) {
	var g db.Goal
	return g, c.do(ctx, http.MethodPost, "/api/goals", p, &g)
}

// ListSteps lists steps in the given lanes (all lanes when empty).
func (c *Client) ListSteps(ctx context.Context, lanes ...string) ([]db.Step, error) {
	var ss []db.Step
	path := "/api/steps"
	if len(lanes) > 0 {
		path += "?lane=" + url.QueryEscape(strings.Join(lanes, ","))
	}
	return ss, c.do(ctx, http.MethodGet, path, nil, &ss)
}

// CreateStep creates a step in lane.
func (c *Client) CreateStep(ctx context.Context, lane string, p db.StepPatch) (db.Step, error) {
	body := struct {
		db.StepPatch
		Lane string `json:"lane,omitempty"`
	}{p, lane}
	var s db.Step
	return s, c.do(ctx, http.MethodPost, "/api/steps", body, &s)
}

// MoveStep moves a step to the end of lane.
func (c *Client) MoveStep(ctx context.Context, id int64, lane string) (db.Step, error) {
	var s db.Step
	return s, c.do(ctx, http.MethodPatch, fmt.Sprintf("/api/steps/%d", id), map[string]string{"lane": lane}, &s)
}

// CompleteStep marks a step done with optional 0-10 ratings.
func (c *Client) CompleteStep(ctx context.Context, id int64, mastery, pleasure *int) (db.Step, error) {
	var s db.Step
	body := map[string]*int{"mastery": mastery, "pleasure": pleasure}
	return s, c.do(ctx, http.MethodPost, fmt.Sprintf("/api/steps/%d/complete", id), body, &s)
}

// Export downloads the full data export as raw JSON.
func (c *Client) Export(ctx context.Context) (json.RawMessage, error) {
	var raw json.RawMessage
	return raw, c.do(ctx, http.MethodGet, "/api/export", nil, &raw)
}

// Import restores an export into an empty instance.
func (c *Client) Import(ctx context.Context, dump json.RawMessage) error {
	return c.do(ctx, http.MethodPost, "/api/import", dump, nil)
}

// do performs a request, decoding a 2xx body into out (when non-nil) and
// converting other responses into errors using the API's {"error": "..."}
// body when present.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		raw, ok := body.(json.RawMessage)
		if !ok {
			var err error
			if raw, err = json.Marshal(body); err != nil {
				return err
			}
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(raw, &e) == nil && e.Error != "" {
			return fmt.Errorf("%s %s: %s", method, path, e.Error)
		}
		if resp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("%s %s: unauthorized (set APP_SECRET)", method, path)
		}
		return fmt.Errorf("%s %s: unexpected status %d", method, path, resp.StatusCode)
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	if r, ok := out.(*json.RawMessage); ok {
		*r = raw
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}
