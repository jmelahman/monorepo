package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	log "github.com/sirupsen/logrus"

	"github.com/jmelahman/git-orchard/config"
	"github.com/jmelahman/git-orchard/history"
)

// ListOptions holds options for the list command
type ListOptions struct {
	UseHistory bool
	Debug      bool
}

// NewListCommand creates a new list command
func NewListCommand() *cobra.Command {
	opts := &ListOptions{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all configured subtrees",
		Run: func(cmd *cobra.Command, args []string) {
			if opts.Debug {
				log.SetLevel(log.DebugLevel)
			}
			runList(opts)
		},
	}

	cmd.Flags().BoolVar(&opts.UseHistory, "use-history", false, "determine subtrees from git log history instead of config")
	cmd.Flags().BoolVar(&opts.Debug, "debug", false, "run in debug mode")

	return cmd
}

func runList(opts *ListOptions) {
	if opts.UseHistory {
		listSubtreesFromHistory()
	} else {
		listSubtreesFromConfig()
	}
}

func listSubtreesFromConfig() {
	reader := config.NewGitConfigReader(".")
	subtrees, _, err := reader.ReadSubtreeConfigs()
	if err != nil {
		log.Errorf("Failed to read subtree configs: %v", err)
		return
	}

	if len(subtrees) == 0 {
		fmt.Println("No subtrees configured.")
		return
	}

	fmt.Printf("Found %d configured subtree(s):\n\n", len(subtrees))
	for _, subtree := range subtrees {
		fmt.Printf("Name: %s\n", subtree.Name)
		fmt.Printf("  Repository: %s\n", subtree.Repository)
		fmt.Printf("  Prefix: %s\n", subtree.Prefix)
		if subtree.Branch != "" {
			fmt.Printf("  Branch: %s\n", subtree.Branch)
		}
		fmt.Println()
	}
}

func listSubtreesFromHistory() {
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
