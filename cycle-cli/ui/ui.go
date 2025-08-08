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
	ftp           int
}

// NewUI creates a new UI instance
func NewUI(unitSystem string, ftp int) *UI {
	app := tview.NewApplication()

	// Create text views for each metric
	powerBox := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)
	powerBox.SetBorder(true).
		SetTitle(" Power ").
		SetTitleColor(tcell.ColorYellow).
		SetBorderColor(tcell.ColorYellow)

	cadenceBox := tview.NewTextView().
		SetTextAlign(tview.AlignCenter)
	cadenceBox.SetBorder(true).
		SetTitle(" Cadence ").
		SetTitleColor(tcell.ColorGreen).
		SetBorderColor(tcell.ColorGreen)

	speedBox := tview.NewTextView().
		SetTextAlign(tview.AlignCenter)
	speedBox.SetBorder(true).
		SetTitle(" Speed ").
		SetTitleColor(tcell.ColorDarkCyan).
		SetBorderColor(tcell.ColorDarkCyan)

	distanceBox := tview.NewTextView().
		SetTextAlign(tview.AlignCenter)
	distanceBox.SetBorder(true).
		SetTitle(" Distance ").
		SetTitleColor(tcell.ColorBlue).
		SetBorderColor(tcell.ColorBlue)

	durationBox := tview.NewTextView().
		SetTextAlign(tview.AlignCenter)
	durationBox.SetBorder(true).
		SetTitle(" Duration ").
		SetTitleColor(tcell.ColorRed).
		SetBorderColor(tcell.ColorRed)

	resistanceBox := tview.NewTextView().
		SetTextAlign(tview.AlignCenter)
	resistanceBox.SetBorder(true).
		SetTitle(" Resistance ").
		SetTitleColor(tcell.ColorDarkMagenta).
		SetBorderColor(tcell.ColorDarkMagenta)

	statusBox := tview.NewTextView().
		SetChangedFunc(func() {
			app.Draw()
		})
	statusBox.SetBorder(true).
		SetTitle(" Status ").
		SetTitleColor(tcell.ColorWhite).
		SetBorderColor(tcell.ColorWhite)

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
		ftp:           ftp,
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

// SetInputCapture sets a custom input capture function for the application.
// This allows the main application logic to define keybindings.
func (ui *UI) SetInputCapture(capture func(event *tcell.EventKey) *tcell.EventKey) {
	ui.app.SetInputCapture(capture)
}

// UpdateStatus updates the status message
func (ui *UI) UpdateStatus(format string, args ...any) {
	ui.statusBox.SetText(fmt.Sprintf(format, args...))
}

// UpdateTelemetry updates the UI with new telemetry data
func (ui *UI) UpdateTelemetry(data ble.Telemetry) {
	ui.app.QueueUpdateDraw(func() {
		// Update power
		powerStr := ""
		if ui.ftp > 0 {
			powerPercentage := float64(data.Power) / float64(ui.ftp) * 100.0
			var color string

			if powerPercentage < 60 {
				color = "white" // Zone 1: Recovery
			} else if powerPercentage <= 75 {
				color = "blue" // Zone 2: Endurance
			} else if powerPercentage <= 89 {
				color = "green" // Zone 3: Tempo
			} else if powerPercentage <= 104 {
				color = "yellow" // Zone 4: Threshold
			} else if powerPercentage <= 118 {
				color = "orange" // Zone 5: VO2 Max
			} else {
				color = "red" // Zone 6: Anaerobic
			}
			powerStr = fmt.Sprintf("[%s]%d[white] W", color, data.Power)
		} else {
			powerStr = fmt.Sprintf("%d W", data.Power)
		}
		ui.powerBox.SetText(powerStr)

		// Update cadence
		ui.cadenceBox.SetText(fmt.Sprintf("%d rpm", data.Cadence))

		// Update speed
		speedUnit := "mph"
		speedValue := data.Speed
		if ui.unitSystem == "metric" {
			speedUnit = "km/h"
		}
		ui.speedBox.SetText(fmt.Sprintf("%.1f %s", speedValue, speedUnit))

		// Update distance
		distanceUnit := "mi"
		distanceValue := data.Distance
		if ui.unitSystem == "metric" {
			distanceUnit = "km"
		}
		ui.distanceBox.SetText(fmt.Sprintf("%.2f %s", distanceValue, distanceUnit))

		// Update duration
		elapsed := time.Since(ui.startTime)
		totalSeconds := int(elapsed.Seconds())
		if totalSeconds >= 3600 {
			hours := totalSeconds / 3600
			minutes := (totalSeconds % 3600) / 60
			seconds := totalSeconds % 60
			ui.durationBox.SetText(fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds))
		} else {
			minutes := totalSeconds / 60
			seconds := totalSeconds % 60
			ui.durationBox.SetText(fmt.Sprintf("%02d:%02d", minutes, seconds))
		}
	})
}

// UpdateResistance updates the resistance display
func (ui *UI) UpdateResistance(level int8) {
	ui.app.QueueUpdateDraw(func() {
		ui.resistanceBox.SetText(fmt.Sprintf("%d%%", level))
	})
}
