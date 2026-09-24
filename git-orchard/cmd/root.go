package cmd

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/jmelahman/git-orchard/orchard"
)

var (
	Version string
	Commit  string
)

// RootOptions holds options for the root command
type RootOptions struct {
	Debug bool
}

// NewRootCommand creates the root command
func NewRootCommand() *cobra.Command {
	opts := &RootOptions{}

	cmd := &cobra.Command{
		Use:   "git-orchard",
		Short: "Utilities for managing git subtrees",
		Long: `Utilities for managing git subtrees.

Subtrees are listed in a committed manifest at the repository root,
.gitsubtrees or .config/git-orchard/subtrees, in gitconfig syntax:

  [subtree "path/to/dir"]
  	remote = git@github.com:owner/dir.git
  	branch = master

The same keys in git's own configuration override it.`,
		Version:       fmt.Sprintf("%s\ncommit %s", Version, Commit),
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if opts.Debug {
				log.SetLevel(log.DebugLevel)
			}
		},
	}

	cmd.PersistentFlags().BoolVar(&opts.Debug, "debug", false, "run in debug mode")

	// Add subcommands
	cmd.AddCommand(NewAddCommand())
	cmd.AddCommand(NewGitHubAppCommand())
	cmd.AddCommand(NewInitCommand())
	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewPullCommand())
	cmd.AddCommand(NewPushCommand())
	cmd.AddCommand(NewReleaseCommand())
	cmd.AddCommand(NewStatusCommand())

	return cmd
}

func openOrchard() (*orchard.Orchard, error) {
	return orchard.Open(".")
}
