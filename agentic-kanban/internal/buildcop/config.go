package buildcop

import (
	"fmt"
	"log"
	"time"

	"github.com/jmelahman/kanban/internal/kanbantoml"
	"github.com/jmelahman/kanban/internal/slug"
)

// DefaultInterval is the poll cadence applied when [buildcop].interval is
// unset. Two minutes balances detection latency against GitHub rate budget;
// see docs/guide/observability.md for the math on the jobs-endpoint fanout.
const DefaultInterval = 2 * time.Minute

// Config is the resolved [buildcop] section: a list of board configurations
// to poll. Enabled is the master switch; even when true, an empty Boards
// slice means the poller is a no-op.
type Config struct {
	Enabled  bool
	Interval time.Duration
	Boards   []BoardConfig
}

// BoardConfig is one Build Cop board's full set of knobs after defaults
// have been applied. RepoPath is the only field with no useful default.
type BoardConfig struct {
	Name                string
	Slug                string
	RepoPath            string
	Branch              string  // "" or "*" means all branches
	FailureThreshold    float64 // 0..1, default 0.10
	MinRuns             int     // default 5
	GreenStreakRequired int     // default 10
	WindowDays          int     // default 7
	FlakyThreshold      int     // default 3
}

// MatchesAllBranches reports whether this board accepts every head_branch.
func (b BoardConfig) MatchesAllBranches() bool {
	return b.Branch == "" || b.Branch == "*"
}

// ResolveConfig reads .kanban.toml (project + user, merged) and returns a
// Config with defaults applied per board. Disabled when the [buildcop]
// section is absent or `enabled = false`.
func ResolveConfig(repoPath string) Config {
	cfg := Config{Enabled: false, Interval: DefaultInterval}
	f := kanbantoml.Load(repoPath)
	if f.BuildCop == nil {
		return cfg
	}
	if f.BuildCop.Enabled != nil {
		cfg.Enabled = *f.BuildCop.Enabled
	}
	if f.BuildCop.Interval != nil && *f.BuildCop.Interval != "" {
		d, err := time.ParseDuration(*f.BuildCop.Interval)
		switch {
		case err != nil:
			log.Printf("buildcop: ignoring invalid interval %q: %v (using default %s)", *f.BuildCop.Interval, err, DefaultInterval)
		case d < time.Second:
			// Guard against a user typo like "5" (parsed as 5ns) or "100ms"
			// that would hammer the API. Sub-second polling makes no sense
			// for workflow runs which complete on the order of minutes.
			log.Printf("buildcop: interval %s is below 1s minimum; using default %s", d, DefaultInterval)
		default:
			cfg.Interval = d
		}
	}
	for _, b := range f.BuildCop.Boards {
		cfg.Boards = append(cfg.Boards, resolveBoard(b))
	}
	return cfg
}

func resolveBoard(in kanbantoml.BuildCopBoard) BoardConfig {
	out := BoardConfig{
		FailureThreshold:    0.10,
		MinRuns:             5,
		GreenStreakRequired: 10,
		WindowDays:          7,
		FlakyThreshold:      3,
	}
	if in.RepoPath != nil {
		out.RepoPath = *in.RepoPath
	}
	if in.Branch != nil {
		out.Branch = *in.Branch
	}
	if in.Name != nil && *in.Name != "" {
		out.Name = *in.Name
	} else {
		out.Name = defaultName(out.Branch)
	}
	out.Slug = slug.Make(out.Name, "build-cop")
	if in.FailureThreshold != nil {
		out.FailureThreshold = *in.FailureThreshold
	}
	if in.MinRuns != nil && *in.MinRuns > 0 {
		out.MinRuns = *in.MinRuns
	}
	if in.GreenStreakRequired != nil && *in.GreenStreakRequired > 0 {
		out.GreenStreakRequired = *in.GreenStreakRequired
	}
	if in.WindowDays != nil && *in.WindowDays > 0 {
		out.WindowDays = *in.WindowDays
	}
	if in.FlakyThreshold != nil && *in.FlakyThreshold > 0 {
		out.FlakyThreshold = *in.FlakyThreshold
	}
	return out
}

func defaultName(branch string) string {
	if branch == "" || branch == "*" {
		return "Build Cop: all branches"
	}
	return fmt.Sprintf("Build Cop: %s", branch)
}
