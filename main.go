package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	var major, minor, push bool

	rootCmd := &cobra.Command{
		Use:     "tag",
		Short:   "Calculate the next semantic version tag",
		Version: fmt.Sprintf("%s\ncommit %s", version, commit),
		Run: func(cmd *cobra.Command, args []string) {
			latestTag, err := getLatestSemverTag()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}

			nextVersion, err := calculateNextVersion(latestTag, major, minor)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Next version: %s\n", nextVersion)

			if push {
				if err := createAndPushTag(nextVersion); err != nil {
					fmt.Printf("Error: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Tag %s created and pushed to remote.\n", nextVersion)
			}
		},
	}

	rootCmd.Flags().BoolVar(&major, "major", false, "increment the major version")
	rootCmd.Flags().BoolVar(&minor, "minor", false, "increment the minor version")
	rootCmd.Flags().BoolVar(&push, "push", false, "create and push the tag to remote")

	rootCmd.AddCommand(addCompletionCmd(rootCmd))

	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func addCompletionCmd(cmd *cobra.Command) *cobra.Command {
	var completionCmd = &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate completion script",
		Long: fmt.Sprintf(`To load completions:

Bash:

  $ source <(%[1]s completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ %[1]s completion bash > /etc/bash_completion.d/%[1]s
  # macOS:
  $ %[1]s completion bash > $(brew --prefix)/etc/bash_completion.d/%[1]s

Zsh:

  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ %[1]s completion zsh > "${fpath[1]}/_%[1]s"

  # You will need to start a new shell for this setup to take effect.

fish:

  $ %[1]s completion fish | source

  # To load completions for each session, execute once:
  $ %[1]s completion fish > ~/.config/fish/completions/%[1]s.fish

PowerShell:

  PS> %[1]s completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> %[1]s completion powershell > %[1]s.ps1
  # and source this file from your PowerShell profile.
`, cmd.Root().Name()),
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		Run: func(cmd *cobra.Command, args []string) {
			switch args[0] {
			case "bash":
				cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			}
		},
	}
	return completionCmd
}

func getLatestSemverTag() (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0", "--match", "v[0-9].[0-9].[0-9]")
	output, err := cmd.Output()
	if err != nil {
		return "v0.0.0", nil
	}
	return strings.TrimSpace(string(output)), nil
}

func calculateNextVersion(tag string, incMajor, incMinor bool) (string, error) {
	re := regexp.MustCompile(`v(\d+)\.(\d+)\.(\d+)`)
	matches := re.FindStringSubmatch(tag)
	if matches == nil {
		return "", fmt.Errorf("invalid semver tag: %s", tag)
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	if incMajor {
		major++
		minor = 0
		patch = 0
	} else if incMinor {
		minor++
		patch = 0
	} else {
		patch++
	}

	return fmt.Sprintf("v%d.%d.%d", major, minor, patch), nil
}

func createAndPushTag(tag string) error {
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
