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

var suffix string

func main() {
	var major, minor, patch, push bool

	rootCmd := &cobra.Command{
		Use:     "tag",
		Short:   "Calculate the next semantic version tag",
		Version: fmt.Sprintf("%s\ncommit %s", version, commit),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Validate that only one version increment flag is set
			incrementFlags := []bool{major, minor, patch}
			var setFlags int
			for _, flag := range incrementFlags {
				if flag {
					setFlags++
				}
			}
			if setFlags > 1 {
				return fmt.Errorf("only one version increment flag (--major, --minor, or --patch) can be used at a time")
			}
			return nil
		},
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

			nextVersion, err := semver.CalculateNextVersion(latestTag, major, minor, patch, suffix)
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
	rootCmd.Flags().BoolVar(&patch, "patch", false, "increment the patch version")
	rootCmd.Flags().BoolVar(&push, "push", false, "create and push the tag to remote")
	rootCmd.Flags().StringVar(&suffix, "suffix", "", "set the pre-release suffix (e.g., rc, alpha, beta)")

	rootCmd.AddCommand(completion.AddCompletionCmd(rootCmd))

	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
