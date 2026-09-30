package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jmelahman/git-orchard/worktree"
)

// NewDetachCommand creates a new detach command
func NewDetachCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "detach [<worktree>]",
		Short: "Convert a linked worktree into a standalone clone",
		Long: `Convert a linked worktree into a standalone clone.

The worktree made with git worktree add, the current one by default, gets
its own .git directory with all of the repository's refs, config, hooks and
excludes, and keeps its HEAD and index, so uncommitted changes carry over.
It is then removed from the repository's worktrees. Objects are hardlinked
where possible, as for a local git clone.

A worktree that is locked, has submodules, or is in the middle of a merge,
rebase, cherry-pick, revert or bisect is refused.`,
		Example: `  git orchard detach
  git orchard detach ../feature`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			top, err := worktree.Detach(dir)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Detached %s into a standalone clone.\n", top)
			return nil
		},
	}
	return cmd
}
