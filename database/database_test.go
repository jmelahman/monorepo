package database

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/jmelahman/work/database/models"
	"github.com/stretchr/testify/assert"
)

func setupTestDB(t *testing.T) *WorkDAL {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	dal, err := NewWorkDAL(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize WorkDAL: %v", err)
	}

	return dal
}

func TestNewWorkDAL(t *testing.T) {
	dal := setupTestDB(t)
	assert.NotNil(t, dal)
	assert.NoError(t, dal.db.Ping())
}

func TestCreateTask(t *testing.T) {
	dal := setupTestDB(t)

	task := models.Task{
		ID:          1,
		Description: "Test Task",
		Start:       time.Now(),
		End:         time.Now().Add(1 * time.Hour),
	}

	err := dal.CreateTask(task)
	assert.NoError(t, err)

	tasks, err := dal.ListTasks(1)
	assert.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, "Test Task", tasks[0].Description)
}

func TestEndTask(t *testing.T) {
	dal := setupTestDB(t)

	startTime := time.Now()
	task := models.Task{
		ID:          1,
		Description: "Task to End",
		Start:       startTime,
		End:         startTime.Add(1 * time.Hour),
	}

	err := dal.CreateTask(task)
	assert.NoError(t, err)

	err = dal.EndTask(1)
	assert.NoError(t, err)

	latestTask, err := dal.GetLatestTask()
	assert.NoError(t, err)
	assert.NotZero(t, latestTask.End)
}

func TestCreateShift(t *testing.T) {
	dal := setupTestDB(t)

	shift := models.Shift{
		ID:    1,
		Start: time.Now(),
		End:   time.Now().Add(8 * time.Hour),
	}

	err := dal.CreateShift(shift)
	assert.NoError(t, err)

	shifts, err := dal.ListShifts(1)
	assert.NoError(t, err)
	assert.Len(t, shifts, 1)
	assert.Equal(t, 1, shifts[0].ID)
}

func TestEndShift(t *testing.T) {
	dal := setupTestDB(t)

	startTime := time.Now()
	shift := models.Shift{
		ID:    1,
		Start: startTime,
		End:   startTime.Add(8 * time.Hour),
	}

	err := dal.CreateShift(shift)
	assert.NoError(t, err)

	err = dal.EndShift(1)
	assert.NoError(t, err)

	latestShift, err := dal.GetLatestShift()
	assert.NoError(t, err)
	assert.NotZero(t, latestShift.End)
}

func TestListTasks(t *testing.T) {
	dal := setupTestDB(t)

	for i := 1; i <= 3; i++ {
		task := models.Task{
			ID:          i,
			Description: "Task " + string(rune(i)),
			Start:       time.Now(),
			End:         time.Now().Add(time.Duration(i) * time.Hour),
		}
		err := dal.CreateTask(task)
		assert.NoError(t, err)
	}

	tasks, err := dal.ListTasks(2)
	assert.NoError(t, err)
	assert.Len(t, tasks, 2)
}

func TestListShifts(t *testing.T) {
	dal := setupTestDB(t)

	for i := 1; i <= 3; i++ {
		shift := models.Shift{
			ID:    i,
			Start: time.Now(),
			End:   time.Now().Add(time.Duration(i) * time.Hour),
		}
		err := dal.CreateShift(shift)
		assert.NoError(t, err)
	}

	shifts, err := dal.ListShifts(2)
	assert.NoError(t, err)
	assert.Len(t, shifts, 2)
}
