package widgets

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/jmelahman/work/api"
	"github.com/rivo/tview"
)

func NewWorkWidget() *tview.TextView {
	widget := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)

	widget.SetBorder(true).SetBorderColor(tcell.ColorGray).SetTitle("Work")

	// Initial load
	RefreshWorkWidget(widget)

	return widget
}

// RefreshWorkWidget updates the work widget with current task status
func RefreshWorkWidget(widget *tview.TextView) {
	workAPI, err := api.NewWorkAPI("")
	if err != nil {
		widget.SetText(fmt.Sprintf("[red]Error initializing work API: %v", err))
		return
	}

	status, err := workAPI.GetCurrentStatus()
	if err != nil {
		widget.SetText(fmt.Sprintf("[red]Error getting work status: %v", err))
		return
	}

	if !status.HasActiveTask {
		widget.SetText("[yellow]No active task")
		return
	}

	// Format the active task display
	var color string
	switch status.Classification {
	case "Work":
		color = "green"
	case "Chore":
		color = "blue"
	case "Toil":
		color = "orange"
	case "Break":
		color = "purple"
	default:
		color = "white"
	}

	text := fmt.Sprintf("[%s]%s[white]\n%s\n[gray]%s",
		color,
		status.Classification,
		status.Task.Description,
		status.Duration,
	)

	widget.SetText(text)
}
