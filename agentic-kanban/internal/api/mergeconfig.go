package api

import (
	"github.com/jmelahman/kanban/internal/kanbantoml"
)

// MergeConfig captures the per-board merge strategies allowed by the merged
// kanban config (user config layered over the project's .kanban.toml).
// Defaults are all-true so existing repos behave unchanged.
type MergeConfig struct {
	AllowMergeCommit bool `json:"allow_merge_commit"`
	AllowSquash      bool `json:"allow_squash"`
	AllowRebase      bool `json:"allow_rebase"`
	// DefaultStrategy is the strategy a merge request with none of its own
	// falls back to, and the one the UI preselects. Empty means unset.
	DefaultStrategy string `json:"default_strategy"`
}

func defaultMergeConfig() MergeConfig {
	return MergeConfig{AllowMergeCommit: true, AllowSquash: true, AllowRebase: true}
}

// SyncConfig captures the per-board sync strategies allowed by the merged
// kanban config. Defaults are all-true so existing repos behave unchanged.
type SyncConfig struct {
	AllowRebase bool `json:"allow_rebase"`
	AllowMerge  bool `json:"allow_merge"`
}

func defaultSyncConfig() SyncConfig {
	return SyncConfig{AllowRebase: true, AllowMerge: true}
}

func loadMergeConfig(repoPath string) MergeConfig {
	cfg := defaultMergeConfig()
	m := kanbantoml.Load(repoPath).Merge
	if m == nil {
		return cfg
	}
	if m.AllowMergeCommit != nil {
		cfg.AllowMergeCommit = *m.AllowMergeCommit
	}
	if m.AllowSquash != nil {
		cfg.AllowSquash = *m.AllowSquash
	}
	if m.AllowRebase != nil {
		cfg.AllowRebase = *m.AllowRebase
	}
	if m.DefaultStrategy != nil {
		cfg.DefaultStrategy = *m.DefaultStrategy
	}
	return cfg
}

func loadSyncConfig(repoPath string) SyncConfig {
	cfg := defaultSyncConfig()
	s := kanbantoml.Load(repoPath).Sync
	if s == nil {
		return cfg
	}
	if s.AllowRebase != nil {
		cfg.AllowRebase = *s.AllowRebase
	}
	if s.AllowMerge != nil {
		cfg.AllowMerge = *s.AllowMerge
	}
	return cfg
}

func (mc MergeConfig) allows(strategy string) bool {
	switch strategy {
	case "merge-commit":
		return mc.AllowMergeCommit
	case "squash":
		return mc.AllowSquash
	case "rebase":
		return mc.AllowRebase
	}
	return false
}

func (sc SyncConfig) allows(strategy string) bool {
	switch strategy {
	case "rebase":
		return sc.AllowRebase
	case "merge":
		return sc.AllowMerge
	}
	return false
}
