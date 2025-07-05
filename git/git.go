package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jmelahman/tag/semver"
)

func genTagPattern(prefix string) string {
	tagPattern := "v[0-9]*.[0-9]*.[0-9]*"
	if prefix != "" {
		tagPattern = fmt.Sprintf("%s/%s", prefix, tagPattern)
	}
	return tagPattern
}

func GetLatestSemverTag(prefix string) (string, error) {
	tagPattern := genTagPattern(prefix)
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0", "--match", tagPattern)
	output, err := cmd.Output()
	if err != nil {
		if prefix == "" {
			return "v0.0.0", nil
		} else {
			return fmt.Sprintf("%s/v0.0.0", prefix), nil
		}
	}

	tagsAt, err := ListTagsAt(strings.TrimSpace(string(output)))
	if err != nil {
		if prefix == "" {
			return "v0.0.0", nil
		} else {
			return fmt.Sprintf("%s/v0.0.0", prefix), nil
		}
	}

	var largestTag string
	var largestVersion *semver.Version

	for _, tag := range tagsAt {
		if largestTag == "" {
			largestTag = tag
		}

		version, err := semver.ParseSemver(tag)
		if err != nil {
			continue
		}

		if fmt.Sprintf("%s/", prefix) != version.Prefix {
			continue
		}

		if largestVersion == nil {
			largestTag = tag
			largestVersion = version
			continue
		}

		if semver.CompareSemver(version, largestVersion) {
			largestTag = tag
			largestVersion = version
		}
	}

	return largestTag, nil
}

// List all git tags
func ListTags(prefix string) ([]string, error) {
	tagPattern := genTagPattern(prefix)
	cmd := exec.Command("git", "tag", "-l", tagPattern)
	cmd.Stderr = os.Stderr
	tagsOutput, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	tagList := strings.Split(strings.TrimSpace(string(tagsOutput)), "\n")

	return tagList, nil
}

func ListTagsAt(ref string) ([]string, error) {
	cmd := exec.Command("git", "tag", "--points-at", ref)
	cmd.Stderr = os.Stderr
	tagsOutput, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	tagList := strings.Split(strings.TrimSpace(string(tagsOutput)), "\n")

	return tagList, nil
}

func TagExists(tag string) (bool, error) {
	tagRef := fmt.Sprintf("refs/tags/%s", tag)
	cmd := exec.Command("git", "show-ref", "--tags", "--quiet", tagRef)
	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			return false, nil
		}
		return false, err
	}
	return true, nil
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

func FetchSemverTags(remote string, prefix string) error {
	tagPattern := "refs/tags/v*:refs/tags/v*"
	if prefix != "" {
		tagPattern = fmt.Sprintf("refs/tags/%s/%s:refs/tags/%s/%s", prefix, "v*", prefix, "v*")
	}
	cmd := exec.Command("git", "fetch", "--quiet", "--prune", remote, tagPattern)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to fetch tags from %s: %w", remote, err)
	}
	return nil
}

func IsHEADAlreadyTagged(prefix string) (bool, error) {
	tagPattern := genTagPattern(prefix)
	cmd := exec.Command("git", "tag", "--points-at", "HEAD", "--list", tagPattern)
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check tags for HEAD: %w", err)
	}
	return len(strings.TrimSpace(string(output))) > 0, nil
}
