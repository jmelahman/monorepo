package orchard

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmelahman/git-orchard/config"
	"github.com/jmelahman/git-orchard/ghrun"
)

// MonorepoPrefix stands for the monorepo itself among the subtrees.
const MonorepoPrefix = "."

// CITarget is a repository whose runs to show.
type CITarget struct {
	Prefix string
	// Repo is OWNER/NAME, or empty if the remote isn't on GitHub.
	Repo   string
	Branch string
	// Error is why the target can't be checked, if it can't.
	Error string
}

// CIResult is the state of a CITarget's latest runs.
type CIResult struct {
	Prefix string  `json:"prefix"`
	Repo   string  `json:"repo,omitempty"`
	Branch string  `json:"branch"`
	Runs   []CIRun `json:"runs"`
	Error  string  `json:"error,omitempty"`
}

// CIRun is the latest run of a workflow.
type CIRun struct {
	ghrun.Run
	Logs     []ghrun.Excerpt `json:"logs,omitempty"`
	LogError string          `json:"logError,omitempty"`
	// Stale is why the run's workflow won't run again, if it won't.
	Stale string `json:"stale,omitempty"`
}

// Failing reports whether the run failed and that still matters: a stale
// workflow's failures are history.
func (r CIRun) Failing() bool {
	return r.Failed() && r.Stale == ""
}

// CICheckOptions configure CheckCI.
type CICheckOptions struct {
	// Jobs is how many repositories to check at once.
	Jobs int
	// Logs adds the failing steps' logs of failed runs, keeping the last Lines
	// lines of each failed job (0 for all).
	Logs  bool
	Lines int
	// OnStart and OnDone, if set, are called with each target's prefix, from
	// the goroutine checking it.
	OnStart, OnDone func(prefix string)
}

// CITargets returns the repositories at prefixes: every subtree and the
// monorepo if there are none.
func (o *Orchard) CITargets(ctx context.Context, prefixes []string) ([]CITarget, error) {
	monorepo := len(prefixes) == 0
	var rest []string
	for _, p := range prefixes {
		if config.Clean(p) == MonorepoPrefix {
			monorepo = true
		} else {
			rest = append(rest, p)
		}
	}
	var targets []CITarget
	if monorepo {
		t := CITarget{Prefix: MonorepoPrefix}
		if owner, name, ok := ghrun.ParseGitHubRepo(o.RemoteURL("origin")); ok {
			t.Repo = owner + "/" + name
		}
		// origin/HEAD is only set by a clone, or git remote set-head.
		if head, err := o.Repo.Output("symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
			t.Branch = strings.TrimPrefix(head, "origin/")
		} else if t.Repo != "" {
			if t.Branch, err = ghrun.DefaultBranch(ctx, t.Repo); err != nil {
				t.Error = fmt.Sprintf("can't tell origin's default branch: %v", err)
			}
		}
		targets = append(targets, t)
	}
	if monorepo && len(rest) == 0 && len(prefixes) > 0 {
		return targets, nil
	}
	subtrees, err := o.Config.Select(rest)
	if err != nil {
		return nil, err
	}
	for _, s := range subtrees {
		t := CITarget{Prefix: s.Prefix, Branch: s.Branch}
		if owner, name, ok := ghrun.ParseGitHubRepo(s.Remote); ok {
			t.Repo = owner + "/" + name
		}
		targets = append(targets, t)
	}
	return targets, nil
}

// CheckCI lists the targets' latest runs through gh. A target not on GitHub
// has no runs, and needs no gh.
func CheckCI(ctx context.Context, targets []CITarget, opts CICheckOptions) []CIResult {
	results := make([]CIResult, len(targets))
	Each(len(targets), opts.Jobs, func(i int) {
		t := targets[i]
		if opts.OnStart != nil {
			opts.OnStart(t.Prefix)
		}
		if opts.OnDone != nil {
			defer opts.OnDone(t.Prefix)
		}
		r := CIResult{Prefix: t.Prefix, Repo: t.Repo, Branch: t.Branch, Runs: []CIRun{}, Error: t.Error}
		if t.Repo != "" && t.Error == "" {
			runs, err := ghrun.Latest(ctx, t.Repo, t.Branch)
			if err != nil {
				r.Error = err.Error()
			}
			stale := staleness(ctx, t, runs)
			for _, run := range runs {
				cr := CIRun{Run: run, Stale: stale[run.WorkflowID]}
				if opts.Logs && cr.Failing() {
					log, err := ghrun.FailedLog(ctx, t.Repo, run.ID)
					if err != nil {
						cr.LogError = err.Error()
					} else {
						cr.Logs = ghrun.Excerpts(log, opts.Lines)
					}
				}
				r.Runs = append(r.Runs, cr)
			}
		}
		results[i] = r
	})
	return results
}

// staleness returns why each workflow of runs that failed won't run again,
// by ID, for those found stale. A repository it can't tell about is taken to
// be live: a failure shown in error beats one hidden.
func staleness(ctx context.Context, t CITarget, runs []ghrun.Run) map[int64]string {
	stale := map[int64]string{}
	for _, run := range runs {
		if run.Workflow != ghrun.DependabotWorkflow || !run.Failed() {
			continue
		}
		if why, err := ghrun.DependabotStale(ctx, t.Repo, t.Branch); err == nil && why != "" {
			stale[run.WorkflowID] = why
		}
	}
	return stale
}
