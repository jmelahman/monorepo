package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/jmelahman/jview"
	"github.com/rivo/tview"
)

func main() {
	app := tview.NewApplication()

	preview := tview.NewBox().
		SetBorder(true).
		SetTitle("Preview")

	// Create color grid
	colorGrid := jview.NewColorGrid(5, 10).SetLabel("Color").
		SetChangedFunc(func(idx int, color tcell.Color) {
			preview.SetBorderColor(color)
		})

	form := tview.NewForm().
		AddFormItem(colorGrid).
		AddButton("Quit", func() { app.Stop() })

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(preview, 3, 1, false). // height 3 lines
		AddItem(form, 10, 1, true)     // height 10 lines

	if err := app.SetRoot(flex, true).SetFocus(colorGrid).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}
