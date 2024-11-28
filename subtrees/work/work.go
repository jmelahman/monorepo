package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"database/sql"

	"database/models"
	"github.com/jessevdk/go-flags"
	"github.com/posener/complete/v2"
	"github.com/posener/complete/v2/install"
	"github.com/posener/complete/v2/predict"
	_ "modernc.org/sqlite"
)

type Options struct {
}

type ClockInCommand struct {
}

type ClockOutCommand struct {
}

type InstallCompleteCommand struct {
}

type ListCommand struct {
}

type ReportCommand struct {
}

type StatusCommand struct {
	Quiet bool `short:"q" long:"quiet" description:"Exit with status code"`
}

type TaskCommand struct {
}

type Shift struct {
	id    int
	start time.Time
	end   time.Time
}

type Task struct {
	id          int
	description string
	start       time.Time
	end         time.Time
}

func handleClockIn() (int, error) {
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

func handleClockOut() (int, error) {
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

func handleList() (int, error) {
	// db, err := initializeDB()
	// if err != nil {
	// 	return 1, fmt.Errorf("failed to initialize database: %v", err)
	// }
	// defer db.Close()

	// rows, err = db.Query(`SELECT start, end FROM shift ORDER BY end DESC LIMIT 5`)
	// if err != nil {
	// 	return 1, fmt.Errorf("failed to query for shifts: %v", err)
	// }
	// defer rows.Close()

	// var tasks []Task
	// for rows.Next() {
	// }
	// if err = rows.Scan(&id, &end); err != nil {
	// 	return 1, fmt.Errorf("failed to scan tasks: %v", err)
	// }
	// rows.Close()
	return 0, nil
}

func handleReport() (int, error) {
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

func handleStatus(status *StatusCommand) (int, error) {
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
			if status.Quiet != true {
				fmt.Printf("Hours left: %v\n", endTime.Format(time.TimeOnly))
			}
			return 0, nil
		}
	}
	if status.Quiet != true {
		fmt.Println("Not clocked in.")
		return 0, nil
	}
	return 1, nil
}

func handleTask(args []string) (int, error) {
	if len(args) > 1 {
		return 2, fmt.Errorf("too many arguments")
	}
	if len(args) == 0 {
		return handleList()
	}
	var description = args[0]

	returncode, err := handleClockIn()
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

func main() {
	var opts Options
	var clockIn ClockInCommand
	var clockOut ClockOutCommand
	var installComplete InstallCompleteCommand
	var list ListCommand
	var report ReportCommand
	var status StatusCommand
	var task TaskCommand

	parser := flags.NewParser(&opts, flags.Default)
	parser.AddCommand("clock-in", "Clock in", "Clock in to a shift", &clockIn)
	parser.AddCommand("clock-out", "Clock Out", "Clock out of a shift", &clockOut)
	parser.AddCommand("install-completion", "Install Autocomplete", "Install shell autocompletion", &installComplete)
	parser.AddCommand("list", "List Tasks", "List most recent tasks", &list)
	parser.AddCommand("report", "Report", "Print report", &report)
	parser.AddCommand("status", "Status", "Print current shift", &status)
	parser.AddCommand("task", "Start Task", "Start a task", &task)

	cmd := &complete.Command{
		Flags: map[string]complete.Predictor{
			"--help": predict.Nothing,
		},
		Sub: map[string]*complete.Command{
			"clock-in":           nil,
			"clock-out":          nil,
			"install-completion": nil,
			"report":             nil,
			"status":             nil,
			"task":               nil,
		},
	}
	cmd.Complete("work")

	if len(os.Args) == 0 {
		parser.WriteHelp(os.Stderr)
		os.Exit(2)
	}

	args, err := parser.Parse()
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		}
		os.Exit(2)
	}

	command := os.Args[1]
	var returncode int

	switch command {
	case "clock-in":
		if returncode, err = handleClockIn(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	case "clock-out":
		if returncode, err = handleClockOut(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	case "install-completion":
		install.Install("work")
	case "list":
		if returncode, err = handleList(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	case "report":
		if returncode, err = handleReport(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	case "status":
		if returncode, err = handleStatus(&status); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	case "task":
		if returncode, err = handleTask(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	default:
		parser.WriteHelp(os.Stderr)
		returncode = 2
	}
	os.Exit(returncode)
}
