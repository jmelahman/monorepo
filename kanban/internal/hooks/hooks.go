package hooks

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jmelahman/kanban/internal/db"
)

const (
	EventSessionStarted      = "session.started"
	EventSessionStopped      = "session.stopped"
	EventSessionIdle         = "session.idle"
	EventSessionWorking      = "session.working"
	EventSessionAwaitingPerm = "session.awaiting_perm"
	EventTaskStarted         = "task.started"
	EventTaskExited          = "task.exited"
	EventPortExposed         = "port.exposed"
	EventPortClosed          = "port.closed"
	EventTicketCreated       = "ticket.created"
	EventTicketArchived      = "ticket.archived"
)

type Runner struct {
	store *db.Store
}

func NewRunner(store *db.Store) *Runner { return &Runner{store: store} }

// Fire executes all enabled hooks for the event in fire-and-forget goroutines.
func (r *Runner) Fire(boardID *int64, event string, vars map[string]string) {
	go r.fire(boardID, event, vars)
}

func (r *Runner) fire(boardID *int64, event string, vars map[string]string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	hooks, err := r.store.ListHooks(ctx, boardID, event)
	if err != nil {
		log.Printf("hooks list %s: %v", event, err)
		return
	}
	if len(hooks) == 0 {
		return
	}

	env := os.Environ()
	env = append(env, fmt.Sprintf("KANBAN_EVENT=%s", event))
	for k, v := range vars {
		env = append(env, fmt.Sprintf("KANBAN_%s=%s", strings.ToUpper(k), v))
	}

	for _, h := range hooks {
		cmd := exec.CommandContext(ctx, "sh", "-c", h.Command)
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("hook %d (%s) failed: %v: %s", h.ID, event, err, strings.TrimSpace(string(out)))
		}
	}
}
