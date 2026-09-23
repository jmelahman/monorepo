package server

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jmelahman/local-preview/internal/client"
)

func openCmd() *cobra.Command {
	var serverURL, repoFlag string
	var printOnly bool

	cmd := &cobra.Command{
		Use:   "open [ref]",
		Short: "Open a deploy's preview URL in the browser",
		Long: "Find the deploy for a ref (default: HEAD of the current repo) and open\n" +
			"its preview URL in the browser. The ref may be a branch (its newest\n" +
			"deploy wins), a tag, or a full or abbreviated commit sha. The repo is\n" +
			"auto-detected from the working directory unless --repo is given.",
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
			return runOpen(cmd.Context(), url, cmd.OutOrStdout(), repoFlag, ref, printOnly)
		},
	}
	addServerFlag(cmd, &serverURL)
	cmd.Flags().StringVar(&repoFlag, "repo", "", "Registered repo name (default: auto-detect from cwd)")
	cmd.Flags().BoolVar(&printOnly, "print", false, "Print the preview URL instead of opening a browser")
	return cmd
}

func runOpen(ctx context.Context, url string, out io.Writer, repoName, ref string, printOnly bool) error {
	c, err := newClient(url)
	if err != nil {
		return err
	}
	if repoName == "" {
		if repoName, err = detectRepo(ctx, c); err != nil {
			return err
		}
	}
	d, err := findDeploy(ctx, c, out, repoName, ref)
	if err != nil {
		return err
	}
	switch d.Status {
	case "queued", "building":
		return fmt.Errorf("deploy %d (%s@%s) is still %s; retry once it's ready, or wait on it with `preview deploy %s`",
			d.ID, d.Repo, d.ShortSHA, d.Status, d.SHA)
	case "failed":
		return fmt.Errorf("deploy %d (%s@%s) failed: %s (full logs: preview deploy logs %d)",
			d.ID, d.Repo, d.ShortSHA, d.Error, d.ID)
	case "evicted":
		return fmt.Errorf("deploy %d (%s@%s) was evicted; rebuild it with `preview deploy %s`",
			d.ID, d.Repo, d.ShortSHA, d.SHA)
	}
	fmt.Fprintln(out, d.PreviewURL)
	if !printOnly {
		if err := openBrowser(d.PreviewURL); err != nil {
			fmt.Fprintf(out, "could not open a browser: %v\n", err)
		}
	}
	return nil
}

// findDeploy resolves ref to a deploy of the repo. An empty ref means the
// current checkout's HEAD, falling back to the checked-out branch's newest
// deploy when that exact commit was never deployed. A non-empty ref is tried
// as a branch (newest deploy wins), then as a locally resolvable revision,
// then as a free-text search (sha-prefix matches preferred).
func findDeploy(ctx context.Context, c *client.Client, out io.Writer, repoName, ref string) (client.Deploy, error) {
	if ref == "" {
		sha, err := localHeadSHA(".")
		if err != nil {
			return client.Deploy{}, fmt.Errorf("no ref given and no commit found in the current directory: %w", err)
		}
		if d, ok, err := deployBySHA(ctx, c, repoName, sha); err != nil || ok {
			return d, err
		}
		if branch := localHeadBranch("."); branch != "" {
			if d, ok, err := newestOnBranch(ctx, c, repoName, branch); err != nil {
				return client.Deploy{}, err
			} else if ok {
				fmt.Fprintf(out, "commit %.7s has no deploy; using branch %s's newest deploy (%s)\n",
					sha, branch, d.ShortSHA)
				return d, nil
			}
		}
		return client.Deploy{}, fmt.Errorf("commit %.7s has no deploy; create one with `preview deploy`", sha)
	}

	if d, ok, err := newestOnBranch(ctx, c, repoName, ref); err != nil || ok {
		return d, err
	}
	if sha, err := localResolveRevision(".", ref); err == nil {
		if d, ok, err := deployBySHA(ctx, c, repoName, sha); err != nil || ok {
			return d, err
		}
		return client.Deploy{}, fmt.Errorf("%s resolves to %.7s, which has no deploy; create one with `preview deploy %s`", ref, sha, ref)
	}
	deploys, err := c.ListDeploys(ctx, client.DeployFilter{Repo: repoName, Query: ref})
	if err != nil {
		return client.Deploy{}, err
	}
	// Newest-first; an exact sha-prefix hit beats a mere substring match.
	for _, d := range deploys {
		if strings.HasPrefix(d.SHA, strings.ToLower(ref)) {
			return d, nil
		}
	}
	if len(deploys) > 0 {
		return deploys[0], nil
	}
	return client.Deploy{}, fmt.Errorf("no deploy of %q matches %q; create one with `preview deploy %s`", repoName, ref, ref)
}

// deployBySHA finds the repo's deploy of the exact commit.
func deployBySHA(ctx context.Context, c *client.Client, repoName, sha string) (client.Deploy, bool, error) {
	deploys, err := c.ListDeploys(ctx, client.DeployFilter{Repo: repoName, Query: sha})
	if err != nil {
		return client.Deploy{}, false, err
	}
	for _, d := range deploys {
		if d.SHA == strings.ToLower(sha) {
			return d, true, nil
		}
	}
	return client.Deploy{}, false, nil
}

// newestOnBranch finds the repo's newest deploy on the branch, if any.
func newestOnBranch(ctx context.Context, c *client.Client, repoName, branch string) (client.Deploy, bool, error) {
	deploys, err := c.ListDeploys(ctx, client.DeployFilter{Repo: repoName, Branch: branch})
	if err != nil || len(deploys) == 0 {
		return client.Deploy{}, false, err
	}
	return deploys[0], true, nil
}

// openBrowser hands the URL to the platform's opener.
func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Run()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Run()
	default:
		return exec.Command("xdg-open", url).Run()
	}
}
