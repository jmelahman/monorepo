package server

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/jmelahman/local-preview/internal/client"
)

// logsCmd is the top-level `preview logs [ref]`: the live run log (stdout+
// stderr) of a preview's supervised process, addressed by ref rather than a
// numeric deploy id. It's the run-log half of `preview deploy logs --run`
// wrapped with the same ref resolution as `preview open`, so you can say
// `preview logs my-branch -f` without first looking up the deploy id.
func logsCmd() *cobra.Command {
	var serverURL, repoFlag, side string
	var follow bool

	cmd := &cobra.Command{
		Use:   "logs [ref]",
		Short: "Stream a preview's run log (its process stdout+stderr)",
		Long: "Print the run log — the preview server's own stdout and stderr — for\n" +
			"the deploy matching a ref (default: HEAD of the current repo). The ref\n" +
			"may be a branch (its newest deploy wins), a tag, or a full or\n" +
			"abbreviated commit sha. The repo is auto-detected from the working\n" +
			"directory unless --repo is given. For a deploy's build logs, use\n" +
			"`preview deploy logs <id>`.",
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
			c, err := newClient(url)
			if err != nil {
				return err
			}
			d, err := runtimeDeploy(cmd.Context(), c, cmd.OutOrStdout(), repoFlag, ref)
			if err != nil {
				return err
			}
			return printRunLog(cmd.Context(), c, cmd.OutOrStdout(), d.ID, side, follow)
		},
	}
	addServerFlag(cmd, &serverURL)
	cmd.Flags().StringVar(&repoFlag, "repo", "", "Registered repo name (default: auto-detect from cwd)")
	cmd.Flags().StringVar(&side, "side", "be", "Which process: be (backend) or fe (process-mode frontend)")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Keep polling for new run-log output until interrupted")
	return cmd
}

// statsCmd is the top-level `preview stats [ref]`: live CPU/memory for a
// preview's processes, addressed by ref. It wraps `preview deploy stats` with
// the same ref resolution as `preview open`.
func statsCmd() *cobra.Command {
	var serverURL, repoFlag string

	cmd := &cobra.Command{
		Use:   "stats [ref]",
		Short: "Show live CPU/memory of a preview's processes",
		Long: "Sample the CPU and memory of the supervised processes backing the\n" +
			"deploy matching a ref (default: HEAD of the current repo). The ref may\n" +
			"be a branch (its newest deploy wins), a tag, or a full or abbreviated\n" +
			"commit sha. The repo is auto-detected from the working directory\n" +
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
			c, err := newClient(url)
			if err != nil {
				return err
			}
			d, err := runtimeDeploy(cmd.Context(), c, cmd.OutOrStdout(), repoFlag, ref)
			if err != nil {
				return err
			}
			return runDeployStats(cmd.Context(), url, cmd.OutOrStdout(), d.ID)
		},
	}
	addServerFlag(cmd, &serverURL)
	cmd.Flags().StringVar(&repoFlag, "repo", "", "Registered repo name (default: auto-detect from cwd)")
	return cmd
}

// runtimeDeploy resolves repo + ref to a deploy whose processes can actually
// be inspected. It mirrors `preview open`: auto-detect the repo, resolve the
// ref, then reject states with no live process to talk to (a build that never
// produced a running backend has neither a run log nor stats).
func runtimeDeploy(ctx context.Context, c *client.Client, out io.Writer, repoName, ref string) (client.Deploy, error) {
	if repoName == "" {
		var err error
		if repoName, err = detectRepo(ctx, c); err != nil {
			return client.Deploy{}, err
		}
	}
	d, err := findDeploy(ctx, c, out, repoName, ref)
	if err != nil {
		return client.Deploy{}, err
	}
	switch d.Status {
	case "queued", "building":
		return client.Deploy{}, fmt.Errorf("deploy %d (%s@%s) is still %s; retry once it's ready, or wait on it with `preview deploy %s`",
			d.ID, d.Repo, d.ShortSHA, d.Status, d.SHA)
	case "failed":
		return client.Deploy{}, fmt.Errorf("deploy %d (%s@%s) failed: %s (build logs: preview deploy logs %d)",
			d.ID, d.Repo, d.ShortSHA, d.Error, d.ID)
	case "evicted":
		return client.Deploy{}, fmt.Errorf("deploy %d (%s@%s) was evicted; rebuild it with `preview deploy %s`",
			d.ID, d.Repo, d.ShortSHA, d.SHA)
	}
	return d, nil
}
