package server

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"

	"github.com/jmelahman/kanban/internal/client"
)

// pickerItem is one selectable row of the `ticket attach` list: a ticket
// plus the context that tells similar tickets apart (its column and the
// state of its session, if any).
type pickerItem struct {
	ID      int64
	Title   string
	Column  string
	Status  string // session status; "" when the ticket has no session yet
	Harness string // the session's own harness choice; "" for the default
}

// loadBoardTickets fetches a board's tickets in board order (columns left to
// right, tickets top to bottom) with each ticket's session status, and the
// board label the picker shows in its header. With archived set it lists the
// board's archived tickets instead of its open ones, for the subcommands
// (`unarchive`, `delete`) that only ever act on those.
func loadBoardTickets(ctx context.Context, url, ident string, archived bool) (label string, items []pickerItem, err error) {
	c := client.New(url, nil)
	id, err := c.ResolveBoardID(ctx, ident)
	if err != nil {
		return "", nil, err
	}
	raw, err := c.BoardState(ctx, id)
	if err != nil {
		return "", nil, err
	}
	var st struct {
		Board struct {
			Name string `json:"name"`
			Slug string `json:"slug"`
		} `json:"board"`
		Columns []struct {
			ID       int64  `json:"id"`
			Name     string `json:"name"`
			Position int    `json:"position"`
		} `json:"columns"`
		Tickets  []client.Ticket `json:"tickets"`
		Sessions []struct {
			TicketID int64  `json:"ticket_id"`
			Status   string `json:"status"`
			Harness  string `json:"harness"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		return "", nil, fmt.Errorf("decode board state: %w", err)
	}

	type column struct {
		name     string
		position int
	}
	columns := make(map[int64]column, len(st.Columns))
	for _, col := range st.Columns {
		columns[col.ID] = column{name: col.Name, position: col.Position}
	}
	status := make(map[int64]string, len(st.Sessions))
	harnesses := make(map[int64]string, len(st.Sessions))
	for _, s := range st.Sessions {
		status[s.TicketID] = s.Status
		harnesses[s.TicketID] = s.Harness
	}
	// Board state carries only open tickets; archived ones come from their
	// own endpoint but are grouped by the same columns.
	tickets := append([]client.Ticket(nil), st.Tickets...)
	if archived {
		rawArchived, err := c.ListArchived(ctx, id)
		if err != nil {
			return "", nil, err
		}
		tickets = nil
		if err := json.Unmarshal(rawArchived, &tickets); err != nil {
			return "", nil, fmt.Errorf("decode archived tickets: %w", err)
		}
	}
	sort.SliceStable(tickets, func(i, j int) bool {
		ci, cj := columns[tickets[i].ColumnID], columns[tickets[j].ColumnID]
		if ci.position != cj.position {
			return ci.position < cj.position
		}
		if tickets[i].Position != tickets[j].Position {
			return tickets[i].Position < tickets[j].Position
		}
		return tickets[i].ID < tickets[j].ID
	})
	items = make([]pickerItem, 0, len(tickets))
	for _, t := range tickets {
		items = append(items, pickerItem{
			ID:      t.ID,
			Title:   t.Title,
			Column:  columns[t.ColumnID].name,
			Status:  status[t.ID],
			Harness: harnesses[t.ID],
		})
	}
	return formatBoardLabel(st.Board.Name, st.Board.Slug), items, nil
}

// pickerAction names what a run of the picker stands in for: the heading it
// shows and the verb on its Enter key. Every `kanban ticket` subcommand
// invoked without a ticket id opens the same list under its own action, so
// the screen says which one you're about to run.
type pickerAction struct {
	title string // heading, e.g. "Archive ticket"
	verb  string // Enter's label in the footer, e.g. "archive"
}

// promptTicketPicker takes over the terminal with a tcell screen, runs the
// ticket list, and returns the chosen ticket. ok is false when the user
// cancelled (Esc / Ctrl+C). A non-nil harnesses adds a harness row for the
// highlighted ticket (used by `ticket attach`); chosen.Harness is then the
// harness picked for it — see ticketPicker.chosen.
func promptTicketPicker(action pickerAction, boardLabel string, items []pickerItem, harnesses *harnessOptions) (chosen pickerItem, ok bool, err error) {
	screen, err := tcell.NewScreen()
	if err != nil {
		return chosen, false, fmt.Errorf("open terminal: %w", err)
	}
	if err := screen.Init(); err != nil {
		return chosen, false, fmt.Errorf("init terminal: %w", err)
	}
	// Fini restores the terminal; make sure it runs even if the loop panics,
	// otherwise the shell is left in raw mode with the alternate screen up.
	defer func() {
		screen.Fini()
		if r := recover(); r != nil {
			panic(r)
		}
	}()
	p := newTicketPicker(action, boardLabel, items)
	p.harnesses = harnesses
	return runTicketPicker(screen, p)
}

// runTicketPicker is the event loop, split from promptTicketPicker so tests
// can drive it with a tcell.SimulationScreen.
func runTicketPicker(screen tcell.Screen, p *ticketPicker) (pickerItem, bool, error) {
	for {
		p.render(screen)
		screen.Show()
		switch ev := screen.PollEvent().(type) {
		case nil:
			// Screen was finalized underneath us.
			return pickerItem{}, false, nil
		case *tcell.EventResize:
			screen.Sync()
		case *tcell.EventKey:
			p.handleKey(ev)
		}
		if p.selected {
			return p.chosen(), true, nil
		}
		if p.cancelled {
			return pickerItem{}, false, nil
		}
	}
}

// ---------- model ----------

// ticketPicker is the state behind the list: the tickets, a filter line,
// and which visible row is highlighted. Like ticketForm it knows nothing
// about the screen so key handling can be unit-tested without a terminal.
type ticketPicker struct {
	action     pickerAction
	boardLabel string
	items      []pickerItem
	filter     *textBuffer
	cursor     int // index into visible(), reset to the top when the filter changes
	errMsg     string
	selected   bool
	cancelled  bool

	// harnesses is nil unless the picker offers a harness row; picked holds
	// the ←/→ choices per ticket id (index into harnesses.list), so moving
	// the highlight away and back keeps what was chosen for that ticket.
	harnesses *harnessOptions
	picked    map[int64]int

	// Owned by render(): the first list row on screen and how many list
	// rows fit, which PgUp/PgDn use as their stride.
	scroll   int
	pageRows int
}

func newTicketPicker(action pickerAction, boardLabel string, items []pickerItem) *ticketPicker {
	return &ticketPicker{
		action:     action,
		boardLabel: boardLabel,
		items:      items,
		filter:     newTextBuffer("", false),
		pageRows:   10,
	}
}

// visible returns the indices into items that match the filter: every
// whitespace-separated term must appear (case-insensitively) in the row's
// "#id title column status" text, so "#12", "bug", or "progress idle" all
// narrow the list the way you'd expect.
func (p *ticketPicker) visible() []int {
	terms := strings.Fields(strings.ToLower(p.filter.String()))
	idx := make([]int, 0, len(p.items))
	for i, it := range p.items {
		if len(terms) == 0 {
			idx = append(idx, i)
			continue
		}
		hay := strings.ToLower("#" + strconv.FormatInt(it.ID, 10) + " " + it.Title + " " + it.Column + " " + it.Status)
		match := true
		for _, term := range terms {
			if !strings.Contains(hay, term) {
				match = false
				break
			}
		}
		if match {
			idx = append(idx, i)
		}
	}
	return idx
}

// harnessIdx is the harness shown for it: the ←/→ choice, else the
// session's stored harness, else the default.
func (p *ticketPicker) harnessIdx(it pickerItem) int {
	if i, ok := p.picked[it.ID]; ok {
		return i
	}
	return p.harnesses.initial(it.Harness)
}

// harnessChanged reports whether the harness shown for it differs from the
// one its session launches today.
func (p *ticketPicker) harnessChanged(it pickerItem) bool {
	return p.harnessIdx(it) != p.harnesses.initial(it.Harness)
}

// cycleHarness steps the highlighted ticket's harness choice.
func (p *ticketPicker) cycleHarness(delta int) {
	vis := p.visible()
	if len(vis) == 0 {
		return
	}
	it := p.current()
	if p.picked == nil {
		p.picked = map[int64]int{}
	}
	p.picked[it.ID] = p.harnesses.cycle(p.harnessIdx(it), delta)
}

// chosen is the highlighted item as the caller should act on it: with a
// harness row, Harness is replaced by the ID picked with ←/→, or cleared
// when the pick matches what the session already launches (nothing to
// change).
func (p *ticketPicker) chosen() pickerItem {
	it := p.current()
	if p.harnesses == nil {
		return it
	}
	if p.harnessChanged(it) {
		it.Harness = p.harnesses.id(p.harnessIdx(it))
	} else {
		it.Harness = ""
	}
	return it
}

// current returns the highlighted item, or the zero item when the filter
// matches nothing.
func (p *ticketPicker) current() pickerItem {
	vis := p.visible()
	if len(vis) == 0 {
		return pickerItem{}
	}
	p.clampCursor(len(vis))
	return p.items[vis[p.cursor]]
}

func (p *ticketPicker) clampCursor(n int) {
	if p.cursor >= n {
		p.cursor = n - 1
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
}

func (p *ticketPicker) move(delta int) {
	n := len(p.visible())
	if n == 0 {
		p.cursor = 0
		return
	}
	p.cursor += delta
	p.clampCursor(n)
}

func (p *ticketPicker) submit() {
	if len(p.visible()) == 0 {
		p.errMsg = "no tickets match the filter"
		return
	}
	p.selected = true
}

// handleKey applies one key event. Bindings:
//
//	Up / Down, Ctrl+P / Ctrl+N   move the highlight
//	PgUp / PgDn, Home / End      move by a page / to either end
//	Enter                        run the action on the highlighted ticket
//	Esc / Ctrl+C                 cancel
//	printable keys               narrow the list; Backspace widens it again
//	Left / Right                 cycle the harness when there's a harness row,
//	                             else move in the filter
//	Ctrl+B / Ctrl+F, Ctrl+A / Ctrl+E, Ctrl+U / Ctrl+K / Ctrl+W   edit the filter
func (p *ticketPicker) handleKey(ev *tcell.EventKey) {
	p.errMsg = ""
	before := p.filter.String()
	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyCtrlC:
		p.cancelled = true
	case tcell.KeyEnter, tcell.KeyCtrlJ:
		p.submit()
	case tcell.KeyUp, tcell.KeyCtrlP:
		p.move(-1)
	case tcell.KeyDown, tcell.KeyCtrlN:
		p.move(1)
	case tcell.KeyPgUp:
		p.move(-p.pageRows)
	case tcell.KeyPgDn:
		p.move(p.pageRows)
	case tcell.KeyHome:
		p.cursor = 0
	case tcell.KeyEnd:
		p.move(len(p.items))
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		p.filter.backspace()
	case tcell.KeyDelete:
		p.filter.del()
	case tcell.KeyLeft:
		if p.harnesses != nil {
			p.cycleHarness(-1)
			return
		}
		p.filter.left()
	case tcell.KeyRight:
		if p.harnesses != nil {
			p.cycleHarness(1)
			return
		}
		p.filter.right()
	case tcell.KeyCtrlB:
		p.filter.left()
	case tcell.KeyCtrlF:
		p.filter.right()
	case tcell.KeyCtrlA:
		p.filter.home()
	case tcell.KeyCtrlE:
		p.filter.end()
	case tcell.KeyCtrlK:
		p.filter.killToEnd()
	case tcell.KeyCtrlU:
		p.filter.killToStart()
	case tcell.KeyCtrlW:
		p.filter.deleteWordBack()
	case tcell.KeyRune:
		if ev.Modifiers()&(tcell.ModCtrl|tcell.ModAlt|tcell.ModMeta) != 0 {
			return
		}
		p.filter.insert(ev.Rune())
	}
	// A changed filter reorders what's under the cursor; start from the top
	// like fzf does rather than landing on an arbitrary row.
	if p.filter.String() != before {
		p.cursor = 0
		p.scroll = 0
	}
}

// ---------- rendering ----------

const (
	pickerFilterRow  = 2
	pickerListRow    = 4
	pickerFooterRows = 3 // blank + help + error
	minPickerWidth   = 24
	minPickerHeight  = 8
	noSessionLabel   = "no session"
)

// pickerRow is one screen row of the list: a column heading or an item.
type pickerRow struct {
	header string
	item   int // index into items; valid when header == ""
}

// rows lays the visible items out with a heading wherever the column
// changes, so the list reads like the board.
func (p *ticketPicker) rows() (rows []pickerRow, cursorRow int) {
	vis := p.visible()
	p.clampCursor(len(vis))
	prev := ""
	for i, idx := range vis {
		if col := p.items[idx].Column; i == 0 || col != prev {
			rows = append(rows, pickerRow{header: col})
			prev = col
		}
		if i == p.cursor {
			cursorRow = len(rows)
		}
		rows = append(rows, pickerRow{item: idx})
	}
	return rows, cursorRow
}

func (p *ticketPicker) render(s tcell.Screen) {
	s.Clear()
	w, h := s.Size()
	base := tcell.StyleDefault
	if w < minPickerWidth || h < minPickerHeight {
		putText(s, 0, 0, w, base, "terminal too small for the ticket list")
		s.HideCursor()
		return
	}
	width := w - 2*formPad

	putText(s, formPad, 0, width, base.Bold(true), p.action.title+" · "+p.boardLabel)

	// Filter line: a prompt plus the single-line buffer, scrolled so the
	// cursor stays on screen.
	const prompt = "> "
	putText(s, formPad, pickerFilterRow, width, base.Bold(true), prompt)
	fx := formPad + len(prompt)
	fw := width - len(prompt)
	line := p.filter.lines[0]
	cursor := runesWidth(line[:p.filter.col])
	scroll := 0
	if cursor >= fw {
		scroll = cursor - fw + 1
	}
	pos := 0
	for _, r := range line {
		rw := runeWidth(r)
		if pos >= scroll && pos+rw <= scroll+fw {
			s.SetContent(fx+pos-scroll, pickerFilterRow, displayRune(r), nil, base)
		}
		pos += rw
	}
	if len(line) == 0 {
		putText(s, fx, pickerFilterRow, fw, base.Dim(true), "type to filter")
	}
	s.ShowCursor(fx+cursor-scroll, pickerFilterRow)

	// List.
	footer := pickerFooterRows
	if p.harnesses != nil {
		footer++ // the harness row, between the list and the help line
	}
	listH := h - footer - pickerListRow
	if listH < 1 {
		listH = 1
	}
	p.pageRows = listH
	rows, cursorRow := p.rows()
	if len(rows) == 0 {
		putText(s, formPad, pickerListRow, width, base.Dim(true), "no tickets match")
	}
	if cursorRow < p.scroll {
		p.scroll = cursorRow
		// Keep the column heading with its first ticket when scrolling up.
		if cursorRow > 0 && rows[cursorRow-1].header != "" {
			p.scroll = cursorRow - 1
		}
	}
	if cursorRow >= p.scroll+listH {
		p.scroll = cursorRow - listH + 1
	}
	if p.scroll > len(rows)-listH {
		p.scroll = max(0, len(rows)-listH)
	}
	idWidth := 0
	for _, it := range p.items {
		if n := len(strconv.FormatInt(it.ID, 10)); n > idWidth {
			idWidth = n
		}
	}
	for i := p.scroll; i < len(rows) && i-p.scroll < listH; i++ {
		y := pickerListRow + i - p.scroll
		row := rows[i]
		if row.header != "" {
			putText(s, formPad, y, width, base.Bold(true).Dim(true), row.header)
			continue
		}
		p.renderItem(s, formPad, y, width, idWidth, p.items[row.item], i == cursorRow)
	}

	help := "↑↓ move · Enter " + p.action.verb + " · type to filter · Esc cancel"
	if p.harnesses != nil {
		help = "↑↓ move · ←→ harness · Enter " + p.action.verb + " · type to filter · Esc cancel"
		if vis := p.visible(); len(vis) > 0 {
			it := p.current()
			note := ""
			if p.harnessChanged(it) && it.Status != "" && it.Status != "stopped" && it.Status != "error" {
				note = "restarts the running agent"
			}
			drawHarnessRow(s, formPad, h-3, width, true, p.harnesses.label(p.harnessIdx(it)), note)
		}
	}
	putText(s, formPad, h-2, width, base.Dim(true), help)
	if p.errMsg != "" {
		putText(s, formPad, h-1, width, base.Foreground(tcell.ColorRed).Bold(true), p.errMsg)
	}
}

// renderItem draws "  #id  title …  status" on one row. The highlighted
// row is drawn in reverse video across the full width; the status sits at
// the right edge and the title is cut with an ellipsis to make room.
func (p *ticketPicker) renderItem(s tcell.Screen, x, y, width, idWidth int, it pickerItem, highlighted bool) {
	style := tcell.StyleDefault
	if highlighted {
		style = style.Reverse(true).Bold(true)
		for i := 0; i < width; i++ {
			s.SetContent(x+i, y, ' ', nil, style)
		}
	}
	status := it.Status
	statusStyle := style
	if status == "" {
		status = noSessionLabel
		statusStyle = style.Dim(true)
	} else if status == "stopped" || status == "error" {
		statusStyle = style.Dim(true)
	}
	prefix := fmt.Sprintf("  #%-*d  ", idWidth, it.ID)
	titleW := width - len(prefix) - len(status) - 2
	if titleW < 4 {
		// Too narrow for a status column; give the whole row to the title.
		status, titleW = "", width-len(prefix)
	}
	putText(s, x, y, width, style, prefix+truncateText(it.Title, titleW))
	if status != "" {
		putText(s, x+width-len(status), y, len(status), statusStyle, status)
	}
}

// truncateText fits text into width cells, replacing the tail with an
// ellipsis when it doesn't fit.
func truncateText(text string, width int) string {
	if width <= 0 {
		return ""
	}
	rs := []rune(text)
	if runesWidth(rs) <= width {
		return text
	}
	pos := 0
	for i, r := range rs {
		rw := runeWidth(r)
		if pos+rw > width-1 {
			return string(rs[:i]) + "…"
		}
		pos += rw
	}
	return text
}

// formatBoardLabel renders "Name (slug)" for headers, collapsing to just
// the name when the slug adds nothing.
func formatBoardLabel(name, slug string) string {
	if slug == "" || slug == name {
		return name
	}
	return name + " (" + slug + ")"
}
