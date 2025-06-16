package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"maps"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
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
	selectedCards   map[string]bool
	categories      map[string]Group
	currentMatchRow int
}

func fetch(urlString string) ([]byte, error) {
	resp, err := http.Get(urlString)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from URL: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()

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
	if err != nil {
		log.Fatal(err)
	}

	grid := tview.NewGrid().
		SetRows(3, 3, 3, 3, 3). // Added extra row for submit button
		SetColumns(20, 20, 20, 20)

	buttons := [4][4]*tview.Button{}
	var focusedRow, focusedCol int

	baseStyle := tcell.StyleDefault.
		Background(tcell.ColorDefault).
		Foreground(tcell.ColorBlack.TrueColor()).
		Bold(true)
	defaultStyle := baseStyle.Foreground(tcell.ColorDefault)
	selectedStyle := baseStyle.Foreground(tcell.ColorGray)
	selectedActivatedStyle := baseStyle.Foreground(tcell.ColorLightGray)

	var shuffleButton, submitButton, deselectButton *tview.Button

	resetSubmitButton := func() {
		if len(gameState.selectedCards) != 4 {
			submitButton.SetStyle(selectedActivatedStyle).SetActivatedStyle(selectedActivatedStyle)
		} else {
			submitButton.SetStyle(defaultStyle).SetActivatedStyle(defaultStyle)
		}
		submitButton.SetLabel("Submit (s)")
	}

	setFocus := func(r, c int) {
		focusedRow = r
		focusedCol = c
	}

	findButton := func(r, c int) *tview.Button {
		if r == 4 {
			switch c {
			case 0:
				return shuffleButton
			case 1, 2:
				return submitButton
			case 3:
				return deselectButton
			}
		}
		return buttons[r][c]
	}

	handleClick := func(r, c int) func() {
		return func() {
			previousButton := findButton(focusedRow, focusedCol).
				SetStyle(defaultStyle).
				SetActivatedStyle(defaultStyle)
			if focusedRow < 4 && gameState.selectedCards[previousButton.GetLabel()] {
				previousButton.SetStyle(selectedStyle)
			}

			label := buttons[r][c].GetLabel()
			if gameState.selectedCards[label] {
				delete(gameState.selectedCards, label)
				buttons[r][c].SetStyle(defaultStyle).SetActivatedStyle(defaultStyle)
			} else if len(gameState.selectedCards) < 4 {
				gameState.selectedCards[label] = true
				buttons[r][c].SetStyle(selectedStyle).SetActivatedStyle(selectedStyle)
			} else {
				return
			}
			setFocus(r, c)
			resetSubmitButton()
		}
	}

	handleDeselect := func() {
		setFocus(4, 3)
		deselectButton.SetActivatedStyle(selectedStyle)
		for cardContent := range gameState.selectedCards {
			delete(gameState.selectedCards, cardContent)
		}
		for i := 0; i < 4; i++ {
			for j := 0; j < 4; j++ {
				buttons[i][j].SetStyle(defaultStyle)
			}
		}
		resetSubmitButton()
	}

	handleShuffle := func() {
		setFocus(4, 0)
		shuffleButton.SetActivatedStyle(selectedStyle)
		// Flatten the buttons array for rows greater than currentMatchRow into a slice for shuffling
		var flatButtons []*tview.Button
		for i := gameState.currentMatchRow; i < 4; i++ {
			for j := 0; j < 4; j++ {
				flatButtons = append(flatButtons, buttons[i][j])
			}
		}

		// Shuffle the flatButtons slice
		rand.Shuffle(len(flatButtons), func(i, j int) {
			flatButtons[i], flatButtons[j] = flatButtons[j], flatButtons[i]
		})

		// Reassign the shuffled buttons back to the grid
		index := 0
		for i := gameState.currentMatchRow; i < 4; i++ {
			for j := 0; j < 4; j++ {
				button := flatButtons[index].SetSelectedFunc(handleClick(i, j))
				index++
				grid.RemoveItem(button)
				grid.AddItem(button, i, j, 1, 1, 0, 0, false)
				buttons[i][j] = button
			}
		}
		resetSubmitButton()
	}

	handleSubmit := func() {
		setFocus(4, 1)
		if len(gameState.selectedCards) != 4 {
			submitButton.SetStyle(selectedActivatedStyle).SetActivatedStyle(selectedActivatedStyle)
			return
		}

		var categoryTitle string
		var categoryIndex int
		categoryMap := make(map[string](int))
		const (
			correct = iota
			offByOne
			incorrect
		)
		result := incorrect

		for cardContent := range gameState.selectedCards {
			categoryIndex = gameState.categories[cardContent].Index
			categoryTitle = gameState.categories[cardContent].Title
			categoryMap[categoryTitle]++
			switch categoryMap[categoryTitle] {
			case 3:
				result = offByOne
			case 4:
				result = correct
			}
		}

		switch result {
		case correct:
			contents := fmt.Sprintf(
				"%s: %s",
				categoryTitle,
				strings.Join(slices.Collect(maps.Keys(gameState.selectedCards)), ", "),
			)
			button := tview.NewButton(contents).SetDisabled(true)
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
			grid.AddItem(button, gameState.currentMatchRow, 0, 1, 4, 0, 0, false)

			buttonsToMove := []*tview.Button{}
			for i := 0; i < 4; i++ {
				for j := 0; j < 4; j++ {
					button := buttons[i][j]
					wasSelected := gameState.selectedCards[button.GetLabel()]
					if i == gameState.currentMatchRow && !wasSelected {
						buttonsToMove = append(buttonsToMove, button)
						grid.RemoveItem(button)
					}
					if wasSelected {
						grid.RemoveItem(button)
						if i != gameState.currentMatchRow && len(buttonsToMove) > 0 {
							grid.AddItem(buttonsToMove[0], i, j, 1, 1, 0, 0, false)
							buttons[i][j] = buttonsToMove[0]
							buttonsToMove = buttonsToMove[1:]
						}
					}
				}
			}
			if focusedRow == gameState.currentMatchRow {
				focusedRow++
			}
			gameState.currentMatchRow++
			for cardContent := range gameState.selectedCards {
				delete(gameState.selectedCards, cardContent)
			}
			submitButton.
				SetStyle(baseStyle.Background(tcell.ColorGreen)).
				SetActivatedStyle(baseStyle.Background(tcell.ColorGreen))
		case offByOne:
			submitButton.
				SetStyle(baseStyle.Background(tcell.ColorYellow)).
				SetActivatedStyle(baseStyle.Background(tcell.ColorYellow)).
				SetLabel("One away...")
		default:
			submitButton.
				SetStyle(baseStyle.Background(tcell.ColorRed)).
				SetActivatedStyle(baseStyle.Background(tcell.ColorRed))
		}
	}

	for row := 0; row < 4; row++ {
		category := response.Categories[row]
		for col := 0; col < 4; col++ {
			card := category.Cards[col]
			label := cases.Title(language.AmericanEnglish).String(card.Content)
			title := cases.Title(language.AmericanEnglish).String(category.Title)
			gRow := card.Position / 4
			gCol := card.Position % 4

			gameState.categories[label] = Group{title, row}

			button := tview.NewButton(label).
				SetSelectedFunc(handleClick(gRow, gCol)).
				SetStyle(defaultStyle).
				SetActivatedStyle(defaultStyle)
			button.SetBorder(true).SetBorderColor(tcell.ColorDarkGray)
			buttons[gRow][gCol] = button
			grid.AddItem(button, gRow, gCol, 1, 1, 0, 0, false)
		}
	}

	shuffleButton = tview.NewButton("Shuffle (a)").
		SetSelectedFunc(handleShuffle).
		SetStyle(defaultStyle).
		SetActivatedStyle(selectedStyle)

	submitButton = tview.NewButton("Submit (s)").
		SetSelectedFunc(handleSubmit).
		SetStyle(selectedActivatedStyle).
		SetActivatedStyle(selectedActivatedStyle)

	deselectButton = tview.NewButton("Deselect All (d)").
		SetSelectedFunc(handleDeselect).
		SetStyle(defaultStyle).
		SetActivatedStyle(selectedStyle)

	grid.AddItem(shuffleButton, 4, 0, 1, 1, 0, 0, false)
	grid.AddItem(submitButton, 4, 1, 1, 2, 0, 0, false)
	grid.AddItem(deselectButton, 4, 3, 1, 1, 0, 0, false)

	grid.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		previousButton := findButton(focusedRow, focusedCol).SetActivatedStyle(defaultStyle)
		if focusedRow < 4 && gameState.selectedCards[previousButton.GetLabel()] {
			previousButton.SetStyle(selectedStyle)
		} else if focusedRow == 4 {
			previousButton.SetStyle(defaultStyle)
		}

		switch {
		case event.Key() == tcell.KeyRune && event.Rune() == 'a':
			r := focusedRow
			c := focusedCol
			handleShuffle()
			setFocus(r, c)
		case event.Key() == tcell.KeyRune && event.Rune() == 's':
			r := focusedRow
			c := focusedCol
			handleSubmit()
			if r < gameState.currentMatchRow {
				r++
			}
			setFocus(r, c)
		case event.Key() == tcell.KeyRune && event.Rune() == 'd':
			r := focusedRow
			c := focusedCol
			handleDeselect()
			setFocus(r, c)
		case event.Key() == tcell.KeyUp, event.Key() == tcell.KeyRune && event.Rune() == 'k':
			if focusedRow > gameState.currentMatchRow {
				focusedRow--
			}
			resetSubmitButton()
		case event.Key() == tcell.KeyDown, event.Key() == tcell.KeyRune && event.Rune() == 'j':
			if focusedRow < 4 {
				focusedRow++
			}
			resetSubmitButton()
		case event.Key() == tcell.KeyLeft, event.Key() == tcell.KeyRune && event.Rune() == 'h':
			if focusedCol > 0 {
				if focusedRow == 4 && focusedCol == 2 {
					focusedCol--
				}
				focusedCol--
			}
			resetSubmitButton()
		case event.Key() == tcell.KeyRight, event.Key() == tcell.KeyRune && event.Rune() == 'l':
			if focusedCol < 3 {
				if focusedRow == 4 && focusedCol == 1 {
					focusedCol++
				}
				focusedCol++
			}
			resetSubmitButton()
		case event.Key() == tcell.KeyEnter, event.Key() == tcell.KeyRune && event.Rune() == ' ':
			if focusedRow == 4 {
				switch focusedCol {
				case 0:
					handleShuffle()
				case 3:
					handleDeselect()
				default:
					handleSubmit()
				}
			} else {
				label := buttons[focusedRow][focusedCol].GetLabel()
				if gameState.selectedCards[label] {
					delete(gameState.selectedCards, label)
					buttons[focusedRow][focusedCol].
						SetStyle(defaultStyle)
				} else if len(gameState.selectedCards) < 4 {
					gameState.selectedCards[label] = true
					buttons[focusedRow][focusedCol].
						SetStyle(selectedStyle).
						SetActivatedStyle(selectedActivatedStyle)
				}
			}
			resetSubmitButton()
		default:
			return nil
		}

		button := findButton(focusedRow, focusedCol)
		if focusedRow == 4 && (focusedCol == 0 || focusedCol == 3) {
			button.SetActivatedStyle(selectedStyle)
		}
		if gameState.selectedCards[button.GetLabel()] {
			button.SetActivatedStyle(selectedStyle)
		}
		app.SetFocus(button)
		return nil
	})

	app.SetFocus(buttons[0][0])

	// Create a flexbox to center the grid horizontally.
	flex := tview.NewFlex().
		AddItem(tview.NewBox(), 0, 1, false). // Left spacer.
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
							AddItem(tview.NewBox(), 0, 1, false).               // Top spacer.
							AddItem(grid, 0, 3, true).                          // The grid, fixed width of 80.
							AddItem(tview.NewBox(), 0, 1, false), 80, 1, true). // Bottom spacer.
		AddItem(tview.NewBox(), 0, 1, false) // Right spacer.

	if err := app.SetRoot(flex, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}
