package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jmelahman/git-orchard/config"
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
so a release whose publishing failed can be finished by running it again.`,
		Example: `  git orchard release connections
  git orchard release connections --minor --suffix rc
  git orchard release git-orchard v1.0.0 --upstream`,
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
	cmd.MarkFlagsMutuallyExclusive("major", "minor", "patch")

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
	return nil
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
