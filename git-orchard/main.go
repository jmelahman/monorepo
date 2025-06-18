package main

import (
	"fmt"
	"os"

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

func main() {
	var rootCmd = &cobra.Command{
		Use:   "git-orchard [paths...]",
		Short: "Utilities for managing git subtrees",
		Run:   gitOrchard,
	}

	rootCmd.Flags().BoolVar(&debug, "debug", false, "run in debug mode")

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
