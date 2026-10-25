// Package ghrun follows GitHub Actions runs through the gh CLI, which brings
// its own authentication: the runs a tag push triggers, and the latest run of
// each workflow on a branch.
//
// Runs are found rather than named: a tag push's runs have event "push", the
// tag as their head branch and the tagged commit as their head SHA, whatever
// their workflows are called.
package ghrun

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// Run is a workflow run, as gh run list --json describes it.
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

// runFields are the fields of Run, for gh run list --json.
const runFields = "databaseId,workflowDatabaseId,workflowName,headBranch,headSha,status,conclusion,url,event,displayTitle,createdAt"

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
	if !r.Done() {
		return false
	}
	switch r.Conclusion {
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

// GH is the Lister that runs gh run list.
func GH(ctx context.Context, repo, sha string) ([]Run, error) {
	return listRuns(ctx, repo, "--event", "push", "--commit", sha, "--limit", "100")
}

func listRuns(ctx context.Context, repo string, args ...string) ([]Run, error) {
	args = append([]string{"run", "list", "--repo", repo}, args...)
	stdout, err := gh(ctx, append(args, "--json", runFields)...)
	if err != nil {
		return nil, err
	}
	var runs []Run
	if err := json.Unmarshal(stdout, &runs); err != nil {
		return nil, fmt.Errorf("gh run list --repo %s: %w", repo, err)
	}
	return runs, nil
}

// transient matches gh's errors that trying again might get past: dropped
// connections and the API's server errors.
var transient = regexp.MustCompile(`(?i)\bEOF\b|connection reset|timed? ?out|TLS handshake|HTTP 5\d\d`)

// gh runs the gh CLI and returns its output, or an error carrying its stderr.
// The commands only read, so a transient failure is tried again once.
func gh(ctx context.Context, args ...string) ([]byte, error) {
	var err error
	for attempt := range 2 {
		if attempt > 0 {
			if err := Sleep(ctx, time.Second); err != nil {
				return nil, err
			}
		}
		var stdout, stderr bytes.Buffer
		cmd := exec.CommandContext(ctx, "gh", args...)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err = cmd.Run(); err == nil {
			return stdout.Bytes(), nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		msg := strings.TrimSpace(stderr.String())
		err = fmt.Errorf("gh %s %s: %w: %s", args[0], args[1], err, msg)
		if !transient.MatchString(msg) {
			break
		}
	}
	return nil, err
}

// DefaultBranch returns repo's default branch.
func DefaultBranch(ctx context.Context, repo string) (string, error) {
	out, err := gh(ctx, "repo", "view", repo, "--json", "defaultBranchRef", "--jq", ".defaultBranchRef.name")
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" {
		return "", fmt.Errorf("gh repo view %s: no default branch", repo)
	}
	return branch, nil
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

// CheckGH fails unless gh is installed and signed in to github.com; what
// names what needs it, e.g. "--watch".
func CheckGH(what string) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("%s needs the GitHub CLI, gh (https://cli.github.com)", what)
	}
	if err := exec.Command("gh", "auth", "status", "--hostname", "github.com").Run(); err != nil {
		return fmt.Errorf("%s needs gh signed in to github.com; run gh auth login", what)
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
