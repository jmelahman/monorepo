package client

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/gen2brain/beeep"
	"github.com/godbus/dbus/v5"
	"github.com/jmelahman/work/client/systemd"
	"github.com/jmelahman/work/database"
	"github.com/jmelahman/work/database/models"
)

func HandleInstall(uninstall bool) error {
	var err error

	stopServiceName := "work-stop.service"
	notificationServiceName := "work-notification.service"
	notificationTimerName := "work-notification.timer"

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return fmt.Errorf("Failed to connect to session bus: %v", err)
	}
	defer conn.Close()

	obj := conn.Object("org.freedesktop.systemd1", "/org/freedesktop/systemd1")

	if uninstall {
		err := systemd.DisableUnitFiles(obj, []string{stopServiceName})
		if err != nil {
			return err
		}
		err = systemd.StopUnit(obj, stopServiceName)
		if err != nil {
			return err
		}
		err = systemd.DisableUnitFiles(obj, []string{notificationServiceName})
		if err != nil {
			return err
		}
		err = systemd.StopUnit(obj, notificationServiceName)
		if err != nil {
			return err
		}
		return nil
	}

	executablePath, err := os.Executable()
	if err != nil {
		return err
	}

	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome == "" {
		xdgConfigHome, err = os.UserConfigDir()
		if err != nil {
			return err
		}
	}

	systemdUserConfigDir := filepath.Join(xdgConfigHome, "systemd", "user")
	err = os.MkdirAll(systemdUserConfigDir, 0755)
	if err != nil {
		return err
	}

	// Shutdown service
	stopServicePath := filepath.Join(systemdUserConfigDir, stopServiceName)
	stopServiceContent := `[Unit]
Description=Stop tracking work on shutdown
DefaultDependencies=no
Before=shutdown.target

[Service]
Type=oneshot
ExecStart=` + executablePath + ` stop
RemainAfterExit=yes

[Install]
WantedBy=default.target
`
	err = os.WriteFile(stopServicePath, []byte(stopServiceContent), 0644)
	if err != nil {
		return err
	}

	err = systemd.EnableUnitFiles(obj, []string{stopServiceName})
	if err != nil {
		return err
	}

	err = systemd.StartUnit(obj, stopServiceName)
	if err != nil {
		return err
	}

	// Notification service
	notificationServicePath := filepath.Join(systemdUserConfigDir, notificationServiceName)
	notificationServiceContent := `[Unit]
Description=Alert when not tracking a work task

[Service]
Type=simple
ExecStart=` + executablePath + ` status --notify
`
	err = os.WriteFile(notificationServicePath, []byte(notificationServiceContent), 0644)
	if err != nil {
		return err
	}

	notificationTimerPath := filepath.Join(systemdUserConfigDir, notificationTimerName)
	notificationTimerContent := `[Unit]
Description=Notify when not tracking tasksE every 10 minutes

[Timer]
OnBootSec=10min
OnUnitActiveSec=10min
Persistent=true

[Install]
WantedBy=timers.target
`
	err = os.WriteFile(notificationTimerPath, []byte(notificationTimerContent), 0644)
	if err != nil {
		return err
	}

	err = systemd.EnableUnitFiles(obj, []string{notificationServiceName})
	if err != nil {
		return err
	}

	return nil
}

func HandleStop(dal *database.WorkDAL) error {
	latestTask, err := dal.GetLatestTask()
	if err != nil {
		return err
	}

	if latestTask.End.IsZero() {
		err := dal.EndTask(latestTask.ID)
		if err != nil {
			return err
		}
	}
	return nil
}

func HandleList(dal *database.WorkDAL, limit int) error {
	tasks, err := dal.ListTasks(0, 1)
	if err != nil {
		return err
	}

	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 0, '\t', 0)
	for _, t := range tasks {
		var end time.Time
		if t.End.IsZero() {
			end = time.Now()
		} else {
			end = t.End
		}
		fmt.Fprintf(
			w,
			"%s - %s\t%s\t%s\t%s\n",
			t.Start.Format("15:04"),
			end.Format("15:04"),
			t.Classification,
			t.Description,
			timeOnly(end.Sub(t.Start)),
		)
	}
	w.Flush()
	return nil
}

func HandleReport(dal *database.WorkDAL) error {
	tasks, err := dal.ListTasks(0, 5)
	if err != nil {
		return err
	}

	type DayStats struct {
		total            time.Duration
		byClassification map[models.TaskClassification]time.Duration
	}
	statsByDay := make(map[string]DayStats)
	var weekTotal time.Duration

	for _, t := range tasks {
		var end time.Time
		if t.End.IsZero() {
			end = time.Now()
		} else {
			end = t.End
		}
		duration := end.Sub(t.Start)
		day := t.Start.Format("2006-01-02")

		stats := statsByDay[day]
		if stats.byClassification == nil {
			stats.byClassification = make(map[models.TaskClassification]time.Duration)
		}

		stats.total += duration
		stats.byClassification[t.Classification] += duration
		statsByDay[day] = stats
		weekTotal += duration
	}

	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 5, 8, 1, '\t', 0)

	for day, stats := range statsByDay {
		fmt.Fprintf(w, "%s\t%v\t(Total)\n", day, timeOnly(stats.total))
		for classification, duration := range stats.byClassification {
			fmt.Fprintf(w, "\t%v\t(%s)\n", timeOnly(duration), classification)
		}
		fmt.Fprintln(w, "")
	}

	fmt.Fprintf(w, "\nWeek Total:\t%v\n", timeOnly(weekTotal))
	w.Flush()
	return nil
}

func HandleStatus(dal *database.WorkDAL, quiet bool, notify bool) error {
	task, err := dal.GetLatestTask()
	if err != nil {
		return err
	}

	if task.ID == 0 || !task.End.IsZero() {
		if notify {
			err := beeep.Notify("Work Reminder", "No active tasks.", "assets/information.png")
			if err != nil {
				return err
			}
		}
		if quiet {
			os.Exit(1)
		}
		fmt.Println("No active tasks.")
		return nil
	}
	if !quiet {
		fmt.Printf(
			"Current task: \"%s\"\nType: %s\nDuration: %s\n",
			task.Description,
			task.Classification,
			timeOnly(time.Since(task.Start)),
		)
	}
	return nil
}

func HandleTask(
	dal *database.WorkDAL,
	classification models.TaskClassification,
	description string,
) error {
	latestTask, err := dal.GetLatestTask()
	if err != nil {
		return err
	}

	if latestTask.End.IsZero() {
		if err := dal.EndTask(latestTask.ID); err != nil {
			return err
		}
	}
	err = dal.CreateTask(
		models.Task{
			ID:             latestTask.ID + 1,
			Description:    description,
			Classification: classification,
			Start:          time.Now(),
			End:            time.Time{},
		})
	if err != nil {
		return err
	}
	return nil
}

func timeOnly(duration time.Duration) string {
	return fmt.Sprintf("%dh %dmin", int(duration.Hours()), int(duration.Minutes())%60)
}
