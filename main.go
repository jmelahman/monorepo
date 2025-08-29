package main

import (
	"github.com/jmelahman/connections/game"

	"github.com/rivo/tview"
)

func main() {
	app := tview.NewApplication()
	if err := game.Run(app); err != nil {
		panic(err)
	}
}
