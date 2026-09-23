package server

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

func formKey(k tcell.Key) *tcell.EventKey { return tcell.NewEventKey(k, 0, tcell.ModNone) }

func formType(f *ticketForm, s string) {
	for _, r := range s {
		f.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
}

func TestTicketFormTypeAndSubmit(t *testing.T) {
	f := newTicketForm("Board (b)", "", "")
	if f.focus != focusTitle {
		t.Fatalf("initial focus = %d, want title", f.focus)
	}
	formType(f, "  Fix the bug ")
	f.handleKey(formKey(tcell.KeyEnter))
	if f.focus != focusBody {
		t.Fatalf("Enter in the title should focus the body; focus = %d", f.focus)
	}
	formType(f, "line one")
	f.handleKey(formKey(tcell.KeyEnter))
	formType(f, "line two")
	f.handleKey(formKey(tcell.KeyCtrlS))
	if !f.submitted {
		t.Fatal("Ctrl+S did not submit")
	}
	got := f.result()
	want := ticketFormResult{Title: "Fix the bug", Body: "line one\nline two"}
	if got != want {
		t.Errorf("result = %+v, want %+v", got, want)
	}
}

func TestTicketFormRequiresTitle(t *testing.T) {
	f := newTicketForm("b", "", "")
	f.handleKey(formKey(tcell.KeyTab))
	formType(f, "description only")
	f.handleKey(formKey(tcell.KeyCtrlS))
	if f.submitted {
		t.Fatal("submitted with an empty title")
	}
	if f.errMsg != "a title is required" {
		t.Errorf("errMsg = %q", f.errMsg)
	}
	if f.focus != focusTitle {
		t.Errorf("focus = %d, want title after rejected submit", f.focus)
	}
	// A whitespace-only title is still empty.
	formType(f, "   ")
	if f.errMsg != "" {
		t.Errorf("typing should clear the error; errMsg = %q", f.errMsg)
	}
	f.handleKey(formKey(tcell.KeyCtrlS))
	if f.submitted {
		t.Fatal("submitted with a blank title")
	}
	formType(f, "T")
	f.handleKey(formKey(tcell.KeyCtrlS))
	if !f.submitted {
		t.Fatal("did not submit once a title was typed")
	}
	if got := f.result(); got.Title != "T" || got.Body != "description only" {
		t.Errorf("result = %+v", got)
	}
}

func TestTicketFormCancel(t *testing.T) {
	for _, k := range []tcell.Key{tcell.KeyEscape, tcell.KeyCtrlC} {
		f := newTicketForm("b", "", "")
		formType(f, "x")
		f.handleKey(formKey(k))
		if !f.cancelled || f.submitted {
			t.Errorf("key %v: cancelled=%v submitted=%v", k, f.cancelled, f.submitted)
		}
	}
}

func TestTicketFormPasteMode(t *testing.T) {
	f := newTicketForm("b", "", "")
	f.pasting = true
	formType(f, "pasted title")
	f.handleKey(formKey(tcell.KeyEnter)) // a pasted trailing newline must not move focus
	if f.focus != focusTitle {
		t.Errorf("Enter while pasting switched focus")
	}
	f.handleKey(formKey(tcell.KeyTab)) // pasted tabs are literal
	if got := f.title.String(); got != "pasted title\t" {
		t.Errorf("title = %q", got)
	}
	f.pasting = false
	f.handleKey(formKey(tcell.KeyTab))
	if f.focus != focusBody {
		t.Errorf("Tab after paste should switch fields")
	}
	f.handleKey(formKey(tcell.KeyCtrlS))
	if got := f.result().Title; got != "pasted title" {
		t.Errorf("Title = %q, want trailing tab trimmed", got)
	}
}

func TestTicketFormEditing(t *testing.T) {
	f := newTicketForm("b", "", "")
	f.handleKey(formKey(tcell.KeyTab))
	formType(f, "ab")
	f.handleKey(formKey(tcell.KeyEnter))
	formType(f, "cd")
	if got := f.body.String(); got != "ab\ncd" {
		t.Fatalf("body = %q", got)
	}
	f.handleKey(formKey(tcell.KeyHome))
	f.handleKey(formKey(tcell.KeyBackspace2)) // joins the lines
	if got := f.body.String(); got != "abcd" {
		t.Errorf("after backspace at line start: %q", got)
	}
	if f.body.row != 0 || f.body.col != 2 {
		t.Errorf("cursor = (%d,%d), want (0,2)", f.body.row, f.body.col)
	}
	f.handleKey(formKey(tcell.KeyLeft))
	f.handleKey(formKey(tcell.KeyDelete))
	if got := f.body.String(); got != "acd" {
		t.Errorf("after Delete: %q", got)
	}
	f.handleKey(formKey(tcell.KeyCtrlK))
	if got := f.body.String(); got != "a" {
		t.Errorf("after Ctrl+K: %q", got)
	}
	f.handleKey(formKey(tcell.KeyCtrlU))
	if got := f.body.String(); got != "" {
		t.Errorf("after Ctrl+U: %q", got)
	}
	formType(f, "one two")
	f.handleKey(formKey(tcell.KeyCtrlW))
	if got := f.body.String(); got != "one " {
		t.Errorf("after Ctrl+W: %q", got)
	}
	f.handleKey(tcell.NewEventKey(tcell.KeyRune, 'z', tcell.ModAlt))
	if got := f.body.String(); got != "one " {
		t.Errorf("Alt+rune should be ignored: %q", got)
	}

	// Up from the body's first line returns to the title; Down goes back.
	f.handleKey(formKey(tcell.KeyUp))
	if f.focus != focusTitle {
		t.Errorf("Up on body row 0 should focus the title")
	}
	f.handleKey(formKey(tcell.KeyDown))
	if f.focus != focusBody {
		t.Errorf("Down in the title should focus the body")
	}
	f.handleKey(formKey(tcell.KeyBacktab))
	if f.focus != focusTitle {
		t.Errorf("Shift+Tab should switch fields")
	}
	// Ctrl+J (line feed) behaves like Enter: a field switch from the title.
	f.handleKey(formKey(tcell.KeyCtrlJ))
	if f.focus != focusBody {
		t.Errorf("Ctrl+J in the title should focus the body")
	}
}

func TestTicketFormPrefill(t *testing.T) {
	if f := newTicketForm("b", "Given", ""); f.focus != focusBody {
		t.Errorf("a prefilled title should start in the body; focus = %d", f.focus)
	}
	f := newTicketForm("b", "", "from\r\nflag")
	if f.focus != focusTitle {
		t.Errorf("a prefilled body should start in the title; focus = %d", f.focus)
	}
	if got := f.body.String(); got != "from\nflag" {
		t.Errorf("body = %q", got)
	}
	if f.body.row != 1 || f.body.col != 4 {
		t.Errorf("cursor = (%d,%d), want end of text", f.body.row, f.body.col)
	}
	single := newTextBuffer("a\nb", false)
	single.insert('\n')
	if got := single.String(); got != "a b " {
		t.Errorf("single-line buffer = %q, want newlines flattened to spaces", got)
	}
}

func TestWrapLines(t *testing.T) {
	got := wrapLines([][]rune{[]rune("abcdefgh"), []rune(""), []rune("abcd"), []rune("abcde")}, 4)
	want := []visualRow{
		{line: 0, start: 0, end: 4}, {line: 0, start: 4, end: 8}, {line: 0, start: 8, end: 8},
		{line: 1, start: 0, end: 0},
		{line: 2, start: 0, end: 4}, {line: 2, start: 4, end: 4},
		{line: 3, start: 0, end: 4}, {line: 3, start: 4, end: 5},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("wrapLines = %+v\nwant %+v", got, want)
	}
	if w := runesWidth([]rune("日本\tx")); w != 6 {
		t.Errorf("runesWidth = %d, want 6", w)
	}
}

func newFormScreen(t *testing.T, w, h int) tcell.SimulationScreen {
	t.Helper()
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(w, h)
	return screen
}

func screenText(s tcell.SimulationScreen) string {
	cells, w, h := s.GetContents()
	var sb strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := cells[y*w+x]
			if len(c.Runes) == 0 {
				continue
			}
			sb.WriteString(string(c.Runes))
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func TestTicketFormRender(t *testing.T) {
	screen := newFormScreen(t, 60, 20)
	f := newTicketForm("CLI Board (cli-board)", "", "")
	formType(f, "Hello")
	f.handleKey(formKey(tcell.KeyTab))
	formType(f, "world")
	f.render(screen)
	screen.Show()

	text := screenText(screen)
	for _, want := range []string{
		"New ticket · CLI Board (cli-board)",
		"Title",
		"Hello",
		"Description (optional, markdown)",
		"world",
		"Tab switch field · Ctrl+S create ticket · Esc cancel",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("screen missing %q:\n%s", want, text)
		}
	}
	// The cursor follows the focused field: just past "world" inside the
	// body box (box x=1, text starts at x=2; box row 8, text on row 9).
	if x, y, vis := screen.GetCursor(); !vis || x != 2+len("world") || y != bodyBoxRow+1 {
		t.Errorf("cursor = (%d,%d,%v), want (%d,%d,true)", x, y, vis, 2+len("world"), bodyBoxRow+1)
	}

	f.handleKey(formKey(tcell.KeyUp))
	f.handleKey(formKey(tcell.KeyCtrlU))
	f.handleKey(formKey(tcell.KeyCtrlS))
	f.render(screen)
	screen.Show()
	if text := screenText(screen); !strings.Contains(text, "a title is required") {
		t.Errorf("error row not rendered:\n%s", text)
	}
	if x, y, _ := screen.GetCursor(); x != 2 || y != titleBoxRow+1 {
		t.Errorf("cursor = (%d,%d), want start of the title box", x, y)
	}

	screen.SetSize(40, 5)
	f.render(screen)
	screen.Show()
	if text := screenText(screen); !strings.Contains(text, "too small") {
		t.Errorf("small-terminal notice missing:\n%s", text)
	}
}

func TestTicketFormBodyScrolls(t *testing.T) {
	screen := newFormScreen(t, 30, minFormHeight) // body box is 3 rows: 1 text row
	f := newTicketForm("b", "t", "")
	for i := 0; i < 5; i++ {
		formType(f, string(rune('a'+i)))
		f.handleKey(formKey(tcell.KeyEnter))
	}
	formType(f, "last")
	f.render(screen)
	screen.Show()
	text := screenText(screen)
	if !strings.Contains(text, "last") {
		t.Errorf("cursor line scrolled out of view:\n%s", text)
	}
	if strings.Contains(text, "\na") || strings.Contains(text, "│a") {
		t.Errorf("first body line should have scrolled away:\n%s", text)
	}
	f.handleKey(formKey(tcell.KeyCtrlA))
	for i := 0; i < 5; i++ {
		f.handleKey(formKey(tcell.KeyUp))
	}
	f.render(screen)
	screen.Show()
	if text := screenText(screen); !strings.Contains(text, "│a") {
		t.Errorf("scrolling back up should show the first line:\n%s", text)
	}
}

// TestRunTicketFormEndToEnd drives the real event loop with a simulation
// screen, including bracketed-paste events.
func TestRunTicketFormEndToEnd(t *testing.T) {
	screen := newFormScreen(t, 60, 20)
	f := newTicketForm("b", "", "")
	type outcome struct {
		res ticketFormResult
		ok  bool
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		res, ok, err := runTicketForm(screen, f)
		done <- outcome{res, ok, err}
	}()
	// InjectKey blocks while the queue is full; PostEvent doesn't, so
	// retry it until the loop has drained the queue.
	post := func(ev tcell.Event) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for screen.PostEvent(ev) != nil {
			if time.Now().After(deadline) {
				t.Fatal("event queue never drained")
			}
			time.Sleep(time.Millisecond)
		}
	}

	for _, r := range "Title" {
		screen.InjectKey(tcell.KeyRune, r, tcell.ModNone)
	}
	screen.InjectKey(tcell.KeyEnter, 0, tcell.ModNone)
	post(tcell.NewEventPaste(true))
	for _, r := range "pasted" {
		screen.InjectKey(tcell.KeyRune, r, tcell.ModNone)
	}
	screen.InjectKey(tcell.KeyEnter, 0, tcell.ModNone)
	post(tcell.NewEventPaste(false))
	screen.InjectKey(tcell.KeyCtrlS, 0, tcell.ModNone)

	select {
	case o := <-done:
		if o.err != nil || !o.ok {
			t.Fatalf("ok=%v err=%v", o.ok, o.err)
		}
		if want := (ticketFormResult{Title: "Title", Body: "pasted"}); o.res != want {
			t.Errorf("result = %+v, want %+v", o.res, want)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("form did not finish")
	}

	// Cancelling returns ok=false.
	f2 := newTicketForm("b", "", "")
	done2 := make(chan outcome, 1)
	go func() {
		res, ok, err := runTicketForm(screen, f2)
		done2 <- outcome{res, ok, err}
	}()
	screen.InjectKey(tcell.KeyEscape, 0, tcell.ModNone)
	select {
	case o := <-done2:
		if o.err != nil || o.ok {
			t.Fatalf("cancel: ok=%v err=%v", o.ok, o.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("cancel did not finish")
	}
}
