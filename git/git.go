package git

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

func GetLatestSemverTag() (string, error) {
	// Fetch all tags matching semver pattern
	cmd := exec.Command("git", "tag", "-l", "v[0-9]*.[0-9]*.[0-9]*")
	output, err := cmd.Output()
	if err != nil {
		return "v0.0.0", nil
	}

	// Split tags and trim whitespace
	tagList := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(tagList) == 0 {
		return "v0.0.0", nil
	}

	var latestTag string
	var latestVersion *semver.Version

	for _, tag := range tagList {
		version, err := semver.ParseSemver(tag)
		if err != nil {
			continue
		}

		if latestVersion == nil || semver.CompareSemver(version, latestVersion) {
			latestTag = tag
			latestVersion = version
		}
	}

	if latestTag == "" {
		return "v0.0.0", nil
	}

	return latestTag, nil
}

func CreateAndPushTag(tag string) error {
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

func FetchSemverTags() error {
	cmd := exec.Command("git", "fetch", "--prune", "origin", "refs/tags/v*:refs/tags/v*")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to fetch tags: %w", err)
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
