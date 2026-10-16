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
	"time"
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
	// HTTP is the client to send requests with; one with a one-minute
	// timeout when nil.
	HTTP *http.Client
	// URL is the batch endpoint; DefaultURL when empty.
	URL string
}

// batchSize is the most queries OSV takes in one batch request.
const batchSize = 1000

// defaultHTTP bounds a request, so a stalled OSV fails the audit rather
// than hanging it.
var defaultHTTP = &http.Client{Timeout: time.Minute}

type query struct {
	Package struct {
		Ecosystem string `json:"ecosystem"`
		Name      string `json:"name"`
	} `json:"package"`
	Version   string `json:"version"`
	PageToken string `json:"page_token,omitempty"`
}

type result struct {
	NextPageToken string `json:"next_page_token"`
	Vulns         []struct {
		ID string `json:"id"`
	} `json:"vulns"`
}

// QueryBatch returns, for each package in order, the IDs of the advisories
// that affect it. It splits the packages into batches OSV accepts and
// follows each query's pages.
func (c *Client) QueryBatch(ctx context.Context, pkgs []Package) ([][]string, error) {
	ids := make([][]string, len(pkgs))
	// pending holds the index into pkgs of each query still to send, with
	// its page token after the first page.
	type pendingQuery struct {
		token string
		i     int
	}
	pending := make([]pendingQuery, len(pkgs))
	for i := range pkgs {
		pending[i].i = i
	}
	for len(pending) > 0 {
		n := min(len(pending), batchSize)
		batch := pending[:n]
		queries := make([]query, n)
		for j, pq := range batch {
			queries[j].Package.Ecosystem = "npm"
			queries[j].Package.Name = pkgs[pq.i].Name
			queries[j].Version = pkgs[pq.i].Version
			queries[j].PageToken = pq.token
		}
		results, err := c.post(ctx, queries)
		if err != nil {
			return nil, err
		}
		rest := pending[n:]
		for j, r := range results {
			for _, v := range r.Vulns {
				ids[batch[j].i] = append(ids[batch[j].i], v.ID)
			}
			if r.NextPageToken != "" {
				rest = append(rest, pendingQuery{i: batch[j].i, token: r.NextPageToken})
			}
		}
		pending = rest
	}
	return ids, nil
}

// post sends one batch request and returns its results, one per query.
func (c *Client) post(ctx context.Context, queries []query) ([]result, error) {
	body, err := json.Marshal(struct {
		Queries []query `json:"queries"`
	}{queries})
	if err != nil {
		return nil, err
	}

	url := c.URL
	if url == "" {
		url = DefaultURL
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = defaultHTTP
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
		Results []result `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("osv: %w", err)
	}
	if len(res.Results) != len(queries) {
		return nil, fmt.Errorf("osv: %d results for %d queries", len(res.Results), len(queries))
	}
	return res.Results, nil
}
