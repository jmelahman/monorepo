package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/jmelahman/tag/completion"
	"github.com/jmelahman/tag/git"
	"github.com/jmelahman/tag/semver"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	var major, minor, patch, push, print, check bool
	var metadata, prefix, suffix, remote string
	var debug, noFetch, allowUntagged bool

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
			if debug {
				log.SetLevel(log.DebugLevel)
			} else {
				log.SetLevel(log.InfoLevel)
			}

			log.WithFields(log.Fields{
				"prefix":        prefix,
				"suffix":        suffix,
				"remote":        remote,
				"major":         major,
				"minor":         minor,
				"patch":         patch,
				"check":         check,
				"noFetch":       noFetch,
				"allowUntagged": allowUntagged,
			}).Debug("Configuration")

			if !noFetch {
				if err := git.FetchSemverTags(remote, prefix, suffix); err != nil {
					fmt.Printf("Error fetching tags: %v\n", err)
					os.Exit(1)
				}
			}

			// Handle --check flag: validate that the tag at HEAD has its previous version as an ancestor
			// and no larger version is an ancestor
			if check {
				currentTag, err := git.GetTagAtHEAD(prefix, suffix)
				if err != nil {
					fmt.Printf("Error getting tag at HEAD: %v\n", err)
					os.Exit(1)
				}
				if currentTag == "" {
					if allowUntagged {
						fmt.Println("HEAD is not tagged (allowed)")
						os.Exit(0)
					}
					fmt.Println("Error: HEAD is not tagged")
					os.Exit(1)
				}

				allTags, err := git.ListTags(prefix, suffix)
				if err != nil {
					fmt.Printf("Error listing tags: %v\n", err)
					os.Exit(1)
				}

				// Check that no larger version is an ancestor of HEAD
				largerTags, err := semver.FindLargerVersions(currentTag, allTags)
				if err != nil {
					fmt.Printf("Error finding larger versions: %v\n", err)
					os.Exit(1)
				}

				for _, largerTag := range largerTags {
					isAncestor, err := git.IsAncestor(largerTag, "HEAD")
					if err != nil {
						fmt.Printf("Error checking ancestry: %v\n", err)
						os.Exit(1)
					}
					if isAncestor {
						fmt.Printf("Error: Larger tag '%s' is an ancestor of current tag '%s'\n", largerTag, currentTag)
						os.Exit(1)
					}
				}

				// Check that the expected predecessor exists (no skipped versions)
				expectedPred, err := semver.GetExpectedPredecessor(currentTag)
				if err != nil {
					fmt.Printf("Error getting expected predecessor: %v\n", err)
					os.Exit(1)
				}

				if expectedPred == "" {
					// v0.0.0 or similar - no predecessor expected
					fmt.Printf("Tag '%s' is valid (first version)\n", currentTag)
					os.Exit(0)
				}

				// Check if expected predecessor exists
				predExists, err := git.TagExists(expectedPred)
				if err != nil {
					fmt.Printf("Error checking tag existence: %v\n", err)
					os.Exit(1)
				}

				if !predExists {
					fmt.Printf("Error: Expected predecessor tag '%s' does not exist (version skipped)\n", expectedPred)
					os.Exit(1)
				}

				// Check that the expected predecessor is an ancestor
				isAncestor, err := git.IsAncestor(expectedPred, "HEAD")
				if err != nil {
					fmt.Printf("Error checking ancestry: %v\n", err)
					os.Exit(1)
				}

				if !isAncestor {
					fmt.Printf("Error: Previous tag '%s' is not an ancestor of current tag '%s'\n", expectedPred, currentTag)
					os.Exit(1)
				}

				fmt.Printf("Tag '%s' is valid (previous version '%s' is an ancestor)\n", currentTag, expectedPred)
				os.Exit(0)
			}

			// Check if HEAD is already tagged
			alreadyTagged, err := git.IsHEADAlreadyTagged(prefix, suffix)
			if err != nil {
				fmt.Printf("Error checking tags: %v\n", err)
				os.Exit(1)
			}
			if alreadyTagged {
				fmt.Println("Error: Current HEAD is already tagged")
				os.Exit(1)
			}

		latestTag, err := git.GetLatestSemverTag(prefix, suffix)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		// When using a suffix, also check the latest stable tag to ensure we
		// don't create a pre-release based on an older version than the latest stable
		if suffix != "" {
			latestStableTag, err := git.GetLatestStableSemverTag(prefix)
			if err == nil && latestStableTag != "" {
				stableVer, stableErr := semver.ParseSemver(latestStableTag)
				currentVer, currentErr := semver.ParseSemver(latestTag)

				if stableErr == nil && currentErr == nil {
					// Compare base versions (major.minor.patch only)
					stableBase := &semver.Version{Major: stableVer.Major, Minor: stableVer.Minor, Patch: stableVer.Patch}
					currentBase := &semver.Version{Major: currentVer.Major, Minor: currentVer.Minor, Patch: currentVer.Patch}

					// Use stable tag if it has a higher or equal base version
					if semver.CompareSemver(stableBase, currentBase) || (stableBase.Major == currentBase.Major && stableBase.Minor == currentBase.Minor && stableBase.Patch == currentBase.Patch) {
						log.WithFields(log.Fields{
							"stableTag":     latestStableTag,
							"preReleaseTag": latestTag,
						}).Debug("Using stable tag as base (higher or equal base version)")
						latestTag = latestStableTag
					}
				}
			}
		}

		allTags, err := git.ListTags(prefix, suffix)
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
	rootCmd.Flags().BoolVar(&check, "check", false, "validate that the tag at HEAD has its previous version as an ancestor")
	rootCmd.Flags().BoolVar(&noFetch, "no-fetch", false, "skip fetching tags from remote")
	rootCmd.Flags().BoolVar(&allowUntagged, "allow-untagged", false, "allow HEAD to be untagged when using --check")
	rootCmd.Flags().BoolVar(&debug, "debug", false, "enable debug logging")
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
