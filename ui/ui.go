package ui

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/jmelahman/cycle-cli/ble"
	"github.com/rivo/tview"
)

// UI represents the application UI
type UI struct {
	app           *tview.Application
	grid          *tview.Grid
	powerBox      *tview.TextView
	cadenceBox    *tview.TextView
	speedBox      *tview.TextView
	distanceBox   *tview.TextView
	durationBox   *tview.TextView
	resistanceBox *tview.TextView
	statusBox     *tview.TextView
	unitSystem    string
	startTime     time.Time
}

// NewUI creates a new UI instance
func NewUI(unitSystem string) *UI {
	app := tview.NewApplication()
	
	// Create text views for each metric
	powerBox := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)
	powerBox.SetBorder(true).SetTitle(" Power ")

	cadenceBox := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)
	cadenceBox.SetBorder(true).SetTitle(" Cadence ")

	speedBox := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)
	speedBox.SetBorder(true).SetTitle(" Speed ")

	distanceBox := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)
	distanceBox.SetBorder(true).SetTitle(" Distance ")

	durationBox := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)
	durationBox.SetBorder(true).SetTitle(" Duration ")

	resistanceBox := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)
	resistanceBox.SetBorder(true).SetTitle(" Resistance ")

	statusBox := tview.NewTextView().
		SetDynamicColors(true).
		SetChangedFunc(func() {
			app.Draw()
		})
	statusBox.SetBorder(true).SetTitle(" Status ")

	// Create a grid layout
	grid := tview.NewGrid().
		SetRows(3, 3, 0, 3).
		SetColumns(0, 0, 0).
		SetBorders(false).
		AddItem(speedBox, 0, 0, 1, 1, 0, 0, false).      // Speed
		AddItem(distanceBox, 0, 1, 1, 1, 0, 0, false).   // Distance
		AddItem(durationBox, 0, 2, 1, 1, 0, 0, false).   // Duration
		AddItem(powerBox, 1, 0, 1, 1, 0, 0, false).      // Power
		AddItem(cadenceBox, 1, 1, 1, 1, 0, 0, false).    // Cadence
		AddItem(resistanceBox, 1, 2, 1, 1, 0, 0, false). // Resistance
		AddItem(statusBox, 2, 0, 1, 3, 0, 0, false)      // Status

	// Set up key handling
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Rune() == 'q' {
			app.Stop()
		}
		return event
	})

	return &UI{
		app:           app,
		grid:          grid,
		powerBox:      powerBox,
		cadenceBox:    cadenceBox,
		speedBox:      speedBox,
		distanceBox:   distanceBox,
		durationBox:   durationBox,
		resistanceBox: resistanceBox,
		statusBox:     statusBox,
		unitSystem:    unitSystem,
		startTime:     time.Now(),
	}
}

// Start starts the UI
func (ui *UI) Start() error {
	return ui.app.SetRoot(ui.grid, true).Run()
}

// Stop stops the UI
func (ui *UI) Stop() {
	ui.app.Stop()
}

// UpdateStatus updates the status message
func (ui *UI) UpdateStatus(format string, args ...interface{}) {
	ui.statusBox.SetText(fmt.Sprintf(format, args...))
}

// UpdateTelemetry updates the UI with new telemetry data
func (ui *UI) UpdateTelemetry(data ble.Telemetry) {
	ui.app.QueueUpdateDraw(func() {
		// Update power
		ui.powerBox.SetText(fmt.Sprintf("[yellow]%d[white] W", data.Power))

		// Update cadence
		ui.cadenceBox.SetText(fmt.Sprintf("[green]%d[white] rpm", data.Cadence))

		// Update speed
		speedUnit := "mph"
		speedValue := data.Speed
		if ui.unitSystem == "metric" {
			speedUnit = "km/h"
		}
		ui.speedBox.SetText(fmt.Sprintf("[cyan]%.1f[white] %s", speedValue, speedUnit))

		// Update distance
		distanceUnit := "mi"
		distanceValue := data.Distance
		if ui.unitSystem == "metric" {
			distanceUnit = "km"
		}
		ui.distanceBox.SetText(fmt.Sprintf("[blue]%.2f[white] %s", distanceValue, distanceUnit))

		// Update duration
		elapsed := time.Since(ui.startTime)
		totalSeconds := int(elapsed.Seconds())
		if totalSeconds >= 3600 {
			hours := totalSeconds / 3600
			minutes := (totalSeconds % 3600) / 60
			seconds := totalSeconds % 60
			ui.durationBox.SetText(fmt.Sprintf("[red]%02d:%02d:%02d", hours, minutes, seconds))
		} else {
			minutes := totalSeconds / 60
			seconds := totalSeconds % 60
			ui.durationBox.SetText(fmt.Sprintf("[red]%02d:%02d", minutes, seconds))
		}
	})
}

// UpdateResistance updates the resistance display
func (ui *UI) UpdateResistance(level uint8) {
	ui.app.QueueUpdateDraw(func() {
		ui.resistanceBox.SetText(fmt.Sprintf("[magenta]%d%%", level))
	})
}
