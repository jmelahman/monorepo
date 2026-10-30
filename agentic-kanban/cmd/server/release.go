package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jmelahman/kanban/internal/client"
)

// releaseBumpFlags are the mutually exclusive --patch, --minor and --major.
type releaseBumpFlags struct{ patch, minor, major bool }

func (f *releaseBumpFlags) add(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&f.patch, "patch", false, "Bump the patch version")
	cmd.Flags().BoolVar(&f.minor, "minor", false, "Bump the minor version")
	cmd.Flags().BoolVar(&f.major, "major", false, "Bump the major version")
	cmd.MarkFlagsMutuallyExclusive("patch", "minor", "major")
}

// value returns the chosen bump, or "" if there's none.
func (f *releaseBumpFlags) value() string {
	switch {
	case f.patch:
		return "patch"
	case f.minor:
		return "minor"
	case f.major:
		return "major"
	}
	return ""
}

func releaseCmd() *cobra.Command {
	var serverURL, repo string
	parent := &cobra.Command{
		Use:   "release",
		Short: "Keep release reminders and release git-orchard subtrees",
		Long: `Keep notes on what to release next, and release git-orchard subtrees.

With no subcommand in a terminal, opens a view of your open reminders. Reminders
are only notes: they watch nothing and never release anything by themselves.

Subcommands use the git repo containing the current directory unless --repo is
given.`,
		Example: `  kanban release remind tools/foo --minor "has the new flag"
  kanban release list
  kanban release run tools/foo
  kanban release log 12
  kanban release dismiss tools/foo`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !stdinIsTerminal() {
				return cmd.Help()
			}
			return promptReleaseView(cmd.Context(), resolveURL(cmd, serverURL))
		},
	}
	addServerFlag(parent, &serverURL)
	parent.PersistentFlags().StringVar(&repo, "repo", "", "Repo path of a board (default: the git repo containing the current directory)")

	var remindBump releaseBumpFlags
	var remindSuffix string
	remind := &cobra.Command{
		Use:   "remind <prefix> [note...]",
		Short: "Note that a subtree is meant to be released",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.New(resolveURL(cmd, serverURL), nil)
			_, repoPath, err := loadReleaseRepo(cmd.Context(), c, repo)
			if err != nil {
				return err
			}
			r, err := c.CreateReleaseReminder(cmd.Context(), client.CreateReleaseReminderArgs{
				RepoPath: repoPath,
				Prefix:   cleanPrefix(args[0]),
				Bump:     remindBump.value(),
				Suffix:   remindSuffix,
				Note:     strings.Join(args[1:], " "),
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "reminder #%d: %s %s\n", r.ID, r.Bump, r.Prefix)
			return nil
		},
	}
	remindBump.add(remind)
	remind.Flags().StringVar(&remindSuffix, "suffix", "", "Pre-release suffix, such as rc")

	var listJSON bool
	list := &cobra.Command{
		Use:   "list",
		Short: "Print open reminders, subtree state and recent runs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c := client.New(resolveURL(cmd, serverURL), nil)
			if listJSON {
				return printReleasesJSON(cmd.Context(), c, cmd.OutOrStdout())
			}
			summary, err := c.ReleaseSummary(cmd.Context())
			if err != nil {
				return err
			}
			if repo != "" {
				summary = filterReleases(summary, canonicalReleaseRepo(summary, repo))
			}
			return printReleases(cmd.OutOrStdout(), summary)
		},
	}
	list.Flags().BoolVar(&listJSON, "json", false, "Print the full API response")

	var runBump releaseBumpFlags
	var runSuffix string
	var runYes bool
	run := &cobra.Command{
		Use:   "run <prefix>",
		Short: "Release a subtree and follow its log",
		Long: `Release a subtree and follow its log, exiting non-zero if it fails.

Uses the bump and suffix from the subtree's open reminder unless flags say
otherwise, and closes that reminder on success. The release runs on the server,
so it carries on if this command is interrupted.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			suffix := (*string)(nil)
			if cmd.Flags().Changed("suffix") {
				suffix = &runSuffix
			}
			return runReleaseRun(cmd.Context(), client.New(resolveURL(cmd, serverURL), nil), cmd.InOrStdin(), cmd.OutOrStdout(), releaseRunOpts{
				repo: repo, prefix: cleanPrefix(args[0]), bump: runBump.value(), suffix: suffix, yes: runYes,
			})
		},
	}
	runBump.add(run)
	run.Flags().StringVar(&runSuffix, "suffix", "", "Pre-release suffix, such as rc")
	run.Flags().BoolVarP(&runYes, "yes", "y", false, "Don't ask for confirmation")

	logCmd := &cobra.Command{
		Use:   "log <run>",
		Short: "Follow a run's log from the start until it finishes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid run id %q", args[0])
			}
			out := cmd.OutOrStdout()
			return client.New(resolveURL(cmd, serverURL), nil).StreamReleaseRunOutput(cmd.Context(), id, func(line string) {
				fmt.Fprintln(out, line)
			})
		},
	}

	dismiss := &cobra.Command{
		Use:   "dismiss <prefix>",
		Short: "Dismiss a subtree's open reminder",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.New(resolveURL(cmd, serverURL), nil)
			summary, repoPath, err := loadReleaseRepo(cmd.Context(), c, repo)
			if err != nil {
				return err
			}
			prefix := cleanPrefix(args[0])
			r := findReleaseReminder(summary, repoPath, prefix)
			if r == nil {
				return fmt.Errorf("%s has no open reminder in %s", prefix, repoPath)
			}
			if err := c.DismissReleaseReminder(cmd.Context(), r.ID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "dismissed reminder #%d\n", r.ID)
			return nil
		},
	}

	parent.AddCommand(remind, list, run, logCmd, dismiss)
	// A failed release or API call isn't a usage error; don't bury any of them
	// under the flag table.
	for _, c := range parent.Commands() {
		c.SilenceUsage = true
	}
	return parent
}

// cleanPrefix drops the trailing slash that shell completion leaves on a
// directory.
func cleanPrefix(prefix string) string {
	return strings.TrimRight(prefix, "/")
}

// loadReleaseRepo returns the summary and the repo a command works on, spelled
// as the server spells it.
func loadReleaseRepo(ctx context.Context, c *client.Client, repo string) (client.ReleaseSummary, string, error) {
	if repo == "" {
		root, err := cwdRepoRoot()
		if err != nil {
			return client.ReleaseSummary{}, "", fmt.Errorf("cannot infer the repo: %w (run inside a board's git repo or pass --repo)", err)
		}
		repo = root
	}
	summary, err := c.ReleaseSummary(ctx)
	if err != nil {
		return summary, "", err
	}
	return summary, canonicalReleaseRepo(summary, repo), nil
}

// canonicalReleaseRepo finds repo among the projects, which may spell it
// differently, such as with a trailing slash.
func canonicalReleaseRepo(summary client.ReleaseSummary, repo string) string {
	for _, p := range summary.Projects {
		if samePath(p.RepoPath, repo) {
			return p.RepoPath
		}
	}
	return repo
}

func findReleaseReminder(summary client.ReleaseSummary, repo, prefix string) *client.ReleaseReminder {
	for i := range summary.Reminders {
		if r := &summary.Reminders[i]; r.RepoPath == repo && r.Prefix == prefix {
			return r
		}
	}
	return nil
}

func findReleaseSubtree(summary client.ReleaseSummary, repo, prefix string) *client.ReleaseSubtree {
	for _, p := range summary.Projects {
		if p.RepoPath != repo {
			continue
		}
		for i := range p.Subtrees {
			if s := &p.Subtrees[i]; s.Prefix == prefix {
				return s
			}
		}
	}
	return nil
}

func filterReleases(summary client.ReleaseSummary, repo string) client.ReleaseSummary {
	out := client.ReleaseSummary{Unavailable: summary.Unavailable}
	for _, p := range summary.Projects {
		if p.RepoPath == repo {
			out.Projects = append(out.Projects, p)
		}
	}
	for _, r := range summary.Reminders {
		if r.RepoPath == repo {
			out.Reminders = append(out.Reminders, r)
		}
	}
	for _, r := range summary.Runs {
		if r.RepoPath == repo {
			out.Runs = append(out.Runs, r)
		}
	}
	return out
}

func printReleasesJSON(ctx context.Context, c *client.Client, out io.Writer) error {
	raw, err := c.Releases(ctx)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, buf.String())
	return err
}

func printReleases(out io.Writer, summary client.ReleaseSummary) error {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if summary.Unavailable != "" {
		fmt.Fprintln(w, summary.Unavailable)
	}
	if len(summary.Reminders) > 0 {
		fmt.Fprintln(w, "REMINDER\tBUMP\tUNRELEASED\tNOTE")
		for _, r := range summary.Reminders {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Prefix, reminderBump(r), unreleasedText(findReleaseSubtree(summary, r.RepoPath, r.Prefix)), r.Note)
		}
		fmt.Fprintln(w)
	}
	for _, p := range summary.Projects {
		header := p.RepoPath
		if p.Refreshing {
			header += " (refreshing)"
		}
		fmt.Fprintln(w, header)
		if p.Error != "" {
			fmt.Fprintln(w, p.Error)
		}
		if len(p.Subtrees) > 0 {
			fmt.Fprintln(w, "SUBTREE\tAHEAD\tBEHIND\tLATEST\tUNRELEASED\tCI")
		}
		for i := range p.Subtrees {
			s := &p.Subtrees[i]
			if s.Error != "" {
				fmt.Fprintf(w, "%s\t%s\n", s.Prefix, s.Error)
				continue
			}
			fmt.Fprintf(w, "%s\t%d\t%d\t%s\t%s\t%s\n", s.Prefix, s.Ahead, s.Behind, s.Latest, unreleasedText(s), ciText(s.CI))
		}
		fmt.Fprintln(w)
	}
	if len(summary.Runs) > 0 {
		fmt.Fprintln(w, "RUN\tSTATUS\tSUBTREE\tVERSION\tERROR")
		for _, r := range summary.Runs {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", r.ID, r.Status, r.Prefix, r.Version, r.Error)
		}
	}
	return w.Flush()
}

func reminderBump(r client.ReleaseReminder) string {
	if r.Suffix != "" {
		return r.Bump + " (" + r.Suffix + ")"
	}
	return r.Bump
}

func unreleasedText(s *client.ReleaseSubtree) string {
	if s == nil || !s.Counted {
		return "?"
	}
	return strconv.Itoa(s.Unreleased)
}

func ciText(ci *client.ReleaseCI) string {
	switch {
	case ci == nil:
		return "?"
	case ci.Status == "failing":
		return "failing: " + strings.Join(ci.Failed, ", ")
	case ci.Error != "":
		return "error: " + ci.Error
	}
	return ci.Status
}

type releaseRunOpts struct {
	repo, prefix, bump string
	suffix             *string
	yes                bool
}

func runReleaseRun(ctx context.Context, c *client.Client, in io.Reader, out io.Writer, opts releaseRunOpts) error {
	if !opts.yes && !stdinIsTerminal() {
		return errors.New("confirming a release needs a terminal; pass --yes")
	}
	summary, repo, err := loadReleaseRepo(ctx, c, opts.repo)
	if err != nil {
		return err
	}
	args := client.RunReleaseArgs{Repo: repo, Prefix: opts.prefix, Bump: "patch"}
	if r := findReleaseReminder(summary, repo, opts.prefix); r != nil {
		args.Bump, args.Suffix, args.ReminderID = r.Bump, r.Suffix, &r.ID
	}
	if opts.bump != "" {
		args.Bump = opts.bump
	}
	if opts.suffix != nil {
		args.Suffix = *opts.suffix
	}

	preview, err := c.PreviewRelease(ctx, repo, opts.prefix, args.Bump, args.Suffix)
	if err != nil {
		return err
	}
	if preview.Blocked != "" {
		return errors.New(preview.Blocked)
	}
	args.Version, args.Rev = preview.Version, preview.Rev
	fmt.Fprintf(out, "%s %s at %s\n", opts.prefix, preview.Version, shortReleaseRev(preview.Rev))
	if !opts.yes {
		fmt.Fprint(out, "Release? [y/N] ")
		line, err := bufio.NewReader(in).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if answer := strings.ToLower(strings.TrimSpace(line)); answer != "y" && answer != "yes" {
			return errors.New("not released")
		}
	}

	run, err := c.RunRelease(ctx, args)
	if err != nil {
		return err
	}
	if err := c.StreamReleaseRunOutput(ctx, run.ID, func(line string) { fmt.Fprintln(out, line) }); err != nil {
		return fmt.Errorf("lost the log of run %d: %w; if the server is up, the run carries on there, and kanban release log %d follows it", run.ID, err, run.ID)
	}
	summary, err = c.ReleaseSummary(ctx)
	if err != nil {
		return err
	}
	for _, r := range summary.Runs {
		if r.ID != run.ID {
			continue
		}
		if r.Status != "released" {
			return fmt.Errorf("run %d %s: %s", r.ID, r.Status, r.Error)
		}
		fmt.Fprintf(out, "released %s %s\n", r.Prefix, r.Version)
	}
	return nil
}

func shortReleaseRev(rev string) string {
	return rev[:min(len(rev), 12)]
}
