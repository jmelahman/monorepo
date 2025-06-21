package main

import (
	"flag"

	"github.com/jmelahman/docker-status/api"
	"github.com/rivo/tview"
)

func main() {
	url := flag.String("url", "http://localhost:9090", "URL to fetch container status from")
	flag.Parse()

	status, err := api.GetDockerStatus(url)
	if err != nil {
		panic(err)
	}

	app := tview.NewApplication()
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true)
	textView.SetTitle("Status").SetBorder(true).SetBorderPadding(0, 0, 1, 1)

	textView.SetText(status)

	if err := app.SetRoot(textView, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}
