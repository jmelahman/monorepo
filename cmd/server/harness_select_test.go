package server

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/jmelahman/kanban/internal/client"
)

// testHarnesses has pi flagged as the board default, so tests can tell
// "the default" apart from "the first entry".
func testHarnesses() *harnessOptions {
	return newHarnessOptions([]client.Harness{
		{ID: "claude", Label: "Claude Code"},
		{ID: "pi", Label: "pi", Default: true},
		{ID: "codex", Label: "Codex"},
	})
}

func TestHarnessOptions(t *testing.T) {
	if newHarnessOptions([]client.Harness{{ID: "claude"}}) != nil {
		t.Error("a single harness should offer no choice")
	}
	o := testHarnesses()
	if o.def != 1 || o.initial("") != 1 || o.initial("gone") != 1 || o.initial("codex") != 2 {
		t.Errorf("def=%d initial(\"\")=%d initial(gone)=%d initial(codex)=%d",
			o.def, o.initial(""), o.initial("gone"), o.initial("codex"))
	}
	if o.cycle(0, -1) != 2 || o.cycle(2, 1) != 0 {
		t.Error("cycle should wrap at both ends")
	}
	if o.label(1) != "pi (default)" || o.label(0) != "Claude Code" {
		t.Errorf("labels = %q, %q", o.label(1), o.label(0))
	}
}

func TestFormHarness(t *testing.T) {
	o := testHarnesses() // default: pi
	for _, tc := range []struct {
		name         string
		opts         *harnessOptions
		flag, picked string
		want         string
	}{
		{"row_left_on_default_follows_default", o, "", "pi", ""},
		{"row_moved_is_stored", o, "", "codex", "codex"},
		{"flag_kept_is_stored", o, "claude", "claude", "claude"},
		{"flag_then_row_moved_to_default_is_stored", o, "claude", "pi", "pi"},
		{"no_row_uses_flag", nil, "codex", "", "codex"},
		{"no_row_no_flag", nil, "", "", ""},
	} {
		if got := formHarness(tc.opts, tc.flag, tc.picked); got != tc.want {
			t.Errorf("%s: formHarness(flag %q, picked %q) = %q; want %q", tc.name, tc.flag, tc.picked, got, tc.want)
		}
	}
}

func TestTicketFormHarnessRow(t *testing.T) {
	f := newTicketForm("b", "", "")
	f.setHarnesses(testHarnesses(), "")
	formType(f, "Title")

	// Down / Tab from the title land on the harness row, which takes ←/→
	// instead of editing text.
	f.handleKey(formKey(tcell.KeyDown))
	if f.focus != focusHarness {
		t.Fatalf("Down from the title: focus = %d, want harness", f.focus)
	}
	f.handleKey(formKey(tcell.KeyRight))
	formType(f, "xyz") // nothing to type into
	if got := f.result(); got.Harness != "codex" || got.Title != "Title" {
		t.Errorf("result = %+v, want codex and an untouched title", got)
	}
	f.handleKey(formKey(tcell.KeyRight))
	f.handleKey(formKey(tcell.KeyRight))
	f.handleKey(formKey(tcell.KeyLeft))
	if got := f.result().Harness; got != "claude" {
		t.Errorf("after wrapping: harness = %q, want claude", got)
	}

	f.handleKey(formKey(tcell.KeyTab))
	if f.focus != focusBody {
		t.Fatalf("Tab from the harness row: focus = %d, want body", f.focus)
	}
	f.handleKey(formKey(tcell.KeyUp))
	if f.focus != focusHarness {
		t.Fatalf("Up from the body's first line: focus = %d, want harness", f.focus)
	}
	f.handleKey(formKey(tcell.KeyBacktab))
	if f.focus != focusTitle {
		t.Fatalf("Shift+Tab from the harness row: focus = %d, want title", f.focus)
	}
	// Enter in the title still goes straight to the description.
	f.handleKey(formKey(tcell.KeyEnter))
	if f.focus != focusBody {
		t.Errorf("Enter in the title: focus = %d, want body", f.focus)
	}
}

func TestTicketFormHarnessInitialAndAbsent(t *testing.T) {
	f := newTicketForm("b", "", "")
	f.setHarnesses(testHarnesses(), "claude")
	if got := f.result().Harness; got != "claude" {
		t.Errorf("initial harness = %q, want the --harness value", got)
	}

	f = newTicketForm("b", "", "")
	f.handleKey(formKey(tcell.KeyTab))
	if f.focus != focusBody {
		t.Errorf("without harnesses Tab should go title → body; focus = %d", f.focus)
	}
	if got := f.result().Harness; got != "" {
		t.Errorf("harness = %q without a harness row", got)
	}
}

func TestTicketFormHarnessRender(t *testing.T) {
	screen := newFormScreen(t, 60, 20)
	f := newTicketForm("b", "", "")
	f.setHarnesses(testHarnesses(), "")
	f.handleKey(formKey(tcell.KeyTab))
	f.render(screen)
	screen.Show()

	lines := strings.Split(screenText(screen), "\n")
	if got := lines[harnessRow]; !strings.Contains(got, "Harness") || !strings.Contains(got, "‹ pi (default) ›") {
		t.Errorf("harness row = %q", got)
	}
	if got := lines[bodyLabelRow+harnessRows]; !strings.Contains(got, "Description") {
		t.Errorf("description label not pushed below the harness row: %q", got)
	}
	if !strings.Contains(screenText(screen), "←→ change harness") {
		t.Errorf("help line missing the ←→ hint:\n%s", screenText(screen))
	}
	if _, _, vis := screen.GetCursor(); vis {
		t.Error("cursor should be hidden on the harness row")
	}
}

func TestTicketPickerHarness(t *testing.T) {
	items := pickerItems()
	items[2].Harness = "claude" // #7, running, pinned to claude
	p := newTicketPicker(attachAction, "b", items)
	p.harnesses = testHarnesses()

	// Untouched: nothing to switch.
	if got := p.chosen(); got.ID != 3 || got.Harness != "" {
		t.Errorf("chosen = %+v, want #3 with no harness change", got)
	}
	p.handleKey(formKey(tcell.KeyRight))
	if got := p.chosen(); got.Harness != "codex" {
		t.Errorf("after →: harness = %q, want codex", got.Harness)
	}
	// Picks are per ticket and survive moving the highlight away and back.
	p.handleKey(formKey(tcell.KeyDown))
	p.handleKey(formKey(tcell.KeyDown))
	if got := p.chosen(); got.ID != 7 || got.Harness != "" {
		t.Errorf("#7 untouched: chosen = %+v", got)
	}
	p.handleKey(formKey(tcell.KeyLeft)) // claude → codex (wraps)
	p.handleKey(formKey(tcell.KeyRight))
	if got := p.chosen(); got.Harness != "" {
		t.Errorf("cycling back to the session's harness should be a no-op; got %q", got.Harness)
	}
	p.handleKey(formKey(tcell.KeyUp))
	p.handleKey(formKey(tcell.KeyUp))
	if got := p.chosen(); got.ID != 3 || got.Harness != "codex" {
		t.Errorf("back on #3: chosen = %+v, want codex kept", got)
	}
	// ←/→ no longer touch the filter; Ctrl+B / Ctrl+F do.
	pickerType(p, "ab")
	p.handleKey(formKey(tcell.KeyCtrlB))
	pickerType(p, "X")
	if got := p.filter.String(); got != "aXb" {
		t.Errorf("filter = %q, want aXb", got)
	}
}

func TestTicketPickerHarnessRender(t *testing.T) {
	screen := newFormScreen(t, 60, 16)
	items := pickerItems()
	p := newTicketPicker(attachAction, "b", items)
	p.harnesses = testHarnesses()
	p.handleKey(formKey(tcell.KeyDown))
	p.handleKey(formKey(tcell.KeyDown)) // #7, working
	p.handleKey(formKey(tcell.KeyRight))
	p.render(screen)
	screen.Show()

	lines := strings.Split(screenLines(screen), "\n")
	row := lines[16-3]
	if !strings.Contains(row, "‹ Codex ›") || !strings.Contains(row, "restarts the running agent") {
		t.Errorf("harness row = %q", row)
	}
	if !strings.Contains(lines[16-2], "←→ harness") {
		t.Errorf("help = %q", lines[16-2])
	}

	// A stopped session has no running agent to warn about.
	p.handleKey(formKey(tcell.KeyUp)) // #12, stopped
	p.handleKey(formKey(tcell.KeyRight))
	p.render(screen)
	screen.Show()
	if row := strings.Split(screenLines(screen), "\n")[16-3]; strings.Contains(row, "restarts") {
		t.Errorf("stopped session warned about a restart: %q", row)
	}
}
