package cmd

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// PullOptions holds options for the pull command
type PullOptions struct {
	Message  string
	NoSquash bool
	NoVerify bool
}

// NewPullCommand creates a new pull command
func NewPullCommand() *cobra.Command {
	opts := &PullOptions{}

	cmd := &cobra.Command{
		Use:   "pull [prefix...]",
		Short: "Merge upstream changes into subtrees",
		Long: `Merge upstream changes into subtrees.

Each subtree (every one listed, unless prefixes are given) gets its upstream
branch merged in, squashed unless the manifest sets orchard.squash = false.
Pulling stops at the first failure, e.g. a merge conflict to resolve.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPull(opts, args)
		},
	}

	cmd.Flags().StringVarP(&opts.Message, "message", "m", "", `merge commit message (default "Update <prefix>")`)
	cmd.Flags().BoolVar(&opts.NoSquash, "no-squash", false, "merge upstream history instead of squashing it")
	cmd.Flags().BoolVar(&opts.NoVerify, "no-verify", false, "run the merge without any hooks")

	return cmd
}

func runPull(opts *PullOptions, prefixes []string) error {
	o, err := openOrchard()
	if err != nil {
		return err
	}
	o.NoVerify = opts.NoVerify
	if opts.NoSquash {
		o.Config.Squash = false
	}

	subtrees, err := o.Config.Select(prefixes)
	if err != nil {
		return err
	}
	for _, s := range subtrees {
		message := opts.Message
		if message == "" {
			message = "Update " + s.Prefix
		}
		log.Infof("Pulling %s from %s %s", s.Prefix, s.Remote, s.Branch)
		if err := o.Pull(s, message); err != nil {
			return fmt.Errorf("%s: %w", s.Prefix, err)
		}
	}
	return nil
}
