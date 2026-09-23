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
	"strings"
)

// Item mirrors the API's item shape.
type Item struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
}

// Client talks to a running `app serve` over HTTP.
type Client struct {
	base string
	http *http.Client
}

// New returns a client for the server at base. Pass nil to use
// http.DefaultClient.
func New(base string, hc *http.Client) *Client {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Client{base: strings.TrimRight(base, "/"), http: hc}
}

// ListItems fetches all items.
func (c *Client) ListItems(ctx context.Context) ([]Item, error) {
	raw, err := c.do(ctx, http.MethodGet, "/api/items", nil)
	if err != nil {
		return nil, err
	}
	var items []Item
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("decode items: %w", err)
	}
	return items, nil
}

// CreateItem creates an item and returns the raw JSON response.
func (c *Client) CreateItem(ctx context.Context, title string) (json.RawMessage, error) {
	body, err := json.Marshal(map[string]string{"title": title})
	if err != nil {
		return nil, err
	}
	return c.do(ctx, http.MethodPost, "/api/items", body)
}

// DeleteItem deletes an item by id.
func (c *Client) DeleteItem(ctx context.Context, id int64) error {
	_, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/api/items/%d", id), nil)
	return err
}

// do performs a request and returns the response body, converting non-2xx
// responses into errors using the API's {"error": "..."} body when present.
func (c *Client) do(ctx context.Context, method, path string, body []byte) (json.RawMessage, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
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
			return nil, fmt.Errorf("%s %s: %s", method, path, e.Error)
		}
		return nil, fmt.Errorf("%s %s: unexpected status %d", method, path, resp.StatusCode)
	}
	return raw, nil
}
