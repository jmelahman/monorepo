package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func main() {
	var major, minor, push bool

	rootCmd := &cobra.Command{
		Use:   "tag",
		Short: "Calculate the next semantic version tag",
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

	rootCmd.Flags().BoolVarP(&major, "major", "M", false, "Increment the major version")
	rootCmd.Flags().BoolVarP(&minor, "minor", "m", false, "Increment the minor version")
	rootCmd.Flags().BoolVar(&push, "push", false, "Create and push the tag to remote")
	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func getLatestSemverTag() (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0", "--match", "v[0-9].[0-9].[0-9]")
	output, err := cmd.Output()
	if err != nil {
		return "v0.0.0", nil
	}
	return strings.TrimSpace(string(output)), nil
}

func calculateNextVersion(tag string, incMajor, incMinor bool) (string, error) {
	re := regexp.MustCompile(`v(\d+)\.(\d+)\.(\d+)`)
	matches := re.FindStringSubmatch(tag)
	if matches == nil {
		return "", fmt.Errorf("invalid semver tag: %s", tag)
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	if incMajor {
		major++
		minor = 0
		patch = 0
	} else if incMinor {
		minor++
		patch = 0
	} else {
		patch++
	}

	return fmt.Sprintf("v%d.%d.%d", major, minor, patch), nil
}

func createAndPushTag(tag string) error {
	cmd := exec.Command("git", "tag", tag)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create tag: %w", err)
	}

	cmd = exec.Command("git", "push", "origin", tag)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to push tag: %w", err)
	}

	return nil
}
