// Command go-pre-commit-hooks gives `go install ./...` something to install.
// pre-commit runs that command with GOPATH set to the new hook environment and
// relies on it creating the directory — a module with no main packages leaves
// the environment missing and installation crashes.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "go-pre-commit-hooks is a pre-commit hook repository, not a standalone tool.")
	fmt.Fprintln(os.Stderr, "See https://github.com/jmelahman/go-pre-commit-hooks for usage.")
	os.Exit(1)
}
