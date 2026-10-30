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

const (
	// abbreviatedDocument asks for dist-tags and versions, without readmes.
	abbreviatedDocument = "application/vnd.npm.install-v1+json"
	// fullDocument asks for everything, which is the only way to get release
	// times.
	fullDocument = "application/json"
)

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
	// Time holds each version's release time, in the full document only. It
	// stays raw because an unpublished package has an object under
	// "unpublished" in it.
	Time map[string]json.RawMessage `json:"time"`
}

// releasedBy reports whether version's release time is known and not after
// cutoff.
func (d *document) releasedBy(version string, cutoff time.Time) bool {
	var released time.Time
	if err := json.Unmarshal(d.Time[version], &released); err != nil {
		return false
	}
	return !released.After(cutoff)
}

// Latest returns the version the registry's latest dist-tag points at.
func (c *Client) Latest(ctx context.Context, name string) (string, error) {
	doc, err := c.document(ctx, name, abbreviatedDocument)
	if err != nil {
		return "", err
	}
	if doc.DistTags["latest"] == "" {
		return "", fmt.Errorf("%s: no latest dist-tag", name)
	}
	return doc.DistTags["latest"], nil
}

// LatestBefore returns the newest release out by cutoff: the version the
// latest dist-tag points at if it is, and otherwise the highest stable
// version below it that is. It returns "" if no release is that old. A
// registry that gives no release times is an error, since nothing could then
// be said to be old enough.
func (c *Client) LatestBefore(ctx context.Context, name string, cutoff time.Time) (string, error) {
	doc, err := c.document(ctx, name, fullDocument)
	if err != nil {
		return "", err
	}
	if doc.DistTags["latest"] == "" {
		return "", fmt.Errorf("%s: no latest dist-tag", name)
	}
	if len(doc.Time) == 0 {
		return "", fmt.Errorf("%s: registry gives no release times", name)
	}
	if doc.releasedBy(doc.DistTags["latest"], cutoff) {
		return doc.DistTags["latest"], nil
	}
	latest, err := semver.NewVersion(doc.DistTags["latest"])
	if err != nil {
		return "", fmt.Errorf("%s: latest dist-tag: %w", name, err)
	}
	var best *semver.Version
	for version := range doc.Versions {
		v, err := semver.NewVersion(version)
		if err != nil || v.Prerelease() != "" || !v.LessThan(latest) || !doc.releasedBy(version, cutoff) {
			continue
		}
		if best == nil || v.GreaterThan(best) {
			best = v
		}
	}
	if best == nil {
		return "", nil
	}
	return best.Original(), nil
}

// Resolve returns the version npm would install for spec, a dist-tag or a
// version range: the one a tag points at, or else latest if it satisfies the
// range, or else the highest release that does.
func (c *Client) Resolve(ctx context.Context, name, spec string) (string, error) {
	doc, err := c.document(ctx, name, abbreviatedDocument)
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

func (c *Client) document(ctx context.Context, name, accept string) (*document, error) {
	base := c.URL
	if base == "" {
		base = DefaultURL
	}
	// A scoped name keeps its @ and has its / escaped.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(base, "/")+"/"+url.PathEscape(name), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
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
