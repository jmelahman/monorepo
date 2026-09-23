package api

import (
	"net/http"

	"github.com/jmelahman/local-preview/orchestrator"

	"github.com/jmelahman/kanban/internal/config"
	"github.com/jmelahman/kanban/internal/db"
	"github.com/jmelahman/kanban/internal/docker"
	"github.com/jmelahman/kanban/internal/errreport"
	"github.com/jmelahman/kanban/internal/hooks"
	"github.com/jmelahman/kanban/internal/metrics"
	"github.com/jmelahman/kanban/internal/session"
	"github.com/jmelahman/kanban/internal/tasks"
	"github.com/jmelahman/kanban/web"
)

// BuildInfo is the build metadata surfaced via /api/version.
type BuildInfo struct {
	Version string `json:"version"`
}

// Deps wires together the dependencies for the HTTP layer.
type Deps struct {
	Store    *db.Store
	Docker   *docker.Client
	Sessions *session.Manager
	Hooks    *hooks.Runner
	Config   *config.Config
	Bus      *EventBus
	Build    BuildInfo
	Reporter *errreport.Reporter
	// Previews is the embedded local-preview orchestrator; nil when it
	// failed to start (endpoints report unavailable).
	Previews *orchestrator.Orchestrator
}

// NewMux assembles the HTTP routes and embedded frontend.
func NewMux(d Deps) http.Handler {
	mux := http.NewServeMux()

	taskRunner := tasks.NewRunner(d.Store, d.Docker, d.Hooks)
	bus := d.Bus
	if bus == nil {
		bus = NewEventBus()
	}

	h := &handlers{
		store:    d.Store,
		docker:   d.Docker,
		sessions: d.Sessions,
		hooks:    d.Hooks,
		config:   d.Config,
		tasks:    taskRunner,
		bus:      bus,
		build:    d.Build,
		reporter: d.Reporter,
		previews: d.Previews,
	}

	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("GET /api/health", h.health)
	mux.HandleFunc("GET /api/version", h.version)

	mux.HandleFunc("GET /api/boards", h.listBoards)
	mux.HandleFunc("POST /api/boards", h.createBoard)
	mux.HandleFunc("GET /api/boards/{id}", h.getBoard)
	mux.HandleFunc("PATCH /api/boards/{id}", h.updateBoard)
	mux.HandleFunc("PATCH /api/boards/{id}/move", h.moveBoard)
	mux.HandleFunc("DELETE /api/boards/{id}", h.deleteBoard)
	mux.HandleFunc("GET /api/boards/{id}/env", h.listBoardEnv)
	mux.HandleFunc("PATCH /api/boards/{id}/env", h.patchBoardEnv)
	mux.HandleFunc("GET /api/boards/{id}/state", h.boardState)
	mux.HandleFunc("GET /api/boards/{id}/events", h.boardEvents)
	mux.HandleFunc("GET /api/events", h.boardsEvents)

	mux.HandleFunc("POST /api/boards/{id}/tickets", h.createTicket)
	mux.HandleFunc("GET /api/boards/{id}/archived", h.listArchivedTickets)
	mux.HandleFunc("DELETE /api/boards/{id}/archived", h.deleteAllArchived)
	mux.HandleFunc("POST /api/columns/{id}/archive-all", h.archiveColumnTickets)
	mux.HandleFunc("GET /api/tickets/{id}", h.getTicket)
	mux.HandleFunc("PATCH /api/tickets/{id}", h.updateTicket)
	mux.HandleFunc("PATCH /api/tickets/{id}/move", h.moveTicket)
	mux.HandleFunc("POST /api/tickets/{id}/archive", h.archiveTicket)
	mux.HandleFunc("POST /api/tickets/{id}/unarchive", h.unarchiveTicket)
	mux.HandleFunc("DELETE /api/tickets/{id}", h.deleteTicket)
	mux.HandleFunc("POST /api/tickets/{id}/sync", h.syncTicket)
	mux.HandleFunc("POST /api/tickets/{id}/merge", h.mergeTicket)
	mux.HandleFunc("POST /api/tickets/{id}/done", h.doneTicket)

	mux.HandleFunc("GET /api/sessions/summary", h.sessionSummary)

	mux.HandleFunc("POST /api/tickets/{id}/session", h.ensureSession)
	mux.HandleFunc("POST /api/sessions/{id}/start", h.startSession)
	mux.HandleFunc("POST /api/sessions/{id}/stop", h.stopSession)
	mux.HandleFunc("POST /api/sessions/{id}/restart", h.restartSession)
	mux.HandleFunc("PATCH /api/sessions/{id}/status", h.updateSessionStatus)
	mux.HandleFunc("PATCH /api/sessions/{id}/claude-session", h.updateClaudeSessionID)
	mux.HandleFunc("PATCH /api/sessions/{id}/branch", h.updateSessionBranch)
	mux.HandleFunc("PUT /api/sessions/{id}/harness", h.updateSessionHarness)

	mux.HandleFunc("GET /api/sessions/{id}/plans", h.listSessionPlans)
	mux.HandleFunc("GET /api/sessions/{id}/plans/{name}", h.getSessionPlan)

	mux.HandleFunc("GET /api/sessions/{id}/diff", h.getSessionDiff)
	mux.HandleFunc("GET /api/sessions/{id}/file", h.getSessionFile)
	mux.HandleFunc("GET /api/sessions/{id}/file-diff", h.getSessionFileDiff)

	mux.HandleFunc("GET /api/sessions/{id}/discover-tasks", h.discoverTasks)
	mux.HandleFunc("GET /api/sessions/{id}/task-runs", h.listTaskRuns)
	mux.HandleFunc("POST /api/sessions/{id}/task-runs", h.createTaskRun)
	mux.HandleFunc("DELETE /api/task-runs/{id}", h.stopTaskRun)
	mux.HandleFunc("GET /api/task-runs/{id}/output", h.taskRunOutput)

	mux.HandleFunc("GET /api/sessions/{id}/ports", h.listPorts)
	mux.HandleFunc("POST /api/sessions/{id}/ports", h.createPort)
	mux.HandleFunc("DELETE /api/ports/{id}", h.deletePort)

	mux.HandleFunc("GET /api/sessions/{id}/previews", h.listSessionPreviews)
	mux.HandleFunc("POST /api/sessions/{id}/previews", h.createSessionPreview)
	mux.HandleFunc("GET /api/previews", h.listPreviews)
	mux.HandleFunc("POST /api/boards/{id}/previews", h.createBoardPreview)
	mux.HandleFunc("GET /api/previews/storage", h.previewStorage)
	mux.HandleFunc("GET /api/previews/retention", h.previewRetention)
	mux.HandleFunc("PUT /api/previews/retention", h.updatePreviewRetention)
	mux.HandleFunc("POST /api/previews/gc", h.collectPreviewGarbage)
	mux.HandleFunc("POST /api/previews/{id}/stop", h.stopPreview)
	mux.HandleFunc("DELETE /api/previews/{id}", h.deletePreview)
	mux.HandleFunc("GET /api/previews/{id}/logs", h.previewLogs)
	mux.HandleFunc("GET /api/previews/{id}/artifacts/{artifact}/{file}", h.previewArtifact)

	mux.HandleFunc("GET /api/sessions/{id}/pr-detail", h.prDetail)

	mux.HandleFunc("/ws/sessions/{id}/pty", h.wsPTY)
	mux.HandleFunc("/ws/sessions/{id}/shell", h.wsShell)

	mux.HandleFunc("POST /api/errors", h.reportFrontendError)

	mux.HandleFunc("GET /api/settings", h.getSettings)
	mux.HandleFunc("PATCH /api/settings", h.updateSettings)
	mux.HandleFunc("GET /api/harnesses", h.listHarnesses)

	mux.HandleFunc("GET /api/config", h.getConfig)
	mux.HandleFunc("PATCH /api/config", h.patchConfig)

	mux.HandleFunc("GET /api/fs/check", h.fsCheck)

	mux.Handle("GET /metrics", metrics.Handler())
	mux.Handle("/prometheus/", metrics.PrometheusProxyHandler())

	mux.Handle("/", web.Handler())
	return mux
}
