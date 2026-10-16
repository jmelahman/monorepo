// Package osv queries the OSV vulnerability database (https://osv.dev) for
// npm packages.
package osv

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// DefaultURL is the OSV batch query endpoint.
const DefaultURL = "https://api.osv.dev/v1/querybatch"

// Package is an npm package at one version.
type Package struct {
	Name    string
	Version string
}

// Client queries an OSV batch endpoint.
type Client struct {
	// HTTP is the client to send requests with; http.DefaultClient when nil.
	HTTP *http.Client
	// URL is the batch endpoint; DefaultURL when empty.
	URL string
}

type query struct {
	Package struct {
		Ecosystem string `json:"ecosystem"`
		Name      string `json:"name"`
	} `json:"package"`
	Version string `json:"version"`
}

// QueryBatch returns, for each package in order, the IDs of the advisories
// that affect it.
func (c *Client) QueryBatch(ctx context.Context, pkgs []Package) ([][]string, error) {
	var req struct {
		Queries []query `json:"queries"`
	}
	for _, p := range pkgs {
		var q query
		q.Package.Ecosystem = "npm"
		q.Package.Name = p.Name
		q.Version = p.Version
		req.Queries = append(req.Queries, q)
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	url := c.URL
	if url == "" {
		url = DefaultURL
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("osv: %s: %s", resp.Status, bytes.TrimSpace(msg))
	}

	var res struct {
		Results []struct {
			Vulns []struct {
				ID string `json:"id"`
			} `json:"vulns"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("osv: %w", err)
	}
	if len(res.Results) != len(pkgs) {
		return nil, fmt.Errorf("osv: %d results for %d queries", len(res.Results), len(pkgs))
	}
	ids := make([][]string, len(pkgs))
	for i, r := range res.Results {
		for _, v := range r.Vulns {
			ids[i] = append(ids[i], v.ID)
		}
	}
	return ids, nil
}
