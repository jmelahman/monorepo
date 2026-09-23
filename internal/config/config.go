// Package config resolves where the application keeps its on-disk state and
// how its previews are addressed publicly.
package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
)

// appName is the directory name used under the XDG data home. Keep it in
// sync with the binary name in .goreleaser.yaml and pyproject.toml.
const appName = "preview"

// DefaultPreviewDomain is the base domain previews are served under when
// neither the flag nor $PREVIEW_DOMAIN overrides it. *.localhost resolves to
// loopback in browsers with no DNS setup.
const DefaultPreviewDomain = "preview.localhost"

// PreviewBase is the public address space previews are served under —
// everything needed to turn a deploy into the URL a browser should open.
type PreviewBase struct {
	// Domain is the base domain previews are subdomains of.
	Domain string
	// Scheme is the public scheme, "http" or "https". The zero value is
	// treated as "http".
	Scheme string
	// Port is the public port, empty when previews are reached on the
	// scheme's default port.
	Port string
}

// URL returns the preview URL of one deploy:
// <scheme>://<short-sha>-<repo>.<domain>[:port]/. The port is omitted when
// it's the scheme's default, so a TLS-terminated deployment yields a clean
// https:// URL.
func (b PreviewBase) URL(shortSHA, repoName string) string {
	scheme := b.Scheme
	if scheme == "" {
		scheme = "http"
	}
	host := fmt.Sprintf("%s-%s.%s", shortSHA, repoName, b.Domain)
	if b.Port != "" && b.Port != defaultPort(scheme) {
		host += ":" + b.Port
	}
	return scheme + "://" + host + "/"
}

// defaultPort is the port a scheme implies, and which URLs therefore omit.
func defaultPort(scheme string) string {
	if scheme == "https" {
		return "443"
	}
	return "80"
}

// ResolvePreviewBase determines how previews are addressed from the outside.
//
// baseURL is the explicit public base of previews (e.g.
// "https://preview.example.com") and wins outright: it carries the scheme
// and public port that a listen address can't know behind a TLS-terminating
// proxy. Without it the domain is used as-is and the public port is
// inferred from addr, the server's own listen address — right only when
// clients reach the server directly.
func ResolvePreviewBase(domain, baseURL, addr string) (PreviewBase, error) {
	if baseURL != "" {
		u, err := url.Parse(baseURL)
		if err != nil {
			return PreviewBase{}, fmt.Errorf("parse preview base URL %q: %w", baseURL, err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return PreviewBase{}, fmt.Errorf("preview base URL %q: scheme must be http or https", baseURL)
		}
		if u.Hostname() == "" {
			return PreviewBase{}, fmt.Errorf("preview base URL %q: missing host", baseURL)
		}
		if domain != "" && domain != u.Hostname() {
			return PreviewBase{}, fmt.Errorf(
				"preview domain %q conflicts with preview base URL host %q; set one or the other",
				domain, u.Hostname())
		}
		return PreviewBase{Domain: u.Hostname(), Scheme: u.Scheme, Port: u.Port()}, nil
	}
	if domain == "" {
		domain = DefaultPreviewDomain
	}
	base := PreviewBase{Domain: domain, Scheme: "http"}
	if _, port, err := net.SplitHostPort(addr); err == nil {
		base.Port = port
	}
	return base, nil
}

// Config holds the resolved runtime configuration.
type Config struct {
	DataDir string
	Preview PreviewBase
}

// Options carries `preview serve`'s flag values into Load. Each has an
// environment-variable fallback; an empty field means "not set by a flag".
type Options struct {
	DataDir string
	// PreviewDomain is the base domain previews are served under.
	PreviewDomain string
	// PreviewBaseURL is the public base URL of previews, overriding the
	// domain, scheme, and port derived from PreviewDomain and Addr.
	PreviewBaseURL string
	// Addr is the server's listen address, the fallback source of the
	// public port.
	Addr string
}

// Load resolves the data directory (flag override > $PREVIEW_DATA_DIR >
// $XDG_DATA_HOME/preview > ~/.local/share/preview) and ensures it exists,
// plus the public preview address space (flag overrides > $PREVIEW_DOMAIN /
// $PREVIEW_BASE_URL > derived from the listen address).
func Load(opts Options) (Config, error) {
	dir := opts.DataDir
	if dir == "" {
		dir = os.Getenv("PREVIEW_DATA_DIR")
	}
	if dir == "" {
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			dir = filepath.Join(xdg, appName)
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return Config{}, fmt.Errorf("resolve home dir: %w", err)
			}
			dir = filepath.Join(home, ".local", "share", appName)
		}
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Config{}, fmt.Errorf("resolve data dir: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return Config{}, fmt.Errorf("create data dir: %w", err)
	}
	domain := opts.PreviewDomain
	if domain == "" {
		domain = os.Getenv("PREVIEW_DOMAIN")
	}
	baseURL := opts.PreviewBaseURL
	if baseURL == "" {
		baseURL = os.Getenv("PREVIEW_BASE_URL")
	}
	base, err := ResolvePreviewBase(domain, baseURL, opts.Addr)
	if err != nil {
		return Config{}, err
	}
	return Config{DataDir: abs, Preview: base}, nil
}

// Dir returns the user-level config directory: $PREVIEW_CONFIG_DIR, else
// the platform config dir plus the app name ($XDG_CONFIG_HOME/preview or
// ~/.config/preview on Linux). Returns "" when none is resolvable (e.g. no
// $HOME), which disables every lookup rooted here.
func Dir() string {
	if root := os.Getenv("PREVIEW_CONFIG_DIR"); root != "" {
		return root
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, appName)
}

// ManifestsDir returns the directory searched for local (out-of-repo)
// preview manifests, `<config>/manifests/<repo>.toml`. Returns "" when no
// config dir is resolvable; the lookup is then disabled.
func ManifestsDir() string {
	root := Dir()
	if root == "" {
		return ""
	}
	return filepath.Join(root, "manifests")
}

// DBPath returns the SQLite database path inside the data directory.
func (c Config) DBPath() string {
	return filepath.Join(c.DataDir, appName+".db")
}

// ReposDir holds one bare (mirror) clone per registered repo.
func (c Config) ReposDir() string { return filepath.Join(c.DataDir, "repos") }

// TmpDir is scratch space for builds and state-dir forks. Contents are never
// authoritative; anything stale here is swept at startup.
func (c Config) TmpDir() string { return filepath.Join(c.DataDir, "tmp") }

// ArtifactsDir holds published content-addressed artifacts
// (artifacts/<repo>/{fe,be}/<hash>/).
func (c Config) ArtifactsDir() string { return filepath.Join(c.DataDir, "artifacts") }

// StateDir holds mutable backend state directories (state/<repo>/<be_hash>/).
func (c Config) StateDir() string { return filepath.Join(c.DataDir, "state") }

// LogsDir holds build logs (logs/<repo>/{fe,be}/<hash>.log) and process run
// logs (logs/<repo>/run/<be_hash>/<n>.log).
func (c Config) LogsDir() string { return filepath.Join(c.DataDir, "logs") }
