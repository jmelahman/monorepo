package main

import (
	"encoding/json"
	"net/http"

	"github.com/rivo/tview"
)

type ContainerHealth struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

func main() {
	resp, err := http.Get("http://health.home/health")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	var containers []ContainerHealth
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		panic(err)
	}

	app := tview.NewApplication()
	table := tview.NewTable().SetBorders(true)
	table.SetTitle("Status").SetBorder(true)

	// Set headers
	table.SetCell(0, 0, tview.NewTableCell("Container").SetTextColor(tview.Styles.PrimaryTextColor).SetAlign(tview.AlignCenter))
	table.SetCell(0, 1, tview.NewTableCell("Status").SetTextColor(tview.Styles.PrimaryTextColor).SetAlign(tview.AlignCenter))

	// Add container data
	for i, c := range containers {
		table.SetCell(i+1, 0, tview.NewTableCell(c.Name))
		table.SetCell(i+1, 1, tview.NewTableCell(c.Status))
	}

	if err := app.SetRoot(table, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}
