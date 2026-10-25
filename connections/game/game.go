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
	jsonFilename := date.Format("2006-01-02") + ".json"

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

	err := json.Unmarshal(data, &response)
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

// session holds the state of one running game and the widgets it drives.
type session struct {
	app    *tview.Application
	screen tcell.Screen

	state        GameState
	puzzleNumber int

	grid    *tview.Grid
	buttons [4][4]*tview.Button

	focusedRow, focusedCol int

	selectedStyle tcell.Style
	disabledStyle tcell.Style

	shuffleButton, submitButton, deselectButton, shareButton *tview.Button

	mistakesText *tview.TextView
	gameOver     bool
}

// solidStyle is black text on the given background.
func solidStyle(bg tcell.Color) tcell.Style {
	return tcell.StyleDefault.Background(bg).Foreground(tcell.ColorBlack.TrueColor())
}

// categoryColors maps a category index to its background color.
var categoryColors = [4]tcell.Color{tcell.ColorYellow, tcell.ColorGreen, tcell.ColorBlue, tcell.ColorPurple}

func Run(app *tview.Application, screen tcell.Screen) error {
	s := &session{
		app:    app,
		screen: screen,
		state: GameState{
			selectedCards: make(map[string]bool),
			categories:    make(map[string]Group),
			wrongGuesses:  make(map[string]bool),
		},
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
	s.puzzleNumber = response.ID
	if date, err := time.Parse("2006-01-02", response.PrintDate); err == nil {
		s.puzzleNumber = int(date.Sub(time.Date(2023, 6, 12, 0, 0, 0, 0, time.UTC)).Hours()/24) + 1
	}

	s.grid = tview.NewGrid().
		SetRows(3, 3, 3, 3, 3). // Extra row for submit button.
		SetColumns(20, 20, 20, 20)

	if screen.Colors() < 256 {
		s.selectedStyle = s.selectedStyle.Bold(true).Underline(true)
	} else {
		s.selectedStyle = s.selectedStyle.Foreground(tcell.ColorGray)
	}
	s.disabledStyle = tcell.StyleDefault.Foreground(tcell.ColorDarkGray).StrikeThrough(true)

	s.mistakesText = tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetText("Mistakes Remaining: ● ● ● ●")

	s.buildCards(response)
	s.buildControls()
	s.grid.SetInputCapture(s.handleKey)
	s.setFocus(0, 0)

	headerText := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetText(fmt.Sprintf("Connections #%d\nBy %s", s.puzzleNumber, response.Editor))

	return s.runLayout(headerText)
}

func (s *session) buildCards(response Response) {
	for row := range 4 {
		category := response.Categories[row]
		for col := range 4 {
			card := category.Cards[col]
			label := cases.Title(language.AmericanEnglish).String(card.Content)
			title := cases.Title(language.AmericanEnglish).String(category.Title)
			gRow := card.Position / 4
			gCol := card.Position % 4

			s.state.categories[label] = Group{title, row}

			button := tview.NewButton(label).
				SetSelectedFunc(s.handleClick(gRow, gCol)).
				SetStyle(tcell.StyleDefault).
				SetActivatedStyle(tcell.StyleDefault)
			button.SetBorder(true).SetBorderColor(tcell.ColorDarkGray)
			s.buttons[gRow][gCol] = button
			s.grid.AddItem(button, gRow, gCol, 1, 1, 0, 0, false)
		}
	}
}

func (s *session) buildControls() {
	s.shuffleButton = tview.NewButton("Shuffle (a)").
		SetSelectedFunc(s.handleShuffle).
		SetStyle(tcell.StyleDefault).
		SetActivatedStyle(s.selectedStyle)

	s.submitButton = tview.NewButton("Submit (s)").
		SetSelectedFunc(s.handleSubmit).
		SetStyle(s.disabledStyle).
		SetActivatedStyle(s.disabledStyle)

	s.deselectButton = tview.NewButton("Deselect All (d)").
		SetSelectedFunc(s.handleDeselect).
		SetStyle(tcell.StyleDefault).
		SetActivatedStyle(s.selectedStyle)

	s.grid.AddItem(s.shuffleButton, 4, 0, 1, 1, 0, 0, false)
	s.grid.AddItem(s.submitButton, 4, 1, 1, 2, 0, 0, false)
	s.grid.AddItem(s.deselectButton, 4, 3, 1, 1, 0, 0, false)
}

func (s *session) runLayout(headerText *tview.TextView) error {
	// The game content needs 19 rows: header (2), gap (1), grid (15), and the
	// mistakes counter (1). Extra terminal rows are split evenly above and
	// below, with an odd row going to the top. All sizes are fixed (recomputed
	// before each draw) so nothing drifts as the terminal is resized; only the
	// grid is proportional, so it compresses gracefully below 19 rows.
	contentFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	topSpacer, bottomSpacer := tview.NewBox(), tview.NewBox()
	headerGap, footerGap := tview.NewBox(), tview.NewBox()
	// Rebuild the content column for the given terminal height. Called before
	// every draw (so resizing recenters the layout) and once up front: SetRoot
	// cascades focus down through the column, which only reaches the grid if
	// the items already exist.
	relayout := func(height int) {
		extra := max(height-19, 0)
		footer := tview.Primitive(s.mistakesText)
		if s.gameOver {
			footer = footerGap // Keep the row count stable once the counter disappears.
		}
		contentFlex.Clear().
			AddItem(topSpacer, extra/2+extra%2, 0, false).
			AddItem(headerText, 2, 0, false).
			AddItem(headerGap, 1, 0, false).
			AddItem(s.grid, 0, 1, true).
			AddItem(footer, 1, 0, false).
			AddItem(bottomSpacer, extra/2, 0, false)
	}
	s.app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		_, height := screen.Size()
		relayout(height)
		return false
	})
	_, termHeight := s.screen.Size()
	relayout(termHeight)
	flex := tview.NewFlex().
		AddItem(tview.NewBox(), 0, 1, false). // Left spacer.
		AddItem(contentFlex, 80, 1, true).    // The centered game column.
		AddItem(tview.NewBox(), 0, 1, false)  // Right spacer.

	return s.app.SetRoot(flex, true).EnableMouse(true).Run()
}

func (s *session) updateMistakes() {
	if remaining := 4 - s.state.mistakes; remaining > 0 {
		s.mistakesText.SetText("Mistakes Remaining: " + strings.TrimRight(strings.Repeat("● ", remaining), " "))
	} else {
		s.mistakesText.SetText(fmt.Sprintf("Mistakes: %d", s.state.mistakes))
	}
}

func (s *session) resetSubmitButton() {
	if len(s.state.selectedCards) != 4 {
		s.submitButton.SetStyle(s.disabledStyle).SetActivatedStyle(s.disabledStyle)
	} else {
		s.submitButton.SetStyle(tcell.StyleDefault).SetActivatedStyle(s.selectedStyle)
	}
	s.submitButton.SetLabel("Submit (s)")
}

func (s *session) findButton(r, c int) *tview.Button {
	if r == 4 {
		if s.gameOver {
			return s.shareButton
		}
		switch c {
		case 0:
			return s.shuffleButton
		case 1, 2:
			return s.submitButton
		case 3:
			return s.deselectButton
		}
	}
	return s.buttons[r][c]
}

func (s *session) setFocus(r, c int) {
	// Unset previous button's border.
	if s.focusedRow < 4 {
		s.findButton(s.focusedRow, s.focusedCol).SetBorderColor(tcell.ColorDarkGray)
	}
	s.focusedRow = r
	s.focusedCol = c
	// Set current button's border.
	button := s.findButton(s.focusedRow, s.focusedCol)
	if s.focusedRow < 4 {
		button.SetBorderColor(tcell.ColorGray)
	}
	s.app.SetFocus(button)
}

// toggleCard flips the selection of the card at (r, c). It reports false when
// nothing changed because four cards are already selected.
func (s *session) toggleCard(r, c int) bool {
	label := s.buttons[r][c].GetLabel()
	if s.state.selectedCards[label] {
		delete(s.state.selectedCards, label)
		s.buttons[r][c].SetStyle(tcell.StyleDefault).SetActivatedStyle(tcell.StyleDefault)
		return true
	}
	if len(s.state.selectedCards) < 4 {
		s.state.selectedCards[label] = true
		s.buttons[r][c].SetStyle(s.selectedStyle).SetActivatedStyle(s.selectedStyle)
		return true
	}
	return false
}

func (s *session) handleClick(r, c int) func() {
	return func() {
		if !s.toggleCard(r, c) {
			return
		}
		s.setFocus(r, c)
		s.resetSubmitButton()
	}
}

func (s *session) handleDeselect() {
	s.deselectButton.SetActivatedStyle(s.selectedStyle)
	for cardContent := range s.state.selectedCards {
		delete(s.state.selectedCards, cardContent)
	}
	for i := range 4 {
		for j := range 4 {
			s.buttons[i][j].SetStyle(tcell.StyleDefault).SetActivatedStyle(tcell.StyleDefault)
		}
	}
	s.resetSubmitButton()
}

func (s *session) handleShuffle() {
	s.shuffleButton.SetActivatedStyle(s.selectedStyle)
	// Capture the focused button before the shuffle moves it to a new cell.
	var focusedButton *tview.Button
	if s.focusedRow < 4 {
		focusedButton = s.findButton(s.focusedRow, s.focusedCol)
	}
	// Flatten the buttons array for rows greater than currentMatchRow into a slice for shuffling
	var flatButtons []*tview.Button
	for i := s.state.currentMatchRow; i < 4; i++ {
		for j := range 4 {
			flatButtons = append(flatButtons, s.buttons[i][j])
		}
	}

	// Shuffle the flatButtons slice
	rand.Shuffle(len(flatButtons), func(i, j int) {
		flatButtons[i], flatButtons[j] = flatButtons[j], flatButtons[i]
	})

	// Reassign the shuffled buttons back to the grid
	index := 0
	for i := s.state.currentMatchRow; i < 4; i++ {
		for j := range 4 {
			button := flatButtons[index].SetSelectedFunc(s.handleClick(i, j))
			index++
			s.grid.RemoveItem(button)
			s.grid.AddItem(button, i, j, 1, 1, 0, 0, false)
			s.buttons[i][j] = button
		}
	}

	// The focused word's button moved to a new cell with its focus border,
	// so point the focus bookkeeping at its new position.
	if focusedButton != nil {
		for i := s.state.currentMatchRow; i < 4; i++ {
			for j := range 4 {
				if s.buttons[i][j] == focusedButton {
					s.focusedRow, s.focusedCol = i, j
				}
			}
		}
	}
	s.resetSubmitButton()
}

func (s *session) handleShare() {
	var result strings.Builder
	result.WriteString("Connections\n")
	fmt.Fprintf(&result, "Puzzle #%d\n", s.puzzleNumber)
	for _, row := range s.state.history {
		result.WriteString(row)
		result.WriteByte('\n')
	}
	if err := copyToClipboard(result.String()); err != nil {
		s.shareButton.SetLabel(fmt.Sprintf("Copy failed: %v", err))
		return
	}
	s.shareButton.SetLabel("Copied to clipboard!")
}

// recordGuess appends the current selection to the history as a row of
// category emojis, ordered by category, and returns that row.
func (s *session) recordGuess() string {
	words := slices.Collect(maps.Keys(s.state.selectedCards))
	slices.SortStableFunc(words, func(a, b string) int {
		if ai, bi := s.state.categories[a].Index, s.state.categories[b].Index; ai != bi {
			return ai - bi
		}
		return strings.Compare(a, b)
	})
	var row strings.Builder
	for _, w := range words {
		row.WriteString(tileEmoji[s.state.categories[w].Index])
	}
	s.state.history = append(s.state.history, row.String())
	return row.String()
}

type guessResult int

const (
	guessCorrect guessResult = iota
	guessOffByOne
	guessIncorrect
)

// judgeGuess classifies the selection. The title and index are those of the
// last category visited, which is the matched category for a correct guess.
func (s *session) judgeGuess() (result guessResult, categoryTitle string, categoryIndex int) {
	categoryMap := make(map[string]int)
	result = guessIncorrect
	for cardContent := range s.state.selectedCards {
		categoryIndex = s.state.categories[cardContent].Index
		categoryTitle = s.state.categories[cardContent].Title
		categoryMap[categoryTitle]++
		switch categoryMap[categoryTitle] {
		case 3:
			result = guessOffByOne
		case 4:
			result = guessCorrect
		}
	}
	return result, categoryTitle, categoryIndex
}

func (s *session) handleSubmit() {
	if len(s.state.selectedCards) != 4 {
		return
	}

	row := s.recordGuess()
	result, categoryTitle, categoryIndex := s.judgeGuess()

	switch result {
	case guessCorrect:
		s.handleCorrect(categoryTitle, categoryIndex)
	case guessOffByOne:
		s.handleWrong(row, tcell.ColorYellow, "One away...", "One away...")
	case guessIncorrect:
		s.handleWrong(row, tcell.ColorRed, "Already Guessed", "Incorrect")
	}
}

// handleWrong styles the submit button for a wrong guess. A repeated guess
// shows repeatLabel and costs nothing; a new one shows newLabel and costs a
// mistake.
func (s *session) handleWrong(row string, bg tcell.Color, repeatLabel, newLabel string) {
	label := repeatLabel
	if !s.state.wrongGuesses[row] {
		s.state.wrongGuesses[row] = true
		s.state.mistakes++
		s.updateMistakes()
		label = newLabel
	}
	s.submitButton.
		SetStyle(solidStyle(bg)).
		SetActivatedStyle(solidStyle(bg)).
		SetLabel(label)
}

func (s *session) handleCorrect(categoryTitle string, categoryIndex int) {
	contents := fmt.Sprintf(
		"%s: %s",
		categoryTitle,
		strings.Join(slices.Collect(maps.Keys(s.state.selectedCards)), ", "),
	)
	button := tview.NewButton(contents).SetDisabled(true)
	if categoryIndex >= 0 && categoryIndex < len(categoryColors) {
		button.SetDisabledStyle(solidStyle(categoryColors[categoryIndex]))
	}
	s.grid.AddItem(button, s.state.currentMatchRow, 0, 1, 4, 0, 0, false)

	s.moveRemainingUp()
	if s.focusedRow == s.state.currentMatchRow {
		s.focusedRow++
	}
	s.state.currentMatchRow++
	for cardContent := range s.state.selectedCards {
		delete(s.state.selectedCards, cardContent)
	}

	if s.state.currentMatchRow == 4 {
		// Game over: swap the controls for a single share button.
		s.gameOver = true
		s.grid.RemoveItem(s.shuffleButton)
		s.grid.RemoveItem(s.submitButton)
		s.grid.RemoveItem(s.deselectButton)
		s.shareButton = tview.NewButton("Share Your Result").
			SetSelectedFunc(s.handleShare).
			SetStyle(solidStyle(tcell.ColorGreen)).
			SetActivatedStyle(solidStyle(tcell.ColorGreen))
		s.grid.AddItem(s.shareButton, 4, 0, 1, 4, 0, 0, false)
		s.focusedRow, s.focusedCol = 4, 0
		s.app.SetFocus(s.shareButton)
	}

	s.submitButton.
		SetStyle(solidStyle(tcell.ColorGreen)).
		SetActivatedStyle(solidStyle(tcell.ColorGreen))
}

// moveRemainingUp removes the matched cards from the grid and moves the
// unmatched cards of the current row into the vacated cells.
func (s *session) moveRemainingUp() {
	buttonsToMove := []*tview.Button{}
	for i := range 4 {
		for j := range 4 {
			button := s.buttons[i][j]
			wasSelected := s.state.selectedCards[button.GetLabel()]
			if i == s.state.currentMatchRow && !wasSelected {
				buttonsToMove = append(buttonsToMove, button)
				s.grid.RemoveItem(button)
			}
			if wasSelected {
				s.grid.RemoveItem(button)
				if i != s.state.currentMatchRow && len(buttonsToMove) > 0 {
					s.grid.AddItem(buttonsToMove[0], i, j, 1, 1, 0, 0, false)
					s.buttons[i][j] = buttonsToMove[0]
					buttonsToMove = buttonsToMove[1:]
				}
			}
		}
	}
}

// isRune reports whether the event is the given rune key.
func isRune(event *tcell.EventKey, r rune) bool {
	return event.Key() == tcell.KeyRune && event.Rune() == r
}

// navigate applies a movement key to the focus position, reporting whether
// the event was a movement key.
func (s *session) navigate(event *tcell.EventKey, r, c int) (newR, newC int, ok bool) {
	switch {
	case event.Key() == tcell.KeyUp, isRune(event, 'k'):
		if r > s.state.currentMatchRow {
			r--
		}
	case event.Key() == tcell.KeyDown, isRune(event, 'j'):
		if r < 4 {
			r++
		}
	case event.Key() == tcell.KeyLeft, isRune(event, 'h'):
		if c > 0 {
			if r == 4 && c == 2 {
				c--
			}
			c--
		}
	case event.Key() == tcell.KeyRight, isRune(event, 'l'):
		if c < 3 {
			if r == 4 && c == 1 {
				c++
			}
			c++
		}
	default:
		return r, c, false
	}
	s.resetSubmitButton()
	return r, c, true
}

// activate handles Enter/space on the button at (r, c).
func (s *session) activate(r, c int) {
	if r != 4 {
		s.toggleCard(r, c)
		s.resetSubmitButton()
		return
	}
	if s.gameOver {
		s.handleShare()
		return
	}
	switch c {
	case 0:
		s.handleShuffle()
	case 3:
		s.handleDeselect()
	default:
		s.handleSubmit()
	}
}

func (s *session) handleKey(event *tcell.EventKey) *tcell.EventKey {
	r := s.focusedRow
	c := s.focusedCol

	if nr, nc, ok := s.navigate(event, r, c); ok {
		r, c = nr, nc
	} else {
		switch {
		case isRune(event, 'q'):
			s.app.Stop()
		case isRune(event, 'a') && !s.gameOver:
			s.handleShuffle()
			r, c = s.focusedRow, s.focusedCol
		case isRune(event, 's') && !s.gameOver:
			s.handleSubmit()
			if r < s.state.currentMatchRow {
				r++
			}
		case isRune(event, 'd') && !s.gameOver:
			s.handleDeselect()
		case event.Key() == tcell.KeyEnter, isRune(event, ' '):
			s.activate(r, c)
		default:
			return nil
		}
	}

	s.setFocus(r, c)
	return nil
}
