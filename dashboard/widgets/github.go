package widgets

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/jmelahman/monorepo/dashboard/config"
	"github.com/rivo/tview"
)

type GitHubClient struct {
	authToken string
}

type PullRequest struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
}

func NewGitHubClient() (*GitHubClient, error) {
	// Try to get auth token from gh CLI
	cmd := exec.Command("gh", "auth", "token")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get GitHub auth token from gh CLI: %v", err)
	}

	token := strings.TrimSpace(string(output))
	if token == "" {
		return nil, fmt.Errorf("empty GitHub auth token")
	}

	return &GitHubClient{authToken: token}, nil
}

func (c *GitHubClient) GetOpenPullRequests(repo string) (int, error) {
	cmd := exec.Command("gh", "pr", "list", "--repo", repo, "--state", "open", "--json", "number,title,state")
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to fetch pull requests for %s: %v", repo, err)
	}

	var prs []PullRequest
	err = json.Unmarshal(output, &prs)
	if err != nil {
		return 0, fmt.Errorf("failed to parse pull requests JSON: %v", err)
	}

	return len(prs), nil
}

func NewGitHubPRWidget() *tview.TextView {
	widget := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)

	widget.SetBorder(true).SetBorderColor(tcell.ColorGray).SetTitle("GitHub PRs")

	// Initial load
	RefreshGitHubPRWidget(widget)

	return widget
}

func RefreshGitHubPRWidget(widget *tview.TextView) {
	// Load config
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		widget.SetText(fmt.Sprintf("[red]Config error: %v", err))
		return
	}

	// Create GitHub client
	client, err := NewGitHubClient()
	if err != nil {
		widget.SetText(fmt.Sprintf("[red]GitHub auth error: %v", err))
		return
	}

	// Fetch PR counts for all repositories
	var totalPRs int
	var repoDetails []string

	for _, repo := range cfg.GitHub.Repositories {
		count, err := client.GetOpenPullRequests(repo)
		if err != nil {
			widget.SetText(fmt.Sprintf("[red]Error fetching PRs: %v", err))
			return
		}
		totalPRs += count
		repoDetails = append(repoDetails, fmt.Sprintf("%s: %d", repo, count))
	}

	// Display results
	if len(cfg.GitHub.Repositories) == 1 {
		widget.SetText(fmt.Sprintf("%d open PRs", totalPRs))
	} else {
		details := strings.Join(repoDetails, "\n")
		widget.SetText(fmt.Sprintf("Total: %d\n\n%s", totalPRs, details))
	}
}
