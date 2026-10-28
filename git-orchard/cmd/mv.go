package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/jmelahman/git-orchard/config"
)

// MvOptions holds options for the mv command
type MvOptions struct {
	Message  string
	NoVerify bool
}

// NewMvCommand creates a new mv command
func NewMvCommand() *cobra.Command {
	opts := &MvOptions{}

	cmd := &cobra.Command{
		Use:   "mv <prefix> <new-prefix>",
		Short: "Move a subtree to another prefix",
		Long: `Move a subtree to another prefix.

The directory moves, its manifest entry (and any override in git's own
configuration) is renamed, and the move is committed, along with anything
already staged: stage updates to whatever names the old prefix first, so
they land in the same commit.

The commit records the subtree's split for the new prefix, so pushes from
it continue the upstream's history. Clones fetch that split from the
upstream, so the subtree must have no unpushed commits.`,
		Example: `  git orchard mv tools/foo foo
  git add go.work && git orchard mv foo tools/foo`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMv(cmd.ErrOrStderr(), opts, args[0], args[1])
		},
	}

	cmd.Flags().StringVarP(&opts.Message, "message", "m", "", `commit message (default "Move <prefix> to <new-prefix>")`)
	cmd.Flags().BoolVar(&opts.NoVerify, "no-verify", false, "commit without running hooks")

	return cmd
}

func runMv(stderr io.Writer, opts *MvOptions, from, to string) error {
	o, err := openOrchard()
	if err != nil {
		return err
	}
	o.NoVerify = opts.NoVerify
	s, ok := o.Config.Lookup(from)
	if !ok {
		return fmt.Errorf("%s is not a subtree listed in %s", from, o.Config.Manifest)
	}
	if err := o.Move(s, to, opts.Message); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stderr, "Moved %s to %s.\n", s.Prefix, config.Clean(to))
	return nil
}
