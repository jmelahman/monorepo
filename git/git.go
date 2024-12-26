package git

import (
	"fmt"
	"os/exec"
	"strings"
)

func GetLatestSemverTag() (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0", "--match", "v[0-9].[0-9].[0-9]")
	output, err := cmd.Output()
	if err != nil {
		return "v0.0.0", nil
	}
	return strings.TrimSpace(string(output)), nil
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
