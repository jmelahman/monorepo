package main

import (
	"os"

	"github.com/jmelahman/git-orchard/cmd"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	// Set the version in the cmd package
	cmd.Version = version
	cmd.Commit = commit

	os.Exit(cmd.Main(os.Args[1:], os.Stdout, os.Stderr))
}
