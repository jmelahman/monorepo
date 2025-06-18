package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	log "github.com/sirupsen/logrus"

	"github.com/jmelahman/git-orchard/config"
)

var (
	Version = "dev"
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
		Version: Version,
	}

	cmd.Flags().BoolVar(&opts.Debug, "debug", false, "run in debug mode")

	// Add subcommands
	cmd.AddCommand(NewListCommand())

	return cmd
}

func runRoot(opts *RootOptions, args []string) {
	if opts.Debug {
		log.SetLevel(log.DebugLevel)
	}

	fmt.Printf("Welcome to git-orchard (%v).\n", Version)

	// Read subtree configurations from git config
	reader := config.NewGitConfigReader(".")
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
