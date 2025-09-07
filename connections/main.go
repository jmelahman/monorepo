package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/jmelahman/connections/game"

	"github.com/rivo/tview"
)

func main() {
	screen, err := tcell.NewScreen()
	if err != nil {
		panic(err)
	}

	app := tview.NewApplication()
	if err := game.Run(app, screen); err != nil {
		panic(err)
	}
}
