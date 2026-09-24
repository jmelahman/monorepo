package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// StatusOptions holds options for the status command
type StatusOptions struct {
	Rev string
}

// NewStatusCommand creates a new status command
func NewStatusCommand() *cobra.Command {
	opts := &StatusOptions{}

	cmd := &cobra.Command{
		Use:   "status [prefix...]",
		Short: "Compare subtrees with their upstreams",
		Long: `Compare subtrees with their upstreams.

Each subtree's upstream branch is fetched (to refs/orchard/upstream/<prefix>)
and compared with the subtree's split of --rev: "ahead" commits are ones a
push would publish, "behind" ones a pull would bring in.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(opts, args)
		},
	}

	cmd.Flags().StringVar(&opts.Rev, "rev", "HEAD", "monorepo revision to compare")

	return cmd
}

func runStatus(opts *StatusOptions, prefixes []string) error {
	o, err := openOrchard()
	if err != nil {
		return err
	}
	subtrees, err := o.Config.Select(prefixes)
	if err != nil {
		return err
	}

	// Print each line as it's ready, since each one can take a while, so
	// align by hand rather than with a tabwriter.
	width := 0
	for _, s := range subtrees {
		width = max(width, len(s.Prefix))
	}
	failed := 0
	for _, s := range subtrees {
		st, err := o.Status(s, opts.Rev)
		if err != nil {
			failed++
			fmt.Printf("%-*s  error: %v\n", width, s.Prefix, err)
		} else {
			fmt.Printf("%-*s  %s\n", width, s.Prefix, st)
		}
	}
	if failed > 0 {
		return fmt.Errorf("failed to compare %d of %d subtree(s)", failed, len(subtrees))
	}
	return nil
}
