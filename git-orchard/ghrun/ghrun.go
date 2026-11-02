// Package ghrun follows GitHub Actions runs through GitHub's REST API: the runs
// a tag push triggers, and the latest run of each workflow on a branch.
//
// Runs are found rather than named: a tag push's runs have event "push", the
// tag as their head branch and the tagged commit as their head SHA, whatever
// their workflows are called.
package ghrun

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Run is a workflow run. Its JSON is part of orchard's output.
type Run struct {
	ID         int64     `json:"databaseId"`
	WorkflowID int64     `json:"workflowDatabaseId"`
	Workflow   string    `json:"workflowName"`
	HeadBranch string    `json:"headBranch"`
	HeadSHA    string    `json:"headSha"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion"`
	URL        string    `json:"url"`
	Event      string    `json:"event"`
	Title      string    `json:"displayTitle"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Done reports whether the run has finished.
func (r Run) Done() bool {
	return r.Status == "completed"
}

// OK reports whether a finished run didn't fail.
func (r Run) OK() bool {
	switch r.Conclusion {
	case "success", "skipped", "neutral":
		return true
	}
	return false
}

// Failed reports whether a finished run failed. Runs cancelled (usually by
// a newer run), waiting for approval (action_required) or gone stale didn't.
func (r Run) Failed() bool {
	return r.Done() && failedConclusion(r.Conclusion)
}

func failedConclusion(conclusion string) bool {
	switch conclusion {
	case "failure", "timed_out", "startup_failure":
		return true
	}
	return false
}

// State is the run's conclusion once it's done, and its status until then.
func (r Run) State() string {
	if r.Done() {
		return r.Conclusion
	}
	return r.Status
}

// Lister lists the push runs of repo (OWNER/REPO) at sha.
type Lister func(ctx context.Context, repo, sha string) ([]Run, error)

// GH is the Lister that asks GitHub.
func GH(ctx context.Context, repo, sha string) ([]Run, error) {
	return listRuns(ctx, repo, 0, url.Values{"event": {"push"}, "head_sha": {sha}}, 100)
}

// DefaultBranch returns repo's default branch.
func DefaultBranch(ctx context.Context, repo string) (string, error) {
	var r struct {
		DefaultBranch string `json:"default_branch"`
	}
	if _, err := getJSON(ctx, "/repos/"+repo, &r); err != nil {
		return "", err
	}
	if r.DefaultBranch == "" {
		return "", fmt.Errorf("GitHub API GET /repos/%s: no default branch", repo)
	}
	return r.DefaultBranch, nil
}

// Printable replaces s's control characters, which text from GitHub, like a
// commit's title, could use to rewrite a terminal: tabs and line breaks with
// a space, and others with U+FFFD.
func Printable(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t' || r == '\n' || r == '\r':
			return ' '
		case r < 0x20 || (r >= 0x7f && r < 0xa0):
			return '\ufffd'
		}
		return r
	}, s)
}

// CheckGH fails unless there's a token for github.com; what names what needs
// it, e.g. "--watch".
func CheckGH(what string) error {
	if Token() == "" {
		return errors.New(what + " needs a GitHub token: set GH_TOKEN or GITHUB_TOKEN, or run gh auth login")
	}
	return nil
}

// WatchOptions configure Watch.
type WatchOptions struct {
	// Interval is the time between polls; it defaults to 5s.
	Interval time.Duration
	// Grace is how long to wait for a first run before concluding that the
	// tag triggers none; it defaults to 60s.
	Grace time.Duration
	// Sleep waits between polls; it defaults to a timer that ctx cancels.
	Sleep func(ctx context.Context, d time.Duration) error
	// Now defaults to time.Now.
	Now func() time.Time
}

func (o *WatchOptions) defaults() {
	if o.Interval == 0 {
		o.Interval = 5 * time.Second
	}
	if o.Grace == 0 {
		o.Grace = 60 * time.Second
	}
	if o.Sleep == nil {
		o.Sleep = Sleep
	}
	if o.Now == nil {
		o.Now = time.Now
	}
}

// Sleep waits for d, or until ctx is done.
func Sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// FailedError is the runs of a watch that failed.
type FailedError struct {
	Runs []Run
}

func (e *FailedError) Error() string {
	var b strings.Builder
	for i, r := range e.Runs {
		if i > 0 {
			b.WriteString("; ")
		}
		fmt.Fprintf(&b, "%s %s: %s", r.Workflow, r.Conclusion, r.URL)
	}
	return b.String()
}

// Watch polls the runs the push of tag at sha triggered in repo until they
// all finish, calling report with each run whenever its state changes. It
// returns the runs, none if none started within opts.Grace, and a
// *FailedError if any failed. ctx bounds the watch.
func Watch(ctx context.Context, list Lister, repo, tag, sha string, opts WatchOptions, report func(Run)) ([]Run, error) {
	opts.defaults()
	start := opts.Now()
	states := map[int64]string{}
	for {
		all, err := list(ctx, repo, sha)
		if err != nil {
			return nil, err
		}
		var runs, failed []Run
		done := true
		for _, r := range all {
			if r.HeadBranch != tag || r.HeadSHA != sha {
				continue
			}
			runs = append(runs, r)
			if states[r.ID] != r.State() {
				states[r.ID] = r.State()
				if report != nil {
					report(r)
				}
			}
			switch {
			case !r.Done():
				done = false
			case !r.OK():
				failed = append(failed, r)
			}
		}
		if len(runs) == 0 && opts.Now().Sub(start) >= opts.Grace {
			return nil, nil
		}
		if len(runs) > 0 && done {
			if len(failed) > 0 {
				return runs, &FailedError{Runs: failed}
			}
			return runs, nil
		}
		if err := opts.Sleep(ctx, opts.Interval); err != nil {
			return runs, err
		}
	}
}

var githubRepo = regexp.MustCompile(`^(?:git@github\.com:|ssh://git@github\.com/|https://(?:[^/@]+@)?github\.com/)([^/]+)/([^/]+?)(?:\.git)?/?$`)

// ParseGitHubRepo returns the owner and name of a github.com remote URL, which
// may carry credentials, as one rewritten by url.<base>.insteadOf does.
func ParseGitHubRepo(url string) (owner, repo string, ok bool) {
	m := githubRepo.FindStringSubmatch(url)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}
