package tasks

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/pelletier/go-toml/v2"

	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/docker"
	"github.com/jmelahman/kanban/internal/hooks"
)

// substituteVSCodeVars replaces a small set of VS Code task variables with
// their container-side equivalents.
func substituteVSCodeVars(s, workspaceFolder string) string {
	if s == "" {
		return s
	}
	r := strings.NewReplacer(
		"${workspaceFolder}", workspaceFolder,
		"${workspaceRoot}", workspaceFolder,
		"${workspaceFolderBasename}", filepath.Base(workspaceFolder),
	)
	return r.Replace(s)
}

// resolveContainerPath substitutes VS Code variables in a path and forces it
// to be absolute, defaulting to workspaceFolder. Docker exec rejects any
// non-absolute WorkingDir with "Cwd must be an absolute path".
func resolveContainerPath(p, workspaceFolder string) string {
	p = substituteVSCodeVars(strings.TrimSpace(p), workspaceFolder)
	if p == "" {
		return workspaceFolder
	}
	if !filepath.IsAbs(p) {
		return filepath.Join(workspaceFolder, p)
	}
	return p
}

// VSCodeTask is the parsed-down subset of a tasks.json entry.
type VSCodeTask struct {
	Label   string            `json:"label"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Cwd     string            `json:"cwd"`
	Env     map[string]string `json:"env"`
}

// vsTasksFile mirrors the on-disk shape of .vscode/tasks.json.
type vsTasksFile struct {
	Version string         `json:"version"`
	Tasks   []vsTaskRaw    `json:"tasks"`
	Inputs  []json.RawMessage `json:"inputs,omitempty"`
}

type vsTaskRaw struct {
	Label   string   `json:"label"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Options struct {
		Cwd string            `json:"cwd"`
		Env map[string]string `json:"env"`
	} `json:"options"`
}

// vsLaunchFile mirrors .vscode/launch.json (only program/args used).
type vsLaunchFile struct {
	Version        string             `json:"version"`
	Configurations []vsLaunchConfig   `json:"configurations"`
}

type vsLaunchConfig struct {
	Name    string            `json:"name"`
	Program string            `json:"program"`
	Args    []string          `json:"args"`
	Cwd     string            `json:"cwd"`
	Env     map[string]string `json:"env"`
}

// kanbanTOML maps task labels → ports in <worktree>/.kanban.toml.
type kanbanTOML struct {
	Task []struct {
		Label         string `toml:"label"`
		ContainerPort int    `toml:"container_port"`
	} `toml:"task"`
}

// Discover walks the worktree looking for VS Code tasks/launch entries.
func Discover(worktreePath string) ([]VSCodeTask, error) {
	var out []VSCodeTask

	tasksPath := filepath.Join(worktreePath, ".vscode", "tasks.json")
	if data, err := os.ReadFile(tasksPath); err == nil {
		var file vsTasksFile
		if err := json.Unmarshal(stripComments(data), &file); err == nil {
			for _, t := range file.Tasks {
				if t.Label == "" || t.Command == "" {
					continue
				}
				out = append(out, VSCodeTask{
					Label:   t.Label,
					Command: t.Command,
					Args:    t.Args,
					Cwd:     t.Options.Cwd,
					Env:     t.Options.Env,
				})
			}
		}
	}

	launchPath := filepath.Join(worktreePath, ".vscode", "launch.json")
	if data, err := os.ReadFile(launchPath); err == nil {
		var file vsLaunchFile
		if err := json.Unmarshal(stripComments(data), &file); err == nil {
			for _, c := range file.Configurations {
				if c.Program == "" {
					continue
				}
				out = append(out, VSCodeTask{
					Label:   c.Name,
					Command: c.Program,
					Args:    c.Args,
					Cwd:     c.Cwd,
					Env:     c.Env,
				})
			}
		}
	}
	return out, nil
}

// PortFor reads .kanban.toml from the worktree to find a container port for a label.
func PortFor(worktreePath, label string) (int, bool) {
	data, err := os.ReadFile(filepath.Join(worktreePath, ".kanban.toml"))
	if err != nil {
		return 0, false
	}
	var cfg kanbanTOML
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return 0, false
	}
	for _, t := range cfg.Task {
		if t.Label == label && t.ContainerPort > 0 {
			return t.ContainerPort, true
		}
	}
	return 0, false
}

func stripComments(data []byte) []byte {
	out := make([]byte, 0, len(data))
	inString := false
	escape := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if inString {
			out = append(out, c)
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}
		if c == '/' && i+1 < len(data) {
			if data[i+1] == '/' {
				for i < len(data) && data[i] != '\n' {
					i++
				}
				if i < len(data) {
					out = append(out, data[i])
				}
				continue
			}
			if data[i+1] == '*' {
				i += 2
				for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
					i++
				}
				i++
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

// Runner manages task executions inside session containers.
type Runner struct {
	store  *db.Store
	docker *docker.Client
	hooks  *hooks.Runner

	mu       sync.Mutex
	channels map[int64]*broadcastChan
}

func NewRunner(store *db.Store, dc *docker.Client, h *hooks.Runner) *Runner {
	return &Runner{store: store, docker: dc, hooks: h, channels: map[int64]*broadcastChan{}}
}

// Start launches a task inside the container; output is streamed via Subscribe.
func (r *Runner) Start(ctx context.Context, sess *db.Session, task VSCodeTask) (*db.TaskRun, error) {
	if sess.ContainerID == nil || *sess.ContainerID == "" {
		return nil, errors.New("session not running")
	}
	const workspaceFolder = "/workspace"
	cwd := resolveContainerPath(task.Cwd, workspaceFolder)

	full := strings.TrimSpace(task.Command + " " + strings.Join(task.Args, " "))
	tr := &db.TaskRun{SessionID: sess.ID, TaskLabel: task.Label, Command: full, Status: db.TaskRunStatusRunning}
	if err := r.store.CreateTaskRun(ctx, tr); err != nil {
		return nil, err
	}

	env := make([]string, 0, len(task.Env))
	for k, v := range task.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, substituteVSCodeVars(v, workspaceFolder)))
	}

	resp, err := r.docker.Raw().ContainerExecCreate(ctx, *sess.ContainerID, container.ExecOptions{
		Cmd:          []string{"sh", "-c", full},
		AttachStdout: true,
		AttachStderr: true,
		WorkingDir:   cwd,
		Env:          env,
	})
	if err != nil {
		zero := -1
		_ = r.store.UpdateTaskRunStatus(ctx, tr.ID, db.TaskRunStatusExited, &zero)
		return nil, err
	}
	tr.ExecID = &resp.ID

	att, err := r.docker.Raw().ContainerExecAttach(ctx, resp.ID, container.ExecStartOptions{})
	if err != nil {
		zero := -1
		_ = r.store.UpdateTaskRunStatus(ctx, tr.ID, db.TaskRunStatusExited, &zero)
		return nil, err
	}

	bc := r.getOrCreateChannel(tr.ID)

	go func() {
		defer att.Close()
		reader := bufio.NewReader(att.Reader)
		for {
			line, err := reader.ReadBytes('\n')
			if len(line) > 0 {
				bc.publish(string(line))
			}
			if err != nil {
				if !errors.Is(err, io.EOF) {
					log.Printf("task %d read: %v", tr.ID, err)
				}
				break
			}
		}
		inspect, ierr := r.docker.Raw().ContainerExecInspect(context.Background(), resp.ID)
		exitCode := 0
		if ierr == nil {
			exitCode = inspect.ExitCode
		}
		_ = r.store.UpdateTaskRunStatus(context.Background(), tr.ID, db.TaskRunStatusExited, &exitCode)
		bc.close()

		boardID := boardIDForSession(context.Background(), r.store, sess.ID)
		r.hooks.Fire(boardID, hooks.EventTaskExited, map[string]string{
			"session_id": fmt.Sprintf("%d", sess.ID),
			"task_label": task.Label,
			"exit_code":  fmt.Sprintf("%d", exitCode),
		})
	}()

	boardID := boardIDForSession(ctx, r.store, sess.ID)
	r.hooks.Fire(boardID, hooks.EventTaskStarted, map[string]string{
		"session_id": fmt.Sprintf("%d", sess.ID),
		"task_label": task.Label,
	})
	return tr, nil
}

// Stop sends SIGTERM-equivalent by killing the exec'd process.
func (r *Runner) Stop(ctx context.Context, sess *db.Session, tr *db.TaskRun) error {
	if tr.ExecID == nil || sess.ContainerID == nil {
		return nil
	}
	inspect, err := r.docker.Raw().ContainerExecInspect(ctx, *tr.ExecID)
	if err != nil {
		return err
	}
	if inspect.Pid == 0 {
		return nil
	}
	_, err = r.docker.ExecRun(ctx, *sess.ContainerID, []string{"sh", "-c", fmt.Sprintf("kill -TERM -%d 2>/dev/null || kill -TERM %d 2>/dev/null", inspect.Pid, inspect.Pid)})
	return err
}

// Subscribe returns a channel of output lines and a cancel func.
func (r *Runner) Subscribe(taskRunID int64) (<-chan string, func()) {
	bc := r.getOrCreateChannel(taskRunID)
	return bc.subscribe()
}

func boardIDForSession(ctx context.Context, store *db.Store, sessionID int64) *int64 {
	sess, err := store.GetSession(ctx, sessionID)
	if err != nil {
		return nil
	}
	t, err := store.GetTicket(ctx, sess.TicketID)
	if err != nil {
		return nil
	}
	return &t.BoardID
}

func (r *Runner) getOrCreateChannel(id int64) *broadcastChan {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.channels[id]; ok {
		return c
	}
	c := newBroadcastChan()
	r.channels[id] = c
	return c
}

// broadcastChan: simple fan-out of strings to multiple subscribers.
type broadcastChan struct {
	mu      sync.Mutex
	subs    []chan string
	history []string
	closed  bool
}

func newBroadcastChan() *broadcastChan { return &broadcastChan{} }

func (b *broadcastChan) publish(s string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.history = append(b.history, s)
	if len(b.history) > 1000 {
		b.history = b.history[len(b.history)-1000:]
	}
	for _, sub := range b.subs {
		select {
		case sub <- s:
		default:
		}
	}
}

func (b *broadcastChan) close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for _, sub := range b.subs {
		close(sub)
	}
	b.subs = nil
}

func (b *broadcastChan) subscribe() (<-chan string, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan string, 64)
	for _, h := range b.history {
		select {
		case ch <- h:
		default:
		}
	}
	if b.closed {
		close(ch)
		return ch, func() {}
	}
	b.subs = append(b.subs, ch)
	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		for i, s := range b.subs {
			if s == ch {
				b.subs = append(b.subs[:i], b.subs[i+1:]...)
				break
			}
		}
	}
}
