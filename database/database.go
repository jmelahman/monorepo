package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jmelahman/gwork/database/models"
	_ "modernc.org/sqlite"
)

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

func (dal *WorkDAL) CreateTask(task models.Task) error {
	_, err := dal.db.Exec(`INSERT INTO task (id, description, start, end) VALUES (?, ?, ?, ?)`,
		task.ID,
		task.Description,
		task.Start.Format(time.UnixDate),
		task.End.Format(time.UnixDate),
	)
	if err != nil {
		return err
	}
	return nil
}

func (dal *WorkDAL) EndTask(id int) error {
	_, err := dal.db.Exec(`UPDATE task SET end=? WHERE id=?`, time.Now().Format(time.UnixDate), id)
	if err != nil {
		return fmt.Errorf("error closing previous task: %v", err)
	}
	return nil
}

func (dal *WorkDAL) GetLatestTask() (models.Task, error) {
	tasks, err := dal.ListTasks(1)
	if err != nil {
		return models.Task{}, err
	}
	if len(tasks) == 0 {
		return models.Task{}, nil
	}
	return tasks[0], nil
}

func (dal *WorkDAL) ListTasks(limit int) ([]models.Task, error) {
	rows, err := dal.db.Query(`SELECT id, description, start, end FROM task ORDER BY end ASC LIMIT ?`, limit)
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

func (dal *WorkDAL) CreateShift(shift models.Shift) error {
	_, err := dal.db.Exec(`INSERT INTO shift (id, start, end) VALUES (?, ?, ?)`,
		shift.ID,
		shift.Start.Format(time.UnixDate),
		shift.End.Format(time.UnixDate),
	)
	if err != nil {
		return err
	}
	return nil
}

func (dal *WorkDAL) EndShift(id int) error {
	_, err := dal.db.Exec(`UPDATE shift SET end=? WHERE id=?`, time.Now().Format(time.UnixDate), id)
	if err != nil {
		return err
	}
	return nil
}

func (dal *WorkDAL) GetLatestShift() (models.Shift, error) {
	shifts, err := dal.ListShifts(1)
	if err != nil {
		return models.Shift{}, err
	}
	if len(shifts) == 0 {
		return models.Shift{}, nil
	}
	return shifts[0], nil
}

func (dal *WorkDAL) ListShifts(limit int) ([]models.Shift, error) {
	shifts := []models.Shift{}

	rows, err := dal.db.Query(`SELECT id, start, end FROM shift ORDER BY end ASC LIMIT ?`, limit)
	if err != nil {
		return shifts, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id    int
			start string
			end   string
		)
		if err = rows.Scan(&id, &start, &end); err != nil {
			return shifts, err
		}
		startTime, err := time.Parse(time.UnixDate, start)
		if err != nil {
			return shifts, fmt.Errorf("failed to parse start time: %v", err)
		}
		endTime, err := time.Parse(time.UnixDate, end)
		if err != nil {
			return shifts, fmt.Errorf("failed to parse end time: %v", err)
		}
		shifts = append(shifts, models.Shift{ID: id, Start: startTime, End: endTime})
	}
	return shifts, nil
}
