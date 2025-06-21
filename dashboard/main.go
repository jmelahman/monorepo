package main

import (
	"github.com/jmelahman/monorepo/dashboard/widgets"
	"github.com/rivo/tview"
)

func main() {
	app := tview.NewApplication()

	// Create the credits widget
	creditsWidget := widgets.NewCreditsWidget()
	dockerWidget := widgets.NewDockerWidget()

	// Wrap the widget in a flex layout to center it
	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(creditsWidget, 30, 1, false).
		AddItem(dockerWidget, 31, 1, false).
		AddItem(nil, 0, 1, false)

	if err := app.SetRoot(flex, true).Run(); err != nil {
		panic(err)
	}
}
