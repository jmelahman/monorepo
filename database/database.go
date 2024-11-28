package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jmelahman/go-work/database/models"
	_ "modernc.org/sqlite"
)

type Database interface {
	// // Task
	// StartTask(shift models.Task) (int, error)
	// GetLatestTask() (models.Task, error)
	// EndTask(id models.Task.ID) error
	ListTask() ([]models.Task, error)
	// // Shift
	// StartShift(shift models.Shift) (int, error)
	// GetLatestShift() (models.Shift, error)
	// EndShift(id models.Shift.ID) error
	ListShifts() ([]models.Shift, error)
}

type WorkDAL struct {
	db *sql.DB
}

func getApplicationDataDir() (string, error) {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dataHome = filepath.Join(home, ".local", "share")
	}

	return filepath.Join(dataHome, "work"), nil
}

func NewWorkDAL() (*WorkDAL, error) {
	dbDir, err := getApplicationDataDir()
	if err != nil {
		return nil, err
	}

	err = os.MkdirAll(dbDir, 0755)
	if err != nil {
		return nil, err
	}

	databasePath := filepath.Join(dbDir, "database.db")
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, err
	}

	dal := &WorkDAL{db: db}

	_, err = dal.db.Exec(`CREATE TABLE IF NOT EXISTS shift (id INTEGER PRIMARY KEY, start TIME, end TIME)`)
	if err != nil {
		return nil, err
	}
	_, err = dal.db.Exec(`CREATE TABLE IF NOT EXISTS task (id INTEGER PRIMARY KEY, description TEXT, start TIME, end TIME)`)
	if err != nil {
		return nil, err
	}
	return dal, nil
}

func (dal *WorkDAL) ListTasks() ([]models.Task, error) {
	rows, err := dal.db.Query(`SELECT id, description, start, end FROM task ORDER BY end DESC LIMIT 5`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []models.Task
	for rows.Next() {
		var (
			id          int
			description string
			start       string
			end         string
		)
		err := rows.Scan(&id, &description, &start, &end)
		if err != nil {
			return nil, err
		}
		startTime, err := time.Parse(time.UnixDate, start)
		if err != nil {
			return nil, fmt.Errorf("failed to parse start time: %v", err)
		}
		endTime, err := time.Parse(time.UnixDate, end)
		if err != nil {
			return nil, fmt.Errorf("failed to parse end time: %v", err)
		}
		tasks = append(tasks, models.Task{ID: id, Description: description, Start: startTime, End: endTime})
	}
	return tasks, nil
}
