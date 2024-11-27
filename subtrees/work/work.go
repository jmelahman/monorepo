package main

import (
	"fmt"
	"os"
	"time"

	"database/sql"
	"github.com/jessevdk/go-flags"
	_ "modernc.org/sqlite"
)

type Options struct {
	Verbose bool `short:"v" long:"verbose" description:"Enable verbose output"`
}

type ClockInCommand struct {
}

type ClockOutCommand struct {
}

type ReportCommand struct {
}

type StatusCommand struct {
}

func main() {
	var opts Options
	var clockIn ClockInCommand
	var clockOut ClockOutCommand
	var report ReportCommand
	var status StatusCommand

	parser := flags.NewParser(&opts, flags.Default)
	parser.AddCommand("in", "Clock in", "Clock in to a shift", &clockIn)
	parser.AddCommand("out", "Clock Out", "Clock out of a shift", &clockOut)
	parser.AddCommand("report", "Report", "Print report", &report)
	parser.AddCommand("status", "Status", "Print current shift", &status)

	_, err := flags.Parse(&opts)
	if err != nil {
		panic(err)
	}

	if opts.Verbose {
		fmt.Println("Verbose mode enabled")
	}

	db, err := sql.Open("sqlite", "./example.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS shift (id INTEGER PRIMARY KEY, start TIME, end TIME)`)
	if err != nil {
		panic(err)
	}

	var (
		end string
		id  int64
	)

	switch {
	case len(os.Args) > 1 && os.Args[1] == "in":
		{
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
	case len(os.Args) > 1 && os.Args[1] == "out":
		{
			rows, err := db.Query(`SELECT id, end FROM shift ORDER BY end DESC LIMIT 1`)
			if err != nil {
				panic(err)
			}
			defer rows.Close()
			if rows.Next() {
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
	case len(os.Args) > 1 && os.Args[1] == "report":
		{
			_, err := db.Query(`SELECT start, end FROM shift ORDER BY end DESC`)
			if err != nil {
				panic(err)
			}
		}
	case len(os.Args) > 1 && os.Args[1] == "status":
		{
			rows, err := db.Query(`SELECT end FROM shift ORDER BY end DESC LIMIT 1`)
			if err != nil {
				panic(err)
			}
			if !rows.Next() {
				fmt.Println("Not clocked in.")
			} else {
				if err = rows.Scan(&end); err != nil {
					panic(err)
				}
				defer rows.Close()
				endTime, err := time.Parse(time.UnixDate, end)
				if err != nil {
					panic(err)
				}
				if time.Until(endTime) > 0 {
					fmt.Printf("Hours left: %v\n", endTime.Format(time.TimeOnly))
				} else {
					fmt.Printf("Not clocked in.")
				}
			}
		}
	default:
		{
			// print usage
			os.Exit(2)
		}
	}
}
