package git

import (
	"fmt"
	"os/exec"
	"strings"
)

func GetLatestSemverTag() (string, error) {
	// Use git describe to get the most recent tag
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0")
	output, err := cmd.Output()
	if err != nil {
		return "v0.0.0", nil
	}

	return strings.TrimSpace(string(output)), nil

	// Fetch all tags for additional context
	//tagsCmd := exec.Command("git", "tag", "-l", "v[0-9]*.[0-9]*.[0-9]*", "--sort=-v:refname")
	//tagsOutput, err := tagsCmd.Output()
	//if err != nil {
	//	return latestTag, nil, nil
	//}

	//tagList := strings.Split(strings.TrimSpace(string(tagsOutput)), "\n")

	//return latestTag, tagList, nil
}

func CreateAndPushTag(tag string, remote string) error {
	cmd := exec.Command("git", "tag", tag)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create tag: %w", err)
	}

	cmd = exec.Command("git", "push", remote, tag)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to push tag to %s: %w", remote, err)
	}

	return nil
}

func FetchSemverTags(remote string) error {
	cmd := exec.Command("git", "fetch", "--prune", remote, "refs/tags/v*:refs/tags/v*")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to fetch tags from %s: %w", remote, err)
	}
	return nil
}

func IsHEADAlreadyTagged() (bool, error) {
	cmd := exec.Command("git", "tag", "--points-at", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check tags for HEAD: %w", err)
	}
	return len(strings.TrimSpace(string(output))) > 0, nil
}
