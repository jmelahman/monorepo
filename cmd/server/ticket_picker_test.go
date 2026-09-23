package server

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

// attachAction is the picker action most of these tests run under; the
// header and footer it produces are asserted in TestTicketPickerRender.
var attachAction = pickerAction{"Attach to ticket", "attach"}

func pickerItems() []pickerItem {
	return []pickerItem{
		{ID: 3, Title: "Fix the login bug", Column: "Todo", Status: ""},
		{ID: 12, Title: "Write docs", Column: "Todo", Status: "stopped"},
		{ID: 7, Title: "Wire CI", Column: "In Progress", Status: "working"},
		{ID: 9, Title: "Bug bash", Column: "Done", Status: "idle"},
	}
}

func pickerType(p *ticketPicker, s string) {
	for _, r := range s {
		p.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
}

// screenLines is screenText with each row's trailing padding removed, so a
// test can assert on what ends a row.
func screenLines(s tcell.SimulationScreen) string {
	lines := strings.Split(screenText(s), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return strings.Join(lines, "\n")
}

func TestTicketPickerNavigateAndSelect(t *testing.T) {
	p := newTicketPicker(attachAction, "Board (b)", pickerItems())
	if got := p.current().ID; got != 3 {
		t.Fatalf("initial selection = #%d, want #3", got)
	}
	p.handleKey(formKey(tcell.KeyUp)) // clamps at the top
	if got := p.current().ID; got != 3 {
		t.Errorf("Up at the top moved to #%d", got)
	}
	p.handleKey(formKey(tcell.KeyDown))
	p.handleKey(formKey(tcell.KeyCtrlN))
	if got := p.current().ID; got != 7 {
		t.Errorf("after Down, Ctrl+N: #%d, want #7", got)
	}
	p.handleKey(formKey(tcell.KeyCtrlP))
	if got := p.current().ID; got != 12 {
		t.Errorf("after Ctrl+P: #%d, want #12", got)
	}
	p.handleKey(formKey(tcell.KeyEnd))
	if got := p.current().ID; got != 9 {
		t.Errorf("after End: #%d, want #9", got)
	}
	p.handleKey(formKey(tcell.KeyDown)) // clamps at the bottom
	if got := p.current().ID; got != 9 {
		t.Errorf("Down at the bottom moved to #%d", got)
	}
	p.handleKey(formKey(tcell.KeyHome))
	if got := p.current().ID; got != 3 {
		t.Errorf("after Home: #%d, want #3", got)
	}
	p.pageRows = 2
	p.handleKey(formKey(tcell.KeyPgDn))
	if got := p.current().ID; got != 7 {
		t.Errorf("after PgDn: #%d, want #7", got)
	}
	p.handleKey(formKey(tcell.KeyPgUp))
	if got := p.current().ID; got != 3 {
		t.Errorf("after PgUp: #%d, want #3", got)
	}
	p.handleKey(formKey(tcell.KeyDown))
	p.handleKey(formKey(tcell.KeyEnter))
	if !p.selected || p.cancelled {
		t.Fatalf("Enter did not select: selected=%v cancelled=%v", p.selected, p.cancelled)
	}
	if got := p.current().ID; got != 12 {
		t.Errorf("selected #%d, want #12", got)
	}
}

func TestTicketPickerFilter(t *testing.T) {
	p := newTicketPicker(attachAction, "b", pickerItems())
	p.handleKey(formKey(tcell.KeyEnd))
	pickerType(p, "bug")
	if p.cursor != 0 {
		t.Errorf("typing should reset the cursor to the top; cursor = %d", p.cursor)
	}
	if vis := p.visible(); len(vis) != 2 || vis[0] != 0 || vis[1] != 3 {
		t.Errorf("visible = %v, want [0 3] (case-insensitive title match)", vis)
	}
	p.handleKey(formKey(tcell.KeyDown))
	if got := p.current().ID; got != 9 {
		t.Errorf("Down in the filtered list: #%d, want #9", got)
	}

	// Several terms must all match, across title, column, and status.
	pickerType(p, " done")
	if vis := p.visible(); len(vis) != 1 || p.items[vis[0]].ID != 9 {
		t.Errorf("visible = %v, want just #9", vis)
	}
	p.handleKey(formKey(tcell.KeyCtrlU))
	pickerType(p, "#12")
	if vis := p.visible(); len(vis) != 1 || p.items[vis[0]].ID != 12 {
		t.Errorf("visible = %v, want just #12 (id match)", vis)
	}
	p.handleKey(formKey(tcell.KeyCtrlU))
	pickerType(p, "working")
	if vis := p.visible(); len(vis) != 1 || p.items[vis[0]].ID != 7 {
		t.Errorf("visible = %v, want just #7 (status match)", vis)
	}

	// No match: Enter is refused with a message, and the current item is
	// the zero value; Backspace widens the list again.
	pickerType(p, "zzz")
	if vis := p.visible(); len(vis) != 0 {
		t.Fatalf("visible = %v, want none", vis)
	}
	if got := p.current(); got.ID != 0 {
		t.Errorf("current with no matches = %+v", got)
	}
	p.handleKey(formKey(tcell.KeyEnter))
	if p.selected {
		t.Fatal("Enter selected with no matching ticket")
	}
	if p.errMsg != "no tickets match the filter" {
		t.Errorf("errMsg = %q", p.errMsg)
	}
	for i := 0; i < 3; i++ {
		p.handleKey(formKey(tcell.KeyBackspace2))
	}
	if p.errMsg != "" {
		t.Errorf("editing should clear the error; errMsg = %q", p.errMsg)
	}
	p.handleKey(formKey(tcell.KeyEnter))
	if !p.selected || p.current().ID != 7 {
		t.Errorf("selected=%v current=#%d, want #7", p.selected, p.current().ID)
	}
}

func TestTicketPickerFilterEditing(t *testing.T) {
	p := newTicketPicker(attachAction, "b", pickerItems())
	pickerType(p, "one two")
	p.handleKey(formKey(tcell.KeyCtrlW))
	if got := p.filter.String(); got != "one " {
		t.Errorf("after Ctrl+W: %q", got)
	}
	p.handleKey(formKey(tcell.KeyCtrlA))
	p.handleKey(formKey(tcell.KeyDelete))
	if got := p.filter.String(); got != "ne " {
		t.Errorf("after Ctrl+A, Delete: %q", got)
	}
	p.handleKey(formKey(tcell.KeyRight))
	p.handleKey(formKey(tcell.KeyCtrlK))
	if got := p.filter.String(); got != "n" {
		t.Errorf("after Right, Ctrl+K: %q", got)
	}
	p.handleKey(formKey(tcell.KeyLeft))
	p.handleKey(formKey(tcell.KeyCtrlE))
	pickerType(p, "!")
	if got := p.filter.String(); got != "n!" {
		t.Errorf("after Left, Ctrl+E, type: %q", got)
	}
	p.handleKey(tcell.NewEventKey(tcell.KeyRune, 'z', tcell.ModAlt))
	if got := p.filter.String(); got != "n!" {
		t.Errorf("Alt+rune should be ignored: %q", got)
	}
	// Enter in the filter never inserts a newline: it's the select key.
	p.handleKey(formKey(tcell.KeyCtrlJ))
	if p.filter.String() != "n!" {
		t.Errorf("Ctrl+J changed the filter: %q", p.filter.String())
	}
}

func TestTicketPickerCancel(t *testing.T) {
	for _, k := range []tcell.Key{tcell.KeyEscape, tcell.KeyCtrlC} {
		p := newTicketPicker(attachAction, "b", pickerItems())
		pickerType(p, "x")
		p.handleKey(formKey(k))
		if !p.cancelled || p.selected {
			t.Errorf("key %v: cancelled=%v selected=%v", k, p.cancelled, p.selected)
		}
	}
}

func TestTicketPickerRender(t *testing.T) {
	screen := newFormScreen(t, 60, 16)
	p := newTicketPicker(attachAction, "CLI Board (cli-board)", pickerItems())
	p.render(screen)
	screen.Show()

	text := screenText(screen)
	for _, want := range []string{
		"Attach to ticket · CLI Board (cli-board)",
		"> type to filter",
		"Todo",
		"#3   Fix the login bug",
		"no session",
		"#12  Write docs",
		"stopped",
		"In Progress",
		"#7   Wire CI",
		"working",
		"Done",
		"#9   Bug bash",
		"idle",
		"↑↓ move · Enter attach · type to filter · Esc cancel",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("screen missing %q:\n%s", want, text)
		}
	}
	// The cursor sits in the filter line, right after the prompt.
	if x, y, vis := screen.GetCursor(); !vis || x != formPad+2 || y != pickerFilterRow {
		t.Errorf("cursor = (%d,%d,%v), want (%d,%d,true)", x, y, vis, formPad+2, pickerFilterRow)
	}
	// The highlighted row (first ticket, under the "Todo" heading) is drawn
	// in reverse video; its neighbour isn't.
	cells, w, _ := screen.GetContents()
	rowStyle := func(y int) tcell.Style { return cells[y*w+formPad+2].Style }
	_, _, attrs := rowStyle(pickerListRow + 1).Decompose()
	if attrs&tcell.AttrReverse == 0 {
		t.Errorf("highlighted row not in reverse video")
	}
	if _, _, attrs := rowStyle(pickerListRow + 2).Decompose(); attrs&tcell.AttrReverse != 0 {
		t.Errorf("unhighlighted row drawn in reverse video")
	}

	pickerType(p, "nothing here")
	p.handleKey(formKey(tcell.KeyEnter))
	p.render(screen)
	screen.Show()
	text = screenText(screen)
	if !strings.Contains(text, "> nothing here") || strings.Contains(text, "> type to filter") {
		t.Errorf("filter text not rendered:\n%s", text)
	}
	if !strings.Contains(text, "no tickets match") || !strings.Contains(text, "no tickets match the filter") {
		t.Errorf("empty-list notice / error row missing:\n%s", text)
	}
	if strings.Contains(text, "Wire CI") {
		t.Errorf("filtered-out ticket still rendered:\n%s", text)
	}

	screen.SetSize(40, 5)
	p.render(screen)
	screen.Show()
	if text := screenText(screen); !strings.Contains(text, "too small") {
		t.Errorf("small-terminal notice missing:\n%s", text)
	}
}

func TestTicketPickerRenderTruncatesTitle(t *testing.T) {
	screen := newFormScreen(t, 40, 12)
	long := strings.Repeat("abcdefghij", 6)
	p := newTicketPicker(attachAction, "b", []pickerItem{{ID: 1, Title: long, Column: "Todo", Status: "working"}})
	p.render(screen)
	screen.Show()
	text := screenLines(screen)
	if !strings.Contains(text, "…") {
		t.Errorf("long title not truncated:\n%s", text)
	}
	if !strings.Contains(text, "working\n") {
		t.Errorf("status not right-aligned on the row:\n%s", text)
	}
	if strings.Contains(text, long) {
		t.Errorf("full title rendered on a narrow screen:\n%s", text)
	}
	if got := truncateText("日本語テキスト", 7); got != "日本語…" {
		t.Errorf("truncateText wide runes = %q", got)
	}
	if got := truncateText("short", 10); got != "short" {
		t.Errorf("truncateText(short) = %q", got)
	}
}

func TestTicketPickerScrolls(t *testing.T) {
	// 8 rows tall: list gets 8 - 4 - 3 = 1 row.
	screen := newFormScreen(t, 40, minPickerHeight)
	var items []pickerItem
	for i := 1; i <= 6; i++ {
		items = append(items, pickerItem{ID: int64(i), Title: "ticket " + string(rune('a'+i-1)), Column: "Todo"})
	}
	p := newTicketPicker(attachAction, "b", items)
	p.render(screen)
	screen.Show()
	// With one visible row, the heading is scrolled away for the cursor.
	if text := screenText(screen); !strings.Contains(text, "ticket a") || strings.Contains(text, "ticket b") {
		t.Errorf("expected only the highlighted row:\n%s", text)
	}
	p.handleKey(formKey(tcell.KeyEnd))
	p.render(screen)
	screen.Show()
	if text := screenText(screen); !strings.Contains(text, "ticket f") {
		t.Errorf("cursor row scrolled out of view:\n%s", text)
	}

	// With room for the heading, scrolling back to the first ticket brings
	// its column heading along.
	screen.SetSize(40, minPickerHeight+2)
	p.handleKey(formKey(tcell.KeyHome))
	p.render(screen)
	screen.Show()
	text := screenLines(screen)
	if !strings.Contains(text, "Todo\n") || !strings.Contains(text, "ticket a") {
		t.Errorf("heading not shown above the first ticket:\n%s", text)
	}
	if p.scroll != 0 {
		t.Errorf("scroll = %d, want 0", p.scroll)
	}
}

// TestRunTicketPickerEndToEnd drives the real event loop with a
// simulation screen: filter, pick, and cancel.
func TestRunTicketPickerEndToEnd(t *testing.T) {
	screen := newFormScreen(t, 60, 20)
	type outcome struct {
		item pickerItem
		ok   bool
		err  error
	}
	run := func(p *ticketPicker) chan outcome {
		done := make(chan outcome, 1)
		go func() {
			item, ok, err := runTicketPicker(screen, p)
			done <- outcome{item, ok, err}
		}()
		return done
	}
	wait := func(t *testing.T, done chan outcome) outcome {
		t.Helper()
		select {
		case o := <-done:
			return o
		case <-time.After(10 * time.Second):
			t.Fatal("picker did not finish")
			return outcome{}
		}
	}

	done := run(newTicketPicker(attachAction, "b", pickerItems()))
	for _, r := range "ci" {
		screen.InjectKey(tcell.KeyRune, r, tcell.ModNone)
	}
	screen.InjectKey(tcell.KeyEnter, 0, tcell.ModNone)
	o := wait(t, done)
	if o.err != nil || !o.ok {
		t.Fatalf("ok=%v err=%v", o.ok, o.err)
	}
	if o.item.ID != 7 {
		t.Errorf("picked #%d, want #7 (Wire CI)", o.item.ID)
	}

	done = run(newTicketPicker(attachAction, "b", pickerItems()))
	screen.InjectKey(tcell.KeyDown, 0, tcell.ModNone)
	screen.InjectKey(tcell.KeyDown, 0, tcell.ModNone)
	screen.InjectKey(tcell.KeyEnter, 0, tcell.ModNone)
	if o := wait(t, done); o.err != nil || !o.ok || o.item.ID != 7 {
		t.Errorf("arrow selection: ok=%v err=%v item=#%d, want #7", o.ok, o.err, o.item.ID)
	}

	done = run(newTicketPicker(attachAction, "b", pickerItems()))
	screen.InjectKey(tcell.KeyEscape, 0, tcell.ModNone)
	if o := wait(t, done); o.err != nil || o.ok {
		t.Errorf("cancel: ok=%v err=%v", o.ok, o.err)
	}
}
