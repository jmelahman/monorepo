package widgets

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/jmelahman/docker-status/api"
	"github.com/rivo/tview"
)

func NewDockerWidget() *tview.TextView {
	widget := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)

	url := "http://health.home"
	status, err := api.GetDockerStatus(&url)
	if err != nil {
		widget.SetText(fmt.Sprintf("[red]Error: %v", err))
	} else {
		widget.SetText(status)
	}

	widget.SetBorder(true).SetBorderColor(tcell.ColorGray).SetTitle("Docker Status")

	return widget
}
