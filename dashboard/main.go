package main

import (
	"fmt"
	"github.com/rivo/tview"
)

func main() {
	app := tview.NewApplication()

	// Integer to display
	counter := 42

	// Create a primitive widget (Box) to show the integer
	box := tview.NewTextView().
		SetText(fmt.Sprintf("Counter: %d", counter)).
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)

	// Wrap the box in a flex layout to center it
	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(box, 30, 1, false).
		AddItem(nil, 0, 1, false)

	if err := app.SetRoot(flex, true).Run(); err != nil {
		panic(err)
	}
}
