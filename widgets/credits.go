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

type CreditsWidget struct {
	*BaseWidget
	textView *tview.TextView
}

func NewCreditsWidget() *CreditsWidget {
	textView := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)

	// Create a flex container to hold the text view with padding
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 1, 0, false).
		AddItem(tview.NewFlex().
			AddItem(nil, 1, 0, false).
			AddItem(textView, 0, 1, false).
			AddItem(nil, 1, 0, false), 0, 1, false).
		AddItem(nil, 1, 0, false)

	// Create the base widget with border and title
	box := tview.NewBox().
		SetBorder(true).
		SetBorderColor(tcell.ColorGray).
		SetTitle("Credits").
		SetTitleAlign(tview.AlignCenter)

	// Create a container that combines the box and flex
	container := tview.NewFlex().
		AddItem(box, 0, 1, false)

	// Override the box's Draw method to draw the flex content
	box.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		// Draw the border first
		box.DrawForSubclass(screen, box)
		// Get the inner area after border
		innerX, innerY, innerWidth, innerHeight := box.GetInnerRect()
		// Draw the flex content in the inner area
		flex.SetRect(innerX, innerY, innerWidth, innerHeight)
		flex.Draw(screen)
		return x, y, width, height
	})

	widget := &CreditsWidget{
		BaseWidget: &BaseWidget{Box: box},
		textView:   textView,
	}

	// Initial load
	widget.Refresh()

	return widget
}

func (cw *CreditsWidget) Refresh() error {
	credits, err := fetchCredits()
	if err != nil {
		cw.textView.SetText(fmt.Sprintf("[red]Error: %v", err))
		return err
	} else {
		cw.textView.SetText(fmt.Sprintf("$%.2f", credits))
		return nil
	}
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
