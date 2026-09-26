package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jmelahman/git-orchard/config"
	"github.com/jmelahman/git-orchard/ghrun"
	"github.com/jmelahman/git-orchard/orchard"
	"github.com/jmelahman/git-orchard/semver"
)

// ReleaseOptions holds options for the release command
type ReleaseOptions struct {
	orchard.ReleaseOptions

	Suffix              string
	Major, Minor, Patch bool

	NoVerify bool
	DryRun   bool
	Yes      bool

	Watch   bool
	Timeout time.Duration
}

// NewReleaseCommand creates a new release command
func NewReleaseCommand() *cobra.Command {
	opts := &ReleaseOptions{}

	cmd := &cobra.Command{
		Use:   "release <prefix> [version]",
		Short: "Tag a subtree release",
		Long: `Tag a subtree release.

Tags --rev as <prefix>/<version> and pushes the tag to --remote, where the
mirror action publishes it to the prefix's upstream as <version> (it
publishes tags matching **/v*).

Without a version, it's picked after the latest release, much as tag picks
it: the patch version incremented (or --minor or --major), or a
pre-release's stable release. --suffix picks the next pre-release of that
name instead, e.g. v1.2.4-rc, then v1.2.4-rc.1. Releases are the <prefix>/v*
tags here and on --remote, and the v* tags upstream. It asks before
releasing, unless --yes.

The upstream branch must already contain the release, so the upstream tag
lands on its history: publish the subtree first, or pass --upstream to push
the branch and tag upstream directly, e.g. where no action runs.

--force moves an existing tag, unless the upstream has already published it
at another commit: the Go module proxy and release artifacts won't follow a
moved release, so publish a new version instead.

A release that already tags --rev is published again rather than retagged,
so a release whose publishing failed can be finished by running it again.

--watch follows the release through GitHub Actions with gh: the runs the tag
push triggers on --remote (the mirror action), then, once the tag reaches
the upstream, the runs it triggers there (e.g. a release workflow). It fails
if any of them fails. Runs are found by the tag they ran for, so no workflow
needs naming; a tag that starts none within a minute is taken to have none.
Watching a release that already tags --rev checks on it without retagging.`,
		Example: `  git orchard release connections
  git orchard release connections --minor --suffix rc
  git orchard release git-orchard v1.0.0 --upstream
  git orchard release connections --watch`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			version := ""
			if len(args) == 2 {
				version = args[1]
			}
			return runRelease(opts, config.Clean(args[0]), version)
		},
	}

	cmd.Flags().StringVar(&opts.Rev, "rev", "HEAD", "monorepo revision to release")
	cmd.Flags().StringVarP(&opts.Message, "message", "m", "", `tag message (default "<prefix> <version>")`)
	cmd.Flags().StringVar(&opts.Remote, "remote", "origin", "monorepo remote to push the tag to; empty only tags locally")
	cmd.Flags().BoolVar(&opts.Upstream, "upstream", false, "also push the branch and tag to the upstream directly")
	cmd.Flags().BoolVar(&opts.NoVerify, "no-verify", false, "skip the pre-push hook")
	cmd.Flags().BoolVarP(&opts.Force, "force", "f", false, "move an existing tag that the upstream hasn't published")
	cmd.Flags().BoolVar(&opts.Major, "major", false, "increment the major version")
	cmd.Flags().BoolVar(&opts.Minor, "minor", false, "increment the minor version")
	cmd.Flags().BoolVar(&opts.Patch, "patch", false, "increment the patch version")
	cmd.Flags().StringVar(&opts.Suffix, "suffix", "", "release a pre-release, e.g. rc, alpha or beta")
	cmd.Flags().BoolVarP(&opts.Yes, "yes", "y", false, "release without asking")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "print the next version and exit")
	cmd.Flags().BoolVarP(&opts.Watch, "watch", "w", false, "follow the release's GitHub Actions runs until they finish")
	cmd.Flags().DurationVar(&opts.Timeout, "timeout", 30*time.Minute, "how long --watch waits")
	cmd.MarkFlagsMutuallyExclusive("major", "minor", "patch")
	cmd.MarkFlagsMutuallyExclusive("watch", "dry-run")

	return cmd
}

func runRelease(opts *ReleaseOptions, prefix, version string) error {
	if version != "" && (opts.Major || opts.Minor || opts.Patch || opts.Suffix != "" || opts.DryRun) {
		return errors.New("pass either a version or --major, --minor, --patch, --suffix or --dry-run to pick one")
	}
	o, err := openOrchard()
	if err != nil {
		return err
	}
	o.NoVerify = opts.NoVerify
	s, ok := o.Config.Lookup(prefix)
	if !ok {
		return fmt.Errorf("%s is not a subtree listed in %s", prefix, o.Config.Manifest)
	}
	if opts.Watch {
		if opts.Remote == "" && !opts.Upstream {
			return errors.New("--watch needs the release published, to --remote or with --upstream")
		}
		if err := ghrun.CheckGH(); err != nil {
			return err
		}
	}

	if version == "" {
		inc := semver.Auto
		switch {
		case opts.Major:
			inc = semver.Major
		case opts.Minor:
			inc = semver.Minor
		case opts.Patch:
			inc = semver.Patch
		}
		var released bool
		version, released, err = o.NextVersion(s, orchard.NextOptions{
			Rev:       opts.Rev,
			Remote:    opts.Remote,
			Increment: inc,
			Suffix:    opts.Suffix,
		})
		if err != nil {
			return err
		}
		if opts.DryRun {
			fmt.Println(version)
			return nil
		}
		question := fmt.Sprintf("Release %s/%s?", s.Prefix, version)
		if released {
			question = fmt.Sprintf("%s is already released as %s/%s; publish it again?", opts.Rev, s.Prefix, version)
		}
		if !opts.Yes {
			ok, err := confirm(question)
			if err != nil || !ok {
				return err
			}
		}
	}

	tag, err := o.Release(s, version, opts.ReleaseOptions)
	if tag != "" && err != nil {
		fmt.Fprintf(os.Stderr, "Tagged %s, but publishing it failed; fix that and push the tag again.\n", tag)
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Released %s.\n", tag)
	if opts.Watch {
		return watchRelease(o, s, version, tag, opts)
	}
	return nil
}

// watchRelease follows the GitHub Actions runs publishing tag, the release
// of version of s: the mirror's on --remote, then the upstream's.
func watchRelease(o *orchard.Orchard, s config.Subtree, version, tag string, opts *ReleaseOptions) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	err := watchRuns(ctx, o, s, version, tag, opts)
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("stopped watching after %s; %s may still be publishing", opts.Timeout, tag)
	case errors.Is(err, context.Canceled):
		return fmt.Errorf("stopped watching; %s carries on publishing", tag)
	}
	return err
}

func watchRuns(ctx context.Context, o *orchard.Orchard, s config.Subtree, version, tag string, opts *ReleaseOptions) error {
	var watch ghrun.WatchOptions
	watch.Interval = 5 * time.Second
	watch.Grace = time.Minute

	// With --upstream, the tag is already upstream, so the mirror has
	// nothing left to publish.
	if opts.Remote != "" && !opts.Upstream {
		commit, err := o.Repo.Output("rev-parse", "--verify", "refs/tags/"+tag+"^{commit}")
		if err != nil {
			return err
		}
		if owner, name, ok := ghrun.ParseGitHubRepo(o.RemoteURL(opts.Remote)); ok {
			if err := watchRepo(ctx, owner+"/"+name, tag, commit, watch); err != nil {
				return fmt.Errorf("mirroring %s failed: %w", tag, err)
			}
		} else {
			fmt.Fprintf(os.Stderr, "%s isn't on GitHub; not watching the mirror.\n", opts.Remote)
		}
	}

	// The mirror's runs finishing doesn't guarantee the tag is upstream, e.g.
	// where none ran, so wait for it.
	deadline := time.Now().Add(watch.Grace)
	var commit string
	for {
		var err error
		if commit, err = o.UpstreamTag(s, version); err != nil {
			return err
		}
		if commit != "" {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s's upstream has no tag %s; is the mirror action publishing tags?", s.Prefix, version)
		}
		if err := ghrun.Sleep(ctx, watch.Interval); err != nil {
			return err
		}
	}

	owner, name, ok := ghrun.ParseGitHubRepo(s.Remote)
	if !ok {
		fmt.Fprintf(os.Stderr, "%s's upstream isn't on GitHub; not watching it.\n", s.Prefix)
		return nil
	}
	if err := watchRepo(ctx, owner+"/"+name, version, commit, watch); err != nil {
		return fmt.Errorf("releasing %s failed: %w", version, err)
	}
	fmt.Fprintf(os.Stderr, "Published %s.\n", tag)
	return nil
}

// watchRepo follows the runs the push of tag at commit started in repo,
// reporting each as its state changes.
func watchRepo(ctx context.Context, repo, tag, commit string, opts ghrun.WatchOptions) error {
	fmt.Fprintf(os.Stderr, "Watching %s's runs for %s...\n", repo, tag)
	seen := map[int64]bool{}
	runs, err := ghrun.Watch(ctx, ghrun.GH, repo, tag, commit, opts, func(r ghrun.Run) {
		if !seen[r.ID] {
			seen[r.ID] = true
			fmt.Fprintf(os.Stderr, "  %s: %s (%s)\n", r.Workflow, r.State(), r.URL)
			return
		}
		fmt.Fprintf(os.Stderr, "  %s: %s\n", r.Workflow, r.State())
	})
	if err == nil && len(runs) == 0 {
		fmt.Fprintf(os.Stderr, "  No runs started for %s.\n", tag)
	}
	return err
}

// confirm asks a yes/no question on stdin, where an empty answer accepts.
func confirm(question string) (bool, error) {
	fmt.Fprintf(os.Stderr, "%s [Y/n] ", question)
	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if errors.Is(err, io.EOF) && answer == "" {
		fmt.Fprintln(os.Stderr)
		return false, errors.New("no answer; pass --yes to release without asking")
	} else if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "", "y", "yes":
		return true, nil
	}
	return false, nil
}
