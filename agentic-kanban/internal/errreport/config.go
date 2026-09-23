package errreport

import "github.com/jmelahman/kanban/internal/kanbantoml"

// ResolveConfig reads .kanban.toml from repoPath (and the user-level config)
// and returns a Config. Defaults: Enabled=false, BoardName="Errors". The
// reporter is fully opt-in — only `[errors] enabled = true` activates it.
func ResolveConfig(repoPath string) Config {
	cfg := Config{Enabled: false, BoardName: "Errors"}
	f := kanbantoml.Load(repoPath)
	if f.Errors == nil {
		return cfg
	}
	if f.Errors.Enabled != nil {
		cfg.Enabled = *f.Errors.Enabled
	}
	if f.Errors.BoardName != nil {
		if name := *f.Errors.BoardName; name != "" {
			cfg.BoardName = name
		}
	}
	return cfg
}
