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

	// Wrap the widget in a flex layout to center it
	flex := tview.NewFlex().
		AddItem(dockerWidget, 30, 1, false).
		AddItem(creditsWidget, 20, 1, false).
		AddItem(nil, 0, 1, false)

	// Start auto-refresh goroutine
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			app.QueueUpdateDraw(func() {
				widgets.RefreshDockerWidget(dockerWidget)
				widgets.RefreshCreditsWidget(creditsWidget)
			})
		}
	}()

	if err := app.SetRoot(flex, true).Run(); err != nil {
		panic(err)
	}
}
