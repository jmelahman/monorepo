package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/jmelahman/local-preview/internal/client"
)

func deployCmd() *cobra.Command {
	var serverURL string
	var repoFlag string
	var rebuild, noWait, asJSON bool

	parent := &cobra.Command{
		Use:   "deploy [ref]",
		Short: "Deploy a commit as a preview",
		Long: "Deploy a commit (default: HEAD of the current repo) and print its\n" +
			"preview URL. The repo is auto-detected from the working directory\n" +
			"unless --repo is given.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := ""
			if len(args) == 1 {
				ref = args[0]
			}
			url, err := resolveURL(cmd, serverURL)
			if err != nil {
				return err
			}
			return runDeploy(cmd.Context(), url, cmd.OutOrStdout(),
				repoFlag, ref, rebuild, noWait, asJSON)
		},
	}
	addServerFlag(parent, &serverURL)
	parent.Flags().StringVar(&repoFlag, "repo", "", "Registered repo name (default: auto-detect from cwd)")
	parent.Flags().BoolVar(&rebuild, "rebuild", false, "Rebuild artifacts even if cached (needs a restart to affect a live backend)")
	parent.Flags().BoolVar(&noWait, "no-wait", false, "Return immediately instead of waiting for the build")
	parent.Flags().BoolVar(&asJSON, "json", false, "Print the deploy as JSON")

	var listFilter client.DeployFilter
	list := &cobra.Command{
		Use:   "list [query]",
		Short: "List deploys, optionally narrowed by a free-text search",
		Long: "List deploys. The optional query is a free-text search matching a\n" +
			"commit sha prefix or a substring of the repo, branch, ref, or author\n" +
			"(case-insensitive); flags narrow by one field exactly.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				listFilter.Query = args[0]
			}
			url, err := resolveURL(cmd, serverURL)
			if err != nil {
				return err
			}
			return runDeployList(cmd.Context(), url, cmd.OutOrStdout(), listFilter)
		},
	}
	list.Flags().StringVar(&listFilter.Repo, "repo", "", "Only deploys of this repo")
	list.Flags().StringVar(&listFilter.Branch, "branch", "", "Only deploys of this branch")
	list.Flags().StringVar(&listFilter.Author, "author", "", "Only deploys whose commit author name or email contains this (case-insensitive)")
	list.Flags().StringVar(&listFilter.Status, "status", "", "Only deploys with this status (queued, building, ready, crashed, failed, evicted)")

	show := &cobra.Command{
		Use:   "show <id>",
		Short: "Show one deploy",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseInt64(args[0], "deploy id")
			if err != nil {
				return err
			}
			url, err := resolveURL(cmd, serverURL)
			if err != nil {
				return err
			}
			c, err := newClient(url)
			if err != nil {
				return err
			}
			d, err := c.GetDeploy(cmd.Context(), id)
			if err != nil {
				return err
			}
			return printDeployJSON(cmd.OutOrStdout(), d)
		},
	}

	var runLog, follow bool
	var side string
	logs := &cobra.Command{
		Use:   "logs <id>",
		Short: "Print a deploy's build logs, or its process run log with --run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseInt64(args[0], "deploy id")
			if err != nil {
				return err
			}
			url, err := resolveURL(cmd, serverURL)
			if err != nil {
				return err
			}
			c, err := newClient(url)
			if err != nil {
				return err
			}
			if !runLog {
				if follow || cmd.Flags().Changed("side") {
					return fmt.Errorf("--follow and --side apply to the run log; add --run")
				}
				text, err := c.GetDeployLogs(cmd.Context(), id)
				if err != nil {
					return err
				}
				fmt.Fprint(cmd.OutOrStdout(), text)
				return nil
			}
			return printRunLog(cmd.Context(), c, cmd.OutOrStdout(), id, side, follow)
		},
	}
	logs.Flags().BoolVar(&runLog, "run", false, "Print the process run log (the preview server's stdout+stderr) instead of build logs")
	logs.Flags().StringVar(&side, "side", "be", "Which process: be (backend) or fe (process-mode frontend)")
	logs.Flags().BoolVarP(&follow, "follow", "f", false, "Keep polling for new run-log output until interrupted")

	stats := &cobra.Command{
		Use:   "stats <id>",
		Short: "Show live CPU/memory of a deploy's processes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseInt64(args[0], "deploy id")
			if err != nil {
				return err
			}
			url, err := resolveURL(cmd, serverURL)
			if err != nil {
				return err
			}
			return runDeployStats(cmd.Context(), url, cmd.OutOrStdout(), id)
		},
	}

	stop := &cobra.Command{
		Use:   "stop <id>",
		Short: "Stop a deploy's running processes",
		Long: "Stop the supervised processes backing a deploy. They cold-start again\n" +
			"on the next request. Processes are shared per artifact hash, so any\n" +
			"sibling deploy on the same hash stops too.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseInt64(args[0], "deploy id")
			if err != nil {
				return err
			}
			url, err := resolveURL(cmd, serverURL)
			if err != nil {
				return err
			}
			c, err := newClient(url)
			if err != nil {
				return err
			}
			d, err := c.StopDeploy(cmd.Context(), id)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "stopped deploy %d (%s@%s)\n", d.ID, d.Repo, d.ShortSHA)
			return nil
		},
	}

	del := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a deploy and reclaim its artifacts",
		Long: "Remove a deploy: stops its processes and garbage-collects any build\n" +
			"artifacts, backend state, and process history no surviving deploy\n" +
			"still references. Its short-sha subdomain is freed.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseInt64(args[0], "deploy id")
			if err != nil {
				return err
			}
			url, err := resolveURL(cmd, serverURL)
			if err != nil {
				return err
			}
			c, err := newClient(url)
			if err != nil {
				return err
			}
			if err := c.DeleteDeploy(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted deploy %d\n", id)
			return nil
		},
	}

	parent.AddCommand(list, show, logs, stats, stop, del)
	return parent
}

// printRunLog prints the current run-log tail and, with follow, keeps
// polling for appended bytes — the CLI's `docker logs -f`.
func printRunLog(ctx context.Context, c *client.Client, out io.Writer, id int64, side string, follow bool) error {
	chunk, err := c.GetDeployRunLog(ctx, id, side, 0, 0)
	if err != nil {
		return err
	}
	if chunk.Truncated {
		fmt.Fprintln(out, "… (earlier output omitted)")
	}
	fmt.Fprint(out, chunk.Content)
	for follow {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Second):
		}
		next, err := c.GetDeployRunLog(ctx, id, side, chunk.Attempt, chunk.Offset)
		if err != nil {
			return err
		}
		if chunk.Attempt != 0 && next.Attempt != chunk.Attempt {
			fmt.Fprintf(out, "--- process restarted (attempt %d) ---\n", next.Attempt)
		}
		fmt.Fprint(out, next.Content)
		chunk = next
	}
	return nil
}

// runDeployStats samples twice a second apart — a CPU percentage needs a
// delta — and prints one docker-stats-like table.
func runDeployStats(ctx context.Context, url string, out io.Writer, id int64) error {
	c, err := newClient(url)
	if err != nil {
		return err
	}
	if _, err := c.GetDeployStats(ctx, id); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Second):
	}
	s, err := c.GetDeployStats(ctx, id)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SIDE\tSTATE\tRUNTIME\tCPU\tMEM\tSTARTED")
	for _, row := range []struct {
		side  string
		stats *client.SideStats
	}{{"be", s.Backend}, {"fe", s.Frontend}} {
		if row.stats == nil {
			continue
		}
		cpu, mem := "-", "-"
		if row.stats.CPUPercent != nil {
			cpu = fmt.Sprintf("%.1f%%", *row.stats.CPUPercent)
		}
		if row.stats.MemoryBytes != nil {
			mem = formatBytes(*row.stats.MemoryBytes)
			if row.stats.MemoryLimitBytes > 0 {
				mem += " / " + formatBytes(row.stats.MemoryLimitBytes)
			}
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", row.side, row.stats.State,
			orDash(row.stats.Runtime), cpu, mem, orDash(row.stats.StartedAt))
	}
	return tw.Flush()
}

// formatBytes renders a byte count in binary units, docker-stats style.
func formatBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func runDeploy(ctx context.Context, url string, out io.Writer, repoName, ref string, rebuild, noWait, asJSON bool) error {
	c, err := newClient(url)
	if err != nil {
		return err
	}

	if ref == "" {
		sha, err := localHeadSHA(".")
		if err != nil {
			return fmt.Errorf("no ref given and no commit found in the current directory: %w", err)
		}
		ref = sha
	}
	if repoName == "" {
		var err error
		repoName, err = detectRepo(ctx, c)
		if err != nil {
			return err
		}
	}

	d, err := c.CreateDeploy(ctx, repoName, ref, rebuild)
	if err != nil {
		return err
	}
	if noWait {
		if asJSON {
			return printDeployJSON(out, d)
		}
		fmt.Fprintf(out, "deploy %d (%s@%s) %s\n", d.ID, d.Repo, d.ShortSHA, d.Status)
		return nil
	}

	for d.Status == "queued" || d.Status == "building" {
		time.Sleep(500 * time.Millisecond)
		d, err = c.GetDeploy(ctx, d.ID)
		if err != nil {
			return err
		}
	}
	if asJSON {
		return printDeployJSON(out, d)
	}
	switch d.Status {
	case "ready":
		fmt.Fprintf(out, "ready: %s\n", d.PreviewURL)
		for _, a := range d.Artifacts {
			// Artifacts build after the deploy turns ready; report the ones
			// that aren't downloadable yet instead of waiting on them.
			switch a.Status {
			case "building":
				fmt.Fprintf(out, "%s: still building (preview deploy show %d)\n", a.Name, d.ID)
			case "failed":
				fmt.Fprintf(out, "%s: build failed: %s\n", a.Name, a.Error)
			default:
				for _, f := range a.Files {
					fmt.Fprintf(out, "%s: %s%s\n", a.Name, strings.TrimRight(url, "/"), f.URL)
				}
			}
		}
		if rebuild {
			fmt.Fprintf(out, "note: --rebuild replaced the artifacts on disk; a running backend keeps its old binary until restarted\n")
		}
		return nil
	default:
		fmt.Fprintf(out, "deploy %d %s: %s\n", d.ID, d.Status, d.Error)
		fmt.Fprintf(out, "full logs: preview deploy logs %d\n", d.ID)
		return fmt.Errorf("deploy did not become ready")
	}
}

// detectRepo matches the cwd's git toplevel or origin URL against the
// server's registered repos.
func detectRepo(ctx context.Context, c *client.Client) (string, error) {
	repos, err := c.ListRepos(ctx)
	if err != nil {
		return "", err
	}
	top, _ := findWorktreeRoot(".")
	if name, ok := matchRepo(repos, top, localOriginURL(".")); ok {
		return name, nil
	}
	return "", fmt.Errorf("could not match the current directory to a registered repo; pass --repo (registered: %s)", repoNames(repos))
}

// matchRepo resolves a local checkout (its worktree root and origin URL) to a
// registered repo, most-specific rule first:
//
//  1. the registered source is this exact local path;
//  2. the origin URL names the same remote as the registered source — by
//     normalized identity, so git@github.com:org/x, https://github.com/org/x,
//     and either with a .git suffix all match;
//  3. the worktree root's directory name equals a registered repo's name —
//     the ~/code/onyx convention, a heuristic, so it is deliberately last.
func matchRepo(repos []client.Repo, top, origin string) (string, bool) {
	if top != "" {
		for _, r := range repos {
			if absSource, err := filepath.Abs(r.Source); err == nil && absSource == top {
				return r.Name, true
			}
		}
	}
	if id := normalizeGitURL(origin); id != "" {
		for _, r := range repos {
			if normalizeGitURL(r.Source) == id {
				return r.Name, true
			}
		}
	}
	if top != "" {
		base := strings.ToLower(filepath.Base(top))
		for _, r := range repos {
			if r.Name == base {
				return r.Name, true
			}
		}
	}
	return "", false
}

// normalizeGitURL reduces a git remote URL to a comparable "host/path"
// identity: scheme (https, ssh, git) and the scp-like git@host:path form all
// collapse to the same string, a trailing .git or / is dropped, and the
// result is lowercased (GitHub treats owner/repo case-insensitively). A local
// path (no host) returns "" — paths are matched by the caller's absolute-path
// rule, not here.
func normalizeGitURL(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if scheme, rest, ok := strings.Cut(s, "://"); ok {
		_ = scheme // https, ssh, git — all collapse to host/path
		s = rest
	} else {
		// scp-like: [user@]host:path — but a string with no colon (or a
		// slash before it) is a local path, matched elsewhere.
		host, path, ok := strings.Cut(s, ":")
		if !ok || strings.Contains(host, "/") {
			return ""
		}
		s = host + "/" + strings.TrimPrefix(path, "/")
	}
	// Drop any user@ ahead of the host (ssh://git@github.com/…, git@…).
	if slash := strings.IndexByte(s, '/'); slash > 0 {
		if at := strings.LastIndex(s[:slash], "@"); at >= 0 {
			s = s[at+1:]
		}
	}
	s = strings.TrimSuffix(strings.TrimSuffix(s, "/"), ".git")
	return strings.ToLower(s)
}

func repoNames(repos []client.Repo) string {
	if len(repos) == 0 {
		return "none"
	}
	names := make([]string, len(repos))
	for i, r := range repos {
		names[i] = r.Name
	}
	return strings.Join(names, ", ")
}

func runDeployList(ctx context.Context, url string, out io.Writer, f client.DeployFilter) error {
	c, err := newClient(url)
	if err != nil {
		return err
	}
	deploys, err := c.ListDeploys(ctx, f)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tREPO\tSHA\tBRANCH\tAUTHOR\tBY\tSTATUS\tURL")
	for _, d := range deploys {
		url := d.PreviewURL
		if url == "" {
			url = "-"
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			d.ID, d.Repo, d.ShortSHA, orDash(d.Branch), orDash(d.AuthorName),
			orDash(d.CreatedBy), displayState(d), url)
	}
	return tw.Flush()
}

// displayState is the state a human should see: the build status, or for
// ready deploys with supervised processes the merged runtime state —
// crashed if any side died, starting while any side warms up, running only
// once every side is warm, idle otherwise (a cold start awaits the first
// request). A dead side outranks a live one: half a deploy can't serve the
// preview, and reporting it as "ready" hides the only state a user has to
// act on.
func displayState(d client.Deploy) string {
	if d.Status != "ready" {
		return d.Status
	}
	sides := []string{d.Process, d.FeProcess}
	if slices.Contains(sides, "crashed") {
		return "crashed"
	}
	if slices.Contains(sides, "starting") {
		return "starting"
	}
	warm := true
	seen := false
	for _, p := range sides {
		if p == "idle" || p == "running" {
			seen = true
			warm = warm && p == "running"
		}
	}
	if !seen {
		return d.Status
	}
	if warm {
		return "running"
	}
	return "idle"
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func printDeployJSON(out io.Writer, d client.Deploy) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(d)
}

// parseInt64 wraps strconv.ParseInt with a descriptive error.
func parseInt64(s, label string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", label, s, err)
	}
	return id, nil
}
