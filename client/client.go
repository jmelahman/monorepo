package client

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/gen2brain/beeep"
	"github.com/godbus/dbus/v5"
	"github.com/jmelahman/work/database"
	"github.com/jmelahman/work/database/models"
)

func enableUnitFiles(obj dbus.BusObject, files []string) error {
	var enableChanged bool
	result := make([][]interface{}, 0)
	err := obj.Call("org.freedesktop.systemd1.Manager.EnableUnitFiles", 0, files, false, true).Store(&enableChanged, &result)
	if err != nil {
		return fmt.Errorf("Failed to enable service %v: %v", files, err)
	}
	return nil
}

func startUnit(obj dbus.BusObject, serviceName string) error {
	var jobPath dbus.ObjectPath
	err := obj.Call("org.freedesktop.systemd1.Manager.StartUnit", 0, serviceName, "replace").Store(&jobPath)
	if err != nil {
		return fmt.Errorf("Failed to start service %v: %v", serviceName, err)
	}
	return nil
}

func disableUnitFiles(obj dbus.BusObject, files []string) error {
	result := make([][]interface{}, 0)
	err := obj.Call("org.freedesktop.systemd1.Manager.DisableUnitFiles", 0, files, true).Store(&result)
	if err != nil {
		return fmt.Errorf("Failed to enable service %v: %v", files, err)
	}
	return nil
}

func stopUnit(obj dbus.BusObject, serviceName string) error {
	var jobPath dbus.ObjectPath
	err := obj.Call("org.freedesktop.systemd1.Manager.StopUnit", 0, serviceName, "replace").Store(&jobPath)
	if err != nil {
		return fmt.Errorf("Failed to start service %s: %v", serviceName, err)
	}
	return nil
}

func HandleInstall(uninstall bool) (int, error) {
	var err error

	stopServiceName := "work-stop.service"
	notificationServiceName := "work-notification.service"
	notificationTimerName := "work-notification.timer"

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return 1, fmt.Errorf("Failed to connect to session bus: %v", err)
	}
	defer conn.Close()

	obj := conn.Object("org.freedesktop.systemd1", "/org/freedesktop/systemd1")

	if uninstall {
		err := disableUnitFiles(obj, []string{stopServiceName})
		if err != nil {
			return 1, err
		}
		err = stopUnit(obj, stopServiceName)
		if err != nil {
			return 1, err
		}
		err = disableUnitFiles(obj, []string{notificationServiceName})
		if err != nil {
			return 1, err
		}
		err = stopUnit(obj, notificationServiceName)
		if err != nil {
			return 1, err
		}
		return 0, nil
	}

	executablePath, err := os.Executable()
	if err != nil {
		return 1, err
	}

	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome == "" {
		xdgConfigHome, err = os.UserConfigDir()
		if err != nil {
			return 1, err
		}
	}

	systemdUserConfigDir := filepath.Join(xdgConfigHome, "systemd", "user")
	err = os.MkdirAll(systemdUserConfigDir, 0755)
	if err != nil {
		return 1, err
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
		return 1, err
	}

	err = enableUnitFiles(obj, []string{stopServiceName})
	if err != nil {
		return 1, err
	}

	err = startUnit(obj, stopServiceName)
	if err != nil {
		return 1, err
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
		return 1, err
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
		return 1, err
	}

	err = enableUnitFiles(obj, []string{notificationServiceName})
	if err != nil {
		return 1, err
	}

	return 0, nil
}

func HandleStop(dal *database.WorkDAL) (int, error) {
	latestTask, err := dal.GetLatestTask()
	if err != nil {
		return 1, err
	}

	if latestTask.End.IsZero() {
		err := dal.EndTask(latestTask.ID)
		if err != nil {
			return 1, err
		}
	}
	return 0, nil
}

func HandleList(dal *database.WorkDAL, limit int) (int, error) {
	tasks, err := dal.ListTasks(0, 1)
	if err != nil {
		return 1, err
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
	return 0, nil
}

func HandleReport(dal *database.WorkDAL) (int, error) {
	tasks, err := dal.ListTasks(0, 5)
	if err != nil {
		return 1, err
	}

	durationsByDay := make(map[string]time.Duration)

	for _, t := range tasks {
		var end time.Time
		if t.End.IsZero() {
			end = time.Now()
		} else {
			end = t.End
		}
		duration := end.Sub(t.Start)
		day := t.Start.Format("2006-01-02")
		durationsByDay[day] += duration
	}

	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 0, '\t', 0)
	for day, totalDuration := range durationsByDay {
		fmt.Fprintf(
			w,
			"%s\t%v\n",
			day,
			timeOnly(totalDuration),
		)
	}
	w.Flush()
	return 0, nil
}

func HandleStatus(dal *database.WorkDAL, quiet bool, notify bool) (int, error) {
	task, err := dal.GetLatestTask()
	if err != nil {
		return 1, err
	}

	if task.ID == 0 || !task.End.IsZero() {
		if notify {
			err := beeep.Notify("Work Reminder", "No active tasks.", "assets/information.png")
			if err != nil {
				return 1, err
			}
		}
		if quiet {
			return 1, nil
		}
		fmt.Println("No active tasks.")
		return 0, nil
	}
	if !quiet {
		fmt.Printf(
			"Current task: \"%s\"\nType: %s\nDuration: %s\n",
			task.Description,
			task.Classification,
			timeOnly(time.Since(task.Start)),
		)
	}
	return 0, nil
}

func HandleTask(
	dal *database.WorkDAL,
	classification models.TaskClassification,
	description string,
) (int, error) {
	latestTask, err := dal.GetLatestTask()
	if err != nil {
		return 1, err
	}

	if latestTask.End.IsZero() {
		if err := dal.EndTask(latestTask.ID); err != nil {
			return 1, err
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
		return 1, err
	}
	return 0, nil
}

func timeOnly(duration time.Duration) string {
	return fmt.Sprintf("%dh %dmin", int(duration.Hours()), int(duration.Minutes())%60)
}
