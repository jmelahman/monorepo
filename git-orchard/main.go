package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
)

var debug bool

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

// SubtreeHistoryInfo represents subtree information from git history
type SubtreeHistoryInfo struct {
	Prefix      string
	LastCommit  string
	LastMessage string
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "git-orchard [paths...]",
		Short: "Utilities for managing git subtrees",
		Run:   gitOrchard,
	}

	rootCmd.Flags().BoolVar(&debug, "debug", false, "run in debug mode")

	// Add list subcommand
	var useHistory bool
	var listCmd = &cobra.Command{
		Use:   "list",
		Short: "List all configured subtrees",
		Run: func(cmd *cobra.Command, args []string) {
			if debug {
				log.SetLevel(log.DebugLevel)
			}
			listSubtrees(useHistory)
		},
	}
	listCmd.Flags().BoolVar(&useHistory, "use-history", false, "determine subtrees from git log history instead of config")
	rootCmd.AddCommand(listCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}
}

func gitOrchard(cmd *cobra.Command, args []string) {
	if debug {
		log.SetLevel(log.DebugLevel)
	}

	fmt.Printf("Welcome to git-orchard (%v).\n", version)

	// Read subtree configurations from git config
	subtrees, orchardConfig, err := readSubtreeConfigs()
	if err != nil {
		log.Errorf("Failed to read subtree configs: %v", err)
		return
	}

	log.Debugf("Found %d subtree configurations", len(subtrees))
	for _, subtree := range subtrees {
		log.Debugf("Subtree: %s -> %s (%s)", subtree.Name, subtree.Repository, subtree.Prefix)
		if subtree.Branch != "" {
			log.Debugf("  Branch: %s", subtree.Branch)
		}
	}

	if orchardConfig.Squash {
		log.Debug("Orchard config: squash enabled")
	}
}

func readSubtreeConfigs() ([]SubtreeConfig, OrchardConfig, error) {
	// Open the current repository
	repo, err := git.PlainOpen(".")
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
		if section.Name == "subtree" {
			subtree := SubtreeConfig{
				Name: section.Subsection,
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

func listSubtrees(useHistory bool) {
	if useHistory {
		listSubtreesFromHistory()
	} else {
		listSubtreesFromConfig()
	}
}

func listSubtreesFromConfig() {
	subtrees, _, err := readSubtreeConfigs()
	if err != nil {
		log.Errorf("Failed to read subtree configs: %v", err)
		return
	}

	if len(subtrees) == 0 {
		fmt.Println("No subtrees configured.")
		return
	}

	fmt.Printf("Found %d configured subtree(s):\n\n", len(subtrees))
	for _, subtree := range subtrees {
		fmt.Printf("Name: %s\n", subtree.Name)
		fmt.Printf("  Repository: %s\n", subtree.Repository)
		fmt.Printf("  Prefix: %s\n", subtree.Prefix)
		if subtree.Branch != "" {
			fmt.Printf("  Branch: %s\n", subtree.Branch)
		}
		fmt.Println()
	}
}

func listSubtreesFromHistory() {
	// Execute git log to find subtree merge commits
	cmd := exec.Command("git", "log", "--grep=git-subtree-dir:", "--pretty=format:%H %s", "--all")
	output, err := cmd.Output()
	if err != nil {
		log.Errorf("Failed to execute git log: %v", err)
		return
	}

	if len(output) == 0 {
		fmt.Println("No subtree merges found in git history.")
		return
	}

	// Parse the output to extract subtree information
	lines := strings.Split(string(output), "\n")
	subtreeMap := make(map[string]SubtreeHistoryInfo)
	
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

	if len(subtreeMap) == 0 {
		fmt.Println("No subtree merges found in git history.")
		return
	}

	fmt.Printf("Found %d subtree(s) from git history:\n\n", len(subtreeMap))
	for _, info := range subtreeMap {
		fmt.Printf("Prefix: %s\n", info.Prefix)
		fmt.Printf("  Last commit: %s\n", info.LastCommit)
		fmt.Printf("  Last message: %s\n", info.LastMessage)
		fmt.Println()
	}
}
