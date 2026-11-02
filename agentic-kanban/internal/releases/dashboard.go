package releases

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
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
	Unreleased int  `json:"unreleased"`
	Counted    bool `json:"counted"`
	// LatestAt is when Latest was tagged, 0 if it has no tag here.
	LatestAt int64 `json:"latest_at,omitempty"`
	// Suggested is the bump the unreleased commits' titles call for, by
	// SuggestBump, empty unless there are commits since a release.
	Suggested string `json:"suggested,omitempty"`
	Error     string `json:"error,omitempty"`
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
	flagReleasedReminders(sum.Reminders, sum.Projects)
	return sum, nil
}

// flagReleasedReminders marks the reminders whose subtree has had a release
// since they were last edited. A release from here closes its reminder, so these
// come from elsewhere, like git orchard release, and may have covered the note.
func flagReleasedReminders(reminders []db.ReleaseReminder, projects []Project) {
	for i := range reminders {
		r := &reminders[i]
		for _, p := range projects {
			if p.RepoPath != r.RepoPath {
				continue
			}
			for _, sub := range p.Subtrees {
				if sub.Prefix == r.Prefix && sub.LatestAt > r.UpdatedAt {
					r.ReleasedSince = sub.Latest
				}
			}
		}
	}
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
		// What was saved at the last stop may be stale, or for a manifest
		// that has since changed.
		if err := s.Refresh(s.ctx); err != nil {
			log.Printf("releases: refresh: %v", err)
		}
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
		delete(s.refreshing, repo)
		if ok {
			s.projects[repo] = p
		}
		s.mu.Unlock()
		if ok {
			s.saveProject(p)
		}
	})
}

// saveProject keeps p for the next start. A refresh the server cut short isn't
// worth keeping over the last complete one.
func (s *Service) saveProject(p Project) {
	if s.ctx.Err() != nil {
		return
	}
	state, err := json.Marshal(p)
	if err == nil {
		err = s.store.SaveReleaseProject(s.ctx, p.RepoPath, state)
	}
	if err != nil {
		log.Printf("releases: save state of %s: %v", p.RepoPath, err)
	}
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
		status, err := s.status(o, repo, sub, p.Rev)
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
		if u.Latest == "" || !u.Counted {
			return
		}
		// Best effort: the counts stand without the suggestion and the release
		// date, so failing to read them shouldn't hide the subtree.
		if err := readSinceRelease(o, sub.Prefix, p.Rev, st); err != nil {
			log.Printf("releases: read %s since %s: %v", sub.Prefix, u.Latest, redact(err.Error()))
		}
	})
	return p, true
}

// readSinceRelease fills in what's read from the tag of st's latest release and
// the commits to rev since it. Merges are left out: their titles are git's, and
// the commits they brought in are listed anyway.
func readSinceRelease(o *orchard.Orchard, prefix, rev string, st *SubtreeState) error {
	tag := "refs/tags/" + prefix + "/" + st.Latest
	date, err := o.Repo.Output("for-each-ref", "--format=%(creatordate:unix)", tag)
	if err != nil {
		return err
	}
	if st.LatestAt, err = strconv.ParseInt(strings.TrimSpace(date), 10, 64); err != nil {
		return fmt.Errorf("date of %s: %w", tag, err)
	}
	if st.Unreleased == 0 {
		return nil
	}
	out, err := o.Repo.Output("log", "--no-merges", "--format=%s", tag+".."+rev, "--", prefix)
	if err != nil {
		return err
	}
	var subjects []string
	for line := range strings.Lines(out) {
		subjects = append(subjects, strings.TrimSpace(line))
	}
	st.Suggested = SuggestBump(subjects)
	return nil
}
