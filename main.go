package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type Response struct {
	Status     string     `json:"status"`
	ID         int        `json:"id"`
	PrintDate  string     `json:"print_date"`
	Editor     string     `json:"editor"`
	Categories []Category `json:"categories"`
}

type Category struct {
	Title string `json:"title"`
	Cards []Card `json:"cards"`
}

type Card struct {
	Content  string `json:"content"`
	Position int    `json:"position"`
}

func fetch(urlString string) ([]byte, error) {
	resp, err := http.Get(urlString)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}

func getConnectionsJSON(date time.Time) ([]byte, error) {
	jsonFilename := fmt.Sprintf("%s.json", date.Format("2006-01-02"))

	dataUrl, err := url.JoinPath("https://www.nytimes.com/svc/connections/v2/", jsonFilename)
	if err != nil {
		return nil, fmt.Errorf("failed to join URL: %w", err)
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = filepath.Join(os.Getenv("HOME"), ".cache")
	}

	connectionsCache := filepath.Join(cacheDir, "connections")
	if err := os.MkdirAll(connectionsCache, 0755); err != nil {
		connectionsData, err := fetch(dataUrl)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch data: %w", err)
		}

		return connectionsData, nil
	}

	cacheFile := filepath.Join(connectionsCache, jsonFilename)
	cachedData, err := os.ReadFile(cacheFile)
	if err != nil {
		connectionsData, err := fetch(dataUrl)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch data: %w", err)
		}

		err = os.WriteFile(cacheFile, connectionsData, 0644)
		if err != nil {
			log.Printf("failed to save to cache: %v", err)
		}
		return connectionsData, nil
	}
	return cachedData, nil
}

func parseConnectionsJSON(data []byte) (Response, error) {
	var response Response

	err := json.Unmarshal([]byte(data), &response)
	if err != nil {
		return response, fmt.Errorf("error parsing JSON: %w", err)
	}
	return response, nil
}

func main() {
	app := tview.NewApplication()

	today := time.Now()
	connectionsData, err := getConnectionsJSON(today)
	if err != nil {
		log.Fatal(err)
	}

	response, err := parseConnectionsJSON(connectionsData)

	grid := tview.NewGrid().
		SetRows(3, 3, 3, 3).
		SetColumns(15, 15, 15, 15)

	buttons := [4][4]*tview.Button{}
	var focusedRow, focusedCol int

	for row := 0; row < 4; row++ {
		category := response.Categories[row]
		for col := 0; col < 4; col++ {
			card := category.Cards[col]
			label := cases.Title(language.English).String(card.Content)
			gRow := card.Position / 4
			gCol := card.Position % 4

			button := tview.NewButton(label).
				SetSelectedFunc(func(r, c int) func() {
					return func() {
						fmt.Printf("Clicked: %s\n", buttons[r][c].GetLabel())
						focusedCol = r
						focusedRow = c
					}
				}(gRow, gCol))

			buttons[gRow][gCol] = button
			grid.AddItem(button, gRow, gCol, 1, 1, 0, 0, false)
		}
	}

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

			fmt.Printf("Clicked: %s\n", buttons[focusedRow][focusedCol].GetLabel())
		}

		app.SetFocus(buttons[focusedRow][focusedCol])
		return nil
	})

	app.SetFocus(buttons[0][0])

	if err := app.SetRoot(grid, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}
