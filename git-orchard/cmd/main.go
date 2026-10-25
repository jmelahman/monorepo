package cmd

import (
	"errors"
	"fmt"
	"io"
)

// Main runs the command line and returns the process's exit status: 0 on
// success, 1 when sync found files out of date (like a formatter) or ci found
// failed runs (either has already said which), 2 for failures, and 130 when
// interrupted.
func Main(args []string, stdout, stderr io.Writer) int {
	root := NewRootCommand()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.Execute(); err != nil {
		if errors.Is(err, ErrInterrupted) {
			return 130 // as a shell reports SIGINT
		}
		if errors.Is(err, ErrOutOfDate) || errors.Is(err, ErrCIFailed) {
			return 1
		}
		_, _ = fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}
	return 0
}
