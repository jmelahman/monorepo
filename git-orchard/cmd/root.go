package cmd

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/jmelahman/git-orchard/config"
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
		Use:   "git-orchard [paths...]",
		Short: "Utilities for managing git subtrees",
		Run: func(cmd *cobra.Command, args []string) {
			runRoot(opts, args)
		},
		Version: fmt.Sprintf("%s\ncommit %s", Version, Commit),
	}

	cmd.Flags().BoolVar(&opts.Debug, "debug", false, "run in debug mode")

	// Add subcommands
	cmd.AddCommand(NewInitCommand())
	cmd.AddCommand(NewListCommand())

	return cmd
}

func runRoot(opts *RootOptions, args []string) {
	if opts.Debug {
		log.SetLevel(log.DebugLevel)
	}

	// Read subtree configurations from git config
	reader := config.NewGitConfigReader("")
	subtrees, orchardConfig, err := reader.ReadSubtreeConfigs()
	if err != nil {
		log.Errorf("Failed to read subtree configs: %v", err)
		return
	}

	log.Debugf("Found %d subtree configurations", len(subtrees))
	for _, subtree := range subtrees {
		log.Debugf("Subtree: %s -> %s (%s)", subtree.Name, subtree.Repository, subtree.Prefix)
		if subtree.Branch != "" {
			log.Debugf("  Branch: %s", subtree.Branch)
		}
	}

	if orchardConfig.Squash {
		log.Debug("Orchard config: squash enabled")
	}
}
