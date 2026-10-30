package releases

import (
	"context"
	"log"
	"time"

	"github.com/jmelahman/git-orchard/orchard"

	"github.com/jmelahman/kanban/internal/db"
)

const (
	// refreshJobs bounds the subtrees checked at once: each may run git
	// subtree split, and reach two remotes.
	refreshJobs = 4
	// refreshTimeout bounds one refresh of a repo.
	refreshTimeout = 15 * time.Minute
	recentRuns     = 20
)

// SubtreeState is what a refresh found for one subtree.
type SubtreeState struct {
	Prefix string `json:"prefix"`
	Remote string `json:"remote"`
	// Ahead counts commits the upstream lacks, and Behind the reverse.
	Ahead  int `json:"ahead"`
	Behind int `json:"behind"`
	// Latest is the latest release, empty if there are none.
	Latest string `json:"latest"`
	// Unreleased counts commits since Latest. It's meaningless unless
	// Counted: an upstream release may have no tag here to count from.
	Unreleased int    `json:"unreleased"`
	Counted    bool   `json:"counted"`
	Error      string `json:"error,omitempty"`
	// CI is nil until the first check of the repo's CI finishes.
	CI *CIHealth `json:"ci,omitempty"`
}

// Project is a repo with a git-orchard manifest.
type Project struct {
	RepoPath string `json:"repo_path"`
	Base     string `json:"base"`
	// Rev is the tip of the base branch the state was computed at.
	Rev         string         `json:"rev"`
	Subtrees    []SubtreeState `json:"subtrees"`
	RefreshedAt int64          `json:"refreshed_at"`
	Refreshing  bool           `json:"refreshing"`
	Error       string         `json:"error,omitempty"`
}

// Summary is everything the Releases tab shows.
type Summary struct {
	Unavailable string               `json:"unavailable,omitempty"`
	Projects    []Project            `json:"projects"`
	Reminders   []db.ReleaseReminder `json:"reminders"`
	Runs        []db.ReleaseRun      `json:"runs"`
}

// Summary returns the cached state, starting a refresh of any repo not seen
// yet. It never waits on git beyond reading manifests.
func (s *Service) Summary(ctx context.Context) (Summary, error) {
	sum := Summary{Unavailable: Unavailable(), Projects: []Project{}}
	var err error
	if sum.Reminders, err = s.store.ListOpenReleaseReminders(ctx); err != nil {
		return sum, err
	}
	if sum.Runs, err = s.store.ListReleaseRuns(ctx, recentRuns); err != nil {
		return sum, err
	}
	if sum.Unavailable != "" {
		return sum, nil
	}
	boards, order, err := s.repos(ctx)
	if err != nil {
		return sum, err
	}
	for _, repo := range order {
		o, err := orchard.Open(repo)
		if err != nil || len(o.Config.Subtrees) == 0 {
			// Not a repo here, or not one with subtrees: not a project.
			continue
		}
		s.mu.Lock()
		p, seen := s.projects[repo]
		s.mu.Unlock()
		if !seen {
			p = Project{RepoPath: repo, Base: boards[repo].BaseBranch, Subtrees: []SubtreeState{}}
			for _, sub := range o.Config.Subtrees {
				p.Subtrees = append(p.Subtrees, SubtreeState{Prefix: sub.Prefix, Remote: sub.Remote})
			}
			s.startRefresh(repo, boards[repo].BaseBranch)
		}
		s.startCI(repo)
		// The cached subtrees are shared, so CI goes on a copy.
		subtrees := make([]SubtreeState, len(p.Subtrees))
		s.mu.Lock()
		p.Refreshing = s.refreshing[repo]
		for i, sub := range p.Subtrees {
			if health, ok := s.ci[repo][sub.Prefix]; ok {
				sub.CI = &health
			}
			subtrees[i] = sub
		}
		s.mu.Unlock()
		p.Subtrees = subtrees
		sum.Projects = append(sum.Projects, p)
	}
	return sum, nil
}

// Refresh recomputes every repo's state in the background.
func (s *Service) Refresh(ctx context.Context) error {
	boards, order, err := s.repos(ctx)
	if err != nil {
		return err
	}
	for _, repo := range order {
		s.startRefresh(repo, boards[repo].BaseBranch)
	}
	return nil
}

// Start refreshes every repo each interval until Close.
func (s *Service) Start(interval time.Duration) {
	s.wg.Go(func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				if err := s.Refresh(s.ctx); err != nil {
					log.Printf("releases: refresh: %v", err)
				}
			}
		}
	})
}

func (s *Service) startRefresh(repo, base string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refreshing[repo] || s.ctx.Err() != nil {
		return
	}
	s.refreshing[repo] = true
	s.wg.Go(func() {
		p, ok := s.compute(repo, base)
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.refreshing, repo)
		if ok {
			s.projects[repo] = p
		}
	})
}

// compute works out a repo's state; false means it has no subtrees.
func (s *Service) compute(repo, base string) (Project, bool) {
	ctx, cancel := context.WithTimeout(s.ctx, refreshTimeout)
	defer cancel()
	o, err := open(ctx, repo, nil)
	if err != nil || len(o.Config.Subtrees) == 0 {
		return Project{}, false
	}
	p := Project{RepoPath: repo, Base: base, RefreshedAt: time.Now().Unix(), Subtrees: make([]SubtreeState, len(o.Config.Subtrees))}
	for i, sub := range o.Config.Subtrees {
		p.Subtrees[i] = SubtreeState{Prefix: sub.Prefix, Remote: sub.Remote}
	}
	if p.Rev, err = tip(o, base); err != nil {
		p.Error = redact(err.Error())
		return p, true
	}
	// Best effort: a tag moved on the remote makes this fail for that tag
	// alone, and the counts only need the rest.
	if _, err := o.Repo.Output("fetch", "--quiet", "--tags", remote); err != nil {
		log.Printf("releases: fetch tags in %s: %v", repo, redact(err.Error()))
	}
	orchard.Each(len(o.Config.Subtrees), refreshJobs, func(i int) {
		sub := o.Config.Subtrees[i]
		st := &p.Subtrees[i]
		status, err := o.Status(sub, p.Rev)
		if err != nil {
			st.Error = redact(err.Error())
			return
		}
		st.Ahead, st.Behind = status.Ahead, status.Behind
		u, err := o.Unreleased(sub, p.Rev, remote)
		if err != nil {
			st.Error = redact(err.Error())
			return
		}
		st.Latest, st.Unreleased, st.Counted = u.Latest, u.Commits, u.Counted
	})
	return p, true
}
