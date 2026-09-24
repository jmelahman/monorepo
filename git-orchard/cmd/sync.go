package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jmelahman/git-orchard/git"
	"github.com/jmelahman/git-orchard/share"
)

// ErrOutOfDate is returned when sync found files that didn't match their
// profiles, whether it rewrote them or, with --check, only reported them.
var ErrOutOfDate = errors.New("shared files were out of date")

// SyncOptions holds options for the sync command
type SyncOptions struct {
	Check bool
}

// NewSyncCommand creates a new sync command
func NewSyncCommand() *cobra.Command {
	opts := &SyncOptions{}

	cmd := &cobra.Command{
		Use:   "sync [prefix...]",
		Short: "Copy shared files into subtrees",
		Long: `Copy shared files into subtrees.

Each subtree (every one listed, unless prefixes are given) gets the files of
the profiles its shared keys name, from directories under orchard.sharedDir
(.config/git-orchard/shared by default):

  [subtree "tools/foo"]
  	remote = git@github.com:owner/foo.git
  	shared = base
  	shared = go

A file in a profile lands at the same path in the subtree. If the subtree's
file has lines marking a block for the profile, "BEGIN orchard:go" and
"END orchard:go" in any comment syntax, only the lines between them are
replaced; otherwise the profile owns the whole file.

Like a formatter, sync exits 1 when it changes anything, so it works as a
pre-commit hook. --check reports the differences without writing them.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSync(opts, args)
		},
	}

	cmd.Flags().BoolVar(&opts.Check, "check", false, "show differences without writing them")

	return cmd
}

func runSync(opts *SyncOptions, prefixes []string) error {
	o, err := openOrchard()
	if err != nil {
		return err
	}
	subtrees, err := o.Config.Select(prefixes)
	if err != nil {
		return err
	}
	changes, err := share.Plan(o.Repo.Dir, o.Config, subtrees)
	if err != nil {
		return err
	}
	if len(changes) == 0 {
		return nil
	}
	if opts.Check {
		for _, c := range changes {
			if err := printDiff(c); err != nil {
				return err
			}
		}
		fmt.Fprintf(os.Stderr, "%d shared file(s) out of date; run git orchard sync\n", len(changes))
		return ErrOutOfDate
	}
	if err := share.Apply(o.Repo.Dir, changes); err != nil {
		return err
	}
	for _, c := range changes {
		fmt.Fprintf(os.Stderr, "Updated %s\n", c.Path)
	}
	return ErrOutOfDate
}

// printDiff shows c as a unified diff on stdout, labeled with its path.
func printDiff(c share.Change) error {
	dir, err := os.MkdirTemp("", "git-orchard-sync-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	old := os.DevNull
	if c.Old != nil {
		old = filepath.Join("a", c.Path)
		if err := writeFile(filepath.Join(dir, old), c.Old, c.OldMode); err != nil {
			return err
		}
	}
	updated := filepath.Join("b", c.Path)
	if err := writeFile(filepath.Join(dir, updated), c.New, c.Mode); err != nil {
		return err
	}
	// Relative to dir, the paths read as a/<path> and b/<path>; git diff
	// exits 1 when they differ, which they do.
	err = git.Repo{Dir: dir}.RunTo(os.Stdout, "--no-pager", "diff", "--no-index", "--no-prefix", "--", old, updated)
	if err != nil && git.ExitCode(err) != 1 {
		return err
	}
	return nil
}

func writeFile(path string, content []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, content, mode); err != nil {
		return err
	}
	// Apply the exact mode, past the umask, so git diff shows mode changes.
	return os.Chmod(path, mode)
}
