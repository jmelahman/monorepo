package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmelahman/kanban/internal/kanbantoml"
)

type Config struct {
	DataDir        string
	worktreesDir   string
	PortRangeStart int
	PortRangeEnd   int
}

func Load(dataDirOverride, worktreesDirOverride string, portStart, portEnd int) (*Config, error) {
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
	dataDir, err := expandHome(dataDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir data dir: %w", err)
	}

	worktreesDir := worktreesDirOverride
	if worktreesDir == "" {
		worktreesDir = os.Getenv("KANBAN_WORKTREES_DIR")
	}
	if worktreesDir != "" {
		worktreesDir, err = expandHome(worktreesDir)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(worktreesDir, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir worktrees dir: %w", err)
		}
	}

	return &Config{
		DataDir:        dataDir,
		worktreesDir:   worktreesDir,
		PortRangeStart: portStart,
		PortRangeEnd:   portEnd,
	}, nil
}

func (c *Config) DBPath() string { return filepath.Join(c.DataDir, "kanban.db") }

// WorktreesDir is the parent directory used for new boards' default
// worktree_root. Precedence: --worktrees-dir flag, $KANBAN_WORKTREES_DIR,
// the [worktrees].root key in the user-level kanban config, then
// <data-dir>/worktrees. The TOML value is read on each call so changes made
// via the Settings UI take effect without a restart.
func (c *Config) WorktreesDir() string {
	if c.worktreesDir != "" {
		return c.worktreesDir
	}
	if root := userConfigWorktreesRoot(); root != "" {
		if expanded, err := expandHome(root); err == nil {
			return expanded
		}
	}
	return filepath.Join(c.DataDir, "worktrees")
}

// HasWorktreesDirOverride reports whether the worktrees dir is locked at
// startup by --worktrees-dir or $KANBAN_WORKTREES_DIR. The Settings UI uses
// this to disable editing when a startup override is in effect.
func (c *Config) HasWorktreesDirOverride() bool { return c.worktreesDir != "" }

func userConfigWorktreesRoot() string {
	f := kanbantoml.Load("")
	if f.Worktrees == nil || f.Worktrees.Root == nil {
		return ""
	}
	return strings.TrimSpace(*f.Worktrees.Root)
}

// PlansDir returns the configured plans directory verbatim — possibly a
// relative path like "./plans". Callers that need an absolute path must
// resolve it themselves (the per-session handler joins it against the
// session worktree when relative). The default is `~/.claude/plans`,
// expanded to the user's home. TOML is re-read on each call so a config
// edit takes effect without a restart.
func (c *Config) PlansDir() string {
	dir := userConfigPlansDir()
	if dir == "" {
		dir = "~/.claude/plans"
	}
	if expanded, err := expandHome(dir); err == nil {
		return expanded
	}
	return dir
}

func userConfigPlansDir() string {
	f := kanbantoml.Load("")
	if f.Plans == nil || f.Plans.Dir == nil {
		return ""
	}
	return strings.TrimSpace(*f.Plans.Dir)
}

func expandHome(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

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
