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

// TaskManager handles operations related to task management
type TaskManager struct {
	dal *database.WorkDAL
}

// NewTaskManager creates a new TaskManager instance
func NewTaskManager(dal *database.WorkDAL) *TaskManager {
	return &TaskManager{dal: dal}
}

// SystemdConfig holds systemd service configuration
type SystemdConfig struct {
	SystemdUserConfigDir string
	StopService          ServiceConfig
	NotificationService  ServiceConfig
}

// ServiceConfig holds individual service configuration
type ServiceConfig struct {
	Name         string
	Content      string
	TimerContent string
}

// DayStats holds statistics for a single day
type DayStats struct {
	Total            time.Duration
	ByClassification map[models.TaskClassification]time.Duration
}

// Reporter handles report generation
type Reporter struct {
	writer *tabwriter.Writer
	dal    *database.WorkDAL
}

// NewReporter creates a new Reporter instance
func NewReporter(dal *database.WorkDAL) *Reporter {
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 5, 8, 1, '\t', 0)
	return &Reporter{
		writer: w,
		dal:    dal,
	}
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

	stopService := ServiceConfig{
		Name: "work-stop.service",
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

	notificationService := ServiceConfig{
		Name: "work-notification.service",
		Content: fmt.Sprintf(`[Unit]
Description=Alert when not tracking a work task

[Service]
Type=simple
ExecStart=%s status --notify
`, execPath),
		TimerContent: `[Unit]
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

	return nil
}

func uninstallServices(obj dbus.BusObject) error {
	services := []string{"work-stop.service", "work-notification.service"}

	for _, service := range services {
		if err := systemd.DisableUnitFiles(obj, []string{service}); err != nil {
			return fmt.Errorf("failed to disable %s: %v", service, err)
		}
		if err := systemd.StopUnit(obj, service); err != nil {
			return fmt.Errorf("failed to stop %s: %v", service, err)
		}
	}
	return nil
}

func installService(obj dbus.BusObject, configDir string, service ServiceConfig) error {
	servicePath := filepath.Join(configDir, service.Name)
	if err := os.WriteFile(servicePath, []byte(service.Content), 0644); err != nil {
		return fmt.Errorf("failed to write service file %s: %v", service.Name, err)
	}

	if service.TimerContent != "" {
		timerPath := filepath.Join(configDir, service.Name[:len(service.Name)-8]+".timer")
		if err := os.WriteFile(timerPath, []byte(service.TimerContent), 0644); err != nil {
			return fmt.Errorf("failed to write timer file: %v", err)
		}
	}

	if err := systemd.EnableUnitFiles(obj, []string{service.Name}); err != nil {
		return fmt.Errorf("failed to enable service %s: %v", service.Name, err)
	}

	if err := systemd.StartUnit(obj, service.Name); err != nil {
		return fmt.Errorf("failed to start service %s: %v", service.Name, err)
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

	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 0, '\t', 0)
	defer w.Flush()

	for _, task := range tasks {
		if err := tm.printTaskRow(w, task); err != nil {
			return err
		}
	}
	return nil
}

func (tm *TaskManager) printTaskRow(w *tabwriter.Writer, task models.Task) error {
	end := task.End
	if end.IsZero() {
		end = time.Now()
	}

	fmt.Fprintf(
		w,
		"%s - %s\t%s\t%s\t%s\n",
		task.Start.Format("15:04"),
		end.Format("15:04"),
		task.Classification,
		task.Description,
		formatDuration(end.Sub(task.Start)),
	)
	return nil
}

func (r *Reporter) GenerateReport() error {
	tasks, err := r.dal.ListTasks(0, 5)
	if err != nil {
		return fmt.Errorf("failed to list tasks: %v", err)
	}

	stats, weekTotal := r.calculateStats(tasks)
	r.printReport(stats, weekTotal)
	return nil
}

func (r *Reporter) calculateStats(tasks []models.Task) (map[string]DayStats, time.Duration) {
	statsByDay := make(map[string]DayStats)
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
			stats.ByClassification = make(map[models.TaskClassification]time.Duration)
		}

		stats.Total += duration
		stats.ByClassification[task.Classification] += duration
		statsByDay[day] = stats
		weekTotal += duration
	}

	return statsByDay, weekTotal
}

func (r *Reporter) printReport(stats map[string]DayStats, weekTotal time.Duration) {
	defer r.writer.Flush()

	for day, dayStats := range stats {
		fmt.Fprintf(r.writer, "%s\t%v\t(Total)\n", day, formatDuration(dayStats.Total))

		for classification, duration := range dayStats.ByClassification {
			fmt.Fprintf(r.writer, "\t%v\t(%s)\n", formatDuration(duration), classification)
		}
		fmt.Fprintln(r.writer, "")
	}

	fmt.Fprintf(r.writer, "\nWeek Total:\t%v\n", formatDuration(weekTotal))
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
		tm.printCurrentTask(task)
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

func (tm *TaskManager) printCurrentTask(task models.Task) {
	fmt.Printf(
		"Current task: \"%s\"\nType: %s\nDuration: %s\n",
		task.Description,
		task.Classification,
		formatDuration(time.Since(task.Start)),
	)
}

func (tm *TaskManager) CreateTask(classification models.TaskClassification, description string) error {
	latestTask, err := tm.dal.GetLatestTask()
	if err != nil {
		return fmt.Errorf("failed to get latest task: %v", err)
	}

	if latestTask.End.IsZero() {
		if err := tm.dal.EndTask(latestTask.ID); err != nil {
			return fmt.Errorf("failed to end previous task: %v", err)
		}
	}

	newTask := models.Task{
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

// Helper function to format duration
func formatDuration(duration time.Duration) string {
	return fmt.Sprintf("%dh %dmin", int(duration.Hours()), int(duration.Minutes())%60)
}
