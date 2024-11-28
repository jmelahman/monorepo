package client

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"database/sql"

	"github.com/jmelahman/go-work/database"
	_ "modernc.org/sqlite"
)

func HandleClockIn() (int, error) {
	db, err := initializeDB()
	if err != nil {
		return 1, fmt.Errorf("failed to initialize database: %v", err)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT id, end FROM shift ORDER BY end DESC LIMIT 1`)
	if err != nil {
		return 1, fmt.Errorf("failed to query most recent shift: %v", err)
	}
	if !rows.Next() {
		rows.Close()
		now := time.Now()
		_, err := db.Exec(`INSERT INTO shift (id, start, end) VALUES (1, ?, ?)`,
			now.Format(time.UnixDate),
			now.Add(8*time.Hour).Format(time.UnixDate),
		)
		if err != nil {
			return 1, fmt.Errorf("failed to start a new shift: %v", err)
		}
	} else {
		var (
			id  int64
			end string
		)
		if err = rows.Scan(&id, &end); err != nil {
			return 1, fmt.Errorf("failed to scan shift: %v", err)
		}
		rows.Close()
		now := time.Now()
		endTime, err := time.Parse(time.UnixDate, end)
		if err != nil {
			return 1, fmt.Errorf("failed to parse time of shift: %v", err)
		}
		if time.Until(endTime) < 0 {
			_, err := db.Exec(`INSERT INTO shift (id, start, end) VALUES (?, ?, ?)`,
				id+1,
				now.Format(time.UnixDate),
				now.Add(8*time.Hour).Format(time.UnixDate),
			)
			if err != nil {
				return 1, fmt.Errorf("failed to start a new shift: %v", err)
			}
		}
	}
	return 0, nil
}

func HandleClockOut() (int, error) {
	db, err := initializeDB()
	if err != nil {
		return 1, fmt.Errorf("failed to initialize database: %v", err)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT id, end FROM shift ORDER BY end DESC LIMIT 1`)
	if err != nil {
		return 1, fmt.Errorf("failed to query most recent shift: %v", err)
	}
	defer rows.Close()
	if rows.Next() {
		var (
			id  int64
			end string
		)
		if err = rows.Scan(&id, &end); err != nil {
			return 1, fmt.Errorf("failed to scan shift: %v", err)
		}
		rows.Close()
		endTime, err := time.Parse(time.UnixDate, end)
		if err != nil {
			return 1, fmt.Errorf("failed to parse end time: %v", err)
		}
		if time.Until(endTime) > 0 {
			_, err := db.Exec(`UPDATE shift SET end=? WHERE id=?`, time.Now().Format(time.UnixDate), id)
			if err != nil {
				return 1, fmt.Errorf("failed to close out shift: %v", err)
			}
		}
	}
	return 0, nil
}

func HandleList(dal *database.WorkDAL) (int, error) {
	tasks, err := dal.ListTasks()
	if err != nil {
		return 1, err
	}
	fmt.Printf("id, description, start, end\n")
	for _, t := range tasks {
		fmt.Printf("%d %v %v %v\n", t.ID, t.Description, t.Start.Format(time.DateTime), t.End.Format(time.DateTime))
	}
	// var tasks []Task
	// for rows.Next() {
	// }
	// if err = rows.Scan(&id, &end); err != nil {
	// 	return 1, fmt.Errorf("failed to scan tasks: %v", err)
	// }
	// rows.Close()
	return 0, nil
}

func HandleReport() (int, error) {
	db, err := initializeDB()
	if err != nil {
		return 1, fmt.Errorf("failed to initialize database: %v", err)
	}
	defer db.Close()

	_, err = db.Query(`SELECT start, end FROM shift ORDER BY end DESC`)
	if err != nil {
		return 1, fmt.Errorf("failed to query for shifts: %v", err)
	}
	return 1, fmt.Errorf("NOT IMPLEMENTED")
}

func HandleStatus(quiet bool) (int, error) {
	db, err := initializeDB()
	if err != nil {
		panic(err)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT end FROM shift ORDER BY end DESC LIMIT 1`)
	if err != nil {
		panic(err)
	}
	if rows.Next() {
		var end string
		if err = rows.Scan(&end); err != nil {
			panic(err)
		}
		defer rows.Close()
		endTime, err := time.Parse(time.UnixDate, end)
		if err != nil {
			panic(err)
		}
		if time.Until(endTime) > 0 {
			if !quiet {
				fmt.Printf("Hours left: %v\n", endTime.Format(time.TimeOnly))
			}
			return 0, nil
		}
	}
	if quiet != true {
		fmt.Println("Not clocked in.")
		return 0, nil
	}
	return 1, nil
}

func HandleTask(args []string) (int, error) {
	if len(args) > 1 {
		return 2, fmt.Errorf("too many arguments")
	}
	//if len(args) == 0 {
	//	return HandleList()
	//}
	var description = args[0]

	returncode, err := HandleClockIn()
	if err != nil {
		return returncode, err
	}

	db, err := initializeDB()
	if err != nil {
		return 1, fmt.Errorf("failed to initialize database: %v", err)
	}
	defer db.Close()

	shiftRows, err := db.Query(`SELECT id, end FROM shift ORDER BY end DESC LIMIT 1`)
	if err != nil {
		return 1, fmt.Errorf("failed to query database: %v", err)
	}
	var (
		shiftId  int
		shiftEnd string
	)
	if !shiftRows.Next() {
		return 1, fmt.Errorf("no shifts found in the database")
	}
	if err = shiftRows.Scan(&shiftId, &shiftEnd); err != nil {
		return 1, fmt.Errorf("failed to scan shifts: %v", err)
	}
	shiftRows.Close()

	rows, err := db.Query(`SELECT id, end FROM task ORDER BY end DESC LIMIT 1`)
	if err != nil {
		return 1, fmt.Errorf("failed to query database: %v", err)
	}

	shiftEndTime, err := time.Parse(time.UnixDate, shiftEnd)
	if err != nil {
		return 1, fmt.Errorf("failed to parse shift end time: %v", err)
	}

	if !rows.Next() {
		rows.Close()
		now := time.Now()
		_, err := db.Exec(`INSERT INTO task (id, description, start, end) VALUES (1, ?, ?, ?)`,
			description,
			now.Format(time.UnixDate),
			shiftEndTime.Format(time.UnixDate),
		)
		if err != nil {
			return 1, fmt.Errorf("error creating first task: %v", err)
		}
	} else {
		var (
			id  int64
			end string
		)
		if err = rows.Scan(&id, &end); err != nil {
			return 1, fmt.Errorf("error scanning task: %v", err)
		}
		rows.Close()
		now := time.Now()
		endTime, err := time.Parse(time.UnixDate, end)
		if err != nil {
			return 1, fmt.Errorf("error parsing time last task ended: %v", err)
		}
		if time.Until(endTime) > 0 {
			_, err := db.Exec(`UPDATE task SET end=? WHERE id=?`, now.Format(time.UnixDate), id)
			if err != nil {
				return 1, fmt.Errorf("error closing previous task: %v", err)
			}
		}
		_, err = db.Exec(`INSERT INTO task (id, description, start, end) VALUES (?, ?, ?, ?)`,
			id+1,
			description,
			now.Format(time.UnixDate),
			shiftEndTime.Format(time.UnixDate),
		)
		if err != nil {
			return 1, fmt.Errorf("error creating task: %v", err)
		}
	}
	return 0, nil
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

func initializeDB() (*sql.DB, error) {
	dbDir, err := getApplicationDataDir()
	if err != nil {
		return nil, err
	}

	err = os.MkdirAll(dbDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create directory: %v", err)
	}

	databasePath := filepath.Join(dbDir, "database.db")
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS shift (id INTEGER PRIMARY KEY, start TIME, end TIME)`)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS task (id INTEGER PRIMARY KEY, description TEXT, start TIME, end TIME)`)
	if err != nil {
		return nil, err
	}
	return db, nil
}
