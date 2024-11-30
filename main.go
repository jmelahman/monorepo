package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jessevdk/go-flags"
	"github.com/jmelahman/work/client"
	"github.com/jmelahman/work/database"
	"github.com/jmelahman/work/logger"
	"github.com/posener/complete/v2"
	"github.com/posener/complete/v2/install"
	"github.com/posener/complete/v2/predict"
	_ "modernc.org/sqlite"
)

type Options struct {
	Database string `long:"database" description:"Specify a custom database"`
	Verbose  bool   `short:"v" long:"verbose" description:"Run in verbose mode"`
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
	Positional struct {
		Description []string `positional-arg-name:"description" description:"Description of the task"`
	} `positional-args:"yes"`
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
	parser.AddCommand("install-completion", "Install autocomplete", "Install shell autocompletion", &installComplete)
	parser.AddCommand("list", "List most recent tasks", "List most recent tasks", &list)
	parser.AddCommand("report", "Generate a weekly report", "Generate a weekly report", &report)
	parser.AddCommand("status", "Print current shift and task status", "Print current shift and task status", &status)
	parser.AddCommand("task", "Start a new Task", "Start a new task", &task)

	cmd := &complete.Command{
		Flags: map[string]complete.Predictor{
			"--database": predict.Files("*.db"),
			"--help":     predict.Nothing,
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

	_, err := parser.Parse()
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		}
		os.Exit(2)
	}

	var returncode int

	logger := logger.Init(opts.Verbose)
	defer logger.Sync()
	sugar := logger.Sugar()

	dal, err := database.NewWorkDAL(opts.Database, sugar)
	if err != nil {
		log.Fatalf("Failed to initialize DAL: %v", err)
	}

	switch parser.Command.Active.Name {
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
		if task.Positional.Description == nil {
			fmt.Println("Error: The 'task' argument is required.")
			parser.WriteHelp(os.Stderr)
			os.Exit(2)
		}
		if returncode, err = client.HandleTask(dal, strings.Join(task.Positional.Description, " ")); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	default:
		parser.WriteHelp(os.Stderr)
		returncode = 2
	}
	os.Exit(returncode)
}
