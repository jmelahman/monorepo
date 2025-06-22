package widgets

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jmelahman/monorepo/dashboard/config"
	"github.com/jmelahman/monorepo/dashboard/utils"
)

func NewGitWidget() *tview.TextView {
	widget := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)

	widget.SetBorder(true).SetBorderColor(tcell.ColorGreen).SetTitle("Git Status")

	// Initial load
	RefreshGitWidget(widget)

	return widget
}

func RefreshGitWidget(widget *tview.TextView) {
	// Load config
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		widget.SetText(fmt.Sprintf("[red]Config error: %v", err))
		return
	}

	if len(cfg.Git.Repositories) == 0 {
		widget.SetText("[yellow]No git repositories configured")
		return
	}

	data := make(map[string]string)
	keys := make([]string, 0, len(cfg.Git.Repositories))

	for _, repoPath := range cfg.Git.Repositories {
		repoName := filepath.Base(repoPath)
		keys = append(keys, repoName)

		status := getGitStatus(repoPath)
		data[repoName] = status
	}

	formattedOutput := utils.FormatTwoColumnsOrdered(keys, data, ": ")
	widget.SetText(formattedOutput)
}

func getGitStatus(repoPath string) string {
	// Check if directory exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return "[red]Not found"
	}

	// Check if it's a git repository
	gitDir := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return "[red]Not a git repo"
	}

	// Get git status
	cmd := exec.Command("git", "-C", repoPath, "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return "[red]Git error"
	}

	// Count changes
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return "[green]Clean"
	}

	changeCount := len(lines)
	var color string
	if changeCount > 10 {
		color = "red"
	} else if changeCount > 0 {
		color = "yellow"
	} else {
		color = "green"
	}

	return fmt.Sprintf("[%s]%d changes", color, changeCount)
}
