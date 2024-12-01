package client

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/jmelahman/work/database"
	"github.com/jmelahman/work/database/models"
)

func HandleInstall() (int, error) {
	var err error

	serviceName := "work-stop.service"
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
	stopServiceName := filepath.Join(xdgConfigHome, "systemd", "user", serviceName)
	err = os.MkdirAll(filepath.Dir(stopServiceName), 0755)
	if err != nil {
		return 1, err
	}
	serviceContent := `[Unit]
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
	err = os.WriteFile(stopServiceName, []byte(serviceContent), 0644)
	if err != nil {
		return 1, err
	}

	cmd := exec.Command("systemctl", "--user", "enable", "--now", serviceName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
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

func HandleStatus(dal *database.WorkDAL, quiet bool) (int, error) {
	task, err := dal.GetLatestTask()
	if err != nil {
		return 1, err
	}

	if task.ID == 0 || !task.End.IsZero() {
		if !quiet {
			fmt.Println("No task currently.")
			return 0, nil
		}
		return 1, nil
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
