package widgets

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmelahman/work/api"
	"github.com/rivo/tview"
)

// RefreshWorkWidget updates the work widget with current task status
func RefreshWorkWidget(widget *tview.TextView) {
	// Get database path - use same logic as work client
	databasePath := getWorkDatabasePath()
	
	workAPI, err := api.NewWorkAPI(databasePath)
	if err != nil {
		widget.SetText(fmt.Sprintf("[red]Error initializing work API: %v", err))
		return
	}

	status, err := workAPI.GetCurrentStatus()
	if err != nil {
		widget.SetText(fmt.Sprintf("[red]Error getting work status: %v", err))
		return
	}

	if !status.HasActiveTask {
		widget.SetText("[yellow]No active task")
		return
	}

	// Format the active task display
	var color string
	switch status.Classification {
	case "Work":
		color = "green"
	case "Chore":
		color = "blue"
	case "Toil":
		color = "orange"
	case "Break":
		color = "purple"
	default:
		color = "white"
	}

	text := fmt.Sprintf("[%s]%s[white]\n%s\n[gray]%s",
		color,
		status.Classification,
		status.Task.Description,
		status.Duration,
	)

	widget.SetText(text)
}

// getWorkDatabasePath returns the path to the work database
// This mirrors the logic from work/database/database.go
func getWorkDatabasePath() string {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	
	return filepath.Join(dataHome, "work", "database.db")
}
