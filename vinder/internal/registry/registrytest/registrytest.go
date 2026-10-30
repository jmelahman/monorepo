// Package registrytest serves packages over the npm registry protocol, for
// tests that run a real package manager without the network.
package registrytest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // npm's shasum field is SHA-1.
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
)

// Package is one version of a package to serve.
type Package struct {
	// Dependencies goes into the generated package.json.
	Dependencies map[string]string
	// Files are the package's files by path. Unless it has its own, a
	// package.json naming index.js as main is generated.
	Files map[string]string
	// Tags are the dist-tags that point at this version. A version with any
	// is left out when working out latest, unless latest is one of them.
	Tags    []string
	Name    string
	Version string
	// Published is the release time, by default long enough ago to pass any
	// cooldown.
	Published time.Time
}

// Server is a registry that serves what has been published to it.
type Server struct {
	tarballs map[string][]byte
	packages map[string][]Package
	// URL is the registry's base URL. It ends in a path unique to the
	// server, so package-manager caches keyed on the URL never mix servers
	// that reuse a port.
	URL string
	mu  sync.Mutex
}

// New starts a registry serving pkgs, stopped when the test ends.
func New(t testing.TB, pkgs ...Package) *Server {
	t.Helper()
	s := &Server{tarballs: map[string][]byte{}, packages: map[string][]Package{}}
	srv := httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(srv.Close)
	s.URL = srv.URL + "/" + rand.Text()
	for _, p := range pkgs {
		s.Publish(t, p)
	}
	return s
}

// Publish adds a package version; the highest version is latest.
func (s *Server) Publish(t testing.TB, p Package) {
	t.Helper()
	files := map[string]string{}
	maps.Copy(files, p.Files)
	if _, ok := files["package.json"]; !ok {
		data, err := json.Marshal(map[string]any{"name": p.Name, "version": p.Version, "main": "index.js", "dependencies": p.Dependencies})
		if err != nil {
			t.Fatal(err)
		}
		files["package.json"] = string(data)
	}
	tgz, err := Pack(files)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tarballs[tarballPath(p.Name, p.Version)] = tgz
	s.packages[p.Name] = append(s.packages[p.Name], p)
}

// Pack makes a package tarball of files, each under package/ as npm packs
// them. The output depends only on files.
func Pack(files map[string]string) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		hdr := &tar.Header{Name: "package/" + name, Mode: 0o644, Size: int64(len(files[name]))}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write([]byte(files[name])); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func tarballPath(name, version string) string {
	return "/-/" + url.PathEscape(name) + "/" + version + ".tgz"
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	base, err := url.Parse(s.URL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	path, ok := strings.CutPrefix(r.URL.EscapedPath(), base.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	// Package managers cache metadata; tests publish between installs.
	w.Header().Set("Cache-Control", "public, max-age=0")

	s.mu.Lock()
	defer s.mu.Unlock()
	if tgz, ok := s.tarballs[path]; ok {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(tgz)
		return
	}
	name, err := url.PathUnescape(strings.TrimPrefix(path, "/"))
	if err != nil || len(s.packages[name]) == 0 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	doc := s.packument(name)
	// As on the real registry, only the full document has release times.
	if strings.Contains(r.Header.Get("Accept"), "application/vnd.npm.install-v1+json") {
		delete(doc, "time")
	}
	_ = json.NewEncoder(w).Encode(doc)
}

// packument is the registry's document for name: every version, and its
// dist-tags.
func (s *Server) packument(name string) map[string]any {
	versions := map[string]any{}
	times := map[string]string{"created": "2020-01-01T00:00:00.000Z", "modified": "2020-01-01T00:00:00.000Z"}
	var latest *semver.Version
	tags := map[string]string{}
	for _, p := range s.packages[name] {
		tgz := s.tarballs[tarballPath(p.Name, p.Version)]
		sha1sum := sha1.Sum(tgz) //nolint:gosec // npm's shasum field is SHA-1.
		sha512sum := sha512.Sum512(tgz)
		versions[p.Version] = map[string]any{
			"name":         p.Name,
			"version":      p.Version,
			"dependencies": p.Dependencies,
			"dist": map[string]any{
				"tarball":   s.URL + tarballPath(p.Name, p.Version),
				"shasum":    hex.EncodeToString(sha1sum[:]),
				"integrity": "sha512-" + base64.StdEncoding.EncodeToString(sha512sum[:]),
			},
		}
		times[p.Version] = "2020-01-01T00:00:00.000Z"
		if !p.Published.IsZero() {
			times[p.Version] = p.Published.UTC().Format("2006-01-02T15:04:05.000Z07:00")
			// Package managers skip reading release times in a document
			// modified before their cutoff.
			if times[p.Version] > times["modified"] {
				times["modified"] = times[p.Version]
			}
		}
		for _, tag := range p.Tags {
			tags[tag] = p.Version
		}
		if v, err := semver.NewVersion(p.Version); err == nil && len(p.Tags) == 0 && (latest == nil || v.GreaterThan(latest)) {
			latest = v
		}
	}
	if _, tagged := tags["latest"]; !tagged && latest != nil {
		tags["latest"] = latest.Original()
	}
	return map[string]any{"name": name, "versions": versions, "dist-tags": tags, "time": times, "modified": times["modified"]}
}
