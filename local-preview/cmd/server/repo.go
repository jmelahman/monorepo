package server

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/jmelahman/local-preview/internal/client"
)

func repoCmd() *cobra.Command {
	var serverURL string
	parent := &cobra.Command{
		Use:   "repo",
		Short: "Manage registered repositories",
	}
	addServerFlag(parent, &serverURL)

	var source string
	var createWatch bool
	var createBranches string
	var createBackfill bool
	create := &cobra.Command{
		Use:   "create <name>",
		Short: "Register a repository for previews",
		Long: "Register a repository: the server keeps a mirror clone of --source\n" +
			"(a local path or clone URL) and serves previews at <sha>-<name>.<domain>.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url, err := resolveURL(cmd, serverURL)
			if err != nil {
				return err
			}
			return runRepoCreate(cmd.Context(), url, cmd.OutOrStdout(),
				args[0], source, createWatch, createBranches, createBackfill)
		},
	}
	create.Flags().StringVar(&source, "source", "", "Local path or clone URL of the repository (required)")
	create.Flags().BoolVar(&createWatch, "watch", false, "Watch the repo: poll for new commits and deploy them")
	create.Flags().StringVar(&createBranches, "branches", "", "Branches to build, as comma-separated globs; prefix with ! to exclude (default: all)")
	create.Flags().BoolVar(&createBackfill, "backfill", false, "Also deploy the branch tips that already exist, not just new commits")
	_ = create.MarkFlagRequired("source")

	var watchBranches string
	var watchBackfill bool
	watchCmd := &cobra.Command{
		Use:   "watch <name>",
		Short: "Poll a repository and deploy new branch tips automatically",
		Long: "Enable watching: the server periodically fetches the repo and deploys\n" +
			"the tip of every branch that gains commits. --branches narrows which\n" +
			"branches with comma-separated globs (e.g. \"main,release/*\"); a single\n" +
			"* doesn't cross '/' but ** does, and a ! prefix excludes (e.g. \"!main\"\n" +
			"builds every branch but main, \"!gh-readonly-queue/**\" drops merge-queue\n" +
			"refs). The same filter also gates webhook deploys. Only commits pushed\n" +
			"from here on deploy — pass --backfill to deploy the branch tips that\n" +
			"already exist too.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var branches *string
			if cmd.Flags().Changed("branches") {
				branches = &watchBranches
			}
			url, err := resolveURL(cmd, serverURL)
			if err != nil {
				return err
			}
			return runRepoWatch(cmd.Context(), url, cmd.OutOrStdout(),
				args[0], true, branches, watchBackfill)
		},
	}
	watchCmd.Flags().StringVar(&watchBranches, "branches", "", "Branches to build, as comma-separated globs; prefix with ! to exclude (default: all)")
	watchCmd.Flags().BoolVar(&watchBackfill, "backfill", false, "Also deploy the branch tips that already exist, not just new commits")

	unwatch := &cobra.Command{
		Use:   "unwatch <name>",
		Short: "Stop watching a repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url, err := resolveURL(cmd, serverURL)
			if err != nil {
				return err
			}
			return runRepoWatch(cmd.Context(), url, cmd.OutOrStdout(),
				args[0], false, nil, false)
		},
	}

	list := &cobra.Command{
		Use:   "list",
		Short: "List registered repositories",
		RunE: func(cmd *cobra.Command, args []string) error {
			url, err := resolveURL(cmd, serverURL)
			if err != nil {
				return err
			}
			return runRepoList(cmd.Context(), url, cmd.OutOrStdout())
		},
	}

	del := &cobra.Command{
		Use:   "delete <name>",
		Short: "Unregister a repository and delete its previews",
		Long: "Unregister a repository: stops its preview backends and deletes its\n" +
			"deploys, artifacts, state directories, build logs, and mirror clone.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url, err := resolveURL(cmd, serverURL)
			if err != nil {
				return err
			}
			return runRepoDelete(cmd.Context(), url, cmd.OutOrStdout(), args[0])
		},
	}

	parent.AddCommand(create, list, del, watchCmd, unwatch)
	return parent
}

func runRepoDelete(ctx context.Context, url string, out io.Writer, name string) error {
	c, err := newClient(url)
	if err != nil {
		return err
	}
	if err := c.DeleteRepo(ctx, name); err != nil {
		return err
	}
	fmt.Fprintf(out, "deleted %s\n", name)
	return nil
}

func runRepoCreate(ctx context.Context, url string, out io.Writer, name, source string, watch bool, branches string, backfill bool) error {
	c, err := newClient(url)
	if err != nil {
		return err
	}
	repo, err := c.CreateRepo(ctx, name, source, watch, branches, backfill)
	if err != nil {
		return err
	}
	// The server clones in the background; wait it out so the command's exit
	// means the repo is deployable (progress lines are the transport's).
	if repo.Status == "cloning" {
		fmt.Fprintf(out, "cloning %s…\n", repo.Source)
	}
	var lastProgress string
	for repo.Status == "cloning" {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
		if repo, err = c.GetRepo(ctx, name); err != nil {
			return err
		}
		if repo.Progress != "" && repo.Progress != lastProgress {
			fmt.Fprintf(out, "  %s\n", repo.Progress)
			lastProgress = repo.Progress
		}
	}
	if repo.Status == "failed" {
		return fmt.Errorf("clone failed: %s", repo.Error)
	}
	fmt.Fprintf(out, "registered %s (source %s)\n", repo.Name, repo.Source)
	if repo.Watch {
		fmt.Fprintf(out, "watching %s\n", watchLabel(repo))
	}
	return nil
}

func runRepoWatch(ctx context.Context, url string, out io.Writer, name string, watch bool, branches *string, backfill bool) error {
	c, err := newClient(url)
	if err != nil {
		return err
	}
	repo, err := c.SetRepoWatch(ctx, name, watch, branches, backfill)
	if err != nil {
		return err
	}
	if repo.Watch {
		fmt.Fprintf(out, "watching %s: %s — new commits now deploy automatically\n", repo.Name, watchLabel(repo))
	} else {
		fmt.Fprintf(out, "stopped watching %s\n", repo.Name)
	}
	return nil
}

// watchLabel describes which branches a watched repo deploys.
func watchLabel(r client.Repo) string {
	if r.WatchBranches == "" {
		return "all branches"
	}
	return "branches " + r.WatchBranches
}

func runRepoList(ctx context.Context, url string, out io.Writer) error {
	c, err := newClient(url)
	if err != nil {
		return err
	}
	repos, err := c.ListRepos(ctx)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tSOURCE\tSTATUS\tWATCH\tCREATED")
	for _, r := range repos {
		watch := "-"
		if r.Watch {
			watch = "all"
			if r.WatchBranches != "" {
				watch = r.WatchBranches
			}
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Name, r.Source, r.Status, watch, r.CreatedAt)
	}
	return tw.Flush()
}
