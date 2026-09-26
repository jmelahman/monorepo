// Package ghrun follows the GitHub Actions runs a tag push triggers, through
// the gh CLI, which brings its own authentication.
//
// Runs are found rather than named: a tag push's runs have event "push", the
// tag as their head branch and the tagged commit as their head SHA, whatever
// their workflows are called.
package ghrun

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// Run is a workflow run, as gh run list --json describes it.
type Run struct {
	ID         int64  `json:"databaseId"`
	Workflow   string `json:"workflowName"`
	HeadBranch string `json:"headBranch"`
	HeadSHA    string `json:"headSha"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	URL        string `json:"url"`
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

// State is the run's conclusion once it's done, and its status until then.
func (r Run) State() string {
	if r.Done() {
		return r.Conclusion
	}
	return r.Status
}

// Lister lists the push runs of repo (OWNER/REPO) at sha.
type Lister func(ctx context.Context, repo, sha string) ([]Run, error)

// GH is the Lister that runs gh run list.
func GH(ctx context.Context, repo, sha string) ([]Run, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "gh", "run", "list", "--repo", repo, "--event", "push", "--commit", sha,
		"--limit", "100", "--json", "databaseId,workflowName,headBranch,headSha,status,conclusion,url")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("gh run list --repo %s: %w: %s", repo, err, strings.TrimSpace(stderr.String()))
	}
	var runs []Run
	if err := json.Unmarshal(stdout.Bytes(), &runs); err != nil {
		return nil, fmt.Errorf("gh run list --repo %s: %w", repo, err)
	}
	return runs, nil
}

// CheckGH fails unless gh is installed and signed in to github.com.
func CheckGH() error {
	if _, err := exec.LookPath("gh"); err != nil {
		return errors.New("--watch needs the GitHub CLI, gh (https://cli.github.com)")
	}
	if err := exec.Command("gh", "auth", "status", "--hostname", "github.com").Run(); err != nil {
		return errors.New("--watch needs gh signed in to github.com; run gh auth login")
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

var githubRepo = regexp.MustCompile(`^(?:git@github\.com:|ssh://git@github\.com/|https://github\.com/)([^/]+)/([^/]+?)(?:\.git)?/?$`)

// ParseGitHubRepo returns the owner and name of a github.com remote URL.
func ParseGitHubRepo(url string) (owner, repo string, ok bool) {
	m := githubRepo.FindStringSubmatch(url)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}
