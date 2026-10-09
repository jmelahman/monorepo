package cmd

import (
	"fmt"
	"io"
	"strings"

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
		Use:   "add <remote> [<prefix>]",
		Short: "Import a repository as a subtree",
		Long: `Import a repository as a subtree.

The upstream branch is merged in at <prefix>, squashed unless the manifest
sets orchard.squash = false, and the subtree is added to the manifest,
which is left for you to commit.

Like git clone, <prefix> defaults to the repository's name, e.g. foo for
git@github.com:owner/foo.git.`,
		Example: `  git orchard add git@github.com:owner/foo.git
  git orchard add git@github.com:owner/foo.git tools/foo`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			remote := args[0]
			var prefix string
			if len(args) == 2 {
				prefix = args[1]
			} else if prefix = RepoName(remote); prefix == "" {
				return fmt.Errorf("can't guess a prefix from %s; pass one", remote)
			}
			return runAdd(cmd.ErrOrStderr(), opts, config.Subtree{Prefix: config.Clean(prefix), Remote: remote, Branch: opts.Branch})
		},
	}

	cmd.Flags().StringVarP(&opts.Branch, "branch", "b", config.DefaultBranch, "upstream branch")
	cmd.Flags().StringVarP(&opts.Message, "message", "m", "", "merge commit message")
	cmd.Flags().BoolVar(&opts.NoSquash, "no-squash", false, "merge upstream history instead of squashing it")

	return cmd
}

func runAdd(stderr io.Writer, opts *AddOptions, s config.Subtree) error {
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
	_, _ = fmt.Fprintf(stderr, "Added %s to %s; commit it to finish.\n", s.Prefix, o.Config.Manifest)
	return nil
}

// RepoName guesses the name of the repository at url the way git clone
// names its directory: git@github.com:owner/foo.git becomes foo.
func RepoName(url string) string {
	name := strings.TrimRight(url, "/")
	name = strings.TrimSuffix(name, "/.git")
	name = strings.TrimSuffix(name, ".git")
	name = strings.TrimRight(name, "/")
	return name[strings.LastIndexAny(name, "/:")+1:]
}
