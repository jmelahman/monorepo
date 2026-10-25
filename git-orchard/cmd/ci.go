package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/jmelahman/git-orchard/config"
	"github.com/jmelahman/git-orchard/ghrun"
	"github.com/jmelahman/git-orchard/orchard"
)

// ErrCIFailed is returned when ci found failed runs (it has already said
// which).
var ErrCIFailed = errors.New("CI failed")

// ErrInterrupted is returned when a signal stopped a command before it
// finished.
var ErrInterrupted = errors.New("interrupted")

// monorepoPrefix stands for the monorepo itself among the subtrees.
const monorepoPrefix = "."

// CIOptions holds options for the ci command
type CIOptions struct {
	Failed   bool
	Logs     bool
	Lines    int
	JSON     bool
	Watch    bool
	Interval time.Duration
	Color    string
	Jobs     int
	// PruneStale deletes the failed runs of workflows found stale.
	PruneStale bool
	Yes        bool
}

// NewCICommand creates a new ci command
func NewCICommand() *cobra.Command {
	opts := &CIOptions{}

	cmd := &cobra.Command{
		Use:   "ci [prefix...]",
		Short: "Show the GitHub Actions status of the monorepo and subtrees",
		Long: `Show the GitHub Actions status of the monorepo and subtrees.

For each subtree on GitHub, the latest run of each workflow on its upstream
branch; and, unless prefixes are given, the monorepo's own, on origin's
default branch, as the prefix "." (which can also be given). Runs come from
the GitHub CLI, gh, signed in.

--logs adds, for each failed run, the output of the step that failed in each
failed job, as Markdown, for reading or piping elsewhere:

  git orchard ci --failed --logs | claude -p "triage these CI failures"

Failed runs of a workflow that won't run again are stale, and don't count as
failures: so far, Dependabot Updates in a repository with no
.github/dependabot.yml and security updates off, as in a subtree whose
dependencies the monorepo updates. --prune-stale deletes them, after asking
unless --yes.

ci exits 1 if any run failed, even if it couldn't check some repository,
and 2 if it couldn't check some repository and no run failed.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCI(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), opts, args)
		},
	}

	cmd.Flags().BoolVar(&opts.Failed, "failed", false, "show only failed runs")
	cmd.Flags().BoolVar(&opts.Logs, "logs", false, "add the failing steps' logs of failed runs")
	cmd.Flags().IntVar(&opts.Lines, "lines", 150, "with --logs, the most lines to keep of each failed job, the last ones (0 for all)")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "print JSON")
	cmd.Flags().BoolVarP(&opts.Watch, "watch", "w", false, "refresh until interrupted")
	cmd.Flags().DurationVar(&opts.Interval, "interval", time.Minute, "with --watch, the time between refreshes")
	cmd.Flags().StringVar(&opts.Color, "color", "auto", "color output: auto, always or never")
	cmd.Flags().IntVarP(&opts.Jobs, "jobs", "j", 16, "repositories to check at once")
	cmd.Flags().BoolVar(&opts.PruneStale, "prune-stale", false, "delete the failed runs of stale workflows")
	cmd.Flags().BoolVarP(&opts.Yes, "yes", "y", false, "with --prune-stale, delete without asking")
	cmd.MarkFlagsMutuallyExclusive("watch", "json")
	cmd.MarkFlagsMutuallyExclusive("watch", "prune-stale")
	cmd.MarkFlagsMutuallyExclusive("json", "prune-stale")
	cmd.MarkFlagsMutuallyExclusive("watch", "logs")

	return cmd
}

// ciTarget is a repository whose runs to show.
type ciTarget struct {
	Prefix string
	// Repo is OWNER/NAME, or empty if the remote isn't on GitHub.
	Repo   string
	Branch string
	// Error is why the target can't be checked, if it can't.
	Error string
}

type ciResult struct {
	Prefix string  `json:"prefix"`
	Repo   string  `json:"repo,omitempty"`
	Branch string  `json:"branch"`
	Runs   []ciRun `json:"runs"`
	Error  string  `json:"error,omitempty"`
}

type ciRun struct {
	ghrun.Run
	Logs     []ghrun.Excerpt `json:"logs,omitempty"`
	LogError string          `json:"logError,omitempty"`
	// Stale is why the run's workflow won't run again, if it won't.
	Stale string `json:"stale,omitempty"`
}

// failing reports whether the run failed and that still matters: a stale
// workflow's failures are history.
func (r ciRun) failing() bool {
	return r.Failed() && r.Stale == ""
}

func runCI(ctx context.Context, stdout, stderr io.Writer, opts *CIOptions, prefixes []string) error {
	var color bool
	switch opts.Color {
	case "auto":
		color = isTerminal(stdout) && os.Getenv("NO_COLOR") == ""
	case "always":
		color = true
	case "never":
	default:
		return fmt.Errorf("--color is auto, always or never, not %q", opts.Color)
	}

	o, err := openOrchard()
	if err != nil {
		return err
	}
	if err := ghrun.CheckGH("ci"); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	targets, err := ciTargets(ctx, o, prefixes)
	if err != nil {
		return err
	}

	if !opts.Watch {
		results := checkCI(ctx, targets, opts, stderr)
		if ctx.Err() != nil {
			return ErrInterrupted
		}
		if opts.JSON {
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			enc.SetEscapeHTML(false)
			if err := enc.Encode(filterCI(results, opts.Failed)); err != nil {
				return err
			}
		} else {
			printCI(stdout, results, opts, color, time.Now())
			if opts.Logs {
				printCILogs(stdout, results)
			}
		}
		if opts.PruneStale {
			if err := pruneStale(ctx, stdout, results, opts.Yes); err != nil {
				return err
			}
		}
		err := ciError(results)
		if errors.Is(err, ErrCIFailed) {
			if n := unchecked(results); n > 0 {
				_, _ = fmt.Fprintf(stderr, "Couldn't check %d of %d repositories.\n", n, len(results))
			}
		}
		return err
	}

	for {
		results := checkCI(ctx, targets, opts, stderr)
		if ctx.Err() != nil {
			return nil
		}
		if isTerminal(stdout) {
			_, _ = fmt.Fprint(stdout, "\x1b[H\x1b[2J")
		}
		printCI(stdout, results, opts, color, time.Now())
		_, _ = fmt.Fprintf(stdout, "\nUpdated %s; refreshing every %s. Press Ctrl-C to stop.\n", time.Now().Format("15:04:05"), opts.Interval)
		if err := ghrun.Sleep(ctx, opts.Interval); err != nil {
			return nil
		}
	}
}

// ciTargets returns the repositories at prefixes: every subtree and the
// monorepo if there are none.
func ciTargets(ctx context.Context, o *orchard.Orchard, prefixes []string) ([]ciTarget, error) {
	monorepo := len(prefixes) == 0
	var rest []string
	for _, p := range prefixes {
		if config.Clean(p) == monorepoPrefix {
			monorepo = true
		} else {
			rest = append(rest, p)
		}
	}
	var targets []ciTarget
	if monorepo {
		t := ciTarget{Prefix: monorepoPrefix}
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
		t := ciTarget{Prefix: s.Prefix, Branch: s.Branch}
		if owner, name, ok := ghrun.ParseGitHubRepo(s.Remote); ok {
			t.Repo = owner + "/" + name
		}
		targets = append(targets, t)
	}
	return targets, nil
}

// checkCI lists the targets' latest runs, and with --logs the failed ones'
// logs, counting them off on stderr if it's a terminal.
func checkCI(ctx context.Context, targets []ciTarget, opts *CIOptions, stderr io.Writer) []ciResult {
	results := make([]ciResult, len(targets))
	p := newProgress(stderr, "Checking CI", len(targets))
	defer p.Stop()
	orchard.Each(len(targets), opts.Jobs, func(i int) {
		t := targets[i]
		p.Begin(t.Prefix)
		defer p.End(t.Prefix)
		r := ciResult{Prefix: t.Prefix, Repo: t.Repo, Branch: t.Branch, Runs: []ciRun{}, Error: t.Error}
		if t.Repo != "" && t.Error == "" {
			runs, err := ghrun.Latest(ctx, t.Repo, t.Branch)
			if err != nil {
				r.Error = err.Error()
			}
			stale := staleness(ctx, t, runs)
			for _, run := range runs {
				cr := ciRun{Run: run, Stale: stale[run.WorkflowID]}
				if opts.Logs && cr.failing() {
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
func staleness(ctx context.Context, t ciTarget, runs []ghrun.Run) map[int64]string {
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

// filterCI returns results with only their failing runs if failed is set,
// leaving out those with none and no error.
func filterCI(results []ciResult, failed bool) []ciResult {
	if !failed {
		return results
	}
	out := []ciResult{}
	for _, r := range results {
		runs := []ciRun{}
		for _, run := range r.Runs {
			if run.failing() {
				runs = append(runs, run)
			}
		}
		if len(runs) > 0 || r.Error != "" {
			r.Runs = runs
			out = append(out, r)
		}
	}
	return out
}

// ciError is ErrCIFailed if any run is failing, or else an error if some
// repository couldn't be checked: a failure is news whatever else happened.
func ciError(results []ciResult) error {
	for _, r := range results {
		for _, run := range r.Runs {
			if run.failing() {
				return ErrCIFailed
			}
		}
	}
	if n := unchecked(results); n > 0 {
		return fmt.Errorf("failed to check %d of %d repositories", n, len(results))
	}
	return nil
}

// unchecked counts the results that couldn't be checked.
func unchecked(results []ciResult) int {
	n := 0
	for _, r := range results {
		if r.Error != "" {
			n++
		}
	}
	return n
}

const (
	red    = "31"
	green  = "32"
	yellow = "33"
	cyan   = "36"
	dim    = "2"
	bold   = "1"
)

type painter bool

func (p painter) paint(code, s string) string {
	if !p || s == "" {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

// link makes s a terminal hyperlink to url.
func (p painter) link(url, s string) string {
	if !p || url == "" {
		return s
	}
	return "\x1b]8;;" + url + "\x1b\\" + s + "\x1b]8;;\x1b\\"
}

// runStyle returns a run's symbol, color and state.
func runStyle(r ciRun) (symbol, code, state string) {
	if r.Stale != "" {
		return "-", dim, "stale " + r.State()
	}
	symbol, code = runSymbol(r.Run)
	return symbol, code, r.State()
}

func runSymbol(r ghrun.Run) (symbol, code string) {
	switch {
	case !r.Done():
		return "●", yellow
	case r.Failed():
		return "✗", red
	case r.Conclusion == "success":
		return "✓", green
	case r.Conclusion == "action_required":
		return "!", yellow // awaiting approval
	}
	return "-", dim // cancelled, skipped, neutral, stale
}

// printCI prints a line per run: the prefix, its state, the workflow, how
// long ago it started, its commit and title, and with color off its URL
// (with color on, the workflow links to it).
func printCI(w io.Writer, results []ciResult, opts *CIOptions, color bool, now time.Time) {
	p := painter(color)
	shown := filterCI(results, opts.Failed)
	var rows [][]string
	widths := make([]int, 7)
	add := func(cells ...string) {
		// A row's last cell isn't padded, so needn't widen its column; that
		// leaves errors and the like to run on.
		for i, c := range cells[:len(cells)-1] {
			widths[i] = max(widths[i], visibleLen(c))
		}
		rows = append(rows, cells)
	}

	// Stale runs are counted from all the results, since --failed hides them.
	stale := 0
	for _, r := range results {
		for _, run := range r.Runs {
			if run.Stale != "" && run.Failed() {
				stale++
			}
		}
	}
	failed, running, total := 0, 0, 0
	for _, r := range shown {
		prefix := p.paint(bold, r.Prefix)
		switch {
		case r.Error != "":
			add(prefix, p.paint(red, "! error"), ghrun.Printable(r.Error))
		case r.Repo == "":
			add(prefix, p.paint(dim, "- not on GitHub"))
		case len(r.Runs) == 0:
			add(prefix, p.paint(dim, "- no runs on "+r.Branch))
		}
		for _, run := range r.Runs {
			total++
			if run.failing() {
				failed++
			} else if !run.Done() {
				running++
			}
			symbol, code, state := runStyle(run)
			sha := run.HeadSHA[:min(7, len(run.HeadSHA))]
			title := truncate(ghrun.Printable(run.Title), 50)
			url := run.URL
			if color {
				url = ""
			}
			add(prefix, p.paint(code, symbol+" "+state), p.link(run.URL, ghrun.Printable(run.Workflow)),
				p.paint(dim, age(now.Sub(run.CreatedAt))), p.paint(dim, sha), title, url)
			if color {
				prefix = "" // once per repository is enough to read
			}
		}
	}

	for _, row := range rows {
		var b strings.Builder
		for i, c := range row {
			if i > 0 {
				b.WriteString("  ")
			}
			b.WriteString(c)
			if i < len(row)-1 {
				b.WriteString(strings.Repeat(" ", widths[i]-visibleLen(c)))
			}
		}
		_, _ = fmt.Fprintln(w, strings.TrimRight(b.String(), " "))
	}

	summary := fmt.Sprintf("%d failed, %d running, %d run(s) in %d repositories", failed, running, total, len(results))
	if opts.Failed {
		summary = fmt.Sprintf("%d failed run(s) in %d repositories", failed, len(results))
	}
	if failed == 0 && len(shown) == 0 {
		summary = p.paint(green, "Nothing failed") + fmt.Sprintf(" in %d repositories", len(results))
	}
	if stale > 0 {
		summary += fmt.Sprintf("; %d stale failed run(s)", stale)
		if opts.Failed {
			summary += " hidden"
		}
		if !opts.PruneStale {
			summary += ", which --prune-stale deletes"
		}
	}
	if len(rows) > 0 {
		_, _ = fmt.Fprintln(w)
	}
	_, _ = fmt.Fprintln(w, p.paint(dim, summary))
}

// printCILogs prints each failed run's excerpts as Markdown.
func printCILogs(w io.Writer, results []ciResult) {
	for _, r := range results {
		for _, run := range r.Runs {
			if !run.failing() {
				continue
			}
			_, _ = fmt.Fprintf(w, "\n## %s: %s (%s)\n\n", r.Prefix, ghrun.Printable(run.Workflow), run.Conclusion)
			_, _ = fmt.Fprintf(w, "Repository %s, branch %s, commit %s: %s\n%s\n", r.Repo, r.Branch, run.HeadSHA, ghrun.Printable(run.Title), run.URL)
			if run.LogError != "" {
				_, _ = fmt.Fprintf(w, "\nCouldn't get the logs: %s\n", ghrun.Printable(run.LogError))
				continue
			}
			if len(run.Logs) == 0 {
				_, _ = fmt.Fprintln(w, "\nNo failed job logged anything; the run may have failed to start.")
			}
			for _, e := range run.Logs {
				_, _ = fmt.Fprintf(w, "\n### Job %s", ghrun.Printable(e.Job))
				if e.Step != "" {
					_, _ = fmt.Fprintf(w, ", step %s", e.Step)
				}
				_, _ = fmt.Fprintln(w)
				if e.Omitted > 0 {
					_, _ = fmt.Fprintf(w, "\n%d earlier lines left out; gh run view %d --repo %s --log-failed has them all.\n", e.Omitted, run.ID, r.Repo)
				}
				_, _ = fmt.Fprintf(w, "\n```text\n%s\n```\n", strings.Join(e.Lines, "\n"))
			}
		}
	}
}

// age describes a duration coarsely, e.g. "3h ago".
func age(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}

// visibleLen is the width of s on a terminal, without its escape sequences.
func visibleLen(s string) int {
	n, esc := 0, false
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == '\x1b':
			esc = true
			// An OSC hyperlink runs to its ST, ESC \, and a CSI to its final letter.
			if strings.HasPrefix(s[i:], "\x1b]") {
				end := strings.Index(s[i:], "\x1b\\")
				if end < 0 {
					return n
				}
				i += end + 2
				esc = false
				continue
			}
		case esc:
			if r >= '@' && r <= '~' && r != '[' {
				esc = false
			}
		default:
			n++
		}
		i += size
	}
	return n
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// staleWorkflow is a stale workflow of a repository, with its failed runs.
type staleWorkflow struct {
	Prefix, Repo, Workflow, Why string
	Runs                        []ghrun.Run
}

// pruneStale deletes the failed runs of results' stale workflows, after
// listing them and, unless yes, asking. It lists every failed run, not just
// the latest: the one before would take its place.
func pruneStale(ctx context.Context, w io.Writer, results []ciResult, yes bool) error {
	var workflows []staleWorkflow
	total := 0
	for _, r := range results {
		for _, run := range r.Runs {
			if run.Stale == "" || !run.Failed() {
				continue
			}
			runs, err := ghrun.FailedRuns(ctx, r.Repo, run.WorkflowID)
			if err != nil {
				return fmt.Errorf("%s: %w", r.Prefix, err)
			}
			workflows = append(workflows, staleWorkflow{r.Prefix, r.Repo, run.Workflow, run.Stale, runs})
			total += len(runs)
		}
	}
	if total == 0 {
		_, _ = fmt.Fprintln(w, "\nNo stale runs to delete.")
		return nil
	}

	_, _ = fmt.Fprintln(w, "\nStale runs:")
	for _, sw := range workflows {
		_, _ = fmt.Fprintf(w, "  %s (%s): %d failed %s run(s); %s\n", sw.Prefix, sw.Repo, len(sw.Runs), ghrun.Printable(sw.Workflow), sw.Why)
	}
	if !yes {
		ok, err := confirm(fmt.Sprintf("Delete these %d run(s)? Deleted runs can't be restored.", total), false, "pass --yes to delete without asking")
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
	}

	deleted := 0
	var errs []error
	for _, sw := range workflows {
		for _, run := range sw.Runs {
			if err := ghrun.DeleteRun(ctx, sw.Repo, run.ID); err != nil {
				errs = append(errs, fmt.Errorf("%s: run %d: %w", sw.Prefix, run.ID, err))
				continue
			}
			deleted++
		}
	}
	_, _ = fmt.Fprintf(w, "Deleted %d of %d stale run(s).\n", deleted, total)
	return errors.Join(errs...)
}
