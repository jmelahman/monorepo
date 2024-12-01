package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/jmelahman/work/client"
	"github.com/spf13/cobra"
)

var (
	version      = "dev"
	commit       = "none"
	databasePath string

	// Command flags
	days    int
	notify  bool
	quiet   bool
	nonWork bool
	chore   bool
	toil    bool
)

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "work",
		Short:   "Work time tracking tool",
		Version: fmt.Sprintf("%s\ncommit %s", version, commit),
	}

	rootCmd.PersistentFlags().StringVar(&databasePath, "database", "", "Specify a custom database")

	rootCmd.AddCommand(newInstallCmd())
	rootCmd.AddCommand(newListCmd())
	rootCmd.AddCommand(newReportCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newStopCmd())
	rootCmd.AddCommand(newTaskCmd())
	rootCmd.AddCommand(newUninstallCmd())

	return rootCmd
}

func newInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install reminders",
		Long:  "Install reminder notification services",
		RunE: func(cmd *cobra.Command, args []string) error {
			return client.HandleInstall(false)
		},
	}
}

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List most recent tasks",
		Long:  "List most recent tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			return client.NewTaskManager(databasePath).ListTasks(days)
		},
	}

	cmd.Flags().IntVarP(&days, "days", "d", 1, "List task from the last N days")
	return cmd
}

func newReportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "report",
		Short: "Generate a weekly report",
		Long:  "Generate a weekly report",
		RunE: func(cmd *cobra.Command, args []string) error {
			return client.NewTaskManager(databasePath).GenerateReport()
		},
	}
}

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Print current shift and task status",
		Long:  "Print current shift and task status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return client.NewTaskManager(databasePath).GetStatus(quiet, notify)
		},
	}

	cmd.Flags().BoolVarP(&notify, "notify", "n", false, "Send a notification if no active tasks")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Exit with status code")
	return cmd
}

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop any previous task",
		Long:  "Stop any previous task",
		RunE: func(cmd *cobra.Command, args []string) error {
			return client.NewTaskManager(databasePath).StopCurrentTask()
		},
	}
}

func newTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task [description]",
		Short: "Start a new Task",
		Long:  "Start a new task",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if (toil && nonWork) || (toil && chore) || (nonWork && chore) {
				return fmt.Errorf("task should have at most one classification")
			}
			return client.NewTaskManager(databasePath).CreateTask(chore, nonWork, toil, strings.Join(args, " "))
		},
	}

	cmd.Flags().BoolVarP(&nonWork, "break", "b", false, "Classify task as non-work")
	cmd.Flags().BoolVarP(&chore, "chore", "c", false, "Classify the task as a chore")
	cmd.Flags().BoolVarP(&toil, "toil", "t", false, "Classify the task as toil")
	return cmd
}

func newUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall reminders",
		Long:  "Uninstall reminder notification services",
		RunE: func(cmd *cobra.Command, args []string) error {
			return client.HandleInstall(true)
		},
	}
}

func main() {
	rootCmd := newRootCmd()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
