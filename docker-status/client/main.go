package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"text/tabwriter"

	"github.com/rivo/tview"
)

type ContainerHealth struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	State  string `json:"state"`
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
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true)
	textView.SetTitle("Status").SetBorder(true).SetBorderPadding(0, 0, 1, 1)

	// Build the text content with colors using tabwriter
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	
	for _, c := range containers {
		// Determine status text and color
		statusText := c.Status
		if c.Status == "none" {
			statusText = c.State
		}

		var colorTag string
		switch statusText {
		case "healthy":
			colorTag = "green"
		case "running":
			colorTag = "green"
		case "starting":
			colorTag = "yellow"
		case "paused":
			colorTag = "yellow"
		case "unhealthy":
			colorTag = "red"
		case "exited":
			colorTag = "red"
		default:
			colorTag = "white"
		}

		fmt.Fprintf(w, "%s\t[%s]%s[-]\n", c.Name, colorTag, statusText)
	}
	
	w.Flush()
	textView.SetText(buf.String())

	if err := app.SetRoot(textView, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}
