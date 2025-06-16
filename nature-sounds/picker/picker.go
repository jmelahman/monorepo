package picker

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/jmelahman/nature-sounds/sounds"
)

func ListPicker(items []sounds.Sound) (int, error) {
	screen, err := tcell.NewScreen()
	if err != nil {
		return -1, err
	}
	defer screen.Fini()

	if err := screen.Init(); err != nil {
		return -1, err
	}

	style := tcell.StyleDefault
	selectedStyle := tcell.StyleDefault.Bold(true).Underline(true)

	selectedIndex := 0
	draw := func() {
		screen.Clear()
		for i, item := range items {
			styleToUse := style
			if i == selectedIndex {
				styleToUse = selectedStyle
			}

			line := fmt.Sprintf("%d) %s", i+1, item.Name)
			for x, ch := range line {
				screen.SetContent(x, i, ch, nil, styleToUse)
			}
		}
		screen.Show()
	}

	draw()
	for {
		event := screen.PollEvent()
		switch ev := event.(type) {
		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyEscape:
				return -1, fmt.Errorf("selection canceled")
			case tcell.KeyEnter:
				return selectedIndex, nil
			case tcell.KeyUp:
				if selectedIndex > 0 {
					selectedIndex--
					draw()
				}
			case tcell.KeyDown:
				if selectedIndex < len(items)-1 {
					selectedIndex++
					draw()
				}
			}
		}
	}
}
