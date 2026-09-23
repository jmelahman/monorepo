package config

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// clientConfigName is the CLI's config file, a sibling of the manifests
// directory under the user-level config dir.
const clientConfigName = "config.toml"

// ClientConfig is the user-level configuration of the CLI subcommands, read
// from `<config>/config.toml`. It configures which server they talk to; the
// server's own settings are flags and environment variables on
// `preview serve`, not this file.
type ClientConfig struct {
	// Server is the base URL of the `preview serve` instance subcommands
	// talk to, e.g. "https://preview.example.com". Empty means unset, which
	// falls back to localhost.
	Server string `toml:"server"`
	// Token is the bearer token subcommands present when the server requires
	// authentication — a GitHub personal-access token checked against the
	// server's allowlist. Empty means send none. $PREVIEW_TOKEN overrides it.
	Token string `toml:"token"`
}

// ClientConfigPath returns the CLI config file's path, or "" when no config
// dir is resolvable.
func ClientConfigPath() string {
	root := Dir()
	if root == "" {
		return ""
	}
	return filepath.Join(root, clientConfigName)
}

// LoadClientConfig reads the CLI config file. A missing file — or no
// resolvable config dir at all — yields the zero ClientConfig and no error;
// the CLI is fully usable without one. Malformed contents, unknown keys, and
// an unusable server URL are errors rather than silent fallbacks: a config
// pointing at the wrong server is worse than no config, because commands
// still succeed against the default one.
func LoadClientConfig() (ClientConfig, error) {
	path := ClientConfigPath()
	if path == "" {
		return ClientConfig{}, nil
	}
	var cfg ClientConfig
	meta, err := toml.DecodeFile(path, &cfg)
	if errors.Is(err, fs.ErrNotExist) {
		return ClientConfig{}, nil
	}
	if err != nil {
		return ClientConfig{}, fmt.Errorf("read %s: %w", path, err)
	}
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for i, k := range undecoded {
			keys[i] = k.String()
		}
		return ClientConfig{}, fmt.Errorf("read %s: unknown key(s): %s", path, strings.Join(keys, ", "))
	}
	if cfg.Server != "" {
		if err := ValidateServerURL(cfg.Server); err != nil {
			return ClientConfig{}, fmt.Errorf("read %s: %w", path, err)
		}
	}
	return cfg, nil
}

// SaveClientConfig writes the CLI config file, creating the config
// directory. It returns the path written.
func SaveClientConfig(cfg ClientConfig) (string, error) {
	path := ClientConfigPath()
	if path == "" {
		return "", errors.New("no config directory available; set $PREVIEW_CONFIG_DIR")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	var b strings.Builder
	b.WriteString("# local-preview CLI configuration (preview configure)\n")
	if err := toml.NewEncoder(&b).Encode(cfg); err != nil {
		return "", fmt.Errorf("encode config: %w", err)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return path, nil
}

// ValidateServerURL rejects the server URLs that would fail confusingly
// later — a bare host with no scheme being the common one, since the HTTP
// client would read it as a relative path.
func ValidateServerURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid server URL %q: %w", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("invalid server URL %q: needs an http:// or https:// scheme", raw)
	}
	if u.Host == "" {
		return fmt.Errorf("invalid server URL %q: missing host", raw)
	}
	return nil
}
