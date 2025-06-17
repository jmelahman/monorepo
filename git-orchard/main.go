package main

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
)

var debug bool

func main() {
	var rootCmd = &cobra.Command{
		Use:   "git-orchard [paths...]",
		Short: "Utilities for managing git subtrees",
		Run:   gitOrchard,
	}

	rootCmd.Flags().BoolVar(&debug, "debug", false, "run in debug mode")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}
}

func gitOrchard(cmd *cobra.Command, args []string) {
	if debug {
		log.SetLevel(log.DebugLevel)
	}

	fmt.Printf("Welcome to git-orchard (%v).\n", version)
}
