// Package remote looks up what a tag names in a remote git repository.
package remote

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrNoTag is returned by Commit for a tag the repository does not have.
var ErrNoTag = errors.New("no such tag")

// Commit returns the commit tag names in the repository at url. git
// ls-remote reports an annotated tag twice, as the tag object and peeled to
// its commit (^{}); a pre-commit rev wants the commit.
func Commit(ctx context.Context, url, tag string) (string, error) {
	ref := "refs/tags/" + tag
	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--tags", url, ref, ref+"^{}")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git ls-remote %s: %w: %s", url, err, strings.TrimSpace(stderr.String()))
	}
	return parse(string(out), ref, url, tag)
}

func parse(out, ref, url, tag string) (string, error) {
	var direct string
	for line := range strings.Lines(out) {
		sha, name, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if !ok {
			continue
		}
		switch name {
		case ref + "^{}":
			return sha, nil
		case ref:
			direct = sha
		}
	}
	if direct == "" {
		return "", fmt.Errorf("%s: %w %s", url, ErrNoTag, tag)
	}
	return direct, nil
}
