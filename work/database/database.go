package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jmelahman/work/database/types"
	_ "github.com/mattn/go-sqlite3"
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

func makeFileAll(filePath string) error {
	dir := filepath.Dir(filePath)

	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	return nil
}

func NewWorkDAL(databasePath string) (*WorkDAL, error) {
	if databasePath == "" {
		dbDir, err := getApplicationDataDir()
		if err != nil {
			return nil, err
		}

		databasePath = filepath.Join(dbDir, "database.db")
	}

	if err := makeFileAll(databasePath); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		return nil, err
	}

	dal := &WorkDAL{db: db}

	_, err = dal.db.Exec(`CREATE TABLE IF NOT EXISTS task (id INTEGER PRIMARY KEY, description TEXT, classification INT, start TIME, end TIME)`)
	if err != nil {
		return nil, err
	}
	return dal, nil
}

func (dal *WorkDAL) CreateTask(task types.Task) error {
	_, err := dal.db.Exec(`INSERT INTO task (id, description, classification, start, end) VALUES (?, ?, ?, ?, ?)`,
		task.ID,
		task.Description,
		task.Classification,
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

func (dal *WorkDAL) GetLatestTask() (types.Task, error) {
	tasks, err := dal.ListTasks(1, 0)
	if err != nil {
		return types.Task{}, err
	}
	if len(tasks) == 0 {
		return types.Task{}, nil
	}
	return tasks[0], nil
}

func (dal *WorkDAL) ListTasks(limit int, days int) ([]types.Task, error) {
	tasks := []types.Task{}

	query := `SELECT id, description, classification, start, end FROM task`
	args := []interface{}{}

	if days > 0 {
		query += ` WHERE start > datetime('now', '-' || ? || ' days')`
		args = append(args, days)
	}

	query += ` ORDER BY id DESC`

	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := dal.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id             int
			description    string
			classification types.TaskClassification
			start          string
			end            string
		)
		err := rows.Scan(&id, &description, &classification, &start, &end)
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
		tasks = append(
			tasks,
			types.Task{
				ID:             id,
				Description:    description,
				Classification: classification,
				Start:          startTime,
				End:            endTime,
			},
		)
	}
	return tasks, nil
}
