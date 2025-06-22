package api

import (
	"fmt"
	"time"

	"github.com/jmelahman/work/database"
	"github.com/jmelahman/work/database/types"
)

// WorkAPI provides API access to work functionality
type WorkAPI struct {
	dal *database.WorkDAL
}

// TaskStatus represents the current task status
type TaskStatus struct {
	HasActiveTask  bool        `json:"has_active_task"`
	Task           *types.Task `json:"task,omitempty"`
	Duration       string      `json:"duration,omitempty"`
	Classification string      `json:"classification,omitempty"`
}

// NewWorkAPI creates a new WorkAPI instance
func NewWorkAPI(databasePath string) (*WorkAPI, error) {
	dal, err := database.NewWorkDAL(databasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize DAL: %v", err)
	}
	return &WorkAPI{dal: dal}, nil
}

// GetCurrentStatus returns the current work status
func (api *WorkAPI) GetCurrentStatus() (*TaskStatus, error) {
	task, err := api.dal.GetLatestTask()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest task: %v", err)
	}

	status := &TaskStatus{
		HasActiveTask: false,
	}

	// Check if there's an active task
	if task.ID != 0 && task.End.IsZero() {
		status.HasActiveTask = true
		status.Task = &task
		status.Duration = formatDuration(time.Since(task.Start))
		status.Classification = task.Classification.String()
	}

	return status, nil
}

func formatDuration(duration time.Duration) string {
	return fmt.Sprintf("%dh %dmin", int(duration.Hours()), int(duration.Minutes())%60)
}
