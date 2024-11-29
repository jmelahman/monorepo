package main

import (
	"fmt"
	"log"
	"os"

	"github.com/jessevdk/go-flags"
	"github.com/jmelahman/go-work/client"
	"github.com/jmelahman/go-work/database"
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

	dal, err := database.NewWorkDAL()
	if err != nil {
		log.Fatalf("Failed to initialize DAL: %v", err)
	}

	switch command {
	case "clock-in":
		if returncode, err = client.HandleClockIn(dal); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	case "clock-out":
		if returncode, err = client.HandleClockOut(dal); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	case "install-completion":
		install.Install("work")
	case "list":
		if returncode, err = client.HandleList(dal); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	case "report":
		if returncode, err = client.HandleReport(dal); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	case "status":
		if returncode, err = client.HandleStatus(dal, status.Quiet); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	case "task":
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "Error: missing task name")
			os.Exit(2)
		}
		if returncode, err = client.HandleTask(dal, args[0]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	default:
		parser.WriteHelp(os.Stderr)
		returncode = 2
	}
	os.Exit(returncode)
}
