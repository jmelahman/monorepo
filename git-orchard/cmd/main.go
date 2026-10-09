package cmd

import (
	"errors"
	"fmt"
	"io"
)

// Main runs the command line and returns the process's exit status: 0 on
// success, 1 when sync found files out of date (like a formatter; it has
// already said which), 2 for failures.
func Main(args []string, stdout, stderr io.Writer) int {
	root := NewRootCommand()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.Execute(); err != nil {
		if errors.Is(err, ErrOutOfDate) {
			return 1
		}
		_, _ = fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}
	return 0
}
