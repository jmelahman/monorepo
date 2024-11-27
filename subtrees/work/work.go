package main

import (
	"fmt"
	"os"
	"time"

	"database/sql"
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

type ReportCommand struct {
}

type StatusCommand struct {
	Quiet bool `short:"q" long:"quiet" description:"Exit with status code"`
}

func handleClockIn() {
	db, err := initializeDB()
	if err != nil {
		panic(err)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT id, end FROM shift ORDER BY end DESC LIMIT 1`)
	if err != nil {
		panic(err)
	}
	if !rows.Next() {
		rows.Close()
		now := time.Now()
		_, err := db.Exec(`INSERT INTO shift (id, start, end) VALUES (1, ?, ?)`,
			now.Format(time.UnixDate),
			now.Add(8*time.Hour).Format(time.UnixDate),
		)
		if err != nil {
			panic(err)
		}
	} else {
		var (
			id  int64
			end string
		)
		if err = rows.Scan(&id, &end); err != nil {
			panic(err)
		}
		rows.Close()
		now := time.Now()
		endTime, err := time.Parse(time.UnixDate, end)
		if err != nil {
			panic(err)
		}
		if time.Until(endTime) < 0 {
			_, err := db.Exec(`INSERT INTO shift (id, start, end) VALUES (?, ?, ?)`,
				id+1,
				now.Format(time.UnixDate),
				now.Add(8*time.Hour).Format(time.UnixDate),
			)
			if err != nil {
				panic(err)
			}
		}
	}
}

func handleClockOut() {
	db, err := initializeDB()
	if err != nil {
		panic(err)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT id, end FROM shift ORDER BY end DESC LIMIT 1`)
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	if rows.Next() {
		var (
			id  int64
			end string
		)
		if err = rows.Scan(&id, &end); err != nil {
			panic(err)
		}
		rows.Close()
		endTime, err := time.Parse(time.UnixDate, end)
		if err != nil {
			panic(err)
		}
		if time.Until(endTime) > 0 {
			_, err := db.Exec(`UPDATE shift SET end=? WHERE id=?`, time.Now().Format(time.UnixDate), id)
			if err != nil {
				panic(err)
			}
		}
	}
}

func handleReport() {
	db, err := initializeDB()
	if err != nil {
		panic(err)
	}
	defer db.Close()

	_, err = db.Query(`SELECT start, end FROM shift ORDER BY end DESC`)
	if err != nil {
		panic(err)
	}
}

func handleStatus(status StatusCommand) (int, error) {
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
		fmt.Printf("Not clocked in.")
		return 0, nil
	}
	return 1, nil
}

func initializeDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite", "./example.db")
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS shift (id INTEGER PRIMARY KEY, start TIME, end TIME)`)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func main() {
	var opts Options
	var clockIn ClockInCommand
	var clockOut ClockOutCommand
	var report ReportCommand
	var status StatusCommand

	parser := flags.NewParser(&opts, flags.Default)
	parser.AddCommand("clock-in", "Clock in", "Clock in to a shift", &clockIn)
	parser.AddCommand("clock-out", "Clock Out", "Clock out of a shift", &clockOut)
	parser.AddCommand("report", "Report", "Print report", &report)
	parser.AddCommand("status", "Status", "Print current shift", &status)

	cmd := &complete.Command{
		Flags: map[string]complete.Predictor{
			"--help":               predict.Nothing,
			"--install-completion": predict.Nothing,
		},
		Sub: map[string]*complete.Command{
			"clock-in":  nil,
			"clock-out": nil,
			"report":    nil,
			"status":    nil,
		},
	}
	cmd.Complete("work")

	if len(os.Args) == 1 {
		parser.WriteHelp(os.Stderr)
		os.Exit(2)
	}

	_, err := parser.Parse()
	if err != nil {
		os.Exit(2)
	}

	command := os.Args[1]

	switch command {
	case "clock-in":
		handleClockIn()
	case "clock-out":
		handleClockOut()
	case "report":
		handleReport()
	case "status":
		handleStatus(status)
	default:
		parser.WriteHelp(os.Stderr)
		os.Exit(2)
	}
}
