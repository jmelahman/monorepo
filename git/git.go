package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func GetLatestSemverTag() (string, error) {
	// Use git describe to get the most recent tag
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0")
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		return "v0.0.0", nil
	}

	return strings.TrimSpace(string(output)), nil
}

// List all git tags
func ListTags() ([]string, error) {
	cmd := exec.Command("git", "tag", "-l", "v[0-9]*.[0-9]*.[0-9]*")
	cmd.Stderr = os.Stderr
	tagsOutput, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	tagList := strings.Split(strings.TrimSpace(string(tagsOutput)), "\n")

	return tagList, nil
}

func CreateAndPushTag(tag string, remote string) error {
	cmd := exec.Command("git", "tag", tag)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create tag: %w", err)
	}

	cmd = exec.Command("git", "push", "--quiet", remote, tag)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to push tag to %s: %w", remote, err)
	}

	return nil
}

func FetchSemverTags(remote string) error {
	cmd := exec.Command("git", "fetch", "--quiet", "--prune", remote, "refs/tags/v*:refs/tags/v*")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to fetch tags from %s: %w", remote, err)
	}
	return nil
}

func IsHEADAlreadyTagged() (bool, error) {
	cmd := exec.Command("git", "tag", "--points-at", "HEAD")
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check tags for HEAD: %w", err)
	}
	return len(strings.TrimSpace(string(output))) > 0, nil
}
