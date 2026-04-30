package api

import (
	"net/http"

	"github.com/jmelahman/kanban/internal/config"
	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/docker"
	"github.com/jmelahman/kanban/internal/hooks"
	"github.com/jmelahman/kanban/internal/session"
	"github.com/jmelahman/kanban/internal/tasks"
	"github.com/jmelahman/kanban/web"
)

// Deps wires together the dependencies for the HTTP layer.
type Deps struct {
	Store    *db.Store
	Docker   *docker.Client
	Sessions *session.Manager
	Hooks    *hooks.Runner
	Config   *config.Config
}

// NewMux assembles the HTTP routes and embedded frontend.
func NewMux(d Deps) http.Handler {
	mux := http.NewServeMux()

	taskRunner := tasks.NewRunner(d.Store, d.Docker, d.Hooks)
	bus := newEventBus()

	h := &handlers{
		store:    d.Store,
		docker:   d.Docker,
		sessions: d.Sessions,
		hooks:    d.Hooks,
		config:   d.Config,
		tasks:    taskRunner,
		bus:      bus,
	}

	mux.HandleFunc("GET /api/health", h.health)

	mux.HandleFunc("GET /api/boards", h.listBoards)
	mux.HandleFunc("POST /api/boards", h.createBoard)
	mux.HandleFunc("GET /api/boards/{id}", h.getBoard)
	mux.HandleFunc("GET /api/boards/{id}/state", h.boardState)
	mux.HandleFunc("GET /api/boards/{id}/events", h.boardEvents)

	mux.HandleFunc("POST /api/boards/{id}/tickets", h.createTicket)
	mux.HandleFunc("GET /api/boards/{id}/archived", h.listArchivedTickets)
	mux.HandleFunc("PATCH /api/tickets/{id}/move", h.moveTicket)
	mux.HandleFunc("POST /api/tickets/{id}/archive", h.archiveTicket)
	mux.HandleFunc("DELETE /api/tickets/{id}", h.deleteTicket)
	mux.HandleFunc("POST /api/tickets/{id}/sync", h.syncTicket)

	mux.HandleFunc("POST /api/tickets/{id}/session", h.ensureSession)
	mux.HandleFunc("POST /api/sessions/{id}/start", h.startSession)
	mux.HandleFunc("POST /api/sessions/{id}/stop", h.stopSession)

	mux.HandleFunc("GET /api/sessions/{id}/discover-tasks", h.discoverTasks)
	mux.HandleFunc("GET /api/sessions/{id}/task-runs", h.listTaskRuns)
	mux.HandleFunc("POST /api/sessions/{id}/task-runs", h.createTaskRun)
	mux.HandleFunc("DELETE /api/task-runs/{id}", h.stopTaskRun)
	mux.HandleFunc("GET /api/task-runs/{id}/output", h.taskRunOutput)

	mux.HandleFunc("GET /api/sessions/{id}/ports", h.listPorts)
	mux.HandleFunc("POST /api/sessions/{id}/ports", h.createPort)
	mux.HandleFunc("DELETE /api/ports/{id}", h.deletePort)

	mux.HandleFunc("/ws/sessions/{id}/pty", h.wsPTY)

	mux.Handle("/", web.Handler())
	return mux
}
