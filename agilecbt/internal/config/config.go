// Package config resolves where the application keeps its on-disk state.
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// appName is the directory name used under the XDG data home. Keep it in
// sync with the binary name in .goreleaser.yaml and pyproject.toml.
const appName = "app"

// Config holds the resolved runtime configuration.
type Config struct {
	DataDir string
}

// Load resolves the data directory (flag override > $APP_DATA_DIR >
// $XDG_DATA_HOME/app > ~/.local/share/app) and ensures it exists.
func Load(dataDirOverride string) (Config, error) {
	dir := dataDirOverride
	if dir == "" {
		dir = os.Getenv("APP_DATA_DIR")
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
	return Config{DataDir: abs}, nil
}

// DBPath returns the SQLite database path inside the data directory.
func (c Config) DBPath() string {
	return filepath.Join(c.DataDir, appName+".db")
}
