// Package registry asks an npm registry for a package's latest version.
package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// DefaultURL is the public npm registry.
const DefaultURL = "https://registry.npmjs.org"

// Client queries an npm registry.
type Client struct {
	// HTTP defaults to http.DefaultClient.
	HTTP *http.Client
	// URL defaults to DefaultURL.
	URL string
}

// Latest returns the version the registry's latest dist-tag points at.
func (c *Client) Latest(ctx context.Context, name string) (string, error) {
	base := c.URL
	if base == "" {
		base = DefaultURL
	}
	// A scoped name keeps its @ and has its / escaped.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(base, "/")+"/"+url.PathEscape(name), nil)
	if err != nil {
		return "", err
	}
	// The abbreviated document: dist-tags and versions, without readmes.
	req.Header.Set("Accept", "application/vnd.npm.install-v1+json")
	httpc := c.HTTP
	if httpc == nil {
		httpc = http.DefaultClient
	}
	resp, err := httpc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: registry returned %s", name, resp.Status)
	}
	var doc struct {
		DistTags struct {
			Latest string `json:"latest"`
		} `json:"dist-tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return "", fmt.Errorf("%s: %w", name, err)
	}
	if doc.DistTags.Latest == "" {
		return "", fmt.Errorf("%s: no latest dist-tag", name)
	}
	return doc.DistTags.Latest, nil
}
