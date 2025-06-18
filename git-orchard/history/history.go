package history

import (
	"os/exec"
	"regexp"
	"strings"
)

// SubtreeHistoryInfo represents subtree information from git history
type SubtreeHistoryInfo struct {
	Prefix      string
	LastCommit  string
	LastMessage string
}

// Reader interface for reading git history (useful for testing)
type Reader interface {
	GetSubtreesFromHistory() (map[string]SubtreeHistoryInfo, error)
}

// GitHistoryReader reads subtree information from git history
type GitHistoryReader struct{}

// NewGitHistoryReader creates a new GitHistoryReader
func NewGitHistoryReader() *GitHistoryReader {
	return &GitHistoryReader{}
}

// GetSubtreesFromHistory extracts subtree information from git log
func (r *GitHistoryReader) GetSubtreesFromHistory() (map[string]SubtreeHistoryInfo, error) {
	// Execute git log to find subtree merge commits
	cmd := exec.Command("git", "log", "--grep=git-subtree-dir:", "--pretty=format:%H %s", "--all")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	subtreeMap := make(map[string]SubtreeHistoryInfo)

	if len(output) == 0 {
		return subtreeMap, nil
	}

	// Parse the output to extract subtree information
	lines := strings.Split(string(output), "\n")

	// Regex to match git-subtree-dir and git-subtree-mainline in commit messages
	dirRegex := regexp.MustCompile(`git-subtree-dir:\s*(\S+)`)
	mainlineRegex := regexp.MustCompile(`git-subtree-mainline:\s*(\S+)`)

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}

		commit := parts[0]
		message := parts[1]

		// Extract subtree directory
		dirMatches := dirRegex.FindStringSubmatch(message)
		if len(dirMatches) < 2 {
			continue
		}

		prefix := dirMatches[1]

		// Check if this is a merge commit (has git-subtree-mainline)
		isMainline := mainlineRegex.MatchString(message)

		if info, exists := subtreeMap[prefix]; exists {
			// Update with more recent commit info if this is a mainline merge
			if isMainline {
				info.LastCommit = commit
				info.LastMessage = message
				subtreeMap[prefix] = info
			}
		} else {
			subtreeMap[prefix] = SubtreeHistoryInfo{
				Prefix:      prefix,
				LastCommit:  commit,
				LastMessage: message,
			}
		}
	}

	return subtreeMap, nil
}
