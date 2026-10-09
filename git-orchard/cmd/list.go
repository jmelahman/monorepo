package cmd

import (
	"fmt"
	"io"
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
				return listSubtreesFromHistory(cmd.OutOrStdout())
			}
			return listSubtreesFromConfig(cmd.OutOrStdout())
		},
	}

	cmd.Flags().BoolVar(&opts.UseHistory, "use-history", false, "determine subtrees from git log history instead of config")

	return cmd
}

func listSubtreesFromConfig(stdout io.Writer) error {
	o, err := openOrchard()
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	for _, s := range o.Config.Subtrees {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", s.Prefix, s.Remote, s.Branch)
	}
	return w.Flush()
}

func listSubtreesFromHistory(stdout io.Writer) error {
	reader := history.NewGitHistoryReader()
	subtreeMap, err := reader.GetSubtreesFromHistory()
	if err != nil {
		return fmt.Errorf("failed to read git history: %w", err)
	}

	if len(subtreeMap) == 0 {
		_, _ = fmt.Fprintln(stdout, "No subtree merges found in git history.")
		return nil
	}

	_, _ = fmt.Fprintf(stdout, "Found %d subtree(s) from git history:\n\n", len(subtreeMap))
	for _, info := range subtreeMap {
		_, _ = fmt.Fprintf(stdout, "Prefix: %s\n", info.Prefix)
		_, _ = fmt.Fprintf(stdout, "  Last commit: %s\n", info.LastCommit)
		_, _ = fmt.Fprintf(stdout, "  Last message: %s\n", info.LastMessage)
		_, _ = fmt.Fprintln(stdout)
	}
	return nil
}
