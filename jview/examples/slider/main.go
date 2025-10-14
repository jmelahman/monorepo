package main

import (
	"github.com/jmelahman/jview"
	"github.com/rivo/tview"
)

func main() {
	app := tview.NewApplication()

	slider1 := jview.NewSlider(0, 20, 10).
		SetLabel("Brightness").
		SetChangedFunc(func(v int) {})

	slider2 := jview.NewSlider(0, 10, 3).
		SetChangedFunc(func(v int) {})

	form := tview.NewForm().
		AddFormItem(slider1).
		AddFormItem(slider2).
		AddButton("Quit", func() { app.Stop() })

	if err := app.SetRoot(form, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}
