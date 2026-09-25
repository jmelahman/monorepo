package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	neturl "net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/jmelahman/kanban/internal/client"
)

// taskStopGrace bounds how long a foreground run waits for its output to end
// after Ctrl-C asked the server to stop it; a second Ctrl-C cuts it short.
var taskStopGrace = 15 * time.Second

func ticketTasksCmd(serverURL, boardIdent *string) *cobra.Command {
	var (
		runLabel, stopLabel string
		detach, asJSON      bool
	)
	cmd := &cobra.Command{
		Use:   "tasks [id]",
		Short: "List, run, or stop a ticket's VS Code tasks, with their ports proxied to the host",
		Long: `List, run, or stop the tasks a ticket's worktree defines in
.vscode/tasks.json and .vscode/launch.json, the same ones the web UI's
Tasks tab shows.

Without --run or --stop, on a terminal it opens a full-screen view: the
tasks with their ports, run status and proxied URLs on top, the
highlighted task's live output below. Enter runs the highlighted task (or
stops it if it's running), "c" copies its URL, PgUp/PgDn scroll the
output, and Esc closes the view; tasks keep running after it closes.

Piped or redirected, or with --json, it prints the list instead: each
task's container port (from the [[task]] entries in .kanban.toml), the
status of its latest run, and the http://<server host>:<host port> URL its
port is proxied to.

--run LABEL starts the ticket's session if it isn't running, runs the task
inside the session container, opens the proxy for its port, prints the
URL, and streams the task's output. Ctrl-C stops the task. With --detach
the command returns as soon as the task has started and the task keeps
running; stop it later with --stop LABEL.`,
		Example: `  kanban ticket tasks 42
  kanban ticket tasks 42 --run "Kanban Frontend"
  kanban ticket tasks 42 --run "Kanban Backend" --detach
  kanban ticket tasks 42 --stop "Kanban Backend"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if runLabel != "" && stopLabel != "" {
				return errors.New("--run and --stop can't be combined")
			}
			if detach && runLabel == "" {
				return errors.New("--detach needs --run")
			}
			if asJSON && (runLabel != "" || stopLabel != "") {
				return errors.New("--json only applies to the task list")
			}
			ctx := cmd.Context()
			url := resolveURL(cmd, *serverURL)
			id, err := ticketArg(ctx, url, args, *boardIdent, pickerAction{"Ticket tasks", "select"}, false)
			if err != nil {
				return err
			}
			out, errOut := cmd.OutOrStdout(), cmd.ErrOrStderr()
			switch {
			case runLabel != "":
				return runTicketTask(ctx, url, out, errOut, id, runLabel, detach)
			case stopLabel != "":
				return runTicketTaskStop(ctx, url, out, id, stopLabel)
			case !asJSON && stdinIsTerminal():
				return promptTicketTasks(ctx, url, id)
			default:
				return runTicketTasksList(ctx, url, out, id, asJSON)
			}
		},
	}
	cmd.Flags().StringVar(&runLabel, "run", "", "Run the task with this label, streaming its output (Ctrl-C stops it)")
	cmd.Flags().StringVar(&stopLabel, "stop", "", "Stop the running task with this label")
	cmd.Flags().BoolVarP(&detach, "detach", "d", false, "With --run, return once the task has started and leave it running")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print the task list as JSON")
	return cmd
}

// taskRow is one line of the task list, and its --json shape.
type taskRow struct {
	client.Task
	HostPort int             `json:"host_port,omitempty"`
	URL      string          `json:"url,omitempty"`
	LastRun  *client.TaskRun `json:"last_run,omitempty"`
}

// ticketSession returns the ticket's session, or an error saying how to
// create one. Listing and stopping never create a session: there is nothing
// to list or stop in a ticket that never had one.
func ticketSession(ctx context.Context, url string, ticketID int64) (*client.Session, error) {
	info, err := loadTicketInfo(ctx, url, ticketID)
	if err != nil {
		return nil, err
	}
	if info.Session == nil {
		return nil, fmt.Errorf("ticket #%d has no session yet; start one with: kanban ticket tasks %d --run <label>", ticketID, ticketID)
	}
	return info.Session, nil
}

func loadTaskRows(ctx context.Context, url string, sessionID int64) ([]taskRow, []string, error) {
	c := client.New(url, nil)
	found, warnings, err := c.DiscoverTasks(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}
	runs, err := c.ListTaskRuns(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}
	ports, err := c.ListPorts(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}
	rows := make([]taskRow, 0, len(found))
	for _, t := range found {
		row := taskRow{Task: t}
		if p := findPort(ports, t); p != nil {
			row.HostPort = p.HostPort
			if p.ProxyActive {
				row.URL = proxyURL(url, p.HostPort)
			}
		}
		// Runs come newest first; the first match is the latest.
		for i := range runs {
			if runs[i].TaskLabel == t.Label {
				row.LastRun = &runs[i]
				break
			}
		}
		rows = append(rows, row)
	}
	return rows, warnings, nil
}

func runTicketTasksList(ctx context.Context, url string, out io.Writer, ticketID int64, asJSON bool) error {
	sess, err := ticketSession(ctx, url, ticketID)
	if err != nil {
		return err
	}
	rows, warnings, err := loadTaskRows(ctx, url, sess.ID)
	if err != nil {
		return err
	}
	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{"tasks": rows, "warnings": nonNil(warnings)})
	}
	for _, w := range warnings {
		fmt.Fprintf(out, "warning: %s\n", w)
	}
	if len(rows) == 0 {
		fmt.Fprintf(out, "ticket #%d defines no tasks (.vscode/tasks.json or .vscode/launch.json)\n", ticketID)
		return nil
	}
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "TASK\tPORT\tSTATUS\tURL")
	for _, r := range rows {
		port := "-"
		if r.HasPort {
			port = strconv.Itoa(r.ContainerPort)
		}
		url := r.URL
		if url == "" {
			url = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.Label, port, runStatus(r.LastRun), url)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if sess.Status == "stopped" || sess.Status == "error" {
		fmt.Fprintf(out, "session #%d is %s; --run starts it\n", sess.ID, sess.Status)
	}
	return nil
}

// runStatus renders a run's state for the list: "running (run #3)",
// "exited 0", "-" for a task that never ran.
func runStatus(tr *client.TaskRun) string {
	if tr == nil {
		return "-"
	}
	if tr.Status == "running" {
		return fmt.Sprintf("running (run #%d)", tr.ID)
	}
	if tr.ExitCode != nil {
		return fmt.Sprintf("%s %d", tr.Status, *tr.ExitCode)
	}
	return tr.Status
}

// startedTask is what startTicketTask did: the run it started and, for a
// task with a port, where that port is proxied (or why it couldn't be).
type startedTask struct {
	Session sessionInfo
	Task    client.Task
	Run     client.TaskRun
	URL     string
	PortErr error
}

// startTicketTask brings a ticket's session up, starts the task with the
// given label in it, and makes sure the task's port is proxied. progress
// receives the session-start messages. Shared by --run and the task view.
func startTicketTask(ctx context.Context, c *client.Client, url string, progress io.Writer, ticketID int64, label string) (startedTask, error) {
	var st startedTask
	sess, err := ensureRunningSession(ctx, c, progress, ticketID, "")
	if err != nil {
		return st, err
	}
	st.Session = sess
	found, _, err := c.DiscoverTasks(ctx, sess.ID)
	if err != nil {
		return st, err
	}
	if st.Task, err = findTask(found, label); err != nil {
		return st, err
	}
	if st.Run, err = c.StartTaskRun(ctx, sess.ID, st.Task.Label); err != nil {
		return st, err
	}
	if st.Task.HasPort {
		// The server tries to open the proxy when it starts the task but
		// swallows failures; asking again surfaces them.
		ports, err := c.CreatePort(ctx, sess.ID, st.Task.Label, st.Task.ContainerPort)
		switch {
		case err != nil:
			st.PortErr = err
		default:
			if p := findPort(ports, st.Task); p != nil {
				st.URL = proxyURL(url, p.HostPort)
			}
		}
	}
	return st, nil
}

func runTicketTask(ctx context.Context, url string, out, errOut io.Writer, ticketID int64, label string, detach bool) error {
	c := client.New(url, nil)
	// Progress goes to stderr so stdout carries only the task's output.
	st, err := startTicketTask(ctx, c, url, errOut, ticketID, label)
	if err != nil {
		return err
	}
	fmt.Fprintf(errOut, "started %q as run #%d in session #%d\n", st.Task.Label, st.Run.ID, st.Session.ID)
	switch {
	case st.PortErr != nil:
		fmt.Fprintf(errOut, "warning: couldn't proxy container port %d: %v\n", st.Task.ContainerPort, st.PortErr)
	case st.URL != "":
		fmt.Fprintf(errOut, "%s → %s (container port %d)\n", st.Task.Label, st.URL, st.Task.ContainerPort)
	}
	if detach {
		fmt.Fprintf(errOut, "running in the background; stop it with: kanban ticket tasks %d --stop %q\n", ticketID, st.Task.Label)
		return nil
	}
	return followTaskRun(ctx, c, out, errOut, st.Run.ID, st.Session.ID)
}

// followTaskRun streams a run's output until it exits, and turns Ctrl-C
// into a stop of the run rather than just an exit from the stream, so a
// foreground task behaves as if it were running locally.
func followTaskRun(ctx context.Context, c *client.Client, out, errOut io.Writer, runID, sessionID int64) error {
	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigs)

	streamCtx, cancelStream := context.WithCancel(ctx)
	defer cancelStream()
	done := make(chan error, 1)
	go func() {
		done <- c.StreamTaskRunOutput(streamCtx, runID, func(line string) {
			fmt.Fprintln(out, line)
		})
	}()

	stopped := false
	select {
	case err := <-done:
		if err != nil {
			return err
		}
	case <-sigs:
		stopped = true
		fmt.Fprintf(errOut, "\nstopping run #%d (Ctrl-C again to stop waiting)...\n", runID)
		stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		err := c.StopTaskRun(stopCtx, runID)
		cancel()
		if err != nil {
			return fmt.Errorf("stop run #%d: %w", runID, err)
		}
		select {
		case <-done:
		case <-sigs:
			return fmt.Errorf("gave up waiting for run #%d to exit", runID)
		case <-time.After(taskStopGrace):
			return fmt.Errorf("run #%d didn't exit within %s of being stopped", runID, taskStopGrace)
		}
	}

	tr, err := findRun(ctx, c, sessionID, runID)
	if err != nil {
		return err
	}
	if stopped {
		fmt.Fprintf(errOut, "stopped run #%d\n", runID)
		return nil
	}
	if tr.ExitCode != nil && *tr.ExitCode != 0 {
		return fmt.Errorf("run #%d exited with code %d", runID, *tr.ExitCode)
	}
	fmt.Fprintf(errOut, "run #%d exited 0\n", runID)
	return nil
}

func runTicketTaskStop(ctx context.Context, url string, out io.Writer, ticketID int64, label string) error {
	sess, err := ticketSession(ctx, url, ticketID)
	if err != nil {
		return err
	}
	c := client.New(url, nil)
	runs, err := c.ListTaskRuns(ctx, sess.ID)
	if err != nil {
		return err
	}
	n := 0
	for _, r := range runs {
		if r.TaskLabel != label || r.Status != "running" {
			continue
		}
		if err := c.StopTaskRun(ctx, r.ID); err != nil {
			return fmt.Errorf("stop run #%d: %w", r.ID, err)
		}
		fmt.Fprintf(out, "stopped %q (run #%d)\n", label, r.ID)
		n++
	}
	if n == 0 {
		return fmt.Errorf("no running run of %q on ticket #%d", label, ticketID)
	}
	return nil
}

func findTask(tasks []client.Task, label string) (client.Task, error) {
	labels := make([]string, 0, len(tasks))
	for _, t := range tasks {
		if t.Label == label {
			return t, nil
		}
		labels = append(labels, strconv.Quote(t.Label))
	}
	if len(labels) == 0 {
		return client.Task{}, fmt.Errorf("task %q not found: the worktree defines no tasks", label)
	}
	return client.Task{}, fmt.Errorf("task %q not found; available: %s", label, strings.Join(labels, ", "))
}

// findPort matches a task to its port allocation by container port, the
// key the server dedupes allocations on.
func findPort(ports []client.Port, t client.Task) *client.Port {
	if !t.HasPort {
		return nil
	}
	for i := range ports {
		if ports[i].ContainerPort == t.ContainerPort {
			return &ports[i]
		}
	}
	return nil
}

func findRun(ctx context.Context, c *client.Client, sessionID, runID int64) (client.TaskRun, error) {
	runs, err := c.ListTaskRuns(ctx, sessionID)
	if err != nil {
		return client.TaskRun{}, err
	}
	for _, r := range runs {
		if r.ID == runID {
			return r, nil
		}
	}
	return client.TaskRun{}, fmt.Errorf("run #%d not found", runID)
}

// proxyURL is where a host port proxied by the server is reachable: the
// proxies listen on the server's host, so reuse the host from --server.
func proxyURL(serverURL string, hostPort int) string {
	host := "localhost"
	if u, err := neturl.Parse(serverURL); err == nil && u.Hostname() != "" {
		host = u.Hostname()
	}
	return "http://" + net.JoinHostPort(host, strconv.Itoa(hostPort))
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
