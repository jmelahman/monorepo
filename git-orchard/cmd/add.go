package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jmelahman/git-orchard/config"
)

// AddOptions holds options for the add command
type AddOptions struct {
	Branch   string
	Message  string
	NoSquash bool
}

// NewAddCommand creates a new add command
func NewAddCommand() *cobra.Command {
	opts := &AddOptions{}

	cmd := &cobra.Command{
		Use:   "add <prefix> <remote>",
		Short: "Import a repository as a subtree",
		Long: `Import a repository as a subtree.

The upstream branch is merged in at <prefix>, squashed unless the manifest
sets orchard.squash = false, and the subtree is added to the manifest,
which is left for you to commit.`,
		Example: `  git orchard add tools/foo git@github.com:owner/foo.git`,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(opts, config.Subtree{Prefix: config.Clean(args[0]), Remote: args[1], Branch: opts.Branch})
		},
	}

	cmd.Flags().StringVarP(&opts.Branch, "branch", "b", config.DefaultBranch, "upstream branch")
	cmd.Flags().StringVarP(&opts.Message, "message", "m", "", "merge commit message")
	cmd.Flags().BoolVar(&opts.NoSquash, "no-squash", false, "merge upstream history instead of squashing it")

	return cmd
}

func runAdd(opts *AddOptions, s config.Subtree) error {
	o, err := openOrchard()
	if err != nil {
		return err
	}
	if opts.NoSquash {
		o.Config.Squash = false
	}
	if err := o.Add(s, opts.Message); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Added %s to %s; commit it to finish.\n", s.Prefix, o.Config.Manifest)
	return nil
}
