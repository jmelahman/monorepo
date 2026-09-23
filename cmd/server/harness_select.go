package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"

	"github.com/jmelahman/kanban/internal/client"
)

// harnessOptions is the list the create form and attach picker cycle through
// with ←/→: the server's harness registry plus which entry a session without
// its own choice launches (the board's default).
type harnessOptions struct {
	list []client.Harness
	def  int // index of the default harness in list
}

// loadHarnessOptions fetches the harness registry, flagged with the board's
// default. A nil result (with no error) means there is nothing to choose
// between, so the TUIs leave the harness row out.
func loadHarnessOptions(ctx context.Context, url, boardIdent string) (*harnessOptions, error) {
	c := client.New(url, nil)
	boardID, err := c.ResolveBoardID(ctx, boardIdent)
	if err != nil {
		return nil, err
	}
	list, err := c.ListHarnesses(ctx, boardID)
	if err != nil {
		return nil, fmt.Errorf("list harnesses: %w", err)
	}
	return newHarnessOptions(list), nil
}

func newHarnessOptions(list []client.Harness) *harnessOptions {
	if len(list) < 2 {
		return nil
	}
	o := &harnessOptions{list: list}
	for i, h := range list {
		if h.Default {
			o.def = i
			break
		}
	}
	return o
}

// initial is the index to show for a session whose stored harness is id: that
// harness when it is known, else the default (which is what it launches).
func (o *harnessOptions) initial(id string) int {
	for i, h := range o.list {
		if h.ID == id {
			return i
		}
	}
	return o.def
}

// cycle steps delta entries from i, wrapping at either end.
func (o *harnessOptions) cycle(i, delta int) int {
	n := len(o.list)
	return ((i+delta)%n + n) % n
}

func (o *harnessOptions) id(i int) string { return o.list[i].ID }

// formHarness is the harness `ticket create` stores for its new session,
// given its --harness flag and the form's pick (opts nil when the form had
// no harness row). Leaving the row on the default stores nothing, so the
// session keeps following the user/project default; moving it, or having
// passed --harness, stores the pick.
func formHarness(opts *harnessOptions, flag, picked string) string {
	if opts == nil || (flag == "" && picked == opts.id(opts.def)) {
		return flag
	}
	return picked
}

func (o *harnessOptions) label(i int) string {
	l := o.list[i].Label
	if l == "" {
		l = o.list[i].ID
	}
	if i == o.def {
		l += " (default)"
	}
	return l
}

// validateHarnessFlag checks a --harness value against the server's registry
// so a typo fails before a ticket is created or a session is touched.
func validateHarnessFlag(ctx context.Context, url, id string) error {
	if id == "" {
		return nil
	}
	list, err := client.New(url, nil).ListHarnesses(ctx, 0)
	if err != nil {
		return fmt.Errorf("list harnesses: %w", err)
	}
	ids := make([]string, len(list))
	for i, h := range list {
		if h.ID == id {
			return nil
		}
		ids[i] = h.ID
	}
	return fmt.Errorf("unknown harness %q (known: %s)", id, strings.Join(ids, ", "))
}

const harnessRowLabel = "Harness"

// drawHarnessRow draws `Harness  ‹ Label ›  note` at (x, y). The arrows are
// always shown so the row reads as something ←/→ changes; focus is shown the
// same way as the form's boxes (bold vs dim).
func drawHarnessRow(s tcell.Screen, x, y, width int, focused bool, value, note string) {
	base := tcell.StyleDefault
	putText(s, x, y, width, base.Dim(!focused), harnessRowLabel)
	vx := x + len(harnessRowLabel) + 2
	text := "‹ " + value + " ›"
	putText(s, vx, y, width-(vx-x), base.Bold(focused).Dim(!focused), text)
	if note != "" {
		nx := vx + runesWidth([]rune(text)) + 2
		putText(s, nx, y, width-(nx-x), base.Dim(true), note)
	}
}
