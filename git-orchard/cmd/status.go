package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/jmelahman/git-orchard/orchard"
)

// StatusOptions holds options for the status command
type StatusOptions struct {
	Rev  string
	Jobs int
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
	cmd.Flags().IntVarP(&opts.Jobs, "jobs", "j", runtime.NumCPU(), "subtrees to compare at once")

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

	// Compare subtrees in parallel, but print in manifest order, each line as
	// soon as it and those above it are ready, since each can take a while;
	// so align by hand rather than with a tabwriter.
	width := 0
	for _, s := range subtrees {
		width = max(width, len(s.Prefix))
	}
	type result struct {
		status orchard.Status
		err    error
	}
	results := make([]result, len(subtrees))
	done := make([]chan struct{}, len(subtrees))
	for i := range done {
		done[i] = make(chan struct{})
	}
	go orchard.Each(len(subtrees), opts.Jobs, func(i int) {
		results[i].status, results[i].err = o.Status(subtrees[i], opts.Rev)
		close(done[i])
	})
	failed := 0
	for i, s := range subtrees {
		<-done[i]
		if err := results[i].err; err != nil {
			failed++
			fmt.Printf("%-*s  error: %v\n", width, s.Prefix, err)
		} else {
			fmt.Printf("%-*s  %s\n", width, s.Prefix, results[i].status)
		}
	}
	if failed > 0 {
		return fmt.Errorf("failed to compare %d of %d subtree(s)", failed, len(subtrees))
	}
	return nil
}
