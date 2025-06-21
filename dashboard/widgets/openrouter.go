package widgets

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type CreditsResponse struct {
	Data struct {
		Credits float64 `json:"credits"`
	} `json:"data"`
}

func NewCreditsWidget() *tview.TextView {
	widget := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)

	// Fetch and display credits
	credits, err := fetchCredits()
	if err != nil {
		widget.SetText(fmt.Sprintf("[red]Error: %v", err))
	} else {
		widget.SetText(fmt.Sprintf("$%.2f", credits))
	}

	widget.SetBorder(true).SetBorderColor(tcell.ColorGray).SetTitle("OpenRouter Credits")

	return widget
}

func fetchCredits() (float64, error) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return 0, fmt.Errorf("OPENROUTER_API_KEY environment variable not set")
	}

	req, err := http.NewRequest("GET", "https://openrouter.ai/api/v1/auth/key", nil)
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
	defer resp.Body.Close()

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

	return creditsResp.Data.Credits, nil
}
