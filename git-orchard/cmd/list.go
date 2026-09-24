package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jmelahman/git-orchard/history"
)

// ListOptions holds options for the list command
type ListOptions struct {
	UseHistory bool
}

// NewListCommand creates a new list command
func NewListCommand() *cobra.Command {
	opts := &ListOptions{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all configured subtrees",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.UseHistory {
				return listSubtreesFromHistory()
			}
			return listSubtreesFromConfig()
		},
	}

	cmd.Flags().BoolVar(&opts.UseHistory, "use-history", false, "determine subtrees from git log history instead of config")

	return cmd
}

func listSubtreesFromConfig() error {
	o, err := openOrchard()
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, s := range o.Config.Subtrees {
		fmt.Fprintf(w, "%s\t%s\t%s\n", s.Prefix, s.Remote, s.Branch)
	}
	return w.Flush()
}

func listSubtreesFromHistory() error {
	reader := history.NewGitHistoryReader()
	subtreeMap, err := reader.GetSubtreesFromHistory()
	if err != nil {
		return fmt.Errorf("failed to read git history: %w", err)
	}

	if len(subtreeMap) == 0 {
		fmt.Println("No subtree merges found in git history.")
		return nil
	}

	fmt.Printf("Found %d subtree(s) from git history:\n\n", len(subtreeMap))
	for _, info := range subtreeMap {
		fmt.Printf("Prefix: %s\n", info.Prefix)
		fmt.Printf("  Last commit: %s\n", info.LastCommit)
		fmt.Printf("  Last message: %s\n", info.LastMessage)
		fmt.Println()
	}
	return nil
}
