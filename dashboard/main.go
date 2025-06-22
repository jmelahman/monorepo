package main

import (
	"time"

	"github.com/jmelahman/monorepo/dashboard/widgets"
	"github.com/rivo/tview"
)

func main() {
	app := tview.NewApplication()

	// Create the widgets
	creditsWidget := widgets.NewCreditsWidget()
	dockerWidget := widgets.NewDockerWidget()
	githubPRWidget := widgets.NewGitHubPRWidget()
	gitWidget := widgets.NewGitWidget()
	workWidget := widgets.NewWorkWidget()

	// Wrap the widget in a flex layout to center it
	flex := tview.NewFlex().
		AddItem(dockerWidget, 30, 1, false).
		AddItem(creditsWidget, 20, 1, false).
		AddItem(githubPRWidget, 0, 1, false).
		AddItem(gitWidget, 0, 1, false).
		AddItem(workWidget, 0, 1, false).
		AddItem(nil, 0, 1, false)

	// Start auto-refresh goroutine for most widgets (5 seconds)
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			app.QueueUpdateDraw(func() {
				widgets.RefreshDockerWidget(dockerWidget)
				widgets.RefreshCreditsWidget(creditsWidget)
				widgets.RefreshGitWidget(gitWidget)
				widgets.RefreshWorkWidget(workWidget)
			})
		}
	}()

	// Start auto-refresh goroutine for GitHub PR widget (1 minute)
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			app.QueueUpdateDraw(func() {
				widgets.RefreshGitHubPRWidget(githubPRWidget)
			})
		}
	}()

	if err := app.SetRoot(flex, true).EnableMouse(false).Run(); err != nil {
		panic(err)
	}
}
