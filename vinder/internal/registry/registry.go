// Package registry asks an npm registry which version of a package a dist-tag
// or range means.
package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
)

// DefaultURL is the public npm registry.
const DefaultURL = "https://registry.npmjs.org"

// defaultHTTP bounds a request, so a stalled registry fails the upgrade
// rather than hanging it.
var defaultHTTP = &http.Client{Timeout: time.Minute}

// Client queries an npm registry.
type Client struct {
	// HTTP defaults to a client with a one-minute timeout.
	HTTP *http.Client
	// URL defaults to DefaultURL.
	URL string
}

// document is the part of a package's registry document vinder reads.
type document struct {
	DistTags map[string]string          `json:"dist-tags"`
	Versions map[string]json.RawMessage `json:"versions"`
}

// Latest returns the version the registry's latest dist-tag points at.
func (c *Client) Latest(ctx context.Context, name string) (string, error) {
	doc, err := c.document(ctx, name)
	if err != nil {
		return "", err
	}
	if doc.DistTags["latest"] == "" {
		return "", fmt.Errorf("%s: no latest dist-tag", name)
	}
	return doc.DistTags["latest"], nil
}

// Resolve returns the version npm would install for spec, a dist-tag or a
// version range: the one a tag points at, or else latest if it satisfies the
// range, or else the highest release that does.
func (c *Client) Resolve(ctx context.Context, name, spec string) (string, error) {
	doc, err := c.document(ctx, name)
	if err != nil {
		return "", err
	}
	if version := doc.DistTags[spec]; version != "" {
		return version, nil
	}
	constraint, err := semver.NewConstraint(spec)
	if err != nil {
		return "", fmt.Errorf("%s: no dist-tag or version range %q", name, spec)
	}
	if latest, err := semver.NewVersion(doc.DistTags["latest"]); err == nil && constraint.Check(latest) {
		return latest.Original(), nil
	}
	var best *semver.Version
	for version := range doc.Versions {
		v, err := semver.NewVersion(version)
		if err == nil && constraint.Check(v) && (best == nil || v.GreaterThan(best)) {
			best = v
		}
	}
	if best == nil {
		return "", fmt.Errorf("%s: no release matches %q", name, spec)
	}
	return best.Original(), nil
}

func (c *Client) document(ctx context.Context, name string) (*document, error) {
	base := c.URL
	if base == "" {
		base = DefaultURL
	}
	// A scoped name keeps its @ and has its / escaped.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(base, "/")+"/"+url.PathEscape(name), nil)
	if err != nil {
		return nil, err
	}
	// The abbreviated document: dist-tags and versions, without readmes.
	req.Header.Set("Accept", "application/vnd.npm.install-v1+json")
	httpc := c.HTTP
	if httpc == nil {
		httpc = defaultHTTP
	}
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: registry returned %s", name, resp.Status)
	}
	var doc document
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return &doc, nil
}
