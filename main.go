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

type Group struct {
	Title string
	Index int
}

type GameState struct {
	selectedCards map[string]bool
	categories    map[string]Group
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
	jsonFilename = "2024-12-17.json"

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

	gameState := GameState{
		selectedCards: make(map[string]bool),
		categories:    make(map[string]Group),
	}

	today := time.Now()
	connectionsData, err := getConnectionsJSON(today)
	if err != nil {
		log.Fatal(err)
	}

	response, err := parseConnectionsJSON(connectionsData)

	grid := tview.NewGrid().
		SetRows(3, 3, 3, 3, 3). // Added extra row for submit button
		SetColumns(15, 15, 15, 15)

	buttons := [4][4]*tview.Button{}
	var focusedRow, focusedCol int

	baseStyle := tcell.StyleDefault.Background(tcell.ColorDefault).Foreground(tcell.ColorBlack)
	defaultStyle := baseStyle.Foreground(tcell.ColorDefault)
	selectedStyle := baseStyle.Background(tcell.ColorGray)
	activatedStyle := baseStyle.Background(tcell.ColorSilver)
	selectedActivatedStyle := baseStyle.Background(tcell.ColorDarkGray)

	for row := 0; row < 4; row++ {
		category := response.Categories[row]
		for col := 0; col < 4; col++ {
			card := category.Cards[col]
			label := cases.Title(language.English).String(card.Content)
			gRow := card.Position / 4
			gCol := card.Position % 4
			// Hacks enabled
			gRow = row
			gCol = col

			// Map card content to its category
			gameState.categories[label] = Group{category.Title, row}

			button := tview.NewButton(label).
				SetSelectedFunc(func(r, c int) func() {
					return func() {
						label := buttons[r][c].GetLabel()
						if gameState.selectedCards[label] {
							gameState.selectedCards[label] = false
							buttons[r][c].SetStyle(defaultStyle)
						} else if len(gameState.selectedCards) < 4 {
							gameState.selectedCards[label] = true
							buttons[r][c].SetStyle(selectedStyle)
						}
						focusedCol = r
						focusedRow = c
					}
				}(gRow, gCol))

			button.SetStyle(defaultStyle).SetActivatedStyle(activatedStyle)
			buttons[gRow][gCol] = button
			grid.AddItem(button, gRow, gCol, 1, 1, 0, 0, false)
		}
	}

	handleDeselect := func() {
		for cardContent := range gameState.selectedCards {
			delete(gameState.selectedCards, cardContent)
		}
		for i := 0; i < 4; i++ {
			for j := 0; j < 4; j++ {
				buttons[i][j].SetStyle(defaultStyle)
			}
		}
	}

	handleShuffle := func() {
		// TODO: Implement shuffle logic
	}

	handleSubmit := func() {
		if len(gameState.selectedCards) != 4 {
			return
		}

		var categoryTitle string
		var categoryIndex int
		first := true
		allSameCategory := true

		for cardContent := range gameState.selectedCards {
			if first {
				category := gameState.categories[cardContent]
				categoryTitle = category.Title
				categoryIndex = category.Index
				first = false
			} else if gameState.categories[cardContent].Title != categoryTitle {
				allSameCategory = false
				break
			}
		}

		if allSameCategory {
			for cardContent := range gameState.selectedCards {
				for i := 0; i < 4; i++ {
					for j := 0; j < 4; j++ {
						button := buttons[i][j]
						if button.GetLabel() == cardContent {
							button.SetDisabled(true)
							switch categoryIndex {
							case 0:
								button.SetDisabledStyle(baseStyle.Background(tcell.ColorYellow))
							case 1:
								button.SetDisabledStyle(baseStyle.Background(tcell.ColorGreen))
							case 2:
								button.SetDisabledStyle(baseStyle.Background(tcell.ColorBlue))
							case 3:
								button.SetDisabledStyle(baseStyle.Background(tcell.ColorPurple))
							}
						}
					}
				}
			}
			for cardContent := range gameState.selectedCards {
				delete(gameState.selectedCards, cardContent)
			}
		} else {
			fmt.Println("Incorrect! Not all cards belong to the same category")
		}
	}

	shuffleButton := tview.NewButton("Shuffle").
		SetSelectedFunc(handleShuffle).
		SetStyle(defaultStyle).
		SetActivatedStyle(activatedStyle)

	submitButton := tview.NewButton("Submit").
		SetSelectedFunc(handleSubmit).
		SetStyle(defaultStyle).
		SetActivatedStyle(activatedStyle)

	deselectButton := tview.NewButton("Deselect All").
		SetSelectedFunc(handleDeselect).
		SetStyle(defaultStyle).
		SetActivatedStyle(activatedStyle)

	grid.AddItem(shuffleButton, 4, 0, 1, 1, 0, 0, false)
	grid.AddItem(submitButton, 4, 1, 1, 2, 0, 0, false)
	grid.AddItem(deselectButton, 4, 3, 1, 1, 0, 0, false)

	grid.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyUp:
			if focusedRow > 0 {
				focusedRow--
			}
		case event.Key() == tcell.KeyDown:
			if focusedRow < 4 {
				focusedRow++
			}
		case event.Key() == tcell.KeyLeft:
			if focusedCol > 0 {
				if focusedRow == 4 && focusedCol == 2 {
					focusedCol--
				}
				focusedCol--
			}
		case event.Key() == tcell.KeyRight:
			if focusedCol < 3 {
				if focusedRow == 4 && focusedCol == 1 {
					focusedCol++
				}
				focusedCol++
			}
		case event.Key() == tcell.KeyEnter, event.Key() == tcell.KeyRune && event.Rune() == ' ':
			if focusedRow == 4 {
				if focusedCol == 0 {
					handleShuffle()
				} else if focusedCol == 3 {
					handleDeselect()
				} else {
					handleSubmit()
				}
			} else {
				label := buttons[focusedRow][focusedCol].GetLabel()
				if gameState.selectedCards[label] {
					gameState.selectedCards[label] = false
					buttons[focusedRow][focusedCol].
						SetStyle(defaultStyle).
						SetActivatedStyle(activatedStyle)
				} else if len(gameState.selectedCards) < 4 {
					gameState.selectedCards[label] = true
					buttons[focusedRow][focusedCol].
						SetStyle(selectedStyle).
						SetActivatedStyle(selectedActivatedStyle)
				}
			}
		}

		if focusedRow == 4 {
			if focusedCol == 0 {
				app.SetFocus(shuffleButton)
			} else if focusedCol == 3 {
				app.SetFocus(deselectButton)
			} else {
				app.SetFocus(submitButton)
			}
		} else {
			app.SetFocus(buttons[focusedRow][focusedCol])
		}
		return nil
	})

	app.SetFocus(buttons[0][0])

	if err := app.SetRoot(grid, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}
