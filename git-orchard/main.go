package main

import (
	"fmt"
	"os"

	"github.com/jmelahman/git-orchard/cmd"
)

var (
	version = "dev"
)

func main() {
	// Set the version in the cmd package
	cmd.Version = version

	rootCmd := cmd.NewRootCommand()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}
}
