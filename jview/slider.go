package jview

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Slider is a horizontal slider that implements tview.FormItem.
type Slider struct {
	*tview.Box
	disabled        bool
	Min, Max, Value int
	changed         func(value int)
	finished        func(tcell.Key)

	// Form attributes
	labelWidth  int
	fieldWidth  int
	fieldHeight int
	label       string
	labelStyle  tcell.Style
	fieldBg     tcell.Color
	fieldFg     tcell.Color
}

// NewSlider creates a new slider.
func NewSlider(min, max, value int) *Slider {
	return &Slider{
		Box:         tview.NewBox(),
		Min:         min,
		Max:         max,
		Value:       value,
		fieldBg:     tcell.ColorDarkGray,
		fieldFg:     tcell.ColorGreen,
		fieldWidth:  max - min + 1,
		fieldHeight: 1,
	}
}

// SetValue sets the slider value.
func (s *Slider) SetValue(v int) {
	if v < s.Min {
		v = s.Min
	} else if v > s.Max {
		v = s.Max
	}
	s.Value = v
	if s.changed != nil {
		s.changed(v)
	}
}

// ---------------- tview.FormItem interface ----------------

func (s *Slider) GetLabel() string    { return s.label }
func (s *Slider) GetFieldWidth() int  { return s.fieldWidth }
func (s *Slider) GetFieldHeight() int { return s.fieldHeight }

// SetLabel sets the text to be displayed before the input area.
func (s *Slider) SetLabel(label string) *Slider {
	s.label = label
	return s
}

// SetDisabled sets whether or not the item is disabled / read-only.
func (s *Slider) SetDisabled(disabled bool) tview.FormItem {
	s.disabled = disabled
	if s.finished != nil {
		s.finished(-1)
	}
	return s
}

// SetChangedFunc sets a handler which is called when the checked state of this
// checkbox was changed. The handler function receives the new state.
func (s *Slider) SetChangedFunc(handler func(value int)) *Slider {
	s.changed = handler
	return s
}

// SetLabelWidth sets the screen width of the label. A value of 0 will cause the
// primitive to use the width of the label string.
func (s *Slider) SetLabelWidth(width int) *Slider {
	s.labelWidth = width
	return s
}

// SetLabelColor sets the color of the label.
func (s *Slider) SetLabelColor(color tcell.Color) *Slider {
	s.labelStyle = s.labelStyle.Foreground(color)
	return s
}

// SetLabelStyle sets the style of the label.
func (s *Slider) SetLabelStyle(style tcell.Style) *Slider {
	s.labelStyle = style
	return s
}

func (s *Slider) SetFormAttributes(labelWidth int, labelColor, bgColor, fieldTextColor, fieldBgColor tcell.Color) tview.FormItem {
	s.labelWidth = labelWidth
	s.SetLabelColor(labelColor)
	s.fieldFg = fieldTextColor
	s.fieldBg = fieldBgColor
	return s
}

func (s *Slider) Draw(screen tcell.Screen) {
	s.DrawForSubclass(screen, s)

	x, y, _, _ := s.GetInnerRect()

	// Draw label
	for i, ch := range s.label {
		if i >= s.labelWidth {
			break
		}
		screen.SetContent(x+i, y, ch, nil, s.labelStyle)
	}

	// Draw slider bar
	barX := x + s.labelWidth
	total := s.Max - s.Min + 1
	for i := range total {
		style := tcell.StyleDefault.Background(s.fieldBg)
		if i <= s.Value-s.Min {
			style = style.Background(s.fieldFg)
		}
		screen.SetContent(barX+i, y, ' ', nil, style)
	}
}

func (s *Slider) SetFinishedFunc(handler func(key tcell.Key)) tview.FormItem {
	s.finished = handler
	return s
}

func (s *Slider) Focus(delegate func(p tview.Primitive)) {
	if s.finished != nil && s.disabled {
		s.finished(-1)
	}
	s.Box.Focus(delegate)
}

func (s *Slider) HasFocus() bool {
	return s.Box.HasFocus()
}

func (s *Slider) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		switch key := event.Key(); key {
		case tcell.KeyRight:
			s.SetValue(s.Value + 1)
		case tcell.KeyLeft:
			s.SetValue(s.Value - 1)
		case tcell.KeyTab, tcell.KeyBacktab, tcell.KeyEnter:
			if s.finished != nil {
				s.finished(key)
			}
		}
	}
}

func (s *Slider) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (bool, tview.Primitive) {
	return func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (bool, tview.Primitive) {
		if action != tview.MouseLeftClick {
			return false, nil
		}
		x, y := event.Position()
		sx, sy, _, _ := s.GetInnerRect()
		barX := sx + s.labelWidth
		if y == sy && x >= barX && x < barX+(s.Max-s.Min+1) {
			s.SetValue(s.Min + (x - barX))
			setFocus(s)
			return true, nil
		}
		return false, nil
	}
}
