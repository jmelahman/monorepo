package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jessevdk/go-flags"
	"github.com/jmelahman/work/client"
	"github.com/jmelahman/work/database"
	"github.com/jmelahman/work/database/models"
	"github.com/posener/complete/v2"
	"github.com/posener/complete/v2/install"
	"github.com/posener/complete/v2/predict"
	_ "modernc.org/sqlite"
)

type Options struct {
	Database string `long:"database" description:"Specify a custom database"`
}

type InstallCommand struct {
}

type InstallCompleteCommand struct {
}

type ListCommand struct {
	Days int `short:"d" long:"days" description:"List task from the last N days" default:"1"`
}

type ReportCommand struct {
}

type StatusCommand struct {
	Quiet bool `short:"q" long:"quiet" description:"Exit with status code"`
}

type StopCommand struct {
}

type TaskCommand struct {
	Break      bool `short:"b" long:"break" description:"Classify task as non-work"`
	Chore      bool `short:"c" long:"chore" description:"Classify the task as a chore"`
	Toil       bool `short:"t" long:"toil" description:"Classify the task as toil"`
	Positional struct {
		Description []string `positional-arg-name:"description" description:"Description of the task"`
	} `positional-args:"yes"`
}

func main() {
	var opts Options
	var installComplete InstallCompleteCommand
	var list ListCommand
	var report ReportCommand
	var status StatusCommand
	var stop StopCommand
	var task TaskCommand

	parser := flags.NewParser(&opts, flags.Default)
	parser.AddCommand("install", "Install reminders", "Install reminder notification services", &installComplete)
	parser.AddCommand("install-completion", "Install autocomplete", "Install shell autocompletion", &installComplete)
	parser.AddCommand("list", "List most recent tasks", "List most recent tasks", &list)
	parser.AddCommand("report", "Generate a weekly report", "Generate a weekly report", &report)
	parser.AddCommand("status", "Print current shift and task status", "Print current shift and task status", &status)
	parser.AddCommand("stop", "Stop any previous task", "Stop any previous task", &stop)
	parser.AddCommand("task", "Start a new Task", "Start a new task", &task)

	cmd := &complete.Command{
		Flags: map[string]complete.Predictor{
			"--database": predict.Files("*.db"),
			"--help":     predict.Nothing,
		},
		Sub: map[string]*complete.Command{
			"install":            nil,
			"install-completion": nil,
			"list": {
				Flags: map[string]complete.Predictor{
					"--days": predict.Nothing,
				},
			},
			"report": nil,
			"status": {
				Flags: map[string]complete.Predictor{
					"--quiet": predict.Nothing,
				},
			},
			"stop": nil,
			"task": {
				Flags: map[string]complete.Predictor{
					"--break": predict.Nothing,
					"--chore": predict.Nothing,
					"--toil":  predict.Nothing,
				},
			},
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

	dal, err := database.NewWorkDAL(opts.Database)
	if err != nil {
		log.Fatalf("Failed to initialize DAL: %v", err)
	}

	switch parser.Command.Active.Name {
	case "install":
		if returncode, err = client.HandleInstall(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	case "install-completion":
		install.Install("work")
	case "list":
		if returncode, err = client.HandleList(dal, list.Days); err != nil {
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
	case "stop":
		if returncode, err = client.HandleStop(dal); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	case "task":
		if task.Positional.Description == nil {
			fmt.Println("Error: The 'task' argument is required.")
			parser.WriteHelp(os.Stderr)
			os.Exit(2)
		}
		if (task.Toil && task.Break) || (task.Toil && task.Chore) || (task.Break && task.Toil) {
			fmt.Println("Error: task should have at most one classification.")
			os.Exit(2)
		}
		var taskClassification models.TaskClassification
		if task.Break {
			taskClassification = models.Break
		} else if task.Chore {
			taskClassification = models.Chore
		} else if task.Toil {
			taskClassification = models.Toil
		} else {
			taskClassification = models.Work
		}
		if returncode, err = client.HandleTask(
			dal, taskClassification, strings.Join(task.Positional.Description, " "),
		); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	default:
		parser.WriteHelp(os.Stderr)
		returncode = 2
	}
	os.Exit(returncode)
}
