package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	var major, minor, push bool

	rootCmd := &cobra.Command{
		Use:     "tag",
		Short:   "Calculate the next semantic version tag",
		Version: fmt.Sprintf("%s\ncommit %s", version, commit),
		Run: func(cmd *cobra.Command, args []string) {
			latestTag, err := getLatestSemverTag()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}

			nextVersion, err := calculateNextVersion(latestTag, major, minor)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Next version: %s\n", nextVersion)

			if push {
				if err := createAndPushTag(nextVersion); err != nil {
					fmt.Printf("Error: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Tag %s created and pushed to remote.\n", nextVersion)
			}
		},
	}

	rootCmd.Flags().BoolVar(&major, "major", false, "increment the major version")
	rootCmd.Flags().BoolVar(&minor, "minor", false, "increment the minor version")
	rootCmd.Flags().BoolVar(&push, "push", false, "create and push the tag to remote")

	rootCmd.AddCommand(addCompletionCmd(rootCmd))

	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
