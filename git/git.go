package git

import (
	"fmt"
	"os/exec"
	"strings"
)

func GetLatestSemverTag() (string, error) {
	// This is slightly preferred over `git describe` as this better handles multiple tags on a
	// given commit.
	// This may also be extended in the future to exhaustively parse each tag and calculate the
	// latest semver tag regardless of refname as at the moment they're assumed to be linear.
	// Moreover, in the future this may accept a subset of tags to consider. For example,
	//    $ tag --base-version v1.2
	// which only searches `git tag -l v1.2.[0-9]*`
	cmd := exec.Command("git", "tag", "-l", "v[0-9]*.[0-9]*.[0-9]*", "--sort=-v:refname")
	output, err := cmd.Output()
	if err != nil {
		return "v0.0.0", nil
	}
	tags := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(tags) == 0 {
		return "v0.0.0", nil
	}
	return tags[0], nil
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
