package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	DataDir        string
	PortRangeStart int
	PortRangeEnd   int
}

func Load(dataDirOverride string, portStart, portEnd int) (*Config, error) {
	if portStart <= 0 || portEnd <= 0 || portStart > portEnd {
		return nil, fmt.Errorf("invalid port range %d-%d", portStart, portEnd)
	}

	dataDir := dataDirOverride
	if dataDir == "" {
		dataDir = os.Getenv("KANBAN_DATA_DIR")
	}
	if dataDir == "" {
		var err error
		dataDir, err = defaultDataDir()
		if err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir data dir: %w", err)
	}

	return &Config{
		DataDir:        dataDir,
		PortRangeStart: portStart,
		PortRangeEnd:   portEnd,
	}, nil
}

func (c *Config) DBPath() string { return filepath.Join(c.DataDir, "kanban.db") }

func (c *Config) WorktreesDir() string { return filepath.Join(c.DataDir, "worktrees") }

func defaultDataDir() (string, error) {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "kanban"), nil
}

func MakeFileAll(filePath string) (err error) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	return nil
}
