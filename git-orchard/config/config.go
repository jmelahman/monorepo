package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
)

// SubtreeConfig represents a subtree configuration
type SubtreeConfig struct {
	Name       string
	Repository string
	Prefix     string
	Branch     string
}

// OrchardConfig represents git-orchard configuration
type OrchardConfig struct {
	Squash bool
}

// Reader interface for reading configurations (useful for testing)
type Reader interface {
	ReadSubtreeConfigs() ([]SubtreeConfig, OrchardConfig, error)
}

// GitConfigReader reads configuration from git config
type GitConfigReader struct {
	repoPath string
}

// NewGitConfigReader creates a new GitConfigReader
// If repoPath is empty, it will search for the git repository root starting from the current directory
func NewGitConfigReader(repoPath string) *GitConfigReader {
	if repoPath == "" {
		if root, err := findGitRoot(); err == nil {
			repoPath = root
		} else {
			repoPath = "."
		}
	}
	return &GitConfigReader{repoPath: repoPath}
}

// findGitRoot searches for the git repository root starting from the current directory
func findGitRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		gitDir := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the root directory
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("not in a git repository")
}

// ReadSubtreeConfigs reads subtree configurations from git config
func (r *GitConfigReader) ReadSubtreeConfigs() ([]SubtreeConfig, OrchardConfig, error) {
	// Open the repository
	repo, err := git.PlainOpen(r.repoPath)
	if err != nil {
		return nil, OrchardConfig{}, fmt.Errorf("failed to open repository: %w", err)
	}

	// Get the repository config
	cfg, err := repo.Config()
	if err != nil {
		return nil, OrchardConfig{}, fmt.Errorf("failed to read config: %w", err)
	}

	var subtrees []SubtreeConfig
	orchardConfig := OrchardConfig{}

	// Parse the raw config to find subtree sections
	for _, section := range cfg.Raw.Sections {
		if strings.HasPrefix(section.Name, "subtree") {
			// Extract subsection name from section name like "subtree \"name\""
			parts := strings.SplitN(section.Name, " ", 2)
			var subsectionName string
			if len(parts) > 1 {
				// Remove quotes from subsection name
				subsectionName = strings.Trim(parts[1], "\"")
			}

			subtree := SubtreeConfig{
				Name: subsectionName,
			}

			for _, option := range section.Options {
				switch option.Key {
				case "repository":
					subtree.Repository = option.Value
				case "prefix":
					subtree.Prefix = option.Value
				case "branch":
					subtree.Branch = option.Value
				}
			}

			if subtree.Repository != "" && subtree.Prefix != "" {
				subtrees = append(subtrees, subtree)
			}
		} else if section.Name == "orchard" {
			for _, option := range section.Options {
				switch option.Key {
				case "squash":
					orchardConfig.Squash = option.Value == "true"
				}
			}
		}
	}

	return subtrees, orchardConfig, nil
}
