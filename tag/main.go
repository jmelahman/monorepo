package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

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
	var major, minor, patch, push, print bool
	var metadata, prefix, suffix, remote string

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
			if err := git.FetchSemverTags(remote, prefix); err != nil {
				fmt.Printf("Error fetching tags: %v\n", err)
				os.Exit(1)
			}

			// Check if HEAD is already tagged
			alreadyTagged, err := git.IsHEADAlreadyTagged(prefix)
			if err != nil {
				fmt.Printf("Error checking tags: %v\n", err)
				os.Exit(1)
			}
			if alreadyTagged {
				fmt.Println("Error: Current HEAD is already tagged")
				os.Exit(1)
			}

			latestTag, err := git.GetLatestSemverTag(prefix)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}

			allTags, err := git.ListTags(prefix)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}

			nextVersion, err := semver.CalculateNextVersion(latestTag, allTags, major, minor, patch, suffix)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}

			if metadata != "" {
				nextVersion = fmt.Sprint(nextVersion, "+", metadata)
			}

			tagExists, err := git.TagExists(nextVersion)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}

			if tagExists {
				fmt.Printf("Next tag '%s' already exists.\n", nextVersion)
				os.Exit(1)
			}

			if print {
				fmt.Println(nextVersion)
				os.Exit(0)
			}

			if !push {
				reader := bufio.NewReader(os.Stdin)
				fmt.Printf("Push tag '%s' to %s? (y/N): ", nextVersion, remote)
				response, _ := reader.ReadString('\n')
				response = strings.TrimSpace(strings.ToLower(response))

				if response == "" || response == "y" || response == "yes" {
					push = true
				}
			}

			if push {
				if err := git.CreateAndPushTag(nextVersion, remote); err != nil {
					fmt.Printf("Error: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Tag '%s' created and pushed to %s.\n", nextVersion, remote)
			}
		},
	}

	rootCmd.Flags().BoolVar(&major, "major", false, "increment the major version")
	rootCmd.Flags().BoolVar(&minor, "minor", false, "increment the minor version")
	rootCmd.Flags().BoolVar(&patch, "patch", false, "increment the patch version")
	rootCmd.Flags().BoolVar(&push, "push", false, "create and push the tag to remote")
	rootCmd.Flags().BoolVar(&print, "print-only", false, "print the next tag and exit")
	rootCmd.Flags().StringVar(&prefix, "prefix", "", "set a prefix for the tag")
	rootCmd.Flags().StringVar(&suffix, "suffix", "", "set the pre-release suffix (e.g., rc, alpha, beta)")
	rootCmd.Flags().StringVar(&metadata, "metadata", "", "set the build metadata")
	rootCmd.Flags().StringVar(&remote, "remote", "origin", "remote repository to push tag to")

	rootCmd.AddCommand(completion.AddCompletionCmd(rootCmd))

	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
