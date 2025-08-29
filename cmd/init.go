package cmd

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/jmelahman/git-orchard/history"
)

// InitOptions holds options for the list command
type InitOptions struct {
	Debug bool
}

// NewInitCommand creates a new list command
func NewInitCommand() *cobra.Command {
	opts := &InitOptions{}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Set up git-orchard in the current worktree",
		Run: func(cmd *cobra.Command, args []string) {
			if opts.Debug {
				log.SetLevel(log.DebugLevel)
			}
			initConfigFromHistory()
		},
	}

	cmd.Flags().BoolVar(&opts.Debug, "debug", false, "run in debug mode")

	return cmd
}

func initConfigFromHistory() {
	reader := history.NewGitHistoryReader()
	subtreeMap, err := reader.GetSubtreesFromHistory()
	if err != nil {
		log.Errorf("Failed to execute git log: %v", err)
		return
	}

	if len(subtreeMap) == 0 {
		fmt.Println("No subtree merges found in git history.")
		return
	}

	fmt.Printf("Found %d subtree(s) from git history:\n\n", len(subtreeMap))
	for _, info := range subtreeMap {
		fmt.Printf("Prefix: %s\n", info.Prefix)
		fmt.Printf("  Last commit: %s\n", info.LastCommit)
		fmt.Printf("  Last message: %s\n", info.LastMessage)
		fmt.Println()
	}
}
