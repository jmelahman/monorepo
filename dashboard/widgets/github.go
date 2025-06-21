package widgets

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/google/go-github/v57/github"
	"github.com/jmelahman/monorepo/dashboard/config"
	"github.com/rivo/tview"
	"golang.org/x/oauth2"
)

type GitHubClient struct {
	client *github.Client
}

func NewGitHubClient() (*GitHubClient, error) {
	var token string

	// First try environment variable
	token = os.Getenv("GITHUB_TOKEN")

	// If not found, try gh CLI
	if token == "" {
		cmd := exec.Command("gh", "auth", "token")
		output, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("failed to get GitHub auth token from gh CLI or GITHUB_TOKEN env var: %v", err)
		}
		token = strings.TrimSpace(string(output))
	}

	if token == "" {
		return nil, fmt.Errorf("empty GitHub auth token")
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	client := github.NewClient(tc)

	return &GitHubClient{client: client}, nil
}

func (c *GitHubClient) GetOpenPullRequests(repo string) (int, error) {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid repository format: %s (expected owner/repo)", repo)
	}

	owner, repoName := parts[0], parts[1]

	prs, _, err := c.client.PullRequests.List(context.Background(), owner, repoName, &github.PullRequestListOptions{
		State: "open",
	})
	if err != nil {
		return 0, fmt.Errorf("failed to fetch pull requests for %s: %v", repo, err)
	}

	return len(prs), nil
}

func NewGitHubPRWidget() *tview.TextView {
	widget := tview.NewTextView().
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
