package widgets

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type CreditsResponse struct {
	Data struct {
		TotalCredits float64 `json:"total_credits"`
		TotalUsage   float64 `json:"total_usage"`
	} `json:"data"`
}

func NewCreditsWidget() *tview.TextView {
	widget := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)

	widget.SetBorder(true).SetBorderColor(tcell.ColorGray).SetTitle("OpenRouter")

	// Initial load
	RefreshCreditsWidget(widget)

	return widget
}

func RefreshCreditsWidget(widget *tview.TextView) {
	// Fetch and display credits
	credits, err := fetchCredits()
	if err != nil {
		widget.SetText(fmt.Sprintf("[red]Error: %v", err))
	} else {
		var color string
		if credits > 10 {
			color = "green"
		} else if credits > 1 {
			color = "yellow"
		} else {
			color = "red"
		}
		widget.SetText(fmt.Sprintf("[%s]$%.2f", color, credits))
	}
}

func fetchCredits() (credits float64, err error) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return 0, fmt.Errorf("OPENROUTER_API_KEY environment variable not set")
	}

	req, err := http.NewRequest("GET", "https://openrouter.ai/api/v1/credits", nil)
	if err != nil {
		return 0, err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() {
		err = errors.Join(err, resp.Body.Close())
	}()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var creditsResp CreditsResponse
	if err := json.Unmarshal(body, &creditsResp); err != nil {
		return 0, err
	}

	return creditsResp.Data.TotalCredits - creditsResp.Data.TotalUsage, nil
}
