// Command pinsync keeps every copy of a pinned version — pre-commit hook
// revs and additional_dependencies, GitHub Action inputs — in step with the
// one place that defines it. See README.md.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/jmelahman/pinsync/internal/config"
	"github.com/jmelahman/pinsync/internal/pins"
	"github.com/jmelahman/pinsync/internal/remote"
	"github.com/jmelahman/pinsync/internal/version"
)

// Named after whatever the binary is actually called, so usage text and
// error messages stay right if it is renamed.
var programName = filepath.Base(os.Args[0])

var (
	// errOutOfDate is returned by check when a copy differs from its pin.
	errOutOfDate = errors.New("pins out of date; run pinsync sync")
	// errUpdated is returned by sync when it rewrote a copy, so a hook
	// fails like a formatter does.
	errUpdated = errors.New("updated pins")
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, programName+":", err)
		os.Exit(1)
	}
}

// run holds everything main does apart from process plumbing, which is what
// makes it testable — see main_test.go.
func run(args []string, stdout, stderr io.Writer) error {
	root := newRootCmd()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return root.ExecuteContext(ctx)
}

func newRootCmd() *cobra.Command {
	var dir string
	root := &cobra.Command{
		Use:     programName,
		Short:   "Keep every copy of a pinned version in step with its source",
		Version: version.Version(),
		// main reports errors itself, and a failed check is not a usage
		// mistake.
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.PersistentFlags().StringVarP(&dir, "directory", "C", ".", "run as if started in this directory")
	root.AddCommand(newSyncCmd(&dir), newCheckCmd(&dir))
	return root
}

func newCheckCmd(dir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Fail if any copy differs from its pin",
		Long: `Check compares every copy with its pin's source and lists the ones that
differ. It reads a hook rev's version from its # frozen: comment, so it needs
no network.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, copies, err := find(cmd.Context(), *dir)
			if err != nil {
				return err
			}
			stale := false
			for _, c := range copies {
				if c.Stale() {
					stale = true
					fmt.Fprintf(cmd.OutOrStdout(), "%s:%d: %s %s, want %s\n", c.File, c.Line, c.Pin, c.Have, c.Want)
				}
			}
			if stale {
				return errOutOfDate
			}
			return nil
		},
	}
}

func newSyncCmd(dir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Rewrite every copy that differs from its pin",
		Long: `Sync rewrites every copy that differs from its pin's source, and exits 1
when it changed anything, like a formatter. A hook rev pinned to a commit
gets the commit of the new tag, from git ls-remote, and its # frozen: comment
the tag.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, copies, err := find(cmd.Context(), *dir)
			if err != nil {
				return err
			}
			if err := pins.Apply(cmd.Context(), root, copies, remote.Commit); err != nil {
				return err
			}
			updated := false
			for _, c := range copies {
				if c.Stale() {
					updated = true
					fmt.Fprintf(cmd.OutOrStdout(), "%s:%d: %s %s → %s\n", c.File, c.Line, c.Pin, c.Have, c.Want)
				}
			}
			if updated {
				return errUpdated
			}
			return nil
		},
	}
}

// find returns the root of the git repository holding dir, and every copy
// of its pins in the files git tracks.
func find(ctx context.Context, dir string) (string, []pins.Copy, error) {
	out, err := git(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", nil, err
	}
	root := strings.TrimSpace(out)
	ps, err := config.Load(root)
	if err != nil {
		return "", nil, err
	}
	out, err = git(ctx, root, "ls-files", "-z")
	if err != nil {
		return "", nil, err
	}
	files := strings.Split(strings.TrimSuffix(out, "\x00"), "\x00")
	copies, err := pins.Find(root, files, ps)
	if err != nil {
		return "", nil, err
	}
	return root, copies, nil
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}
