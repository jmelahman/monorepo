package cmd

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/jmelahman/git-orchard/orchard"
)

// PushOptions holds options for the push command
type PushOptions struct {
	Rev          string
	ChangedSince string
	Tag          string
	DryRun       bool
	NoVerify     bool
	Force        bool
	Jobs         int
}

// NewPushCommand creates a new push command
func NewPushCommand() *cobra.Command {
	opts := &PushOptions{}

	cmd := &cobra.Command{
		Use:   "push [prefix...]",
		Short: "Publish subtrees to their upstreams",
		Long: `Publish subtrees to their upstreams.

Each subtree (every one listed, unless prefixes are given) is split out of
--rev and pushed to its upstream branch. Unless forced, an upstream with
commits the monorepo lacks rejects the push until they are pulled in.
--force overwrites it, leased on the upstream's value when the push starts,
so a concurrent update still fails it.
Subtrees are pushed --jobs at a time, each one's output printed once it's
done. A failed push doesn't stop the others.

With --tag, a release tag <prefix>/<name> is published to that prefix's
upstream as <name>, e.g. connections/v1.2.3 as v1.2.3.`,
		Example: `  git orchard push tag
  git orchard push --changed-since HEAD@{1}
  git orchard push --tag connections/v1.2.3`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Tag != "" && (len(args) > 0 || opts.ChangedSince != "") {
				return fmt.Errorf("--tag takes neither prefixes nor --changed-since")
			}
			return runPush(opts, args)
		},
	}

	cmd.Flags().StringVar(&opts.Rev, "rev", "HEAD", "monorepo revision to publish")
	cmd.Flags().StringVar(&opts.ChangedSince, "changed-since", "", "only push subtrees changed between this revision and --rev (all of them if it isn't a commit here)")
	cmd.Flags().StringVar(&opts.Tag, "tag", "", "publish the release tag <prefix>/<name> as <name>")
	cmd.Flags().BoolVarP(&opts.DryRun, "dry-run", "n", false, "do everything except send the updates")
	cmd.Flags().BoolVar(&opts.NoVerify, "no-verify", false, "skip the pre-push hook")
	cmd.Flags().BoolVarP(&opts.Force, "force", "f", false, "overwrite upstream branches or tags (with a lease)")
	cmd.Flags().IntVarP(&opts.Jobs, "jobs", "j", runtime.NumCPU(), "subtrees to push at once")

	return cmd
}

func runPush(opts *PushOptions, prefixes []string) error {
	o, err := openOrchard()
	if err != nil {
		return err
	}
	o.NoVerify = opts.NoVerify
	pushOpts := orchard.PushOptions{DryRun: opts.DryRun, Force: opts.Force}

	if opts.Tag != "" {
		return o.PushTag(opts.Tag, pushOpts)
	}

	subtrees, err := o.Config.Select(prefixes)
	if err != nil {
		return err
	}
	if opts.ChangedSince != "" {
		if subtrees, err = o.ChangedSince(subtrees, opts.ChangedSince, opts.Rev); err != nil {
			return err
		}
	}

	var (
		mu     sync.Mutex
		failed []string
	)
	orchard.Each(len(subtrees), opts.Jobs, func(i int) {
		s := subtrees[i]
		var out bytes.Buffer
		subOpts := pushOpts
		subOpts.Output = &out
		err := o.Push(s, opts.Rev, subOpts)

		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			log.Errorf("Failed to push %s to %s %s: %v", s.Prefix, s.Remote, s.Branch, err)
			failed = append(failed, s.Prefix)
		} else {
			log.Infof("Pushed %s to %s %s", s.Prefix, s.Remote, s.Branch)
		}
		_, _ = os.Stderr.Write(out.Bytes())
	})
	if len(failed) > 0 {
		return fmt.Errorf("failed to push %d of %d subtree(s): %v", len(failed), len(subtrees), failed)
	}
	return nil
}
