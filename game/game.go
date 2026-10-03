package game

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
	"os/exec"
	"path/filepath"
	"runtime"
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
	history         []string // One emoji row per submitted guess.
	mistakes        int
	wrongGuesses    map[string]bool // Distinct incorrect guesses, keyed by emoji row.
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

// tileEmoji maps a category index to the emoji used in the share string.
var tileEmoji = [4]string{"🟨", "🟩", "🟦", "🟪"}

// copyToClipboard copies text using the platform's clipboard utility.
func copyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "clip")
	case "darwin":
		cmd = exec.Command("pbcopy")
	default:
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		}
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func RunWithScreen(screen tcell.Screen) error {
	app := tview.NewApplication()
	app.SetScreen(screen)
	return Run(app, screen)
}

func Run(app *tview.Application, screen tcell.Screen) error {
	gameState := GameState{
		selectedCards: make(map[string]bool),
		categories:    make(map[string]Group),
		wrongGuesses:  make(map[string]bool),
	}

	today := time.Now()
	connectionsData, err := getConnectionsJSON(today)
	if err != nil {
		return err
	}
	response, err := parseConnectionsJSON(connectionsData)
	if err != nil {
		return err
	}

	// The Connections puzzle number is derived from the print date: puzzle #1
	// was 2023-06-12. Used by the header and the share string.
	puzzleNumber := response.ID
	if date, err := time.Parse("2006-01-02", response.PrintDate); err == nil {
		puzzleNumber = int(date.Sub(time.Date(2023, 6, 12, 0, 0, 0, 0, time.UTC)).Hours()/24) + 1
	}

	grid := tview.NewGrid().
		SetRows(3, 3, 3, 3, 3). // Extra row for submit button.
		SetColumns(20, 20, 20, 20)

	buttons := [4][4]*tview.Button{}
	var focusedRow, focusedCol int

	var selectedStyle tcell.Style
	if screen.Colors() < 256 {
		selectedStyle = selectedStyle.Bold(true).Underline(true)
	} else {
		selectedStyle = selectedStyle.Foreground(tcell.ColorGray)
	}
	disabledStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkGray).StrikeThrough(true)

	var shuffleButton, submitButton, deselectButton, shareButton *tview.Button
	var contentFlex *tview.Flex

	mistakesText := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetText("Mistakes Remaining: ● ● ● ●")

	gameOver := false

	updateMistakes := func() {
		if remaining := 4 - gameState.mistakes; remaining > 0 {
			mistakesText.SetText(fmt.Sprintf("Mistakes Remaining: %s", strings.TrimRight(strings.Repeat("● ", remaining), " ")))
		} else {
			mistakesText.SetText(fmt.Sprintf("Mistakes: %d", gameState.mistakes))
		}
	}

	resetSubmitButton := func() {
		if len(gameState.selectedCards) != 4 {
			submitButton.SetStyle(disabledStyle).SetActivatedStyle(disabledStyle)
		} else {
			submitButton.SetStyle(tcell.StyleDefault).SetActivatedStyle(selectedStyle)
		}
		submitButton.SetLabel("Submit (s)")
	}

	findButton := func(r, c int) *tview.Button {
		if r == 4 {
			if gameOver {
				return shareButton
			}
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

	setFocus := func(r, c int) {
		// Unset previous button's border.
		if focusedRow < 4 {
			findButton(focusedRow, focusedCol).SetBorderColor(tcell.ColorDarkGray)
		}
		focusedRow = r
		focusedCol = c
		// Set current button's border.
		button := findButton(focusedRow, focusedCol)
		if focusedRow < 4 {
			button.SetBorderColor(tcell.ColorGray)
		}
		app.SetFocus(button)
	}

	handleClick := func(r, c int) func() {
		return func() {
			label := buttons[r][c].GetLabel()
			if gameState.selectedCards[label] {
				delete(gameState.selectedCards, label)
				buttons[r][c].SetStyle(tcell.StyleDefault).SetActivatedStyle(tcell.StyleDefault)
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
		deselectButton.SetActivatedStyle(selectedStyle)
		for cardContent := range gameState.selectedCards {
			delete(gameState.selectedCards, cardContent)
		}
		for i := range 4 {
			for j := range 4 {
				buttons[i][j].SetStyle(tcell.StyleDefault).SetActivatedStyle(tcell.StyleDefault)
			}
		}
		resetSubmitButton()
	}

	handleShuffle := func() {
		shuffleButton.SetActivatedStyle(selectedStyle)
		// Capture the focused button before the shuffle moves it to a new cell.
		var focusedButton *tview.Button
		if focusedRow < 4 {
			focusedButton = findButton(focusedRow, focusedCol)
		}
		// Flatten the buttons array for rows greater than currentMatchRow into a slice for shuffling
		var flatButtons []*tview.Button
		for i := gameState.currentMatchRow; i < 4; i++ {
			for j := range 4 {
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
			for j := range 4 {
				button := flatButtons[index].SetSelectedFunc(handleClick(i, j))
				index++
				grid.RemoveItem(button)
				grid.AddItem(button, i, j, 1, 1, 0, 0, false)
				buttons[i][j] = button
			}
		}

		// The focused word's button moved to a new cell with its focus border,
		// so point the focus bookkeeping at its new position.
		if focusedButton != nil {
			for i := gameState.currentMatchRow; i < 4; i++ {
				for j := range 4 {
					if buttons[i][j] == focusedButton {
						focusedRow, focusedCol = i, j
					}
				}
			}
		}
		resetSubmitButton()
	}

	handleShare := func() {
		var result strings.Builder
		result.WriteString("Connections\n")
		result.WriteString(fmt.Sprintf("Puzzle #%d\n", puzzleNumber))
		for _, row := range gameState.history {
			result.WriteString(row)
			result.WriteByte('\n')
		}
		if err := copyToClipboard(result.String()); err != nil {
			shareButton.SetLabel(fmt.Sprintf("Copy failed: %v", err))
			return
		}
		shareButton.SetLabel("Copied to clipboard!")
	}

	handleSubmit := func() {
		if len(gameState.selectedCards) != 4 {
			return
		}

		// Record the guess as a row of category emojis, ordered by category.
		words := slices.Collect(maps.Keys(gameState.selectedCards))
		slices.SortStableFunc(words, func(a, b string) int {
			if ai, bi := gameState.categories[a].Index, gameState.categories[b].Index; ai != bi {
				return ai - bi
			}
			return strings.Compare(a, b)
		})
		row := ""
		for _, w := range words {
			row += tileEmoji[gameState.categories[w].Index]
		}
		gameState.history = append(gameState.history, row)

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
				button.SetDisabledStyle(tcell.StyleDefault.Background(tcell.ColorYellow).Foreground(tcell.ColorBlack.TrueColor()))
			case 1:
				button.SetDisabledStyle(tcell.StyleDefault.Background(tcell.ColorGreen).Foreground(tcell.ColorBlack.TrueColor()))
			case 2:
				button.SetDisabledStyle(tcell.StyleDefault.Background(tcell.ColorBlue).Foreground(tcell.ColorBlack.TrueColor()))
			case 3:
				button.SetDisabledStyle(tcell.StyleDefault.Background(tcell.ColorPurple).Foreground(tcell.ColorBlack.TrueColor()))
			}
			grid.AddItem(button, gameState.currentMatchRow, 0, 1, 4, 0, 0, false)

			buttonsToMove := []*tview.Button{}
			for i := range 4 {
				for j := range 4 {
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

			if gameState.currentMatchRow == 4 {
				// Game over: swap the controls for a single share button.
				gameOver = true
				grid.RemoveItem(shuffleButton)
				grid.RemoveItem(submitButton)
				grid.RemoveItem(deselectButton)
				shareButton = tview.NewButton("Share Your Result").
					SetSelectedFunc(handleShare).
					SetStyle(tcell.StyleDefault.Background(tcell.ColorGreen).Foreground(tcell.ColorBlack.TrueColor())).
					SetActivatedStyle(tcell.StyleDefault.Background(tcell.ColorGreen).Foreground(tcell.ColorBlack.TrueColor()))
				grid.AddItem(shareButton, 4, 0, 1, 4, 0, 0, false)
				focusedRow, focusedCol = 4, 0
				app.SetFocus(shareButton)
			}

			submitButton.
				SetStyle(tcell.StyleDefault.Background(tcell.ColorGreen).Foreground(tcell.ColorBlack.TrueColor())).
				SetActivatedStyle(tcell.StyleDefault.Background(tcell.ColorGreen).Foreground(tcell.ColorBlack.TrueColor()))
		case offByOne:
			if gameState.wrongGuesses[row] {
				submitButton.
					SetStyle(tcell.StyleDefault.Background(tcell.ColorYellow).Foreground(tcell.ColorBlack.TrueColor())).
					SetActivatedStyle(tcell.StyleDefault.Background(tcell.ColorYellow).Foreground(tcell.ColorBlack.TrueColor())).
					SetLabel("One away...")
				break
			}
			gameState.wrongGuesses[row] = true
			gameState.mistakes++
			updateMistakes()
			submitButton.
				SetStyle(tcell.StyleDefault.Background(tcell.ColorYellow).Foreground(tcell.ColorBlack.TrueColor())).
				SetActivatedStyle(tcell.StyleDefault.Background(tcell.ColorYellow).Foreground(tcell.ColorBlack.TrueColor())).
				SetLabel("One away...")
		default:
			if gameState.wrongGuesses[row] {
				submitButton.
					SetStyle(tcell.StyleDefault.Background(tcell.ColorRed).Foreground(tcell.ColorBlack.TrueColor())).
					SetActivatedStyle(tcell.StyleDefault.Background(tcell.ColorRed).Foreground(tcell.ColorBlack.TrueColor())).
					SetLabel("Already Guessed")
				break;
			}
			gameState.wrongGuesses[row] = true
			gameState.mistakes++
			updateMistakes()
			submitButton.
				SetStyle(tcell.StyleDefault.Background(tcell.ColorRed).Foreground(tcell.ColorBlack.TrueColor())).
				SetActivatedStyle(tcell.StyleDefault.Background(tcell.ColorRed).Foreground(tcell.ColorBlack.TrueColor())).
				SetLabel("Incorrect")
		}
	}

	for row := range 4 {
		category := response.Categories[row]
		for col := range 4 {
			card := category.Cards[col]
			label := cases.Title(language.AmericanEnglish).String(card.Content)
			title := cases.Title(language.AmericanEnglish).String(category.Title)
			gRow := card.Position / 4
			gCol := card.Position % 4

			gameState.categories[label] = Group{title, row}

			button := tview.NewButton(label).
				SetSelectedFunc(handleClick(gRow, gCol)).
				SetStyle(tcell.StyleDefault).
				SetActivatedStyle(tcell.StyleDefault)
			button.SetBorder(true).SetBorderColor(tcell.ColorDarkGray)
			buttons[gRow][gCol] = button
			grid.AddItem(button, gRow, gCol, 1, 1, 0, 0, false)
		}
	}

	shuffleButton = tview.NewButton("Shuffle (a)").
		SetSelectedFunc(handleShuffle).
		SetStyle(tcell.StyleDefault).
		SetActivatedStyle(selectedStyle)

	submitButton = tview.NewButton("Submit (s)").
		SetSelectedFunc(handleSubmit).
		SetStyle(disabledStyle).
		SetActivatedStyle(disabledStyle)

	deselectButton = tview.NewButton("Deselect All (d)").
		SetSelectedFunc(handleDeselect).
		SetStyle(tcell.StyleDefault).
		SetActivatedStyle(selectedStyle)

	grid.AddItem(shuffleButton, 4, 0, 1, 1, 0, 0, false)
	grid.AddItem(submitButton, 4, 1, 1, 2, 0, 0, false)
	grid.AddItem(deselectButton, 4, 3, 1, 1, 0, 0, false)

	grid.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		r := focusedRow
		c := focusedCol

		switch {
		case event.Key() == tcell.KeyRune && event.Rune() == 'q':
			app.Stop()
		case event.Key() == tcell.KeyRune && event.Rune() == 'a' && !gameOver:
			handleShuffle()
			r, c = focusedRow, focusedCol
		case event.Key() == tcell.KeyRune && event.Rune() == 's' && !gameOver:
			handleSubmit()
			if r < gameState.currentMatchRow {
				r++
			}
		case event.Key() == tcell.KeyRune && event.Rune() == 'd' && !gameOver:
			handleDeselect()
		case event.Key() == tcell.KeyUp, event.Key() == tcell.KeyRune && event.Rune() == 'k':
			if r > gameState.currentMatchRow {
				r--
			}
			resetSubmitButton()
		case event.Key() == tcell.KeyDown, event.Key() == tcell.KeyRune && event.Rune() == 'j':
			if r < 4 {
				r++
			}
			resetSubmitButton()
		case event.Key() == tcell.KeyLeft, event.Key() == tcell.KeyRune && event.Rune() == 'h':
			if c > 0 {
				if r == 4 && c == 2 {
					c--
				}
				c--
			}
			resetSubmitButton()
		case event.Key() == tcell.KeyRight, event.Key() == tcell.KeyRune && event.Rune() == 'l':
			if c < 3 {
				if r == 4 && c == 1 {
					c++
				}
				c++
			}
			resetSubmitButton()
		case event.Key() == tcell.KeyEnter, event.Key() == tcell.KeyRune && event.Rune() == ' ':
			if r == 4 {
				if gameOver {
					handleShare()
					break
				}
				switch c {
				case 0:
					handleShuffle()
				case 3:
					handleDeselect()
				default:
					handleSubmit()
				}
			} else {
				label := buttons[r][c].GetLabel()
				if gameState.selectedCards[label] {
					delete(gameState.selectedCards, label)
					buttons[r][c].SetStyle(tcell.StyleDefault).SetActivatedStyle(tcell.StyleDefault)
				} else if len(gameState.selectedCards) < 4 {
					gameState.selectedCards[label] = true
					buttons[r][c].SetStyle(selectedStyle).SetActivatedStyle(selectedStyle)
				}
				resetSubmitButton()
			}
		default:
			return nil
		}

		setFocus(r, c)
		return nil
	})

	setFocus(0, 0)

	headerText := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetText(fmt.Sprintf("Connections #%d\nBy %s", puzzleNumber, response.Editor))

	// The game content needs 19 rows: header (2), gap (1), grid (15), and the
	// mistakes counter (1). Extra terminal rows are split evenly above and
	// below, with an odd row going to the top. All sizes are fixed (recomputed
	// before each draw) so nothing drifts as the terminal is resized; only the
	// grid is proportional, so it compresses gracefully below 19 rows.
	contentFlex = tview.NewFlex().SetDirection(tview.FlexRow)
	topSpacer, bottomSpacer := tview.NewBox(), tview.NewBox()
	headerGap, footerGap := tview.NewBox(), tview.NewBox()
	// Rebuild the content column for the given terminal height. Called before
	// every draw (so resizing recenters the layout) and once up front: SetRoot
	// cascades focus down through the column, which only reaches the grid if
	// the items already exist.
	relayout := func(height int) {
		extra := max(height-19, 0)
		footer := tview.Primitive(mistakesText)
		if gameOver {
			footer = footerGap // Keep the row count stable once the counter disappears.
		}
		contentFlex.Clear().
			AddItem(topSpacer, extra/2+extra%2, 0, false).
			AddItem(headerText, 2, 0, false).
			AddItem(headerGap, 1, 0, false).
			AddItem(grid, 0, 1, true).
			AddItem(footer, 1, 0, false).
			AddItem(bottomSpacer, extra/2, 0, false)
	}
	app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		_, height := screen.Size()
		relayout(height)
		return false
	})
	_, termHeight := screen.Size()
	relayout(termHeight)
	flex := tview.NewFlex().
		AddItem(tview.NewBox(), 0, 1, false). // Left spacer.
		AddItem(contentFlex, 80, 1, true).    // The centered game column.
		AddItem(tview.NewBox(), 0, 1, false)  // Right spacer.

	if err := app.SetRoot(flex, true).EnableMouse(true).Run(); err != nil {
		return err
	}
	return nil
}
