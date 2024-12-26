package main

import (
	"fmt"
	"os"

	"github.com/jmelahman/tag/completion"
	"github.com/jmelahman/tag/git"
	"github.com/jmelahman/tag/semver"
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
			if err := git.FetchSemverTags(); err != nil {
				fmt.Printf("Error fetching tags: %v\n", err)
				os.Exit(1)
			}

			// Check if HEAD is already tagged
			alreadyTagged, err := git.IsHEADAlreadyTagged()
			if err != nil {
				fmt.Printf("Error checking tags: %v\n", err)
				os.Exit(1)
			}
			if alreadyTagged {
				fmt.Println("Error: Current HEAD is already tagged")
				os.Exit(1)
			}

			latestTag, err := git.GetLatestSemverTag()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}

			nextVersion, err := semver.CalculateNextVersion(latestTag, major, minor, false)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Next version: %s\n", nextVersion)

			if push {
				if err := git.CreateAndPushTag(nextVersion); err != nil {
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

	rootCmd.AddCommand(completion.AddCompletionCmd(rootCmd))

	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
