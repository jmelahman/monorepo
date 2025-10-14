package jview

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ColorGrid shows a procedurally generated grid of colors (red→violet hues,
// with decreasing saturation from top to bottom).
type ColorGrid struct {
	*tview.Box
	disabled   bool
	Rows, Cols int
	Selected   int
	changed    func(idx int, color tcell.Color)
	finished   func(tcell.Key)

	label       string
	labelWidth  int
	labelStyle  tcell.Style
	cellWidth   int
	cellHeight  int
	borderStyle tcell.Style
	ActiveColor tcell.Color
	colors      []tcell.Color
}

// NewColorGrid creates a new ColorGrid with automatically generated colors.
func NewColorGrid(rows, cols int) *ColorGrid {
	g := &ColorGrid{
		Box:         tview.NewBox(),
		Rows:        rows,
		Cols:        cols,
		cellWidth:   2,
		cellHeight:  1,
		borderStyle: tcell.StyleDefault.Foreground(tcell.ColorBlack),
		ActiveColor: tcell.ColorWhite,
	}
	g.generateColors()
	return g
}

// generateColors fills g.colors with hues red→violet (0–270° hue range),
// decreasing saturation by row.
func (g *ColorGrid) generateColors() {
	total := g.Rows * g.Cols
	g.colors = make([]tcell.Color, total)
	for r := 0; r < g.Rows; r++ {
		saturation := 1.0 - float64(r)/float64(g.Rows-1) // top = full sat, bottom = light
		for c := 0; c < g.Cols; c++ {
			hue := float64(c) / float64(g.Cols-1) * 270 // red→violet
			col := hslToColor(hue, saturation, 0.5)
			g.colors[r*g.Cols+c] = col
		}
	}
}

// Simple HSL→tcell.Color converter.
func hslToColor(h, s, l float64) tcell.Color {
	h /= 360
	var r, g, b float64

	if s == 0 {
		r, g, b = l, l, l
	} else {
		var q float64
		if l < 0.5 {
			q = l * (1 + s)
		} else {
			q = l + s - l*s
		}
		p := 2*l - q
		r = hueToRGB(p, q, h+1.0/3.0)
		g = hueToRGB(p, q, h)
		b = hueToRGB(p, q, h-1.0/3.0)
	}

	return tcell.NewRGBColor(int32(r*255), int32(g*255), int32(b*255))
}

func hueToRGB(p, q, t float64) float64 {
	if t < 0 {
		t++
	}
	if t > 1 {
		t--
	}
	switch {
	case t < 1.0/6.0:
		return p + (q-p)*6*t
	case t < 0.5:
		return q
	case t < 2.0/3.0:
		return p + (q-p)*(2.0/3.0-t)*6
	default:
		return p
	}
}

// --- FormItem methods and UI behavior below ---

func (g *ColorGrid) GetLabel() string    { return g.label }
func (g *ColorGrid) GetFieldWidth() int  { return g.Cols * g.cellWidth }
func (g *ColorGrid) GetFieldHeight() int { return g.Rows * g.cellHeight }

func (g *ColorGrid) SetLabel(label string) *ColorGrid {
	g.label = label
	return g
}

func (g *ColorGrid) SetLabelWidth(width int) *ColorGrid {
	g.labelWidth = width
	return g
}

func (g *ColorGrid) SetLabelColor(color tcell.Color) *ColorGrid {
	g.labelStyle = g.labelStyle.Foreground(color)
	return g
}

func (g *ColorGrid) SetFormAttributes(labelWidth int, labelColor, bgColor, fieldTextColor, fieldBgColor tcell.Color) tview.FormItem {
	g.labelWidth = labelWidth
	g.SetLabelColor(labelColor)
	return g
}

func (g *ColorGrid) SetChangedFunc(handler func(idx int, color tcell.Color)) *ColorGrid {
	g.changed = handler
	return g
}

func (g *ColorGrid) SetFinishedFunc(handler func(key tcell.Key)) tview.FormItem {
	g.finished = handler
	return g
}

func (g *ColorGrid) SetDisabled(disabled bool) tview.FormItem {
	g.disabled = disabled
	return g
}

func (g *ColorGrid) Draw(screen tcell.Screen) {
	g.DrawForSubclass(screen, g)

	x, y, width, _ := g.GetInnerRect()
	labelEnd := x + g.labelWidth

	// Draw label
	for i, ch := range g.label {
		if i >= g.labelWidth {
			break
		}
		screen.SetContent(x+i, y, ch, nil, g.labelStyle)
	}

	// Draw color cells
	startX := labelEnd
	for r := 0; r < g.Rows; r++ {
		for c := 0; c < g.Cols; c++ {
			idx := r*g.Cols + c
			if idx >= len(g.colors) {
				continue
			}
			cx := startX + c*g.cellWidth
			cy := y + r*g.cellHeight
			color := g.colors[idx]
			style := tcell.StyleDefault.Background(color).Foreground(g.ActiveColor).Bold(true)
			if idx == g.Selected {
				style = style.Reverse(true)
			}
			for i := 0; i < g.cellWidth; i++ {
				screen.SetContent(cx+i, cy, ' ', nil, style)
			}
		}
	}

	// Clear remaining space
	for i := labelEnd + g.Cols*g.cellWidth; i < x+width; i++ {
		screen.SetContent(i, y, ' ', nil, tcell.StyleDefault)
	}
}

func (g *ColorGrid) Focus(delegate func(p tview.Primitive)) {
	if g.disabled && g.finished != nil {
		g.finished(-1)
	}
	g.Box.Focus(delegate)
}

func (g *ColorGrid) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		if g.disabled {
			return
		}
		switch key := event.Key(); key {
		case tcell.KeyRight:
			if g.Selected < len(g.colors)-1 {
				g.Selected++
				g.emitChange()
			}
		case tcell.KeyLeft:
			if g.Selected > 0 {
				g.Selected--
				g.emitChange()
			}
		case tcell.KeyUp:
			if g.Selected-g.Cols >= 0 {
				g.Selected -= g.Cols
				g.emitChange()
			}
		case tcell.KeyDown:
			if g.Selected+g.Cols < len(g.colors) {
				g.Selected += g.Cols
				g.emitChange()
			}
		case tcell.KeyEnter, tcell.KeyTab, tcell.KeyBacktab:
			if g.finished != nil {
				g.finished(key)
			}
		}
	}
}

func (g *ColorGrid) emitChange() {
	if g.changed != nil && g.Selected < len(g.colors) {
		g.changed(g.Selected, g.colors[g.Selected])
	}
}

func (g *ColorGrid) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (bool, tview.Primitive) {
	return func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (bool, tview.Primitive) {
		if action != tview.MouseLeftClick || g.disabled {
			return false, nil
		}
		x, y := event.Position()
		sx, sy, _, _ := g.GetInnerRect()
		gridX := x - (sx + g.labelWidth)
		gridY := y - sy
		if gridX < 0 || gridY < 0 {
			return false, nil
		}
		col := gridX / g.cellWidth
		row := gridY / g.cellHeight
		idx := row*g.Cols + col
		if idx >= 0 && idx < len(g.colors) {
			g.Selected = idx
			g.emitChange()
			setFocus(g)
			return true, nil
		}
		return false, nil
	}
}
