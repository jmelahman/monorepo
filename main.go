package main

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func main() {
	app := tview.NewApplication()

	// Create a 4x4 grid layout to hold the buttons
	grid := tview.NewGrid().
		SetRows(3, 3, 3, 3).       // Button heights
		SetColumns(15, 15, 15, 15) // Button widths

	// Create a 4x4 array of buttons and keep track of them
	buttons := [4][4]*tview.Button{}
	var focusedRow, focusedCol int

	for row := 0; row < 4; row++ {
		for col := 0; col < 4; col++ {
			label := fmt.Sprintf("Button %d", row*4+col+1)
			button := tview.NewButton(label).
				SetSelectedFunc(func(r, c int) func() {
					return func() {
						fmt.Printf("Clicked: Button at [%d, %d]\n", r, c)
					}
				}(row, col))

			buttons[row][col] = button
			grid.AddItem(button, row, col, 1, 1, 0, 0, false)
		}
	}

	// Custom navigation logic
	grid.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp:
			if focusedRow > 0 {
				focusedRow--
			}
		case tcell.KeyDown:
			if focusedRow < 3 {
				focusedRow++
			}
		case tcell.KeyLeft:
			if focusedCol > 0 {
				focusedCol--
			}
		case tcell.KeyRight:
			if focusedCol < 3 {
				focusedCol++
			}
		case tcell.KeyEnter:
			app.SetFocus(buttons[focusedRow][focusedCol])
		}

		app.SetFocus(buttons[focusedRow][focusedCol])
		return nil
	})

	app.SetFocus(buttons[0][0])

	if err := app.SetRoot(grid, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}
