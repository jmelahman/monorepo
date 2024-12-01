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
	Notify bool `short:"n" long:"notify" description:"Send a notification if no active tasks"`
	Quiet  bool `short:"q" long:"quiet" description:"Exit with status code"`
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

type UninstallCommand struct {
}

func main() {
	var opts Options
	var installOpts InstallCommand
	var installComplete InstallCompleteCommand
	var list ListCommand
	var report ReportCommand
	var status StatusCommand
	var stop StopCommand
	var task TaskCommand
	var uninstallOpts UninstallCommand

	parser := flags.NewParser(&opts, flags.Default)
	if _, err := parser.AddCommand("install", "Install reminders", "Install reminder notification services", &installOpts); err != nil {
		log.Fatal(err)
	}
	if _, err := parser.AddCommand("install-completion", "Install autocomplete", "Install shell autocompletion", &installComplete); err != nil {
		log.Fatal(err)
	}
	if _, err := parser.AddCommand("list", "List most recent tasks", "List most recent tasks", &list); err != nil {
		log.Fatal(err)
	}
	if _, err := parser.AddCommand("report", "Generate a weekly report", "Generate a weekly report", &report); err != nil {
		log.Fatal(err)
	}
	if _, err := parser.AddCommand("status", "Print current shift and task status", "Print current shift and task status", &status); err != nil {
		log.Fatal(err)
	}
	if _, err := parser.AddCommand("stop", "Stop any previous task", "Stop any previous task", &stop); err != nil {
		log.Fatal(err)
	}
	if _, err := parser.AddCommand("task", "Start a new Task", "Start a new task", &task); err != nil {
		log.Fatal(err)
	}
	if _, err := parser.AddCommand("uninstall", "Uninstall reminders", "Uninstall reminder notification services", &uninstallOpts); err != nil {
		log.Fatal(err)
	}

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
			"uninstall": nil,
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

	dal, err := database.NewWorkDAL(opts.Database)
	if err != nil {
		log.Fatalf("Failed to initialize DAL: %v", err)
	}

	switch parser.Command.Active.Name {
	case "install":
		var uninstall = false
		if err = client.HandleInstall(uninstall); err != nil {
			log.Fatalf("Error: %v\n", err)
		}
	case "install-completion":
		if err := install.Install("work"); err != nil {
			log.Fatalf("Error: %v\n", err)
		}
	case "list":
		if err = client.HandleList(dal, list.Days); err != nil {
			log.Fatalf("Error: %v\n", err)
		}
	case "report":
		if err = client.HandleReport(dal); err != nil {
			log.Fatalf("Error: %v\n", err)
		}
	case "status":
		if err = client.HandleStatus(dal, status.Quiet, status.Notify); err != nil {
			log.Fatalf("Error: %v\n", err)
		}
	case "stop":
		if err = client.HandleStop(dal); err != nil {
			log.Fatalf("Error: %v\n", err)
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
		if err = client.HandleTask(
			dal, taskClassification, strings.Join(task.Positional.Description, " "),
		); err != nil {
			log.Fatalf("Error: %v\n", err)
		}
	case "uninstall":
		var uninstall = true
		if err = client.HandleInstall(uninstall); err != nil {
			log.Fatalf("Error: %v\n", err)
		}
	default:
		parser.WriteHelp(os.Stderr)
		os.Exit(2)
	}
}
