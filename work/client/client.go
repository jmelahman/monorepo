package client

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gen2brain/beeep"
	"github.com/godbus/dbus/v5"
	"github.com/jmelahman/work/client/reporter"
	"github.com/jmelahman/work/client/systemd"
	"github.com/jmelahman/work/database"
	"github.com/jmelahman/work/database/types"
)

// TaskManager handles operations related to task management
type TaskManager struct {
	dal      *database.WorkDAL
	reporter *reporter.Reporter
}

// NewTaskManager creates a new TaskManager instance
func NewTaskManager(databasePath string) *TaskManager {
	dal, err := database.NewWorkDAL(databasePath)
	if err != nil {
		log.Fatalf("failed to initialize DAL: %v", err)
	}
	return &TaskManager{dal: dal, reporter: reporter.NewReporter()}
}

// SystemdConfig holds systemd service configuration
type SystemdConfig struct {
	SystemdUserConfigDir string
	StopService          systemd.ServiceConfig
	NotificationService  systemd.ServiceConfig
	NotificationTimer    systemd.ServiceConfig
}

func getSystemdConfig() (*SystemdConfig, error) {
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %v", err)
	}

	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome == "" {
		xdgConfigHome, err = os.UserConfigDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user config dir: %v", err)
		}
	}

	systemdUserConfigDir := filepath.Join(xdgConfigHome, "systemd", "user")

	stopService := systemd.ServiceConfig{
		Name:  "work-stop.service",
		Start: true,
		Content: fmt.Sprintf(`[Unit]
Description=Stop tracking work on shutdown
DefaultDependencies=no
Before=shutdown.target

[Service]
Type=oneshot
ExecStart=%s stop
RemainAfterExit=yes

[Install]
WantedBy=default.target
`, execPath),
	}

	notificationService := systemd.ServiceConfig{
		Name:  "work-notification.service",
		Start: false,
		Content: fmt.Sprintf(`[Unit]
Description=Alert when not tracking a work task

[Service]
Type=simple
ExecStart=%s status --notify > /dev/null

[Install]
WantedBy=user.target
`, execPath),
	}
	notificationTimer := systemd.ServiceConfig{
		Name:  "work-notification.timer",
		Start: true,
		Content: `[Unit]
Description=Notify when not tracking tasks every 10 minutes

[Timer]
OnBootSec=10min
OnUnitActiveSec=10min
Persistent=true

[Install]
WantedBy=timers.target
`,
	}

	return &SystemdConfig{
		SystemdUserConfigDir: systemdUserConfigDir,
		StopService:          stopService,
		NotificationService:  notificationService,
		NotificationTimer:    notificationTimer,
	}, nil
}

func HandleInstall(uninstall bool) error {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return fmt.Errorf("failed to connect to session bus: %v", err)
	}
	defer conn.Close()

	obj := conn.Object("org.freedesktop.systemd1", "/org/freedesktop/systemd1")

	if uninstall {
		return uninstallServices(obj)
	}

	config, err := getSystemdConfig()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(config.SystemdUserConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create systemd user config dir: %v", err)
	}

	if err := installService(obj, config.SystemdUserConfigDir, config.StopService); err != nil {
		return err
	}

	if err := installService(obj, config.SystemdUserConfigDir, config.NotificationService); err != nil {
		return err
	}

	if err := installService(obj, config.SystemdUserConfigDir, config.NotificationTimer); err != nil {
		return err
	}

	return nil
}

func uninstallServices(obj dbus.BusObject) error {
	services := []string{
		"work-stop.service",
		"work-notification.service",
		"work-notification.timer",
	}

	for _, service := range services {
		if err := systemd.ReloadDaemon(obj); err != nil {
			return err
		}
		if err := systemd.StopUnit(obj, service); err != nil {
			return err
		}
		if err := systemd.DisableUnitFiles(obj, []string{service}); err != nil {
			return err
		}
		if err := systemd.ReloadDaemon(obj); err != nil {
			return err
		}
	}
	return nil
}

func installService(obj dbus.BusObject, configDir string, service systemd.ServiceConfig) error {
	servicePath := filepath.Join(configDir, service.Name)
	if err := os.WriteFile(servicePath, []byte(service.Content), 0644); err != nil {
		return fmt.Errorf("failed to write service file %s: %v", service.Name, err)
	}
	if err := systemd.ReloadDaemon(obj); err != nil {
		return err
	}

	if err := systemd.EnableUnitFiles(obj, []string{service.Name}); err != nil {
		return err
	}

	if service.Start {
		if err := systemd.StartUnit(obj, service.Name); err != nil {
			return err
		}
	}

	if err := systemd.ReloadDaemon(obj); err != nil {
		return err
	}
	return nil
}

func (tm *TaskManager) StopCurrentTask() error {
	latestTask, err := tm.dal.GetLatestTask()
	if err != nil {
		return fmt.Errorf("failed to get latest task: %v", err)
	}

	if latestTask.End.IsZero() {
		if err := tm.dal.EndTask(latestTask.ID); err != nil {
			return fmt.Errorf("failed to end task: %v", err)
		}
	}
	return nil
}

func (tm *TaskManager) ListTasks(limit int) error {
	tasks, err := tm.dal.ListTasks(0, limit)
	if err != nil {
		return fmt.Errorf("failed to list tasks: %v", err)
	}

	tm.reporter.PrintTaskRows(tasks)
	return nil
}

func (tm *TaskManager) calculateStats(tasks []types.Task) (map[string]types.DayStats, time.Duration) {
	statsByDay := make(map[string]types.DayStats)
	var weekTotal time.Duration

	for _, task := range tasks {
		end := task.End
		if end.IsZero() {
			end = time.Now()
		}

		duration := end.Sub(task.Start)
		day := task.Start.Format("2006-01-02")

		stats := statsByDay[day]
		if stats.ByClassification == nil {
			stats.ByClassification = make(map[types.TaskClassification]time.Duration)
		}

		stats.Total += duration
		stats.ByClassification[task.Classification] += duration
		statsByDay[day] = stats
		weekTotal += duration
	}

	return statsByDay, weekTotal
}

func (tm *TaskManager) GenerateReport() error {
	tasks, err := tm.dal.ListTasks(0, 5)
	if err != nil {
		return fmt.Errorf("failed to list tasks: %v", err)
	}

	stats, weekTotal := tm.calculateStats(tasks)
	tm.reporter.PrintReport(stats, weekTotal)
	return nil
}

func (tm *TaskManager) GetStatus(quiet bool, notify bool) error {
	task, err := tm.dal.GetLatestTask()
	if err != nil {
		return fmt.Errorf("failed to get latest task: %v", err)
	}

	if task.ID == 0 || !task.End.IsZero() {
		return tm.handleNoActiveTasks(quiet, notify)
	}

	if !quiet {
		fmt.Printf(
			"Current task: \"%s\"\nType: %s\nDuration: %s\n",
			task.Description,
			task.Classification,
			tm.reporter.FormatDuration(time.Since(task.Start)),
		)
	}
	return nil
}

func (tm *TaskManager) handleNoActiveTasks(quiet bool, notify bool) error {
	if notify {
		if err := beeep.Notify("Work Reminder", "No active tasks.", "assets/information.png"); err != nil {
			return fmt.Errorf("failed to send notification: %v", err)
		}
	}

	if quiet {
		os.Exit(1)
	}

	fmt.Println("No active tasks.")
	return nil
}

func (tm *TaskManager) CreateTask(chore bool, nonWork bool, toil bool, description string) error {
	var classification types.TaskClassification
	switch {
	case nonWork:
		classification = types.Break
	case chore:
		classification = types.Chore
	case toil:
		classification = types.Toil
	default:
		classification = types.Work
	}

	latestTask, err := tm.dal.GetLatestTask()
	if err != nil {
		return fmt.Errorf("failed to get latest task: %v", err)
	}

	if latestTask.End.IsZero() {
		if err := tm.dal.EndTask(latestTask.ID); err != nil {
			return fmt.Errorf("failed to end previous task: %v", err)
		}
	}

	newTask := types.Task{
		ID:             latestTask.ID + 1,
		Description:    description,
		Classification: classification,
		Start:          time.Now(),
		End:            time.Time{},
	}

	if err := tm.dal.CreateTask(newTask); err != nil {
		return fmt.Errorf("failed to create task: %v", err)
	}
	return nil
}
