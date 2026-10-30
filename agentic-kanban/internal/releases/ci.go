package releases

import (
	"context"
	"log"
	"os/exec"
	"time"

	"github.com/jmelahman/git-orchard/orchard"
)

const (
	// ciInterval is the least time between checks of a repo's CI. Checks only
	// start when something reads the summary, so an idle server spends none of
	// the GitHub API quota it shares with everything else using the token.
	ciInterval = 5 * time.Minute
	// ciTimeout is under ciInterval, so checks of a repo can't overlap.
	ciTimeout = 4 * time.Minute
	ciJobs    = 8
)

// CIHealth is the state of a subtree's latest run of each workflow.
type CIHealth struct {
	// Status is "passing", "failing", "running" or "none", which also covers
	// runs that ended without passing or failing, such as cancelled ones.
	Status string `json:"status"`
	// Failed names the failing workflows, and URL is the first one's run.
	Failed []string `json:"failed,omitempty"`
	URL    string   `json:"url,omitempty"`
	Error  string   `json:"error,omitempty"`
}

// Health sums up a result. A workflow that won't run again doesn't count, and
// a failure shows even if some of the result couldn't be read.
func Health(r orchard.CIResult) CIHealth {
	h := CIHealth{Status: "none", Error: redact(r.Error)}
	passing, running := 0, false
	for _, run := range r.Runs {
		switch {
		case run.Stale != "":
		case run.Failing():
			h.Failed = append(h.Failed, run.Workflow)
			if h.URL == "" {
				h.URL = run.URL
			}
		case !run.Done():
			running = true
		case run.OK():
			passing++
		}
	}
	switch {
	case len(h.Failed) > 0:
		h.Status = "failing"
	case running:
		h.Status = "running"
	case passing > 0:
		h.Status = "passing"
	}
	return h
}

// startCI checks repo's CI in the background if it's due.
func (s *Service) startCI(repo string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.ciStarted[repo]) < ciInterval || s.ctx.Err() != nil {
		return
	}
	s.ciStarted[repo] = time.Now()
	s.wg.Go(func() {
		health, err := s.checkCI(repo)
		if err != nil {
			log.Printf("releases: check CI of %s: %v", repo, redact(err.Error()))
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		s.ci[repo] = health
	})
}

func (s *Service) checkCI(repo string) (map[string]CIHealth, error) {
	ctx, cancel := context.WithTimeout(s.ctx, ciTimeout)
	defer cancel()
	o, err := open(ctx, repo, nil)
	if err != nil {
		return nil, err
	}
	prefixes := make([]string, len(o.Config.Subtrees))
	for i, sub := range o.Config.Subtrees {
		prefixes[i] = sub.Prefix
	}
	targets, err := o.CITargets(ctx, prefixes)
	if err != nil {
		return nil, err
	}
	if _, err := exec.LookPath("gh"); err != nil {
		for i := range targets {
			if targets[i].Repo != "" {
				targets[i].Error = "gh isn't installed on the server"
			}
		}
	}
	health := make(map[string]CIHealth, len(targets))
	for _, r := range orchard.CheckCI(ctx, targets, orchard.CICheckOptions{Jobs: ciJobs}) {
		health[r.Prefix] = Health(r)
	}
	return health, nil
}
